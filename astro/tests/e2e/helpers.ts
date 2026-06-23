import { expect } from "@playwright/test";
import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";

export const API_BASE = "http://localhost:2526";

const currentDir = path.dirname(fileURLToPath(import.meta.url));

/** Path to the temporary dummy E2E test script. */
const dummyScriptPath = path.resolve(
  currentDir, "..", "..", "..",
  "scripts", "tests", "e2e-test.sh",
);

/** Content of the dummy script — created and removed by createCompletedRun(). */
const dummyScript = `#!/bin/bash
# Temporary dummy script created by createCompletedRun().
set -euo pipefail
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="\${arg#*=}" ;;
  esac
done
echo "[e2e] Build started"
echo "[e2e] Running dummy tests..."
echo "[e2e] 42 passed, 0 failed"
if [ -n "$output_dir" ]; then
  cat > "$output_dir/results.jsonl" << 'RESULTS'
{"name":"dummy-e2e","status":"passed"}
RESULTS
fi
exit 0
`;

/**
 * Creates a run via the Go API and polls until it completes.
 *
 * Temporarily places a dummy test script at the path the resolver expects
 * so the run can execute and produce output. The script is removed after
 * the run completes — it never persists on disk beyond the test call.
 */
export async function createCompletedRun(): Promise<string> {
  // Write the dummy script so the resolver's path exists during the test.
  fs.writeFileSync(dummyScriptPath, dummyScript, { mode: 0o755 });

  try {
    const createRes = await fetch(`${API_BASE}/api/runs`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ services: ["ci"], suite: "e2e-test" }),
    });
    if (!createRes.ok) {
      throw new Error(
        `Failed to create run: ${createRes.status} ${await createRes.text()}`,
      );
    }

    const run = await createRes.json();
    const runId: string = run.id;

    let status = run.status;
    const deadline = Date.now() + 10000;
    while (status === "running" || status === "pending") {
      if (Date.now() > deadline) {
        throw new Error(
          `Timed out waiting for run ${runId} to complete; last status: ${status}`,
        );
      }
      await new Promise((r) => setTimeout(r, 200));
      const getRes = await fetch(`${API_BASE}/api/runs/${runId}`);
      if (!getRes.ok) {
        throw new Error(
          `Failed to poll run ${runId}: ${getRes.status} ${await getRes.text()}`,
        );
      }
      const updated = await getRes.json();
      status = updated.status;
    }

    // The run must have finished (any terminal status is valid).
    expect(["passed", "failed", "cancelled", "skipped"]).toContain(status);
    return runId;
  } finally {
    // Remove the dummy script — it must never persist beyond the test.
    try {
      fs.unlinkSync(dummyScriptPath);
    } catch {
      // Ignore cleanup errors (e.g. already removed, permissions).
    }
  }
}
