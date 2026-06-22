import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { RunButton } from "@/components/runs/RunButton";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

describe("RunButton", () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 202,
      json: () => Promise.resolve({ id: "run-abc", status: "running" }),
    });
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders a button with Run Tests text when not running", () => {
    render(<RunButton serviceName="ci" isRunning={false} onRunClick={() => {}} />);
    const btn = screen.getByRole("button", { name: /run tests/i });
    expect(btn).toBeInTheDocument();
    expect(btn).not.toBeDisabled();
  });

  it("calls onRunClick with the service name when clicked", async () => {
    const onClick = vi.fn();
    render(<RunButton serviceName="ci" isRunning={false} onRunClick={onClick} />);
    const btn = screen.getByRole("button", { name: /run tests/i });
    await userEvent.click(btn);
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onClick).toHaveBeenCalledWith("ci");
  });

  it("disables the button when the service is running", () => {
    render(<RunButton serviceName="ci" isRunning={true} onRunClick={() => {}} />);
    const btn = screen.getByRole("button");
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute("aria-disabled", "true");
  });

  it("shows a spinner icon when the service is running", () => {
    render(<RunButton serviceName="ci" isRunning={true} onRunClick={() => {}} />);
    // LoaderCircle has aria-label or data-testid or is a SVG child.
    const svg = document.querySelector(".lucide-loader-circle");
    expect(svg).toBeInTheDocument();
  });

  it("shows the service name in the button text", () => {
    render(<RunButton serviceName="agentic" isRunning={false} onRunClick={() => {}} />);
    expect(screen.getByRole("button")).toHaveTextContent(/agentic/i);
  });

  it("POSTs to /api/runs when clicked with apiBase", async () => {
    render(
      <RunButton
        serviceName="ci"
        isRunning={false}
        onRunClick={() => {}}
        apiBase="http://localhost:2526"
      />,
    );
    await userEvent.click(screen.getByRole("button"));
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:2526/api/runs",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ services: ["ci"], suite: "test", flags: [] }),
      }),
    );
  });

  it("disables the button briefly while submitting", async () => {
    // Keep the promise pending so we can check the intermediate state.
    let resolvePromise!: (v: unknown) => void;
    globalThis.fetch = vi.fn().mockReturnValue(
      new Promise((resolve) => {
        resolvePromise = resolve;
      }),
    );

    render(
      <RunButton
        serviceName="ci"
        isRunning={false}
        onRunClick={() => {}}
        apiBase="http://localhost:2526"
      />,
    );

    const btn = screen.getByRole("button");
    await userEvent.click(btn);

    // Button should be disabled while submitting.
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent(/running/i);

    // Resolve the request.
    resolvePromise({ ok: true, status: 202, json: () => Promise.resolve({}) });
  });

  it("shows success feedback briefly after a successful POST", async () => {
    render(
      <RunButton
        serviceName="ci"
        suite="test"
        isRunning={false}
        onRunClick={() => {}}
        apiBase="http://localhost:2526"
      />,
    );

    await userEvent.click(screen.getByRole("button"));

    // Wait for the success indicator to appear.
    await waitFor(() => {
      expect(screen.getByRole("button")).toHaveTextContent(/triggered/i);
    });
  });

  it("includes service name and suite in the success feedback", async () => {
    render(
      <RunButton
        serviceName="ci"
        suite="e2e"
        isRunning={false}
        onRunClick={() => {}}
        apiBase="http://localhost:2526"
      />,
    );

    await userEvent.click(screen.getByRole("button"));

    await waitFor(() => {
      const btn = screen.getByRole("button");
      expect(btn).toHaveTextContent(/ci/i);
      expect(btn).toHaveTextContent(/e2e/i);
    });
  });

  it("shows error feedback after a failed POST", async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    render(
      <RunButton
        serviceName="ci"
        isRunning={false}
        onRunClick={() => {}}
        apiBase="http://localhost:2526"
      />,
    );

    await userEvent.click(screen.getByRole("button"));

    await waitFor(() => {
      expect(screen.getByRole("button")).toHaveTextContent(/failed/i);
    });
  });
});
