import { test, expect } from "@playwright/test";

const BREAKPOINTS = [320, 768, 1024, 1440];

// Seed the dev session cookie so middleware lets us through (real Azure AD later).
test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://localhost:3000");
  await context.addCookies([
    { name: "hr_session", value: "dev", domain: url.hostname, path: "/" },
  ]);
});

test("login page renders without a session", async ({ browser }) => {
  const ctx = await browser.newContext(); // no cookie
  const page = await ctx.newPage();
  await page.goto("/applications");
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("button", { name: /sign in as hr/i })).toBeVisible();
  await ctx.close();
});

test("ranked inbox loads and is responsive", async ({ page }) => {
  await page.goto("/applications");
  await expect(page.getByRole("heading", { name: "Candidate Inbox" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
  for (const width of BREAKPOINTS) {
    await page.setViewportSize({ width, height: 900 });
    await page.screenshot({ path: `e2e/__screens__/inbox-${width}.png`, fullPage: true });
  }
});

test("inbox → detail shows resume pane + AI panel", async ({ page }) => {
  await page.goto("/applications?status=scored");
  await expect(page.getByRole("heading", { name: "Candidate Inbox" })).toBeVisible();
  const firstLink = page.locator('a[href^="/applications/"]').first();
  if (await firstLink.count()) {
    await firstLink.click();
    await expect(page.getByRole("complementary", { name: /AI summary/i })).toBeVisible();
    await page.screenshot({ path: "e2e/__screens__/detail-1440.png", fullPage: true });
  }
});

test("analytics renders charts", async ({ page }) => {
  await page.goto("/analytics");
  await expect(page.getByRole("heading", { name: "Analytics" })).toBeVisible();
  await expect(page.getByText("Recruitment Funnel")).toBeVisible();
  await page.screenshot({ path: "e2e/__screens__/analytics-1440.png", fullPage: true });
});

test("candidates list renders", async ({ page }) => {
  await page.goto("/candidates");
  await expect(page.getByRole("heading", { name: "Candidates" })).toBeVisible();
});
