import { useEffect, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** An event received over the global events WebSocket. */
interface ServiceEvent {
  type: string;
  run_id: string;
  service_name: string;
  /** Which suite (e.g. "test", "e2e") is being run. */
  suite?: string;
  status?: string;
  timestamp?: string;
}

/**
 * Active-suites response from GET /api/runs/active.
 * Shape: { active_suites: { "ci": { "test": true, "e2e": false }, ... } }
 */
interface ActiveSuitesResponse {
  active_suites?: Record<string, Record<string, boolean>>;
}

/** State returned by useServiceStatus. */
export interface UseServiceStatusResult {
  /**
   * Map of service name → suite name → whether that specific suite
   * currently has a run in progress for that service.
   */
  suiteStatus: Record<string, Record<string, boolean>>;
  /**
   * Derived map of service name → whether the service has *any* suite
   * currently running. True when at least one suite for that service
   * has status `true` in suiteStatus.
   */
  serviceStatus: Record<string, boolean>;
  /** Whether the WebSocket is currently connected. */
  connected: boolean;
  /** Error message if the WebSocket encountered an error. */
  error: string | null;
}

/**
 * useServiceStatus connects to the CI API global events WebSocket and
 * tracks which services (and which specific suites) currently have runs
 * in progress.
 *
 * `serviceStatus` is derived from `suiteStatus`: a service is considered
 * "running" when any suite for that service has status `true`.
 *
 * @param apiBase - Base URL of the CI API (defaults to localhost:2526).
 */
export function useServiceStatus(
  apiBase: string = DEFAULT_API_BASE,
): UseServiceStatusResult {
  const [suiteStatus, setSuiteStatus] = useState<
    Record<string, Record<string, boolean>>
  >({});
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fetchedRef = useRef(false);

  // Derive serviceStatus: a service is "running" when any of its suites is.
  const serviceStatus = useMemo(() => {
    const result: Record<string, boolean> = {};
    for (const [service, suites] of Object.entries(suiteStatus)) {
      result[service] = Object.values(suites).some(Boolean);
    }
    return result;
  }, [suiteStatus]);

  useEffect(() => {
    const wsUrl = `${apiBase.replace(/^http/, "ws")}/api/events`;
    const ws = new WebSocket(wsUrl);

    ws.addEventListener("open", () => {
      setConnected(true);

      // Fetch initial active services only once.
      if (!fetchedRef.current) {
        fetchedRef.current = true;
        fetch(`${apiBase}/api/runs/active`)
          .then((res) => (res.ok ? res.json() : Promise.reject()))
          .then((data: ActiveSuitesResponse) => {
            if (data?.active_suites) {
              flushSync(() => {
                // non-null assertion: guarded by `if (data?.active_suites)` above
                setSuiteStatus((prev) => deepMerge(prev, data.active_suites!));
              });
            }
          })
          .catch(() => {
            // Non-fatal: live events will provide state changes.
          });
      }
    });

    ws.addEventListener("message", (event: MessageEvent) => {
      try {
        // type assertion: JSON.parse returns unknown; the try/catch guards
        // against malformed JSON or unexpected shapes.
        const msg = JSON.parse(event.data) as ServiceEvent;

        if (msg.suite) {
          // Suite-level event — update suiteStatus (serviceStatus is derived).
          // Non-null assertion on msg.suite: guarded by `if (msg.suite)` above.
          if (msg.type === "run_started") {
            setSuiteStatus((prev) => ({
              ...prev,
              [msg.service_name]: {
                ...prev[msg.service_name],
                [msg.suite!]: true,
              },
            }));
          } else if (msg.type === "run_finished") {
            setSuiteStatus((prev) => ({
              ...prev,
              [msg.service_name]: {
                ...prev[msg.service_name],
                [msg.suite!]: false,
              },
            }));
          }
        } else {
          // Legacy: no suite field — update serviceStatus directly.
          if (msg.type === "run_started") {
            // Ensure the service has at least one suite entry so the
            // derived serviceStatus reflects "running".
            setSuiteStatus((prev) => ({
              ...prev,
              [msg.service_name]: prev[msg.service_name] ?? { __default: true },
            }));
          } else if (msg.type === "run_finished") {
            setSuiteStatus((prev) => ({
              ...prev,
              [msg.service_name]: prev[msg.service_name] ?? { __default: false },
            }));
          }
        }
      } catch {
        // ignore malformed messages
      }
    });

    ws.addEventListener("error", () => {
      setError("WebSocket connection error");
    });

    return () => {
      ws.close(1000, "component unmounted");
      fetchedRef.current = false;
    };
  }, [apiBase]);

  return { suiteStatus, serviceStatus, connected, error };
}

/**
 * Deep-merges `update` into `base`, overwriting nested record values.
 * Both levels must be plain objects with string keys.
 */
function deepMerge(
  base: Record<string, Record<string, boolean>>,
  update: Record<string, Record<string, boolean>>,
): Record<string, Record<string, boolean>> {
  const result = { ...base };
  for (const [key, val] of Object.entries(update)) {
    result[key] = { ...result[key], ...val };
  }
  return result;
}
