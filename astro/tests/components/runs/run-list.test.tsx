import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RunList } from "@/components/runs/RunList";

describe("RunList", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
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
});
