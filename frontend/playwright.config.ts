import { defineConfig, devices } from "@playwright/test";

// Assumes the frontend is served at :3000 and the Go API + stack are up.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  // Self-start the app: a prebuilt prod server in CI (run `pnpm build` first),
  // or `next dev` locally — reusing an already-running dev server.
  webServer: {
    command: process.env.CI ? "pnpm start" : "pnpm dev",
    url: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
