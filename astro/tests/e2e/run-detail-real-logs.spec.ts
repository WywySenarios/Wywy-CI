import { test, expect } from "@playwright/test";
import { createCompletedRun } from "./helpers";

/**
 * E2E test that validates real (non-placeholder) test output is captured
 * and displayed correctly on the run detail page.
 *
 * createCompletedRun() writes a temporary test script to disk, triggers it
 * via the Go runner, and the runner's stdout capture persists the output.
 * This test confirms the output is:
 *   - Not empty
 *   - Not the placeholder string "test output for ci"
 *   - Contains the expected real output from the temporary script
 *
 * Prerequisite: Both servers running via `run.sh ci playwright`.
 */
test.describe("Run detail real logs", () => {
  test("displays real script output (not placeholder)", async ({ page }) => {
    const runId = await createCompletedRun();

    await page.goto(`/runs?id=${runId}`);

    const logEntries = page.getByTestId("run-detail-log-entries");
    await expect(logEntries).toBeVisible({ timeout: 15000 });

    // Read the full text content of the log entries.
    const text = await logEntries.textContent();

    // Must not be empty.
    expect(text).not.toBeNull();
    expect(text!.trim().length).toBeGreaterThan(0);

    // Must not be the placeholder output.
    expect(text).not.toContain("test output for ci");

    // Must contain the real output from the temporary test script.
    expect(text).toContain("[e2e] Build started");
    expect(text).toContain("[e2e] 42 passed, 0 failed");
  });
});
