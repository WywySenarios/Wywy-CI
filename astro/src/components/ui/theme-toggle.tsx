"use client";

import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

/**
 * ThemeToggle button that switches between light and dark modes.
 * Uses next-themes' useTheme hook.
 */
export interface ThemeToggleProps {
  /** Optional additional CSS classes. */
  className?: string;
}

export function ThemeToggle({ className }: ThemeToggleProps) {
  const { theme, setTheme } = useTheme();

  const isDark = theme === "dark";
  const Icon = isDark ? Sun : Moon;

  return (
    <button
      data-testid="theme-toggle"
      className={cn(
        "inline-flex items-center justify-center rounded-md p-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
        className,
      )}
      onClick={() => setTheme(isDark ? "light" : "dark")}
    >
      <Icon
        data-testid={`theme-toggle-icon-${isDark ? "light" : "dark"}`}
        className="h-5 w-5"
      />
      <span className="sr-only">Toggle theme</span>
    </button>
  );
}
