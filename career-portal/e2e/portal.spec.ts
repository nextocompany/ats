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

test("apply flow returns a status token, which resolves on the status page", async ({ page }, testInfo) => {
  await page.goto("/jobs");
  await expect(page.getByRole("heading", { name: "ตำแหน่งงานที่เปิดรับ" })).toBeVisible();

  // The list and apply flow need seeded open positions. Skip gracefully if none.
  const firstJob = page.locator("ul li a").first();
  if ((await firstJob.count()) === 0) {
    test.skip(true, "no seeded open positions — run make up/migrate-up/seed first");
  }
  await firstJob.click();

  // Detail → apply
  await page.getByRole("link", { name: "สมัครงาน" }).click();

  // Step 1: consent (required)
  await expect(page.getByRole("heading", { name: "ความยินยอมในการใช้ข้อมูล" })).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/apply-consent-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("checkbox").click();
  await page.getByRole("button", { name: "ถัดไป" }).click();

  // Step 2: details (name required)
  await page.getByLabel(/ชื่อ-นามสกุล/).fill("ทดสอบ ระบบ");
  await page.screenshot({ path: `${SCREEN_DIR}/apply-details-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "ถัดไป" }).click();

  // Step 3: resume + LINE login → submit
  await page.getByLabel(/อัปโหลดเรซูเม่/).setInputFiles({
    name: "resume.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4 test resume"),
  });
  await page.getByRole("button", { name: "เข้าสู่ระบบด้วย LINE" }).click();
  await expect(page.getByText("เชื่อมต่อ LINE แล้ว")).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/apply-resume-${testInfo.project.name}.png`, fullPage: true });
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
