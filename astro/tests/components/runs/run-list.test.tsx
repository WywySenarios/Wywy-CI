import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RunList } from "@/components/runs/RunList";

// Mock useServiceStatus so RunTestsBar (added in GREEN) doesn't open a real WS.
vi.mock("@/hooks/useServiceStatus", () => ({
  useServiceStatus: vi.fn(() => ({
    serviceStatus: {},
    connected: true,
    error: null,
  })),
}));

/** Preserve the default timezone so each test starts from a clean state. */
const DEFAULT_TZ = process.env.TZ;

describe("RunList", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    process.env.TZ = DEFAULT_TZ;
  });

  it("shows a loading indicator while fetching runs", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise(() => {}),
    );
    render(<RunList />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it("shows an empty state when no runs exist", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => [],
    } as Response);
    render(<RunList />);
    expect(await screen.findByText(/no runs/i)).toBeInTheDocument();
  });

  it("renders a list of RunCards for each run", async () => {
    const runs = [
      { id: "run-abc", status: "success", created_at: "2024-01-01T00:00:00Z" },
      { id: "run-xyz", status: "failed", created_at: "2024-01-02T00:00:00Z" },
    ];
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => runs,
    } as Response);
    render(<RunList />);
    const list = await screen.findByTestId("run-list");
    expect(list).toBeInTheDocument();
    expect(list.children).toHaveLength(2);
  });

  it("shows an error message when the fetch fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("Network error"));
    render(<RunList />);
    expect(await screen.findByText(/error/i)).toBeInTheDocument();
  });

  it("links to the run detail page using a query parameter", async () => {
    const runs = [
      { id: "run-abc", status: "success", created_at: "2024-01-01T00:00:00Z" },
    ];
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => runs,
    } as Response);
    render(<RunList />);

    const link = await screen.findByRole("link");
    expect(link).toHaveAttribute("href", "/runs?id=run-abc");
  });

  it("displays the creation timestamp for each run aligned to the right", async () => {
    const runs = [
      { id: "run-abc", status: "success", created_at: "2024-01-01T00:00:00Z" },
      { id: "run-xyz", status: "failed", created_at: "2024-06-15T14:30:00Z" },
    ];
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => runs,
    } as Response);
    render(<RunList />);

    const list = await screen.findByTestId("run-list");

    // Each run entry should display the formatted creation timestamp.
    // The year "2024" appears in both timestamps and is unique to them.
    const timestampElements = screen.getAllByText(/2024/);
    expect(timestampElements).toHaveLength(2);

    // Each timestamp should be right-aligned via ml-auto (pushes flex item right).
    timestampElements.forEach((el) => {
      expect(el.className).toContain("ml-auto");
    });
  });

  it("displays timestamps in the user's local timezone instead of forcing UTC", async () => {
    const runs = [
      { id: "run-abc", status: "success", created_at: "2024-01-01T00:00:00Z" },
    ];

    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => runs,
    } as Response);

    // Simulate a non-UTC timezone so we can detect whether the component
    // forces UTC display or uses the local timezone.
    const origTZ = process.env.TZ;
    process.env.TZ = "America/New_York";

    try {
      render(<RunList />);
      await screen.findByTestId("run-list");

      // 2024-01-01T00:00:00Z in America/New_York (UTC-5) = 2023-12-31T19:00:00.
      // If the component forces UTC (bug), the displayed year is "2024".
      // If it uses local time (correct), the displayed year is "2023".
      expect(screen.getByText(/2023/)).toBeInTheDocument();
    } finally {
      process.env.TZ = origTZ;
    }
  });

  // --- F6: RunTestsBar integration ---

  it("shows a Run Tests section with buttons for each service", async () => {
    const runs = [
      { id: "run-abc", status: "success", created_at: "2024-01-01T00:00:00Z" },
    ];
    const services = [
      { name: "ci", repo: "Wywy-CI" },
      { name: "agentic", repo: "Wywy-Codes" },
    ];

    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce({
        ok: true,
        json: async () => runs,
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => services,
      } as Response);

    render(<RunList />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /run tests/i }),
      ).toBeInTheDocument();
    });

    for (const svc of services) {
      expect(
        screen.getByRole("button", { name: new RegExp(svc.name, "i") }),
      ).toBeInTheDocument();
    }
  });
});
