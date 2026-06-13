import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const BASE = "http://localhost:3000";
const OUT = "/tmp/clarity-shots";
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext();
await context.addCookies([{ name: "hr_session", value: "dev", url: BASE }]);

const routes = [
  { path: "/dashboard", name: "dashboard" },
  { path: "/analytics", name: "analytics" },
  { path: "/candidates", name: "candidates" },
];
const sizes = [
  { tag: "1440", width: 1440, height: 1200 },
  { tag: "390", width: 390, height: 1500 },
];

for (const r of routes) {
  for (const s of sizes) {
    const page = await context.newPage();
    await page.setViewportSize({ width: s.width, height: s.height });
    await page.goto(`${BASE}${r.path}`, { waitUntil: "networkidle" });
    await page.waitForTimeout(900);
    await page.screenshot({ path: `${OUT}/${r.name}-${s.tag}.png`, fullPage: false });
    console.log(`shot ${r.name}-${s.tag}`);
    await page.close();
  }
}
await browser.close();
console.log("done");
