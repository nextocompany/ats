// GAN screenshot helper. Seeds the dev HR session cookie, then screenshots the
// requested routes at the given breakpoints into gan-harness/shots/<label>/.
// Usage: node shoot.mjs <label> [baseURL]
//   label   - subfolder name (e.g. "baseline", "iter-1")
//   baseURL - default http://localhost:3003
import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const label = process.argv[2] ?? "shot";
const baseURL = process.argv[3] ?? "http://localhost:3003";
const outDir = new URL(`./shots/${label}/`, import.meta.url).pathname;
mkdirSync(outDir, { recursive: true });

const ROUTES = [
  ["overview", "/dashboard"],
  ["inbox", "/applications"],
  ["candidates", "/candidates"],
  ["analytics", "/analytics"],
  ["search", "/search"],
];
const WIDTHS = [1440, 768, 320];

const browser = await chromium.launch();
const ctx = await browser.newContext();
await ctx.addCookies([
  { name: "hr_session", value: "dev", domain: "localhost", path: "/" },
]);
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => {
  if (m.type() === "error") errors.push(m.text());
});

for (const [name, route] of ROUTES) {
  for (const width of WIDTHS) {
    await page.setViewportSize({ width, height: 900 });
    try {
      await page.goto(`${baseURL}${route}`, { waitUntil: "networkidle", timeout: 20000 });
    } catch {
      await page.goto(`${baseURL}${route}`, { waitUntil: "domcontentloaded", timeout: 20000 });
    }
    await page.waitForTimeout(700); // let charts/data settle
    await page.screenshot({ path: `${outDir}${name}-${width}.png`, fullPage: true });
    console.log(`shot ${name}-${width}`);
  }
}

await browser.close();
if (errors.length) {
  console.log("CONSOLE_ERRORS:");
  for (const e of [...new Set(errors)].slice(0, 20)) console.log("  " + e);
} else {
  console.log("CONSOLE_ERRORS: none");
}
