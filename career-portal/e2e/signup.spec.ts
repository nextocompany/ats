import { test, expect } from "@playwright/test";

// Candidate membership signup. Assumes the Go API stack is up with mock providers
// (LINE/GOOGLE/EMAIL default to mock): LINE/Google bounce straight back with a
// session cookie; email shows the OTP step (the code is delivered by the mock
// sender — full verify needs a real/seeded code, covered by backend unit tests).

const SCREEN_DIR = "e2e/__screens__";

test("signup page offers the three providers", async ({ page }, testInfo) => {
  await page.goto("/signup");
  await expect(page.getByRole("heading", { name: "สมัครสมาชิก" })).toBeVisible();
  await expect(page.getByRole("button", { name: /ด้วย LINE/ })).toBeVisible();
  await expect(page.getByRole("button", { name: /ด้วย Google/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "ใช้อีเมล" })).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/signup-methods-${testInfo.project.name}.png`, fullPage: true });
});

test("email signup advances to the OTP code step", async ({ page }) => {
  await page.goto("/signup");
  await page.getByRole("button", { name: "ใช้อีเมล" }).click();
  await page.getByLabel("อีเมล").fill("e2e@example.com");
  await page.getByRole("button", { name: "ส่งรหัส" }).click();
  await expect(page.getByLabel("รหัสยืนยัน")).toBeVisible();
});

test("LINE mock signup logs in and continues to profile setup", async ({ page }, testInfo) => {
  await page.goto("/signup");
  await page.getByRole("button", { name: /ด้วย LINE/ }).click();
  // Back on /signup, now authenticated → step 1 (profile).
  await expect(page.getByRole("heading", { name: "กรอกข้อมูลเบื้องต้น" })).toBeVisible();
  await expect(page.getByLabel(/ชื่อ-นามสกุล/)).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/signup-profile-${testInfo.project.name}.png`, fullPage: true });
});

test("Google mock signup logs in and continues to profile setup", async ({ page }) => {
  await page.goto("/signup");
  await page.getByRole("button", { name: /ด้วย Google/ }).click();
  await expect(page.getByRole("heading", { name: "กรอกข้อมูลเบื้องต้น" })).toBeVisible();
});
