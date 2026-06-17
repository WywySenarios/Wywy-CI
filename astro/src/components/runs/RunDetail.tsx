import { useEffect, useState } from "react";
import type { Status } from "@/components/ui/status-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { DEFAULT_API_BASE } from "@/lib/ci-types";
import type { Run } from "@/lib/ci-types";
import { useRunStream } from "@/hooks/useRunStream";
import type { UseRunStreamResult } from "@/hooks/useRunStream";

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

  const {
    connected,
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

  return (
    <div data-testid="run-detail" className="space-y-4">
      <div className="flex items-center gap-x-2">
        <StatusBadge status={displayStatus as Status} />
        <span className="font-mono text-lg">{run.id}</span>
        {connected && (
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

      {logEntries.length > 0 && (
        <>
          <h3 className="text-sm font-semibold text-muted-foreground">Logs</h3>
          <div data-testid="run-detail-log-entries" className="space-y-1">
            {logEntries.map((entry, i) => (
              <div key={i} className="font-mono text-xs">
                {entry.content}
              </div>
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
