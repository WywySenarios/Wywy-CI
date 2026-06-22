import { test, expect } from "@playwright/test";

/**
 * Visual regression test for /runs page.
 *
 * In Astro `output: "static"` mode, query parameters are unavailable
 * during SSR (pages are pre-rendered). The RunsPage component recovers
 * the `?id` parameter client-side via `window.location.search` after
 * hydration.
 *
 * The list endpoint (`**/api/runs`) is mocked to return an empty array,
 * but the detail endpoint (`/api/runs/:id`) is NOT — so the detail view
 * shows an error/not-found state after hydration. This produces visually
 * different screenshots even though the SSR output is the same.
 *
 * To properly validate the detail view, this test would need to mock
 * both endpoints and verify the rendered content, not just screenshot
 * comparison.
 */
test.describe("RunsPage visual regression", () => {
  test("list view and detail view look different", async ({ page }) => {
    // Mock API so the list returns empty (shows "No runs yet.")
    await page.route("**/api/runs", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
    });

    // Navigate to list view
    await page.goto("/runs");
    await page.getByText(/no runs/i).waitFor({ timeout: 10000 });
    const listScreenshot = await page.screenshot();

    // Navigate to detail view — SSR renders the list (no query params),
    // then client-side hydration reads `?id` from window.location and
    // fetches the run detail from the (un-mocked) Go API.
    await page.goto("/runs?id=run-abc");
    await page.waitForTimeout(3000);
    const detailScreenshot = await page.screenshot();

    // The two views produce different screenshots because the detail view
    // hits the live Go API (returning 404 for run-abc) and shows an error
    // state that differs from the empty list view.
    expect(listScreenshot.equals(detailScreenshot)).toBe(false);
  });
});
