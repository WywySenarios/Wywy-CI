/** Default base URL of the CI API, configurable via env vars. */
export const DEFAULT_API_BASE = `http://${import.meta.env.PUBLIC_CI_API_HOST || "localhost"}:${import.meta.env.PUBLIC_CI_API_PORT || "2526"}`;

/** A run object as returned by the CI API. */
export interface Run {
  id: string;
  status: string;
  created_at: string;
  finished_at?: string;
  services?: RunService[];
}

/** A single service's result within a run. */
export interface RunService {
  run_id: string;
  service_name: string;
  suite: string;
  status: string;
  exit_code: number | null;
  start_time: string;
  end_time: string;
}
