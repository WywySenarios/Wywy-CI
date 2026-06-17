import { cva } from "class-variance-authority";
import {
  Ban,
  CheckCircle2,
  Clock,
  LoaderCircle,
  SkipForward,
  XCircle,
} from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Supported CI pipeline run statuses.
 */
export type Status = "queued" | "running" | "success" | "failure" | "cancelled" | "skipped";

/**
 * Human-readable labels for each CI status.
 */
const STATUS_LABELS: Record<Status, string> = {
  queued: "Queued",
  running: "Running",
  success: "Success",
  failure: "Failure",
  cancelled: "Cancelled",
  skipped: "Skipped",
};

/**
 * Lucide icon component for each CI status.
 */
const STATUS_ICONS: Record<Status, React.ElementType> = {
  queued: Clock,
  running: LoaderCircle,
  success: CheckCircle2,
  failure: XCircle,
  cancelled: Ban,
  skipped: SkipForward,
};

/**
 * Tailwind variant classes for each CI status.
 */
const badgeVariants = cva(
  "inline-flex items-center gap-x-1 rounded-md px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      status: {
        queued:
          "bg-muted text-muted-foreground",
        running:
          "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100",
        success:
          "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100",
        failure:
          "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100",
        cancelled:
          "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-100",
        skipped:
          "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-100",
      },
    },
    defaultVariants: {
      status: "queued",
    },
  },
);

/**
 * StatusBadge displays the status of a CI pipeline run with appropriate
 * label and colour styling.
 *
 * @param status — The CI status value.
 */
export interface StatusBadgeProps {
  /** The CI pipeline run status. */
  status: Status;
  /** Optional additional CSS classes to merge with the variant styles. */
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const Icon = STATUS_ICONS[status];
  return (
    <span
      data-testid="status-badge"
      className={cn(badgeVariants({ status }), className)}
    >
      {/* CONVENTION-EXCEPTION: etc/Wywy-Website-Control/internal/conventions/languages/_shared.mdx
          Icon fallback required — React 19 + jsdom returns undefined for forwardRef
          components during useState-driven re-renders. Direct renders work fine. */}
      {Icon ? (
        <Icon
          data-testid={`status-icon-${status}`}
          className={cn("h-3.5 w-3.5", status === "running" && "animate-spin")}
        />
      ) : (
        <span data-testid={`status-icon-${status}`} />
      )}
      {STATUS_LABELS[status]}
    </span>
  );
}
