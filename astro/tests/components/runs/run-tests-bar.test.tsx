import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { RunTestsBar } from "@/components/runs/RunTestsBar";
import { useServiceStatus } from "@/hooks/useServiceStatus";

// Mock useServiceStatus to avoid creating a real WebSocket.
vi.mock("@/hooks/useServiceStatus", () => ({
  useServiceStatus: vi.fn(),
}));

const mockUseServiceStatus = useServiceStatus as ReturnType<typeof vi.fn>;

describe("RunTestsBar", () => {
  const mockServices = [
    { name: "ci", repo: "Wywy-CI" },
    { name: "agentic", repo: "Wywy-Codes" },
    { name: "cache", repo: "Wywy-Website-Cache" },
  ];

  beforeEach(() => {
    // Default: all services not running, WebSocket connected.
    mockUseServiceStatus.mockReturnValue({
      serviceStatus: {},
      connected: true,
      error: null,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows a loading indicator while services are being fetched", () => {
    globalThis.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
    render(<RunTestsBar />);
    expect(screen.getByTestId("run-tests-bar-loading")).toBeInTheDocument();
  });

  it("renders a heading and a button per service after fetch resolves", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /^run tests$/i }),
      ).toBeInTheDocument();
    });

    for (const svc of mockServices) {
      expect(
        screen.getByRole("button", { name: new RegExp(svc.name, "i") }),
      ).toBeInTheDocument();
    }
  });

  it("disables the button when useServiceStatus reports that service is running", async () => {
    mockUseServiceStatus.mockReturnValue({
      serviceStatus: { ci: true },
      connected: true,
      error: null,
    });

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /ci running/i }),
      ).toBeDisabled();
    });

    // Other services should remain enabled.
    expect(
      screen.getByRole("button", { name: /agentic/i }),
    ).not.toBeDisabled();
    expect(
      screen.getByRole("button", { name: /cache/i }),
    ).not.toBeDisabled();
  });

  it("calls POST /api/runs when a button is clicked", async () => {
    const mockFetch = vi.fn();
    // First fetch: GET /api/services (from RunTestsBar mount).
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    // Second fetch: POST /api/runs (from RunButton click).
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: () => Promise.resolve({ id: "run-abc", status: "running" }),
    });
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^run tests for ci$/i })).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: /^run tests for ci$/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    // Call #1: GET /api/services.
    expect(mockFetch.mock.calls[0][0]).toBe(
      "http://localhost:2526/api/services",
    );
    expect(mockFetch.mock.calls[0][1]).toBeUndefined();

    // Call #2: POST /api/runs.
    expect(mockFetch.mock.calls[1][0]).toBe(
      "http://localhost:2526/api/runs",
    );
    expect(mockFetch.mock.calls[1][1]).toEqual(
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ services: ["ci"], suite: "test", flags: [] }),
      }),
    );
  });

  it("shows an error state when the services fetch fails", async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("run-tests-bar-error")).toBeInTheDocument();
    });
  });

  it("shows an empty state when no services are configured", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("run-tests-bar-empty")).toBeInTheDocument();
    });
  });

  // --- F7: Inline feedback (toast) ---

  it("shows a success toast with service name and suite name after a successful POST", async () => {
    const mockFetch = vi.fn();
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: () => Promise.resolve({ id: "run-abc", status: "running" }),
    });
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /^run tests for ci$/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /^run tests for ci$/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(/tests triggered for ci\/test/i),
      ).toBeInTheDocument();
    });
  });

  it("shows an error toast with service name and suite name after a failed POST", async () => {
    const mockFetch = vi.fn();
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    mockFetch.mockRejectedValueOnce(new Error("Network error"));
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /^run tests for ci$/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /^run tests for ci$/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(/failed to trigger tests for ci\/test/i),
      ).toBeInTheDocument();
    });
  });

  it("auto-dismisses toast feedback after 3 seconds", async () => {
    const mockFetch = vi.fn();
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: () => Promise.resolve({ id: "run-abc", status: "running" }),
    });
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /^run tests for ci$/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /^run tests for ci$/i }),
    );

    await waitFor(() => {
      expect(screen.getByText(/tests triggered for ci/i)).toBeInTheDocument();
    });

    // Message should auto-dismiss after 3 seconds.
    await waitFor(
      () => {
        expect(
          screen.queryByText(/tests triggered for ci/i),
        ).not.toBeInTheDocument();
      },
      { timeout: 5000 },
    );
  }, 10000);

  it("forwards apiBase to useServiceStatus and fetch", async () => {
    mockUseServiceStatus.mockReturnValue({
      serviceStatus: { ci: false },
      connected: true,
      error: null,
    });

    const customBase = "https://ci.example.com";

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ name: "ci", repo: "Wywy-CI" }]),
    });

    render(<RunTestsBar apiBase={customBase} />);

    // useServiceStatus should have been called with the custom base.
    expect(mockUseServiceStatus).toHaveBeenCalledWith(customBase);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /ci/i }),
      ).toBeInTheDocument();
    });

    // The services fetch should use the custom base.
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://ci.example.com/api/services",
    );
  });
});
