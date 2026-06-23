import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, LoaderCircle, Play, XCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** Possible feedback states after a trigger attempt. */
type FeedbackType = "success" | "error" | null;

/**
 * Props for the RunButton component.
 */
export interface RunButtonProps {
  /** The service name to display and pass to onRunClick. */
  serviceName: string;
  /** The test suite to run (defaults to "test"). */
  suite?: string;
  /** Whether the service currently has a run in progress. */
  isRunning: boolean;
  /** Called when the button is clicked (only when not running). */
  onRunClick: (serviceName: string) => void;
  /** Base URL of the CI API (defaults to localhost:2526). */
  apiBase?: string;
  /** Called after the POST resolves. Receives the result type. */
  onComplete?: (result: "success" | "error") => void;
  /** Optional additional CSS classes. */
  className?: string;
}

/**
 * RunButton renders a per-service "Run Tests" button that POSTs to
 * /api/runs when clicked, disables itself during submission, and shows
 * brief success/error feedback.
 */
export function RunButton({
  serviceName,
  suite = "test",
  isRunning,
  onRunClick,
  apiBase = DEFAULT_API_BASE,
  onComplete,
  className,
}: RunButtonProps) {
  const [submitting, setSubmitting] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackType>(null);

  // Auto-dismiss feedback after 2 seconds.
  useEffect(() => {
    if (feedback === null) return;
    const timer = setTimeout(() => setFeedback(null), 2000);
    return () => clearTimeout(timer);
  }, [feedback]);

  const handleClick = useCallback(async () => {
    onRunClick(serviceName);

    if (!apiBase) return;

    setSubmitting(true);
    try {
      const res = await fetch(`${apiBase}/api/runs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          services: [serviceName],
          suite,
          flags: [],
        }),
      });

      if (res.ok) {
        setFeedback("success");
        onComplete?.("success");
      } else {
        setFeedback("error");
        onComplete?.("error");
      }
    } catch {
      setFeedback("error");
      onComplete?.("error");
    } finally {
      setSubmitting(false);
    }
  }, [serviceName, suite, onRunClick, apiBase, onComplete]);

  const disabled = isRunning || submitting;

  let icon: React.ReactNode;
  let label: string;

  if (isRunning) {
    icon = (
      <LoaderCircle data-testid="spinner-icon" className="h-4 w-4 animate-spin" />
    );
    label = `${serviceName} Running...`;
  } else if (submitting) {
    icon = (
      <LoaderCircle data-testid="submitting-icon" className="h-4 w-4 animate-spin" />
    );
    label = `Running...`;
  } else if (feedback === "success") {
    icon = <CheckCircle2 data-testid="success-icon" className="h-4 w-4 text-green-600" />;
    label = `Triggered ${serviceName}/${suite}`;
  } else if (feedback === "error") {
    icon = <XCircle data-testid="error-icon" className="h-4 w-4 text-red-600" />;
    label = `Failed`;
  } else {
    icon = <Play data-testid="play-icon" className="h-4 w-4" />;
    label = `Run ${serviceName} ${suite}`;
  }

  return (
    <button
      type="button"
      disabled={disabled}
      aria-disabled={disabled}
      onClick={handleClick}
      className={cn(
        "inline-flex items-center gap-x-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
        disabled
          ? "cursor-not-allowed bg-muted text-muted-foreground"
          : "bg-primary text-primary-foreground hover:bg-primary/90 active:bg-primary/80",
        className,
      )}
    >
      {icon}
      {label}
    </button>
  );
}
