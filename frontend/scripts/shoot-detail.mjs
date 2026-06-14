import { chromium } from "@playwright/test";

const BASE = "http://localhost:3000";
const browser = await chromium.launch();
const context = await browser.newContext({ deviceScaleFactor: 2 });
await context.addCookies([{ name: "hr_session", value: "dev", url: BASE }]);

const page = await context.newPage();
await page.setViewportSize({ width: 1280, height: 1000 });
await page.goto(`${BASE}/applications`, { waitUntil: "networkidle" });
await page.waitForTimeout(1000);

// Crisp crop of the desktop table so name/fit/requirements detail is legible.
const table = page.locator("table").first();
await table.screenshot({ path: "/tmp/inbox-shots/desktop-table-hi.png" });
console.log("table crop saved");

await browser.close();
