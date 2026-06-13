import { test, expect } from "@playwright/test";

// These flows assume the Go API + stack are up (make up / migrate-up / seed) so
// /public/positions returns seeded open positions. The apply flow uses the dev
// LINE stub (any non-empty X-LINE-IdToken is accepted by the mock verifier).

const SCREEN_DIR = "e2e/__screens__";

test("jobs list renders with a heading", async ({ page }, testInfo) => {
  await page.goto("/jobs");
  await expect(page.getByRole("heading", { name: "ตำแหน่งงานที่เปิดรับ" })).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/jobs-${testInfo.project.name}.png`, fullPage: true });
});

test("status page accepts a token and shows a not-found message for an unknown one", async ({ page }, testInfo) => {
  await page.goto("/status");
  await expect(page.getByRole("heading", { name: "ตรวจสอบสถานะใบสมัคร" })).toBeVisible();
  await page.getByLabel("รหัสติดตาม").fill("definitely-not-a-real-token");
  await page.getByRole("button", { name: "ตรวจสอบ" }).click();
  await expect(page.getByText(/ไม่พบใบสมัคร/)).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/status-${testInfo.project.name}.png`, fullPage: true });
});

test("account-first apply returns a status token, which resolves on the status page", async ({ page }, testInfo) => {
  await page.goto("/jobs");
  await expect(page.getByRole("heading", { name: "ตำแหน่งงานที่เปิดรับ" })).toBeVisible();

  // The list and apply flow need seeded open positions. Skip gracefully if none.
  const firstJob = page.locator("ul li a").first();
  if ((await firstJob.count()) === 0) {
    test.skip(true, "no seeded open positions — run make up/migrate-up/seed first");
  }
  await firstJob.click();

  // Detail → apply. Unauthenticated, so the page redirects to /login.
  await page.getByRole("link", { name: "สมัครงาน" }).click();
  await expect(page.getByRole("heading", { name: "เข้าสู่ระบบ" })).toBeVisible();

  // LINE mock login → backend creates an account + session cookie → bounce back
  // to the apply page (now account-first).
  await page.getByRole("button", { name: /ด้วย LINE/ }).click();

  // Fresh account has no saved resume → use the edit/upload path.
  await expect(page.getByRole("heading", { name: /สมัครตำแหน่ง|สมัครงาน/ })).toBeVisible();
  await page.getByRole("button", { name: /กรอกข้อมูล|แก้ไขข้อมูล/ }).click();

  await page.getByLabel(/ชื่อ-นามสกุล/).fill("ทดสอบ ระบบ");
  await page.getByLabel(/อัปโหลดเรซูเม่/).setInputFiles({
    name: "resume.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4 test resume"),
  });
  await page.screenshot({ path: `${SCREEN_DIR}/apply-form-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "ส่งใบสมัคร" }).click();

  // Success — a status token is shown.
  await expect(page.getByRole("heading", { name: "ส่งใบสมัครเรียบร้อยแล้ว" })).toBeVisible();
  const token = (await page.locator("#status-token").textContent())?.trim() ?? "";
  expect(token.length).toBeGreaterThan(0);
  await page.screenshot({ path: `${SCREEN_DIR}/apply-success-${testInfo.project.name}.png`, fullPage: true });

  // The token resolves on the status page.
  await page.goto(`/status?token=${encodeURIComponent(token)}`);
  await expect(page.getByText(/วันที่สมัคร/)).toBeVisible();
});
