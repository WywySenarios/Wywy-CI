import { test, expect } from "@playwright/test";
import { createCompletedRun } from "./helpers";

/**
 * E2E happy path test for viewing logs on a completed run.
 *
 * Exercises the real Go backend (port 2526) and Astro frontend (port 3001):
 *   1. Creates a real run via POST /api/runs
 *   2. Polls until the run completes
 *   3. Navigates to the run detail page
 *   4. Asserts log output is visible and non-empty
 *
 * Prerequisite: Both servers must be running (e.g. via `run.sh ci playwright`).
 */
test.describe("Run detail logs", () => {
  test("displays log output for a completed run", async ({ page }) => {
    const runId = await createCompletedRun();

    await page.goto(`/runs?id=${runId}`);

    const logEntries = page.getByTestId("run-detail-log-entries");
    await expect(logEntries).toBeVisible({ timeout: 15000 });

    const text = await logEntries.textContent();
    expect(text).not.toBeNull();
    expect(text!.trim().length).toBeGreaterThan(0);
  });

  test("shows exit code only for the tested service, not for all services", async ({ page }) => {
    const runId = await createCompletedRun();

    await page.goto(`/runs?id=${runId}`);

    // Wait for the run detail to fully render.
    await expect(page.getByTestId("run-detail")).toBeVisible({ timeout: 15000 });

    // The run was created with a SINGLE service ("ci").
    // There MUST be exactly one exit code element on the page — the tested
    // service's exit code. If ALL services' exit codes appear, this fails.
    // Use a regex to match any data-testid starting with "service-exit-code-".
    const exitCodeElements = page.getByTestId(/^service-exit-code-/);
    await expect(exitCodeElements).toHaveCount(1, { timeout: 15000 });

    // The single exit code element must correspond to the tested service "ci".
    await expect(page.getByTestId("service-exit-code-ci")).toBeVisible();
  });
});
