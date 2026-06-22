import { useCallback, useEffect, useRef, useState } from "react";
import { CheckCircle2, XCircle } from "lucide-react";
import { DEFAULT_API_BASE } from "@/lib/ci-types";
import { useServiceStatus } from "@/hooks/useServiceStatus";
import { RunButton } from "@/components/runs/RunButton";

/** A service object as returned by GET /api/services. */
interface Service {
  name: string;
  repo: string;
}

/** Per-service toast feedback state. */
interface FeedbackState {
  type: "success" | "error";
  serviceName: string;
  suite: string;
}

/**
 * Props for the RunTestsBar component.
 */
export interface RunTestsBarProps {
  /** Base URL of the CI API (defaults to localhost:2526). */
  apiBase?: string;
}

/**
 * RunTestsBar fetches the list of services and renders a "Run Tests" section
 * with a RunButton for each service. Running state is driven by the
 * useServiceStatus hook.
 */
export function RunTestsBar({ apiBase = DEFAULT_API_BASE }: RunTestsBarProps) {
  const [services, setServices] = useState<Service[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const { serviceStatus } = useServiceStatus(apiBase);
  const dismissTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Auto-dismiss toast feedback after 3 seconds.
  useEffect(() => {
    if (feedback === null) return;
    dismissTimer.current = setTimeout(() => {
      setFeedback(null);
    }, 3000);
    return () => {
      if (dismissTimer.current) clearTimeout(dismissTimer.current);
    };
  }, [feedback]);

  const handleComplete = useCallback(
    (serviceName: string, suite: string, type: "success" | "error") => {
      setFeedback({ type, serviceName, suite });
    },
    [],
  );

  useEffect(() => {
    let cancelled = false;

    fetch(`${apiBase}/api/services`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json() as Promise<Service[]>;
      })
      .then((data) => {
        if (!cancelled) {
          setServices(data);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [apiBase]);

  if (loading) {
    return <div data-testid="run-tests-bar-loading">Loading services…</div>;
  }

  if (error) {
    return <div data-testid="run-tests-bar-error">Error: {error}</div>;
  }

  if (!services || services.length === 0) {
    return <div data-testid="run-tests-bar-empty">No services configured.</div>;
  }

  return (
    <section>
      <h2 className="text-lg font-semibold">Run Tests</h2>
      <div className="mt-3 flex flex-wrap gap-3">
        {services.map((svc) => (
          <div key={svc.name} className="relative">
            <RunButton
              serviceName={svc.name}
              isRunning={!!serviceStatus[svc.name]}
              onRunClick={() => {}}
              apiBase={apiBase}
              onComplete={(result) => handleComplete(svc.name, "test", result)}
            />
            {feedback !== null && feedback.serviceName === svc.name && (
              <div
                data-testid={
                  feedback.type === "success"
                    ? "toast-feedback-success"
                    : "toast-feedback-error"
                }
                className="mt-1 flex items-center gap-x-1 text-xs"
              >
                {feedback.type === "success" ? (
                  <CheckCircle2 className="h-3.5 w-3.5 text-green-600" />
                ) : (
                  <XCircle className="h-3.5 w-3.5 text-red-600" />
                )}
                <span>
                  {feedback.type === "success"
                    ? `Tests triggered for ${feedback.serviceName}/${feedback.suite}`
                    : `Failed to trigger tests for ${feedback.serviceName}/${feedback.suite}`}
                </span>
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}
