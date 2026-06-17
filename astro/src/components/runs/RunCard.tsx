import type { Status } from "@/components/ui/status-badge";
import { StatusBadge } from "@/components/ui/status-badge";

/**
 * RunCard displays a CI pipeline run with its status badge.
 *
 * @param id — The run identifier.
 * @param status — The CI run status.
 */
export interface RunCardProps {
  id: string;
  status: Status;
}

export function RunCard({ id, status }: RunCardProps) {
  return (
    <div className="flex items-center gap-x-2 rounded-lg border p-3">
      <StatusBadge status={status} />
      <span className="font-mono text-sm">{id}</span>
    </div>
  );
}
