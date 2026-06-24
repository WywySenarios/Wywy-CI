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
    { name: "ci", repo: "Wywy-CI", suites: ["test", "e2e"] },
    { name: "agentic", repo: "Wywy-Codes", suites: ["test"] },
    { name: "cache", repo: "Wywy-Website-Cache", suites: ["test"] },
  ];

  beforeEach(() => {
    // Default: no suites running, WebSocket connected.
    mockUseServiceStatus.mockReturnValue({
      suiteStatus: {},
      serviceStatus: {},
      connected: true,
      error: null,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Loading / error / empty ────────────────────────────────────

  it("shows a loading indicator while services are being fetched", () => {
    globalThis.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
    render(<RunTestsBar />);
    expect(screen.getByTestId("run-tests-bar-loading")).toBeInTheDocument();
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

  // ── Rendering ──────────────────────────────────────────────────

  it("renders a heading and a trigger button per service after fetch resolves", async () => {
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
        screen.getByTestId(`service-trigger-${svc.name}`),
      ).toBeInTheDocument();
    }
  });

  it("opens a dropdown with suite options when a trigger is clicked", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    // Click the ci trigger to open the dropdown.
    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // ci has suites: ["test", "e2e"] → both options visible.
    expect(screen.getByTestId("run-all-ci")).toBeInTheDocument();
    expect(screen.getByTestId("run-test-ci")).toBeInTheDocument();
    expect(screen.getByTestId("run-e2e-ci")).toBeInTheDocument();
  });

  it("renders suite options from each service's suites field, not a hardcoded list", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    // ci has suites ["test", "e2e"].
    await userEvent.click(screen.getByTestId("service-trigger-ci"));
    expect(screen.getByTestId("run-test-ci")).toBeInTheDocument();
    expect(screen.getByTestId("run-e2e-ci")).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("service-trigger-ci")); // close

    // agentic has suites ["test"] only → no "e2e" option.
    await userEvent.click(screen.getByTestId("service-trigger-agentic"));
    expect(screen.getByTestId("run-test-agentic")).toBeInTheDocument();
    expect(
      screen.queryByTestId("run-e2e-agentic"),
    ).not.toBeInTheDocument();
  });

  // ── Triggering runs ────────────────────────────────────────────

  it("calls POST /api/runs for a specific suite when user clicks that suite option", async () => {
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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    // Open the ci dropdown.
    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // Click the "test" suite option.
    await userEvent.click(screen.getByTestId("run-test-ci"));

    await waitFor(() => {
      // Call #1: GET /api/services; call #2: POST /api/runs.
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

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

  it("calls POST /api/runs for each suite when 'All tests' is clicked (ci: 2 suites)", async () => {
    const mockFetch = vi.fn();
    // First call: GET /api/services.
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    // Leave POST calls pending so they can be inspected before resolution.
    const pendingPromise = new Promise(() => {});
    mockFetch.mockImplementation(() => pendingPromise);
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    // Open the ci dropdown.
    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // Click "All tests".
    await userEvent.click(screen.getByTestId("run-all-ci"));

    await waitFor(() => {
      // 1 GET + 2 POSTs = 3 calls.
      expect(mockFetch).toHaveBeenCalledTimes(3);
    });

    // First POST: suite "test".
    expect(mockFetch.mock.calls[1][1].body).toContain('"suite":"test"');
    expect(mockFetch.mock.calls[1][1].body).toContain('"services":["ci"]');

    // Second POST: suite "e2e".
    expect(mockFetch.mock.calls[2][1].body).toContain('"suite":"e2e"');
    expect(mockFetch.mock.calls[2][1].body).toContain('"services":["ci"]');
  });

  it("calls POST /api/runs once when 'All tests' is clicked on agentic (1 suite)", async () => {
    const mockFetch = vi.fn();
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });
    const pendingPromise = new Promise(() => {});
    mockFetch.mockImplementation(() => pendingPromise);
    globalThis.fetch = mockFetch;

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-agentic")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-agentic"));
    await userEvent.click(screen.getByTestId("run-all-agentic"));

    await waitFor(() => {
      // 1 GET + 1 POST = 2 calls (agentic only has ["test"]).
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    expect(mockFetch.mock.calls[1][1].body).toContain('"suite":"test"');
    expect(mockFetch.mock.calls[1][1].body).toContain('"services":["agentic"]');
  });

  // ── Spinner states ─────────────────────────────────────────────

  it("shows a spinner on 'All tests' when any suite is running for that service", async () => {
    mockUseServiceStatus.mockReturnValue({
      suiteStatus: { ci: { test: true } },
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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // "All tests" option has a spinner because a suite is running.
    expect(screen.getByTestId("spinner-run-all-ci")).toBeInTheDocument();
  });

  it("shows a spinner on a specific suite when that suite is running", async () => {
    mockUseServiceStatus.mockReturnValue({
      suiteStatus: { ci: { test: true, e2e: false } },
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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // "test" suite has a spinner because it's running.
    expect(screen.getByTestId("spinner-run-test-ci")).toBeInTheDocument();

    // "e2e" suite has no spinner because it's not running.
    expect(
      screen.queryByTestId("spinner-run-e2e-ci"),
    ).not.toBeInTheDocument();
  });

  it("does not show any spinners when no suites are running", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    expect(screen.queryByTestId("spinner-run-all-ci")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("spinner-run-test-ci"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("spinner-run-e2e-ci"),
    ).not.toBeInTheDocument();
  });

  // ── Disabled states ────────────────────────────────────────────

  it("disables 'All tests' when any suite is running for that service", async () => {
    mockUseServiceStatus.mockReturnValue({
      suiteStatus: { ci: { test: true } },
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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    expect(screen.getByTestId("run-all-ci")).toBeDisabled();
  });

  it("disables a specific suite option when that suite is running", async () => {
    mockUseServiceStatus.mockReturnValue({
      suiteStatus: { ci: { test: true, e2e: false } },
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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    // "test" is running → disabled.
    expect(screen.getByTestId("run-test-ci")).toBeDisabled();

    // "e2e" is not running → enabled.
    expect(screen.getByTestId("run-e2e-ci")).not.toBeDisabled();
  });

  it("does not disable any options when no suites are running", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockServices),
    });

    render(<RunTestsBar />);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));

    expect(screen.getByTestId("run-all-ci")).not.toBeDisabled();
    expect(screen.getByTestId("run-test-ci")).not.toBeDisabled();
    expect(screen.getByTestId("run-e2e-ci")).not.toBeDisabled();
  });

  // ── Toast feedback ────────────────────────────────────────────

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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));
    await userEvent.click(screen.getByTestId("run-test-ci"));

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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));
    await userEvent.click(screen.getByTestId("run-test-ci"));

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
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByTestId("service-trigger-ci"));
    await userEvent.click(screen.getByTestId("run-test-ci"));

    await waitFor(() => {
      expect(screen.getByText(/tests triggered for ci/i)).toBeInTheDocument();
    });

    await waitFor(
      () => {
        expect(
          screen.queryByText(/tests triggered for ci/i),
        ).not.toBeInTheDocument();
      },
      { timeout: 5000 },
    );
  }, 10000);

  // ── apiBase ────────────────────────────────────────────────────

  it("forwards apiBase to useServiceStatus and fetch", async () => {
    const customBase = "https://ci.example.com";

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ name: "ci", repo: "Wywy-CI", suites: ["test"] }]),
    });

    render(<RunTestsBar apiBase={customBase} />);

    expect(mockUseServiceStatus).toHaveBeenCalledWith(customBase);

    await waitFor(() => {
      expect(screen.getByTestId("service-trigger-ci")).toBeInTheDocument();
    });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://ci.example.com/api/services",
    );
  });
});
