// Screenshot every career-portal surface at 3 widths for the GAN design eval.
// Run from career-portal/ so it resolves @playwright/test:
//   node ../gan-harness/career-portal/shoot.mjs <label>
// Output: gan-harness/career-portal/shots/<label>/<page>-<width>.png
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const __dirname = dirname(fileURLToPath(import.meta.url));
// Resolve Playwright from career-portal/node_modules regardless of cwd.
const require = createRequire(resolve(__dirname, "../../career-portal/package.json"));
const { chromium } = require("@playwright/test");
const BASE = process.env.PORTAL_URL ?? "http://localhost:3030";
const label = process.argv[2] ?? "baseline";
const OUT = resolve(__dirname, "shots", label);
mkdirSync(OUT, { recursive: true });

const PAGES = [
  ["landing", "/"],
  ["jobs", "/jobs"],
  ["job-detail", "/jobs/p01"],
  ["apply", "/jobs/p01/apply"],
  ["signup", "/signup"],
  ["login", "/login"],
  ["status", "/status"],
  ["account", "/account"],
  ["interview", "/interview"],
];
const WIDTHS = [1440, 768, 375];

const browser = await chromium.launch();
let ok = 0,
  fail = 0;
for (const w of WIDTHS) {
  const ctx = await browser.newContext({
    viewport: { width: w, height: 900 },
    deviceScaleFactor: 1,
    reducedMotion: "no-preference",
  });
  const page = await ctx.newPage();
  for (const [name, path] of PAGES) {
    try {
      await page.goto(`${BASE}${path}`, { waitUntil: "networkidle", timeout: 20000 });
      await page.waitForTimeout(1400); // let queries resolve + reveals/canvas settle
      await page.screenshot({ path: `${OUT}/${name}-${w}.png`, fullPage: true });
      ok++;
    } catch (e) {
      fail++;
      // eslint-disable-next-line no-console
      console.log(`  ✗ ${name}@${w}: ${String(e).split("\n")[0]}`);
    }
  }
  await ctx.close();
}
await browser.close();
// eslint-disable-next-line no-console
console.log(`[shoot:${label}] ${ok} ok, ${fail} fail → ${OUT}`);
