/** Default base URL of the CI API. */
export const DEFAULT_API_BASE = "http://localhost:2526";

/** A run object as returned by the CI API. */
export interface Run {
  id: string;
  status: string;
  created_at: string;
  finished_at?: string;
}
