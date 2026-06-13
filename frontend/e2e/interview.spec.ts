import { test, expect } from "@playwright/test";

// AI pre-interview — HR-facing invite + review (slice 2.5). Requires the stack up
// with seeded applications (same assumptions as dashboard.spec.ts).
test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://localhost:3000");
  await context.addCookies([{ name: "hr_session", value: "dev", domain: url.hostname, path: "/" }]);
});

test("candidate detail exposes the Send AI interview action", async ({ page }) => {
  await page.goto("/applications");
  await expect(page.getByRole("heading", { name: "Candidate Inbox" })).toBeVisible();

  // Open the first candidate in the inbox.
  const firstRow = page.getByRole("link", { name: /view|detail/i }).first();
  if (await firstRow.count()) {
    await firstRow.click();
  } else {
    // Fallback: click the first table row link.
    await page.locator("a[href*='/applications/']").first().click();
  }

  await expect(page.getByRole("button", { name: /Send AI interview/i })).toBeVisible();
});
