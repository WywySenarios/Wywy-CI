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
});
