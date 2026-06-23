import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RunDetail } from "@/components/runs/RunDetail";
import type { UseRunStreamResult } from "@/hooks/useRunStream";

// Mock the WebSocket hook so we can control its return values.
vi.mock("@/hooks/useRunStream", () => ({
  useRunStream: vi.fn(),
}));

// Dynamic import after mock is hoisted — Vitest resolves to the mock.
// eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
const { useRunStream } = await import("@/hooks/useRunStream");
const mockUseRunStream = vi.mocked(useRunStream);

/** Default neutral return value for the mock hook. */
function defaultMockStream(): UseRunStreamResult {
  return {
    connected: false,
    done: false,
    finalStatus: null,
    logEntries: [],
    error: null,
  };
}

describe("RunDetail WebSocket integration", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    mockUseRunStream.mockReturnValue(defaultMockStream());
  });

  it("shows a live indicator when the WebSocket is connected", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      connected: true,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "running",
        created_at: "2024-01-01T00:00:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    expect(screen.getByTestId("run-detail-live-indicator")).toBeInTheDocument();
  });

  it("does not show live indicator when the run is finished even if the WebSocket is connected", async () => {
    // The REST API says the run is finished but the WebSocket hasn't sent
    // a "done" message yet — still "running" from WS perspective, but the run
    // metadata clearly shows it ended. The indicator should reflect the finished state.
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      connected: true,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "failed",
        created_at: "2024-01-01T00:00:00Z",
        finished_at: "2024-01-01T00:05:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    expect(screen.queryByTestId("run-detail-live-indicator")).not.toBeInTheDocument();
  });

  it("displays log entries from the stream below the run info", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      connected: true,
      logEntries: [
        {
          type: "log",
          run_id: "run-abc",
          service_name: "frontend",
          content: "Build started",
        },
        {
          type: "log",
          run_id: "run-abc",
          service_name: "frontend",
          content: "Tests passed",
        },
      ],
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "running",
        created_at: "2024-01-01T00:00:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    const logSection = screen.getByTestId("run-detail-log-entries");
    expect(logSection).toBeInTheDocument();
    expect(logSection.children).toHaveLength(2);
    expect(screen.getByText("Build started")).toBeInTheDocument();
    expect(screen.getByText("Tests passed")).toBeInTheDocument();
  });

  it("overrides the status badge with finalStatus when the stream is done", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      done: true,
      finalStatus: "success",
    });
    // Fetch returns "running" but WebSocket says "success"
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "running",
        created_at: "2024-01-01T00:00:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    expect(screen.getByTestId("status-badge")).toHaveTextContent("Success");
  });

  it("shows the WebSocket error when the stream encounters an error", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      error: "WebSocket connection error",
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "running",
        created_at: "2024-01-01T00:00:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    expect(screen.getByText(/connection error/i)).toBeInTheDocument();
  });

  it("shows exit code from REST when the run has services", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      done: true,
      finalStatus: null,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "failed",
        created_at: "2026-06-22T00:55:12Z",
        finished_at: "2026-06-22T00:55:12Z",
        services: [
          {
            run_id: "run-abc",
            service_name: "ci",
            suite: "test",
            status: "failed",
            exit_code: 1,
            start_time: "",
            end_time: "2026-06-22T00:55:12Z",
          },
        ],
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();

    // Each service must show its exit code.
    const exitCodeElem = screen.getByTestId("service-exit-code-ci");
    expect(exitCodeElem).toHaveTextContent("1");
  });

  it("does not show exit code for services with null exit_code", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      done: true,
      finalStatus: null,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "passed",
        created_at: "2026-06-22T00:55:12Z",
        finished_at: "2026-06-22T00:55:12Z",
        services: [
          {
            run_id: "run-abc",
            service_name: "ci",
            suite: "test",
            status: "passed",
            exit_code: 0,
            start_time: "",
            end_time: "2026-06-22T00:55:12Z",
          },
          {
            run_id: "run-abc",
            service_name: "frontend",
            suite: "test",
            status: "pending",
            exit_code: null,
            start_time: "",
            end_time: "",
          },
        ],
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();

    // The tested service must show its exit code.
    expect(screen.getByTestId("service-exit-code-ci")).toHaveTextContent("0");

    // The untested service must NOT show an exit code (exit_code is null).
    expect(
      screen.queryByTestId("service-exit-code-frontend"),
    ).not.toBeInTheDocument();

    // Exactly one exit code element must be present (only the tested service).
    const allExitCodes = screen.getAllByTestId(/^service-exit-code-/);
    expect(allExitCodes).toHaveLength(1);
  });

  it("renders ANSI-colored log entries as styled HTML spans", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      connected: true,
      logEntries: [
        {
          type: "log",
          run_id: "run-abc",
          service_name: "ci",
          content: "\x1b[1;33m[INFO]\x1b[0m Running Go tests...",
        },
        {
          type: "log",
          run_id: "run-abc",
          service_name: "ci",
          content: "\x1b[0;32m[PASS]\x1b[0m go vet",
        },
      ],
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "running",
        created_at: "2024-01-01T00:00:00Z",
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();

    const logSection = screen.getByTestId("run-detail-log-entries");
    expect(logSection).toBeInTheDocument();

    // ANSI escape sequences must be converted to inline-styled HTML spans.
    expect(logSection.innerHTML).toContain('<span style="');

    // Visible text must be clean — no raw ANSI escape codes.
    expect(logSection.textContent).not.toContain("\x1b");

    // The styled text must be readable.
    expect(logSection.textContent).toContain("[INFO]");
    expect(logSection.textContent).toContain("Running Go tests...");
    expect(logSection.textContent).toContain("[PASS]");
    expect(logSection.textContent).toContain("go vet");
  });

  it("fetches historical logs from REST when the WebSocket delivers done with no entries", async () => {
    mockUseRunStream.mockReturnValue({
      ...defaultMockStream(),
      done: true,
      finalStatus: "passed",
      logEntries: [],
    });

    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock
      // First call: run metadata
      .mockImplementationOnce(() =>
        Promise.resolve({
          ok: true,
          json: async () => ({
            id: "run-abc",
            status: "passed",
            created_at: "2024-01-01T00:00:00Z",
            finished_at: "2024-01-01T00:05:00Z",
          }),
        } as Response),
      )
      // Second call: historical logs
      .mockImplementationOnce(() =>
        Promise.resolve({
          ok: true,
          json: async () => [
            { run_id: "run-abc", service_name: "ci", line_number: 1, level: "RAW", content: "test output for ci" },
            { run_id: "run-abc", service_name: "ci", line_number: 2, level: "RAW", content: "Build completed" },
          ],
        } as Response),
      );

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();

    // Verify a second fetch call was made for historical logs.
    const logsCall = fetchMock.mock.calls.find(
      (call) => typeof call[0] === "string" && call[0].includes("/logs"),
    );
    expect(logsCall).toBeTruthy();

    // Historical logs should be rendered.
    expect(screen.getByText("test output for ci")).toBeInTheDocument();
    expect(screen.getByText("Build completed")).toBeInTheDocument();
  });
});
