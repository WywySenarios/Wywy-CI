import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { RunCard } from "@/components/runs/RunCard";

describe("RunCard", () => {
  it("renders the run ID", () => {
    render(<RunCard id="abc123" status="success" />);
    expect(screen.getByText("abc123")).toBeInTheDocument();
  });

  it("renders a StatusBadge for the run status", () => {
    render(<RunCard id="abc123" status="success" />);
    expect(screen.getByTestId("status-badge")).toHaveTextContent("Success");
  });
});
