import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const BASE = "http://localhost:3000";
const OUT = "/tmp/inbox-shots";
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext();
// Dev-mode session gate: middleware lets us through with hr_session=dev.
await context.addCookies([
  { name: "hr_session", value: "dev", url: BASE },
]);

const shots = [
  { name: "desktop-1440", width: 1440, height: 1100 },
  { name: "desktop-1024", width: 1024, height: 900 },
  { name: "mobile-390", width: 390, height: 1400 },
];

for (const s of shots) {
  const page = await context.newPage();
  await page.setViewportSize({ width: s.width, height: s.height });
  await page.goto(`${BASE}/applications`, { waitUntil: "networkidle" });
  // Wait for the first candidate name to render (data loaded).
  await page.waitForSelector("table, ul li a", { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${OUT}/${s.name}.png`, fullPage: false });
  console.log(`shot ${s.name} -> ${OUT}/${s.name}.png`);
  await page.close();
}

await browser.close();
console.log("done");
