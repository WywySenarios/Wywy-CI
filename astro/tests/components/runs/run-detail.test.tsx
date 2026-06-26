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

  it("shows passed/failed/skipped counts for each service", async () => {
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
            passed: 3,
            failed: 1,
            skipped: 0,
          },
          {
            run_id: "run-abc",
            service_name: "agentic",
            suite: "test",
            status: "passed",
            exit_code: 0,
            start_time: "",
            end_time: "2026-06-22T00:54:00Z",
            passed: 5,
            failed: 2,
            skipped: 1,
          },
        ],
      }),
    } as Response);

    render(<RunDetail id="run-abc" />);
    expect(await screen.findByTestId("run-detail")).toBeInTheDocument();

    // Each service must display its passed/failed/skipped counts.
    const ciCounts = screen.getByTestId("service-counts-ci");
    expect(ciCounts).toBeInTheDocument();
    expect(ciCounts).toHaveTextContent(/3.*passed/i);
    expect(ciCounts).toHaveTextContent(/1.*failed/i);
    expect(ciCounts).toHaveTextContent(/0.*skipped/i);

    const agenticCounts = screen.getByTestId("service-counts-agentic");
    expect(agenticCounts).toBeInTheDocument();
    expect(agenticCounts).toHaveTextContent(/5.*passed/i);
    expect(agenticCounts).toHaveTextContent(/2.*failed/i);
    expect(agenticCounts).toHaveTextContent(/1.*skipped/i);
  });
});
