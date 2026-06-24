import { useCallback, useEffect, useRef, useState } from "react";
import { CheckCircle2, LoaderCircle, XCircle } from "lucide-react";
import { DEFAULT_API_BASE } from "@/lib/ci-types";
import { useServiceStatus } from "@/hooks/useServiceStatus";

/** A service object as returned by GET /api/services. */
interface Service {
  name: string;
  repo: string;
  /** Available test suite names (e.g. ["test", "e2e", "playwright"]). */
  suites: string[];
}

/** Per-suite toast feedback state. */
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
 * RunTestsBar fetches the list of services and renders a per-service
 * dropdown trigger. Clicking a service trigger opens a menu with options
 * to run "All tests" or any specific suite discovered for that service.
 * Each option shows a spinner when its respective suite is running and is
 * disabled while it is running.
 */
export function RunTestsBar({ apiBase = DEFAULT_API_BASE }: RunTestsBarProps) {
  const [services, setServices] = useState<Service[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [openService, setOpenService] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const { suiteStatus, serviceStatus } = useServiceStatus(apiBase);
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

  // Fetch the list of services.
  useEffect(() => {
    let cancelled = false;

    fetch(`${apiBase}/api/services`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        // type assertion: the CI API is guaranteed to return Service[] for
        // /api/services; any mismatch triggers the catch.
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

  /**
   * Fires a single-suite test run for a service.
   */
  const triggerSuite = useCallback(
    async (serviceName: string, suite: string) => {
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
        setFeedback({
          type: res.ok ? "success" : "error",
          serviceName,
          suite,
        });
      } catch {
        setFeedback({ type: "error", serviceName, suite });
      }
    },
    [apiBase],
  );

  /**
   * Fires test runs for all given suites of a service concurrently.
   *
   * @param serviceName - The service to run tests on.
   * @param suites - The list of suite names to trigger.
   */
  const triggerAll = useCallback(
    async (serviceName: string, suites: string[]) => {
      await Promise.all(
        suites.map((suite) => triggerSuite(serviceName, suite)),
      );
    },
    [triggerSuite],
  );

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
        {services.map((svc) => {
          const isSvcRunning = !!serviceStatus[svc.name];
          const svcSuiteStatus = suiteStatus[svc.name] ?? {};

          return (
            <div key={svc.name} className="relative">
              {/* Dropdown trigger */}
              <button
                data-testid={`service-trigger-${svc.name}`}
                type="button"
                onClick={() =>
                  setOpenService(
                    openService === svc.name ? null : svc.name,
                  )
                }
                className="inline-flex items-center gap-x-1.5 rounded-md border bg-background px-3 py-1.5 text-sm font-medium transition-colors hover:bg-muted/50"
              >
                {isSvcRunning && (
                  <LoaderCircle
                    data-testid={`spinner-${svc.name}`}
                    className="h-4 w-4 animate-spin"
                  />
                )}
                Run {svc.name}
              </button>

              {/* Dropdown menu */}
              {openService === svc.name && (
                <div
                  data-testid={`menu-${svc.name}`}
                  className="absolute left-0 top-full z-10 mt-1 flex min-w-[160px] flex-col gap-1 rounded-lg border bg-popover p-2 shadow-md"
                >
                  {/* All tests */}
                  <button
                    data-testid={`run-all-${svc.name}`}
                    type="button"
                    disabled={isSvcRunning}
                    onClick={async () => {
                      setOpenService(null);
                      await triggerAll(svc.name, svc.suites);
                    }}
                    className="inline-flex items-center gap-x-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
                  >
                    {isSvcRunning && (
                      <LoaderCircle
                        data-testid={`spinner-run-all-${svc.name}`}
                        className="h-4 w-4 animate-spin"
                      />
                    )}
                    All tests
                  </button>

                  {/* Per-suite options */}
                  {(svc.suites ?? []).map((suite) => {
                    const isSuiteRunning = !!svcSuiteStatus[suite];
                    return (
                      <button
                        key={suite}
                        data-testid={`run-${suite}-${svc.name}`}
                        type="button"
                        disabled={isSuiteRunning}
                        onClick={async () => {
                          setOpenService(null);
                          await triggerSuite(svc.name, suite);
                        }}
                        className="inline-flex items-center gap-x-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
                      >
                        {isSuiteRunning && (
                          <LoaderCircle
                            data-testid={`spinner-run-${suite}-${svc.name}`}
                            className="h-4 w-4 animate-spin"
                          />
                        )}
                        {suite}
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Inline toast feedback */}
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
          );
        })}
      </div>
    </section>
  );
}
