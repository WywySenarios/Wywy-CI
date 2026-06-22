import { expect } from "@playwright/test";

export const API_BASE = "http://localhost:2526";

/**
 * Creates a run via the Go API and polls until it completes.
 *
 * The runner on the Go server executes either a placeholder echo or
 * a real test script depending on whether `CI_SCRIPT_OVERRIDE` was set
 * at server start (as it is in `run.sh ci playwright`).
 *
 * Valid service names (from services.txt): cache, website, backup,
 * master-database, agentic, ci.
 */
export async function createCompletedRun(): Promise<string> {
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

  expect(status).toBe("passed");
  return runId;
}
