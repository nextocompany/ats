// GAN prototype screenshotter. Shoots every careers + admin page at 3 widths
// into gan-harness/shots/<label>/. Serve the prototype first:
//   (cd gan-harness/prototype && python3 -m http.server 4173)
// Usage: node shoot-proto.mjs <label> [baseURL]
import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const label = process.argv[2] ?? "shot";
const baseURL = process.argv[3] ?? "http://localhost:4173";
const outDir = new URL(`./shots/${label}/`, import.meta.url).pathname;
mkdirSync(outDir, { recursive: true });

const PAGES = [
  ["home", "/index.html"],
  ["careers-home", "/careers/index.html"],
  ["jobs", "/careers/jobs.html"],
  ["job-detail", "/careers/job.html"],
  ["apply", "/careers/apply.html"],
  ["about", "/careers/about.html"],
  ["admin-dashboard", "/admin/dashboard.html"],
  ["admin-jobs", "/admin/jobs.html"],
  ["admin-job-edit", "/admin/job-edit.html"],
  ["admin-applicants", "/admin/applicants.html"],
  ["admin-applicant", "/admin/applicant.html"],
  ["admin-users", "/admin/users.html"],
];
const WIDTHS = [1440, 768, 320];

const browser = await chromium.launch();
const ctx = await browser.newContext();
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));

for (const [name, route] of PAGES) {
  for (const width of WIDTHS) {
    await page.setViewportSize({ width, height: 900 });
    try {
      const resp = await page.goto(`${baseURL}${route}`, { waitUntil: "networkidle", timeout: 20000 });
      if (resp && resp.status() >= 400) { console.log(`MISSING ${name} (${resp.status()})`); continue; }
    } catch {
      try { await page.goto(`${baseURL}${route}`, { waitUntil: "domcontentloaded", timeout: 20000 }); }
      catch { console.log(`SKIP ${name} (no route)`); continue; }
    }
    await page.waitForTimeout(500);
    await page.screenshot({ path: `${outDir}${name}-${width}.png`, fullPage: true });
    console.log(`shot ${name}-${width}`);
  }
}

await browser.close();
console.log(errors.length ? "CONSOLE_ERRORS:\n  " + [...new Set(errors)].slice(0, 20).join("\n  ") : "CONSOLE_ERRORS: none");
