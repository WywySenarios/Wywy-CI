import { useEffect, useState } from "react";
import { RunDetail } from "@/components/runs/RunDetail";
import { RunList } from "@/components/runs/RunList";

/**
 * Returns the `id` query parameter from the browser URL, or null.
 * On the server, returns null (no `window` available).
 */
function getIdFromUrl(): string | null {
  if (typeof window === "undefined") return null;
  const params = new URLSearchParams(window.location.search);
  return params.get("id");
}

/**
 * RunsPage is the top-level page component for /runs.
 * It conditionally renders the run list or a single run detail
 * based on whether an `id` query parameter is present.
 *
 * During SSR (Astro static build) the `id` prop is always null
 * because query params are not available. On the client, we re-read
 * the query parameter from `window.location.search` after hydration.
 */
export interface RunsPageProps {
  /** The run ID from the query string, or null/undefined to show the list. */
  id?: string | null;
  /** Base URL of the CI API (defaults to localhost:2526). */
  apiBase?: string;
}

export function RunsPage({ id, apiBase }: RunsPageProps) {
  // Use the prop during SSR, then read from the URL on the client
  // when the server couldn't provide one (static build has no query params).
  const [resolvedId, setResolvedId] = useState<string | null | undefined>(id);

  useEffect(() => {
    if (!id) {
      setResolvedId(getIdFromUrl());
    }
  }, [id]);

  if (resolvedId) {
    return (
      <>
        <a
          href="/runs"
          className="mb-4 inline-block text-sm text-muted-foreground hover:underline"
        >
          &larr; Back to runs
        </a>
        <RunDetail id={resolvedId} apiBase={apiBase} />
      </>
    );
  }

  return (
    <>
      <h1 className="text-2xl font-bold mb-6">Test Runs</h1>
      <RunList apiBase={apiBase} />
    </>
  );
}
