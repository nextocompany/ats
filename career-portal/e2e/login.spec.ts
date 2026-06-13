import { test, expect } from "@playwright/test";

// Login (returning user). Mock providers: LINE/Google bounce back logged in.

const SCREEN_DIR = "e2e/__screens__";

test("login page offers providers and links to signup", async ({ page }, testInfo) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "เข้าสู่ระบบ" })).toBeVisible();
  await expect(page.getByRole("button", { name: /ด้วย LINE/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "สมัครสมาชิก" })).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/login-${testInfo.project.name}.png`, fullPage: true });
});

test("LINE mock login honors the return URL", async ({ page }) => {
  await page.goto("/login?return=%2Fjobs");
  await page.getByRole("button", { name: /ด้วย LINE/ }).click();
  // After login the session provider redirects back to the jobs list.
  await expect(page.getByRole("heading", { name: "ตำแหน่งงานที่เปิดรับ" })).toBeVisible();
});

test("account page redirects to login when unauthenticated", async ({ page }) => {
  await page.goto("/account");
  await expect(page.getByRole("heading", { name: "เข้าสู่ระบบ" })).toBeVisible();
});
