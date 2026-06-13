import { test, expect } from "@playwright/test";

// AI pre-interview chat (slice 2.5). The full conversational flow needs the Go
// API + a seeded interview session (HR invite). The guard test below needs only
// the portal server and is always deterministic.
test.describe("AI pre-interview", () => {
  test("shows a guard when no token is supplied", async ({ page }) => {
    await page.goto("/interview");
    await expect(page.getByText("ไม่พบรหัสสัมภาษณ์")).toBeVisible();
  });

  test("shows a not-found state for an unknown token", async ({ page }) => {
    // Token lives in the URL fragment (read client-side), not the query string.
    await page.goto("/interview#token=does-not-exist");
    // The backend returns 404 → the chat renders its not-found card.
    await expect(page.getByText("ไม่พบการสัมภาษณ์นี้")).toBeVisible();
  });

  // Full flow: requires `make up && migrate && seed` plus an invited session.
  // Set INTERVIEW_TOKEN to a started session's access token to exercise it.
  test("candidate can answer until the interview completes", async ({ page }) => {
    const token = process.env.INTERVIEW_TOKEN;
    test.skip(!token, "set INTERVIEW_TOKEN to a seeded session to run the full flow");

    await page.goto(`/interview#token=${token}`);
    await expect(page.getByRole("heading", { name: /พูดคุยกับ HR/ })).toBeVisible();
    // The first AI question should appear.
    await expect(page.locator("li", { hasText: /\?/ }).first()).toBeVisible();

    // Answer up to a generous cap; the mock interviewer ends within a few turns.
    for (let i = 0; i < 8; i++) {
      const done = await page.getByText("การสัมภาษณ์เสร็จสิ้นแล้ว").isVisible().catch(() => false);
      if (done) break;
      await page.getByLabel("คำตอบ").fill("นี่คือคำตอบของผม");
      await page.getByRole("button", { name: "ส่ง" }).click();
      await page.waitForResponse((r) => r.url().includes("/interview/") && r.url().includes("/message"));
    }
    await expect(page.getByText("การสัมภาษณ์เสร็จสิ้นแล้ว")).toBeVisible();
  });
});
