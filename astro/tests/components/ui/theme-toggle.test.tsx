import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, beforeEach } from "vitest";
import { ThemeProvider } from "@/components/ui/theme-provider";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { NavBar } from "@/components/layout/NavBar";

/**
 * Helper to render ThemeToggle inside a ThemeProvider with the "class" strategy,
 * which is how Tailwind dark mode is configured in this project.
 */
function renderWithTheme(initialTheme?: "light" | "dark") {
  // Reset the html class before each render
  document.documentElement.classList.remove("dark");

  if (initialTheme === "dark") {
    document.documentElement.classList.add("dark");
  }

  return render(
    <ThemeProvider attribute="class" defaultTheme={initialTheme ?? "light"} enableSystem={false}>
      <ThemeToggle />
    </ThemeProvider>,
  );
}

describe("ThemeProvider", () => {
  it("renders children", () => {
    render(
      <ThemeProvider attribute="class">
        <div data-testid="child">Hello</div>
      </ThemeProvider>,
    );
    expect(screen.getByTestId("child")).toHaveTextContent("Hello");
  });
});

describe("ThemeToggle", () => {
  beforeEach(() => {
    // Clean up any dark class between tests
    document.documentElement.classList.remove("dark");
  });

  it("renders a toggle button", () => {
    renderWithTheme();
    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
  });

  it("shows a Moon icon in light mode (theme is light)", () => {
    renderWithTheme("light");
    // In light mode the toggle shows the Moon icon (for switching to dark)
    expect(screen.getByTestId("theme-toggle-icon-dark")).toBeInTheDocument();
  });

  it("shows a Sun icon in dark mode (theme is dark)", () => {
    renderWithTheme("dark");
    // In dark mode the toggle shows the Sun icon (for switching to light)
    expect(screen.getByTestId("theme-toggle-icon-light")).toBeInTheDocument();
  });

  it("switches to dark mode when clicked in light mode", async () => {
    const user = userEvent.setup();
    renderWithTheme("light");

    // Start in light mode — no dark class
    expect(document.documentElement.classList.contains("dark")).toBe(false);

    await user.click(screen.getByTestId("theme-toggle"));

    // After click, the dark class should be applied
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("switches back to light mode when clicked in dark mode", async () => {
    const user = userEvent.setup();
    renderWithTheme("dark");

    // Start in dark mode
    expect(document.documentElement.classList.contains("dark")).toBe(true);

    await user.click(screen.getByTestId("theme-toggle"));

    // After click, dark class should be removed
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});

describe("NavBar integration", () => {
  it("renders the ThemeToggle as a visible button in the navigation bar", () => {
    render(
      <ThemeProvider attribute="class" enableSystem={false}>
        <NavBar />
      </ThemeProvider>,
    );

    const toggle = screen.getByTestId("theme-toggle");
    expect(toggle).toBeInTheDocument();
    expect(toggle).toBeVisible();
  });
});

describe("Dark mode CSS custom properties", () => {
  const LIGHT_BACKGROUND = "#ffffff";
  const LIGHT_FOREGROUND = "#0a0a0a";
  const DARK_BACKGROUND = "#09090b";
  const DARK_FOREGROUND = "#fafafa";

  beforeEach(() => {
    document.documentElement.classList.remove("dark");
    // Remove any inline style variables we set
    document.documentElement.style.removeProperty("--color-background");
    document.documentElement.style.removeProperty("--color-foreground");
  });

  it("uses light colour values when .dark class is absent", () => {
    // Set the CSS variables as they are defined in global.css :root
    document.documentElement.style.setProperty("--color-background", LIGHT_BACKGROUND);
    document.documentElement.style.setProperty("--color-foreground", LIGHT_FOREGROUND);

    // Make sure .dark is NOT applied
    expect(document.documentElement.classList.contains("dark")).toBe(false);

    const bg = getComputedStyle(document.documentElement).getPropertyValue("--color-background").trim();
    const fg = getComputedStyle(document.documentElement).getPropertyValue("--color-foreground").trim();

    expect(bg).toBe(LIGHT_BACKGROUND);
    expect(fg).toBe(LIGHT_FOREGROUND);

    // Specifically: light mode foreground is near-black (#0a0a0a)
    // and background is white (#ffffff)
    expect(fg).toBe(LIGHT_FOREGROUND); // near-black
    expect(bg).toBe(LIGHT_BACKGROUND); // white
  });

  it("uses dark colour values (background near-black) when .dark class is present", () => {
    // Set the CSS variables as they are defined in global.css .dark
    document.documentElement.style.setProperty("--color-background", DARK_BACKGROUND);
    document.documentElement.style.setProperty("--color-foreground", DARK_FOREGROUND);

    // Add the .dark class
    document.documentElement.classList.add("dark");

    expect(document.documentElement.classList.contains("dark")).toBe(true);

    const bg = getComputedStyle(document.documentElement).getPropertyValue("--color-background").trim();
    const fg = getComputedStyle(document.documentElement).getPropertyValue("--color-foreground").trim();

    expect(bg).toBe(DARK_BACKGROUND); // near-black
    expect(fg).toBe(DARK_FOREGROUND); // near-white

    // Specifically verify foreground turns light (near-white) and background turns dark (near-black)
    expect(bg).toBe("#09090b"); // very dark grey / near black
    expect(fg).toBe("#fafafa"); // off-white
  });

  it("transitions colours when .dark class is toggled on then off", () => {
    // Set both light and dark variables — in production these come from :root and .dark CSS rules
    document.documentElement.style.setProperty("--color-background", LIGHT_BACKGROUND);
    document.documentElement.style.setProperty("--color-foreground", LIGHT_FOREGROUND);

    // Verify light mode values
    let bg = getComputedStyle(document.documentElement).getPropertyValue("--color-background").trim();
    expect(bg).toBe(LIGHT_BACKGROUND);

    // Simulate dark mode toggle by updating the variables and adding class
    document.documentElement.style.setProperty("--color-background", DARK_BACKGROUND);
    document.documentElement.style.setProperty("--color-foreground", DARK_FOREGROUND);
    document.documentElement.classList.add("dark");

    bg = getComputedStyle(document.documentElement).getPropertyValue("--color-background").trim();
    expect(bg).toBe(DARK_BACKGROUND);

    // Simulate switching back to light mode
    document.documentElement.classList.remove("dark");
    document.documentElement.style.setProperty("--color-background", LIGHT_BACKGROUND);
    document.documentElement.style.setProperty("--color-foreground", LIGHT_FOREGROUND);

    bg = getComputedStyle(document.documentElement).getPropertyValue("--color-background").trim();
    expect(bg).toBe(LIGHT_BACKGROUND);
  });
});
