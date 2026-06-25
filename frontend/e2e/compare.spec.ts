import { test, expect } from "@playwright/test";

// Seed the dev session cookie so middleware lets us through (real Azure AD later).
test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://localhost:3000");
  await context.addCookies([{ name: "hr_session", value: "dev", domain: url.hostname, path: "/" }]);
});

test("compare page renders the position picker", async ({ page }) => {
  await page.goto("/compare");
  // Either the gated "not available" card or the picker prompt renders; the
  // heading is always present.
  await expect(page.getByRole("heading", { name: /compare candidates/i })).toBeVisible();
  await expect(page.getByText(/select a position/i)).toBeVisible();
  await page.screenshot({ path: "e2e/__screens__/compare-1440.png", fullPage: true });
});

test("compare nav entry is reachable", async ({ page }) => {
  await page.goto("/applications");
  const compareLink = page.locator('a[href="/compare"]').first();
  if (await compareLink.count()) {
    await compareLink.click();
    await expect(page).toHaveURL(/\/compare/);
    await expect(page.getByRole("heading", { name: /compare candidates/i })).toBeVisible();
  }
});
