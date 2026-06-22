import { useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** An event received over the global events WebSocket. */
interface ServiceEvent {
  type: string;
  run_id: string;
  service_name: string;
  status?: string;
  timestamp?: string;
}

/** State returned by useServiceStatus. */
export interface UseServiceStatusResult {
  /** Map of service name → whether it currently has a run in progress. */
  serviceStatus: Record<string, boolean>;
  /** Whether the WebSocket is currently connected. */
  connected: boolean;
  /** Error message if the WebSocket encountered an error. */
  error: string | null;
}

/**
 * useServiceStatus connects to the CI API global events WebSocket and
 * tracks which services currently have runs in progress.
 *
 * @param apiBase - Base URL of the CI API (defaults to localhost:2526).
 */
export function useServiceStatus(
  apiBase: string = DEFAULT_API_BASE,
): UseServiceStatusResult {
  const [serviceStatus, setServiceStatus] = useState<Record<string, boolean>>(
    {},
  );
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fetchedRef = useRef(false);

  useEffect(() => {
    const wsUrl = `${apiBase.replace(/^http/, "ws")}/api/events`;
    const ws = new WebSocket(wsUrl);

    ws.addEventListener("open", () => {
      setConnected(true);

      // Fetch initial active services only once.
      if (!fetchedRef.current) {
        fetchedRef.current = true;
        fetch(`${apiBase}/api/runs/active`)
          .then((res) => res.ok ? res.json() : Promise.reject())
          .then((data) => {
            if (data?.active_services) {
              flushSync(() => {
                setServiceStatus((prev) => ({
                  ...prev,
                  ...data.active_services,
                }));
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
        const msg = JSON.parse(event.data) as ServiceEvent;

        if (msg.type === "run_started") {
          setServiceStatus((prev) => ({
            ...prev,
            [msg.service_name]: true,
          }));
        } else if (msg.type === "run_finished") {
          setServiceStatus((prev) => ({
            ...prev,
            [msg.service_name]: false,
          }));
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

  return { serviceStatus, connected, error };
}
