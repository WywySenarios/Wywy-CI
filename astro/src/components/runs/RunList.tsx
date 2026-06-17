import { useEffect, useState } from "react";
import type { Status } from "@/components/ui/status-badge";
import { StatusBadge } from "@/components/ui/status-badge";
import { DEFAULT_API_BASE } from "@/lib/ci-types";
import type { Run } from "@/lib/ci-types";

/**
 * RunList fetches CI pipeline runs from the API and renders them as RunCards.
 */
export interface RunListProps {
  /** Base URL of the CI API (defaults to localhost:2526). */
  apiBase?: string;
}

export function RunList({ apiBase = DEFAULT_API_BASE }: RunListProps) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    fetch(`${apiBase}/api/runs`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json() as Promise<Run[]>;
      })
      .then((data) => {
        if (!cancelled) {
          setRuns(data);
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
    return <div data-testid="run-list-loading">Loading runs…</div>;
  }

  if (error) {
    return <div data-testid="run-list-error">Error: {error}</div>;
  }

  if (runs.length === 0) {
    return <div data-testid="run-list-empty">No runs yet.</div>;
  }

  return (
    <div data-testid="run-list" className="space-y-3">
      {runs.map((run) => (
        <a
          key={run.id}
          href={`/runs/${run.id}`}
          className="block rounded-lg transition-colors hover:bg-muted/50"
        >
          <div className="flex items-center gap-x-2 rounded-lg border p-3">
            <StatusBadge status={run.status as Status} />
            <span className="font-mono text-sm">{run.id}</span>
          </div>
        </a>
      ))}
    </div>
  );
}
