import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RunDetail } from "@/components/runs/RunDetail";

describe("RunDetail", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("shows a loading indicator while fetching the run", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise(() => {}),
    );
    render(<RunDetail id="run-abc" />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it("shows run details when the fetch succeeds", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "run-abc",
        status: "success",
        created_at: "2024-01-01T00:00:00Z",
        finished_at: "2024-01-01T00:05:00Z",
      }),
    } as Response);
    render(<RunDetail id="run-abc" />);

    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();
    expect(screen.getByTestId("status-badge")).toHaveTextContent("Success");
    expect(screen.getByText(/run-abc/i)).toBeInTheDocument();
    expect(screen.getByText(/2024-01-01T00:00:00Z/i)).toBeInTheDocument();
    expect(screen.getByText(/2024-01-01T00:05:00Z/i)).toBeInTheDocument();
  });

  it("shows an error message when the fetch fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("Network error"));
    render(<RunDetail id="run-abc" />);
    expect(await screen.findByText(/error/i)).toBeInTheDocument();
  });

  it("shows a not-found message when the API returns 404", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({ error: "run not found" }),
    } as Response);
    render(<RunDetail id="run-missing" />);
    expect(await screen.findByText(/not found/i)).toBeInTheDocument();
  });
});
