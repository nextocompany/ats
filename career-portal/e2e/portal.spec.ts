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

test("full account-first flow: signup → quick-apply → status", async ({ page }, testInfo) => {
  await page.goto("/jobs");
  await expect(page.getByRole("heading", { name: "ตำแหน่งงานที่เปิดรับ" })).toBeVisible();

  // The apply flow needs seeded open positions. Skip gracefully if none.
  // Match real job-detail links (/jobs/<id>), not the nav/footer "/jobs" link.
  const firstJob = page.locator('a[href^="/jobs/"]').first();
  if ((await firstJob.count()) === 0) {
    test.skip(true, "no seeded open positions — run make up/migrate-up/seed first");
  }
  const href = await firstJob.getAttribute("href");

  // 1) Sign up fully via LINE: login → profile (+ PDPA consent) → save resume.
  await page.goto("/signup");
  await page.getByRole("button", { name: /ด้วย LINE/ }).click();
  await expect(page.getByRole("heading", { name: "กรอกข้อมูลเบื้องต้น" })).toBeVisible();
  await page.getByLabel(/ชื่อ-นามสกุล/).fill("ทดสอบ ระบบ");
  await page.getByRole("checkbox").check(); // PDPA consent (required to proceed)
  await page.getByRole("button", { name: "ถัดไป" }).click();
  await page.getByLabel(/อัปโหลดเรซูเม่/).setInputFiles({
    name: "resume.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4 test resume"),
  });
  await page.getByRole("button", { name: "เสร็จสิ้นการสมัคร" }).click();
  // Wait for the upload to finish + signup to complete (redirects to /jobs) before
  // navigating away — otherwise the resume POST is aborted mid-flight.
  await page.waitForURL(/\/jobs(\?|$)/, { timeout: 15_000 });

  // 2) Account-first apply with the saved resume (one tap).
  await page.goto(`${href}/apply`);
  const quickBtn = page.getByRole("button", { name: "สมัครด้วยเรซูเม่ที่บันทึกไว้" });
  await expect(quickBtn).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/apply-review-${testInfo.project.name}.png`, fullPage: true });
  await quickBtn.click();

  // 3) Success — a status token is shown and resolves on the status page.
  await expect(page.getByRole("heading", { name: "ส่งใบสมัครเรียบร้อยแล้ว" })).toBeVisible();
  const token = (await page.locator("#status-token").textContent())?.trim() ?? "";
  expect(token.length).toBeGreaterThan(0);
  await page.screenshot({ path: `${SCREEN_DIR}/apply-success-${testInfo.project.name}.png`, fullPage: true });

  await page.goto(`/status?token=${encodeURIComponent(token)}`);
  await expect(page.getByText(/วันที่สมัคร/)).toBeVisible();
});
