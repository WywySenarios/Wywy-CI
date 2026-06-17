"use client";

import { ThemeToggle } from "@/components/ui/theme-toggle";
import { cn } from "@/lib/utils";

/**
 * NavBar renders the top navigation bar with branding, page links,
 * and a ThemeToggle button for switching between light and dark modes.
 */
export interface NavBarProps {
  /** Optional additional CSS classes. */
  className?: string;
}

export function NavBar({ className }: NavBarProps) {
  return (
    <nav
      data-testid="navbar"
      className={cn(
        "border-b border-border bg-background/80 sticky top-0 z-10 backdrop-blur",
        className,
      )}
    >
      <div className="max-w-6xl mx-auto px-6 py-3 flex items-center gap-6">
        <a
          href="/"
          className="font-semibold text-lg text-foreground hover:text-primary transition-colors"
        >
          Wywy-CI
        </a>
        <div className="flex gap-4 text-sm flex-1">
          <a
            href="/"
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            Dashboard
          </a>
          <a
            href="/runs/_spa/"
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            Run Detail
          </a>
        </div>
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </div>
    </nav>
  );
}
