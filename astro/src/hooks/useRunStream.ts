import { useEffect, useState } from "react";
import { DEFAULT_API_BASE } from "@/lib/ci-types";

/** A log event received over the run stream WebSocket. */
export interface LogMessage {
  type: "log" | "done";
  run_id: string;
  service_name?: string;
  level?: string;
  content?: string;
  status?: string;
}

/** State returned by useRunStream. */
export interface UseRunStreamResult {
  /** Whether the WebSocket is currently connected. */
  connected: boolean;
  /** Whether a "done" message has been received. */
  done: boolean;
  /** The final run status, if known. */
  finalStatus: string | null;
  /** All log messages received so far (including the "done" message). */
  logEntries: LogMessage[];
  /** Error message if the WebSocket encountered an error. */
  error: string | null;
}

/**
 * useRunStream connects to the CI API WebSocket for a specific run and
 * streams log entries and completion status.
 *
 * @param runId - The run ID to subscribe to.
 * @param apiBase - Base URL of the CI API (defaults to localhost:2526).
 */
export function useRunStream(
  runId: string,
  apiBase: string = DEFAULT_API_BASE,
): UseRunStreamResult {
  const [connected, setConnected] = useState(false);
  const [done, setDone] = useState(false);
  const [finalStatus, setFinalStatus] = useState<string | null>(null);
  const [logEntries, setLogEntries] = useState<LogMessage[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const wsUrl = `${apiBase.replace(/^http/, "ws")}/api/runs/${runId}/stream`;
    const ws = new WebSocket(wsUrl);

    ws.addEventListener("open", () => {
      setConnected(true);
    });

    ws.addEventListener("message", (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data) as LogMessage;
        if (msg.type === "done") {
          setDone(true);
          setFinalStatus(msg.status ?? null);
        }
        setLogEntries((prev) => [...prev, msg]);
      } catch {
        // ignore malformed messages
      }
    });

    ws.addEventListener("error", () => {
      setError("WebSocket connection error");
    });

    return () => {
      ws.close(1000, "component unmounted");
    };
  }, [runId, apiBase]);

  return { connected, done, finalStatus, logEntries, error };
}
