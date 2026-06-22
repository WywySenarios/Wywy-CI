import { render, screen, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RunsPage } from "@/components/runs/RunsPage";

// Mock child components so we test the page-level routing logic in isolation.
vi.mock("@/components/runs/RunList", () => ({
  RunList: vi.fn(() => <div data-testid="run-list-mock">RunList</div>),
}));
vi.mock("@/components/runs/RunDetail", () => ({
  RunDetail: vi.fn(({ id }: { id: string }) => (
    <div data-testid="run-detail-mock">RunDetail for {id}</div>
  )),
}));

describe("RunsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders the run list when no id is provided", () => {
    render(<RunsPage />);
    expect(screen.getByTestId("run-list-mock")).toBeInTheDocument();
    expect(screen.queryByTestId("run-detail-mock")).not.toBeInTheDocument();
  });

  it("renders the run detail when an id is provided", () => {
    render(<RunsPage id="run-abc" />);
    expect(screen.getByTestId("run-detail-mock")).toBeInTheDocument();
    expect(screen.queryByTestId("run-list-mock")).not.toBeInTheDocument();
  });

  it("passes the id prop to RunDetail", () => {
    render(<RunsPage id="run-abc" />);
    expect(screen.getByText(/run-abc/i)).toBeInTheDocument();
  });

  it("shows a back link to /runs when an id is provided", () => {
    render(<RunsPage id="run-abc" />);
    const backLink = screen.getByRole("link", { name: /back to runs/i });
    expect(backLink).toBeInTheDocument();
    expect(backLink).toHaveAttribute("href", "/runs");
  });

  it("does not show a back link when no id is provided", () => {
    render(<RunsPage />);
    expect(
      screen.queryByRole("link", { name: /back to runs/i }),
    ).not.toBeInTheDocument();
  });

  it("produces different DOM with and without an id (snapshot)", () => {
    const { container: listView } = render(<RunsPage />);
    const listHtml = listView.innerHTML;

    cleanup();

    const { container: detailView } = render(<RunsPage id="run-abc" />);
    const detailHtml = detailView.innerHTML;

    // The two states must render different DOM
    expect(listHtml).not.toEqual(detailHtml);
    expect(listHtml).toMatchSnapshot("runs-page-list");
    expect(detailHtml).toMatchSnapshot("runs-page-detail");
  });
});
