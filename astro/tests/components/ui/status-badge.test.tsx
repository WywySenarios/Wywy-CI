import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StatusBadge } from "@/components/ui/status-badge";

describe("StatusBadge", () => {
  it.each([
    { status: "queued", expected: "Queued" },
    { status: "running", expected: "Running" },
    { status: "success", expected: "Success" },
    { status: "failure", expected: "Failure" },
    { status: "cancelled", expected: "Cancelled" },
    { status: "skipped", expected: "Skipped" },
  ] as const)('renders "$expected" for "$status" status', ({ status, expected }) => {
    render(<StatusBadge status={status} />);
    expect(screen.getByTestId("status-badge")).toHaveTextContent(expected);
  });
});

describe("StatusBadge variant classes", () => {
  it.each([
    { status: "queued", expectedClass: "text-muted-foreground" },
    { status: "running", expectedClass: "text-blue-800" },
    { status: "success", expectedClass: "text-green-800" },
    { status: "failure", expectedClass: "text-red-800" },
    { status: "cancelled", expectedClass: "text-gray-800" },
    { status: "skipped", expectedClass: "text-yellow-800" },
  ] as const)('applies "$expectedClass" class for "$status" status', ({ status, expectedClass }) => {
    render(<StatusBadge status={status} />);
    expect(screen.getByTestId("status-badge")).toHaveClass(expectedClass);
  });
});

describe("StatusBadge className prop", () => {
  it("merges custom className with variant classes", () => {
    render(<StatusBadge status="success" className="my-custom-class" />);
    const badge = screen.getByTestId("status-badge");
    expect(badge).toHaveClass("my-custom-class");
    expect(badge).toHaveClass("text-green-800");
  });
});

describe("StatusBadge icons", () => {
  it.each([
    { status: "queued" as const, iconTestId: "status-icon-queued" },
    { status: "running" as const, iconTestId: "status-icon-running" },
    { status: "success" as const, iconTestId: "status-icon-success" },
    { status: "failure" as const, iconTestId: "status-icon-failure" },
    { status: "cancelled" as const, iconTestId: "status-icon-cancelled" },
    { status: "skipped" as const, iconTestId: "status-icon-skipped" },
  ])('renders $iconTestId for $status status', ({ status, iconTestId }) => {
    render(<StatusBadge status={status} />);
    expect(screen.getByTestId(iconTestId)).toBeInTheDocument();
  });
});
