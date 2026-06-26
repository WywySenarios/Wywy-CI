import { useEffect, useState } from "react";
import type { Status } from "@/components/ui/status-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { DEFAULT_API_BASE } from "@/lib/ci-types";
import type { Run, RunService } from "@/lib/ci-types";
import { useRunStream } from "@/hooks/useRunStream";
import type { UseRunStreamResult } from "@/hooks/useRunStream";
import { ansiToHtml } from "@/lib/ansi";

/** A log entry from the REST API (as opposed to the live WebSocket stream). */
interface HistoricalLogEntry {
  content: string;
}

/**
 * RunDetail fetches a single CI pipeline run from the API and renders its details.
 */
export interface RunDetailProps {
  /** The run ID to fetch. */
  id: string;
  /** Base URL of the CI API (defaults to localhost:2526). */
  apiBase?: string;
}

export function RunDetail({ id, apiBase = DEFAULT_API_BASE }: RunDetailProps) {
  const [run, setRun] = useState<Run | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [historicalLogEntries, setHistoricalLogEntries] = useState<
    HistoricalLogEntry[]
  >([]);

  const {
    connected,
    done,
    finalStatus,
    logEntries,
    error: streamError,
  }: UseRunStreamResult = useRunStream(id, apiBase);

  useEffect(() => {
    let cancelled = false;

    fetch(`${apiBase}/api/runs/${id}`)
      .then((res) => {
        if (res.status === 404) {
          if (!cancelled) {
            setNotFound(true);
            setLoading(false);
          }
          return null;
        }
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json() as Promise<Run>;
      })
      .then((data) => {
        if (!cancelled && data) {
          setRun(data);
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
  }, [id, apiBase]);

  // Fetch historical logs from REST when the WebSocket delivers "done" without
  // any live log entries (e.g. for runs that completed before the client connected).
  useEffect(() => {
    if (!done || logEntries.length > 0) return;

    let cancelled = false;
    fetch(`${apiBase}/api/runs/${id}/logs`)
      .then((res) => (res.ok ? res.json() : Promise.resolve(null)))
      .then((entries: unknown) => {
        if (!cancelled && Array.isArray(entries)) {
          setHistoricalLogEntries(entries as HistoricalLogEntry[]);
        }
      })
      .catch((_err: unknown) => {
        // Non-critical — historical logs are best-effort.
      });

    return () => {
      cancelled = true;
    };
  }, [done, logEntries, id, apiBase]);

  if (loading) {
    return <div data-testid="run-detail-loading">Loading run…</div>;
  }

  if (notFound) {
    return <div data-testid="run-detail-not-found">Run not found.</div>;
  }

  if (error) {
    return <div data-testid="run-detail-error">Error: {error}</div>;
  }

  if (!run) {
    return null;
  }

  // WebSocket finalStatus overrides the initially fetched status.
  const displayStatus = finalStatus ?? run.status;

  // Merge live WS log entries with any fetched historical logs.
  const allLogEntries = [...historicalLogEntries, ...logEntries];

  return (
    <div data-testid="run-detail" className="space-y-4">
      <div className="flex items-center gap-x-2">
        <StatusBadge status={displayStatus as Status} />
        <span className="font-mono text-lg">{run.id}</span>
        {connected && !run.finished_at && (
          <span
            data-testid="run-detail-live-indicator"
            className="ml-auto text-xs text-green-500"
          >
            ● Live
          </span>
        )}
      </div>
      <dl className="space-y-2">
        <div>
          <dt className="text-sm text-muted-foreground">Created</dt>
          <dd data-testid="run-detail-created">{run.created_at}</dd>
        </div>
        {run.finished_at && (
          <div>
            <dt className="text-sm text-muted-foreground">Finished</dt>
            <dd data-testid="run-detail-finished">{run.finished_at}</dd>
          </div>
        )}
      </dl>

      {run.services && run.services.length > 0 && (
        <section data-testid="run-detail-services">
          <h3 className="text-sm font-semibold text-muted-foreground">Services</h3>
          <div className="space-y-1">
            {run.services.map((service: RunService) => (
              <div
                key={service.service_name}
                className="flex items-center gap-x-2 text-sm"
              >
                <StatusBadge status={service.status as Status} />
                <span className="font-mono">{service.service_name}</span>
                <span
                  data-testid={`service-counts-${service.service_name}`}
                  className="text-xs text-muted-foreground"
                >
                  {service.passed} passed, {service.failed} failed, {service.skipped} skipped
                </span>
                {service.exit_code !== null && (
                  <span
                    data-testid={`service-exit-code-${service.service_name}`}
                    className="text-xs text-muted-foreground"
                  >
                    exit: {service.exit_code}
                  </span>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {allLogEntries.length > 0 && (
        <>
          <h3 className="text-sm font-semibold text-muted-foreground">Logs</h3>
          <div data-testid="run-detail-log-entries" className="space-y-1">
            {allLogEntries.map((entry, i) => (
              <div
                key={i}
                className="font-mono text-xs"
                dangerouslySetInnerHTML={{ __html: ansiToHtml(entry.content ?? "") }}
              />
            ))}
          </div>
        </>
      )}

      {streamError && (
        <div className="text-xs text-red-500">{streamError}</div>
      )}
    </div>
  );
}
