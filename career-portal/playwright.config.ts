import { defineConfig } from "@playwright/test";

// The portal runs at :3001; the Go API + stack must be up (make up/migrate/seed)
// for the apply/status flows. Tests target mobile viewports (primary in-LINE sizes).
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3001",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  // 320 (smallest), 375 (typical phone), 768 (tablet) — the documented breakpoints.
  // All chromium (the only browser cached on this machine); set the device frame
  // via explicit viewport rather than a WebKit device profile.
  projects: [
    { name: "mobile-320", use: { browserName: "chromium", viewport: { width: 320, height: 720 }, isMobile: true, hasTouch: true } },
    { name: "mobile-375", use: { browserName: "chromium", viewport: { width: 375, height: 812 }, isMobile: true, hasTouch: true } },
    { name: "tablet-768", use: { browserName: "chromium", viewport: { width: 768, height: 1024 }, isMobile: true, hasTouch: true } },
  ],
  // Self-start the portal: prod `pnpm start` (-p 3001) in CI so the Serwist service
  // worker is generated (pwa.spec exercises it); `next dev` locally with reuse.
  webServer: {
    command: process.env.CI ? "pnpm start" : "pnpm dev",
    url: process.env.E2E_BASE_URL ?? "http://localhost:3001",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
