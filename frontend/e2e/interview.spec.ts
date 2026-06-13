import { test, expect } from "@playwright/test";

// AI pre-interview — HR-facing invite + review (slice 2.5). Requires the stack up
// with seeded applications (same assumptions as dashboard.spec.ts). Mirrors that
// spec's proven inbox→detail navigation rather than ad-hoc selectors.
test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://localhost:3000");
  await context.addCookies([{ name: "hr_session", value: "dev", domain: url.hostname, path: "/" }]);
});

test("candidate detail exposes the Send AI interview action", async ({ page }) => {
  await page.goto("/applications?status=scored");
  await expect(page.getByRole("heading", { name: "Candidate Inbox" })).toBeVisible();

  const firstLink = page.locator('a[href^="/applications/"]').first();
  if (!(await firstLink.count())) {
    test.skip(true, "no scored applications seeded in this environment");
    return;
  }

  await firstLink.click();
  // Wait for the detail's AI summary pane (the proven anchor used by dashboard.spec)
  // before asserting the action button lives within it.
  await expect(page.getByRole("complementary", { name: /AI summary/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /Send AI interview/i })).toBeVisible();
});
