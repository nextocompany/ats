import { expect, test } from "@playwright/test";

// PWA installability + offline-shell coverage. Most assertions (manifest, icons,
// /offline) hold in any build. The service-worker registration only happens in a
// production build (`next dev` disables Serwist), so that assertion self-guards on
// whether the generated /sw.js is actually served — run against `pnpm build &&
// pnpm start` to exercise it. None of these flows need the Go API stack.

const SCREEN_DIR = "e2e/__screens__";

test("manifest is linked and served with the expected PWA fields", async ({ page }) => {
  await page.goto("/jobs");

  const manifestLink = page.locator('link[rel="manifest"]');
  await expect(manifestLink).toHaveAttribute("href", /manifest\.webmanifest/);

  const res = await page.request.get("/manifest.webmanifest");
  expect(res.status()).toBe(200);

  const manifest = (await res.json()) as {
    name: string;
    short_name: string;
    start_url: string;
    display: string;
    theme_color: string;
    icons: Array<{ src: string; sizes: string; purpose?: string }>;
  };
  expect(manifest.name).toBe("ร่วมงานกับเรา");
  expect(manifest.start_url).toBe("/jobs");
  expect(manifest.display).toBe("standalone");
  expect(manifest.theme_color).toBe("#0B47B8");
  expect(manifest.icons.length).toBeGreaterThanOrEqual(2);
  expect(manifest.icons.some((i) => i.purpose === "maskable")).toBe(true);
});

test("PWA icons and apple-touch metadata resolve", async ({ page }) => {
  await page.goto("/jobs");

  // Apple-touch icon + manifest are wired through layout metadata.
  await expect(page.locator('link[rel="apple-touch-icon"]')).toHaveAttribute("href", /apple-touch-icon\.png/);

  for (const asset of ["/icon-192.png", "/icon-512.png", "/icon-maskable-512.png", "/apple-touch-icon.png", "/favicon.ico"]) {
    const res = await page.request.get(asset);
    expect(res.status(), `${asset} should be served`).toBe(200);
  }
});

test("offline fallback renders the branded shell", async ({ page }, testInfo) => {
  await page.goto("/offline");
  await expect(page.getByRole("heading", { name: "คุณกำลังออฟไลน์" })).toBeVisible();
  // The shared shell brands the page (header + footer both link home).
  await expect(page.getByRole("link", { name: "ร่วมงานกับเรา" }).first()).toBeVisible();
  await page.screenshot({ path: `${SCREEN_DIR}/offline-${testInfo.project.name}.png`, fullPage: true });
});

test("service worker registers in a production build", async ({ page }) => {
  // Serwist is disabled in `next dev`; if /sw.js isn't served we're not in a prod
  // build and there's nothing to assert.
  const swRes = await page.request.get("/sw.js");
  test.skip(swRes.status() !== 200, "SW only generated in a production build (pnpm build && pnpm start)");

  await page.goto("/jobs");
  const registered = await page
    .waitForFunction(
      async () => {
        if (!("serviceWorker" in navigator)) return false;
        const reg = await navigator.serviceWorker.getRegistration();
        return Boolean(reg);
      },
      undefined,
      { timeout: 15_000 },
    )
    .then(() => true)
    .catch(() => false);
  expect(registered).toBe(true);
});
