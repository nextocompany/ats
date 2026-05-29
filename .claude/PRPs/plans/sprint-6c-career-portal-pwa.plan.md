# Plan: Sprint 6c — Career Portal PWA

## Summary
Turn the candidate-facing **career portal** into an installable, offline-capable **PWA**: a web manifest + icons, a service worker that caches the app shell and static assets with a sensible runtime strategy, an "Add to Home Screen" install affordance, and a Lighthouse PWA pass — all tuned for Thai job-seekers on mobile inside the LINE in-app browser. The HR dashboard is unchanged.

## User Story
As a **job-seeker on my phone (often via LINE)**, I want **to install the career portal and have it load instantly (even on a flaky connection)**, so that **browsing jobs and checking my application status feels like a fast native app**.

## Problem → Solution
**Current state:** `career-portal/` is a normal Next.js 16 app — no `public/` dir, no manifest, no icons, no service worker; not installable; a cold/offline load shows a network error. `layout.tsx` already has `metadata` + a `viewport` with `themeColor`.
**Desired state:** Installable PWA — manifest (name/icons/theme/display=standalone), service worker (precache the shell + static assets, network-first for navigations with offline fallback, stale-while-revalidate for the positions list), an install prompt, and a clean Lighthouse PWA report.

## Metadata
- **Complexity**: Medium (config + assets + SW + small UI; ~12 files, mostly assets/config)
- **Source PRD**: Nexto PRP v1.0 — Sprint 6 (PWA); roadmap §20 (S6–7)
- **Decisions locked**: use **Serwist** (`@serwist/next`) — the maintained Next 16 / App Router PWA toolchain; manifest via App Router `app/manifest.ts`; icons committed under `public/`; offline fallback page; mobile-first (matches the warm 5-sprint-old portal direction)
- **Estimated Files**: ~12
- **Depends on**: nothing (independent of 6a/6b). If 6a merged first, the SW + CSP must agree (note in tasks).

---

## UX Design

### Before
```
Open portal in LINE browser → blank/spinner while JS loads → /jobs
Offline / flaky → fetch error, dead page. Not installable.
```
### After
```
First visit → manifest + SW install in background
"เพิ่มลงหน้าจอหลัก" (Add to Home Screen) prompt → installs as standalone app
Re-open offline → cached shell renders instantly; cached positions show;
  actions needing network show a friendly offline notice
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Install | not possible | installable (manifest + SW) | display: standalone |
| Cold load | network-dependent | precached shell renders instantly | service worker |
| Offline `/jobs` | error | last-cached positions or offline notice | SWR / cache fallback |
| Offline navigation | browser error page | branded offline fallback | precached `/offline` |
| Icon/splash | none | maskable icons + theme color | manifest icons |

---

## Mandatory Reading (the contract + patterns to reuse)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `career-portal/app/layout.tsx` | all | `metadata` + `viewport` (themeColor) — add `manifest`, `icons`, `appleWebApp`; register the SW |
| P0 | `career-portal/next.config.ts` | all | empty config — wrap with `withSerwist({ swSrc, swDest })` |
| P0 | `career-portal/package.json` | all | Next 16.2.6, scripts (dev/build/start -p 3001); add `@serwist/next` + `serwist` |
| P0 | `career-portal/app/globals.css` | 40-64 | `:root` tokens — `--primary: oklch(58% 0.15 150)` (theme color) + `--background` (manifest bg) |
| P1 | `career-portal/components/PortalShell.tsx` | all | the shared shell to render in the offline fallback (brand-consistent) |
| P1 | `career-portal/lib/queries.ts` | all | the GET endpoints to cache (positions list/detail, status) vs the POST apply (never cache) |
| P1 | `career-portal/lib/api.ts` | all | `NEXT_PUBLIC_API_URL` base — SW runtime caching must match this origin |
| P1 | `career-portal/app/jobs/page.tsx` | all | `/jobs` is the primary offline target (static + client fetch) |
| P1 | `career-portal/.gitignore` | all | add the generated SW (`public/sw.js`) to ignore |
| P2 | `career-portal/playwright.config.ts` | all | mobile viewport projects to add a PWA/manifest assertion |
| P2 | `career-portal/README.md` | all | document PWA build + Lighthouse step |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Serwist + Next | serwist.pages.dev/docs/next | `@serwist/next` `withSerwist({swSrc:"app/sw.ts", swDest:"public/sw.js"})`; write `app/sw.ts` using `defaultCache`; disable in dev to avoid HMR conflicts |
| App Router manifest | nextjs.org/docs/app/api-reference/file-conventions/metadata/manifest | `app/manifest.ts` exports a `MetadataRoute.Manifest` — name, icons, theme_color, background_color, display, start_url |
| Maskable icons | web.dev/maskable-icon | provide 192/512 + a `purpose:"maskable"` icon so Android masks cleanly |
| iOS PWA | nextjs.org metadata `appleWebApp` | iOS needs `apple-touch-icon` + `appleWebApp.capable`; no SW install banner on iOS (manual “Add to Home Screen”) |
| Lighthouse PWA | developer.chrome.com/docs/lighthouse/pwa | installability = manifest + SW + offline 200 for start_url + icons + theme/viewport |

### Research Notes
```
KEY_INSIGHT: Serwist is the maintained PWA path for Next 16 App Router (next-pwa is stale).
APPLIES_TO: 6c toolchain.
GOTCHA: register SW only in production builds. @serwist/next disables the SW in `next dev` by default — keep that so dev/HMR isn't cached. Lighthouse must be run against `pnpm build && pnpm start`, not dev.

KEY_INSIGHT: apply is a multipart POST that must NEVER be cached/replayed offline.
APPLIES_TO: SW runtime caching.
GOTCHA: cache only GETs to the API origin (positions/detail/status) with NetworkFirst/StaleWhileRevalidate; explicitly exclude POST /api/v1/public/apply. Don't background-sync resume uploads (PII + the LINE token would be stale) — show an offline notice instead.

KEY_INSIGHT: portal pages are client-fetched; the HTML shell is the cacheable unit.
APPLIES_TO: offline shell.
GOTCHA: precache the build assets + an `/offline` fallback route; navigations use NetworkFirst falling back to `/offline`. The positions data is cached separately (SWR) so a warm user sees last-known jobs.

KEY_INSIGHT: no icons exist and there's no public/ dir.
APPLIES_TO: assets.
GOTCHA: create public/ + icons (192, 512, maskable-512, apple-touch-180, favicon). Generate from the portal's "N" mark on the brand green (--primary). Commit them (binary PNGs).

KEY_INSIGHT: if 6a's CSP ships, it must allow the SW + manifest.
APPLIES_TO: cross-sprint.
GOTCHA: CSP needs `worker-src 'self'`, `manifest-src 'self'`; service worker script is same-origin 'self'. Coordinate if 6a merges first.
```

---

## Patterns to Mirror

### MANIFEST (app/manifest.ts — MetadataRoute.Manifest)
```ts
import type { MetadataRoute } from "next";
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "ร่วมงานกับเรา", short_name: "สมัครงาน",
    description: "ดูตำแหน่งงานที่เปิดรับและสมัครงานได้ในไม่กี่ขั้นตอน",
    start_url: "/jobs", display: "standalone",
    background_color: "#fffdf7", theme_color: "#16a34a", lang: "th",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png" },
      { src: "/icon-maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
```

### SERWIST_CONFIG (next.config.ts)
```ts
import withSerwistInit from "@serwist/next";
const withSerwist = withSerwistInit({ swSrc: "app/sw.ts", swDest: "public/sw.js", disable: process.env.NODE_ENV === "development" });
export default withSerwist({ /* existing config */ });
```

### SERWIST_SW (app/sw.ts)
```ts
import { defaultCache } from "@serwist/next/worker";
import { Serwist } from "serwist";
const serwist = new Serwist({
  precacheEntries: self.__SW_MANIFEST,
  skipWaiting: true, clientsClaim: true, navigationPreload: true,
  runtimeCaching: defaultCache, // + a NetworkFirst rule for GET <API>/api/v1/public/*
  fallbacks: { entries: [{ url: "/offline", matcher: ({ request }) => request.destination === "document" }] },
});
serwist.addEventListeners();
```

### LAYOUT_METADATA (extend layout.tsx)
```ts
export const metadata: Metadata = {
  title: "ร่วมงานกับเรา | สมัครงาน",
  description: "...",
  manifest: "/manifest.webmanifest",
  appleWebApp: { capable: true, statusBarStyle: "default", title: "สมัครงาน" },
  icons: { icon: "/favicon.ico", apple: "/apple-touch-icon.png" },
};
```

### INSTALL_PROMPT (client component; mirror existing "use client" components)
```tsx
"use client";
// capture beforeinstallprompt; show a dismissible "เพิ่มลงหน้าจอหลัก" button that calls prompt()
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `career-portal/package.json` | UPDATE | add `@serwist/next` + `serwist` (dev dep ok) |
| `career-portal/next.config.ts` | UPDATE | wrap with `withSerwist({swSrc,swDest,disable:dev})` |
| `career-portal/app/sw.ts` | CREATE | Serwist service worker (precache + runtime + offline fallback) |
| `career-portal/app/manifest.ts` | CREATE | App Router web manifest |
| `career-portal/app/layout.tsx` | UPDATE | manifest/icons/appleWebApp metadata |
| `career-portal/app/offline/page.tsx` | CREATE | branded offline fallback (uses PortalShell) |
| `career-portal/components/InstallPrompt.tsx` | CREATE | dismissible Add-to-Home-Screen affordance |
| `career-portal/app/jobs/page.tsx` (or layout) | UPDATE | mount `InstallPrompt` (once, non-intrusive) |
| `career-portal/public/icon-192.png` … `icon-512.png` … `icon-maskable-512.png` … `apple-touch-icon.png` … `favicon.ico` | CREATE | PWA + favicon assets (brand "N" on green) |
| `career-portal/.gitignore` | UPDATE | ignore generated `public/sw.js` (+ `public/swe-worker-*.js`) |
| `career-portal/e2e/pwa.spec.ts` | CREATE | assert manifest linked + served, SW registers (prod build), offline shell renders |
| `career-portal/README.md` | UPDATE | PWA build + Lighthouse instructions |
| `career-portal/tsconfig.json` | UPDATE (if needed) | add the webworker lib for `app/sw.ts` typing |

## NOT Building (later / out of scope)
- **Background sync / offline apply queue** — apply (PII + LINE token) is never queued offline; show a notice.
- **Push notifications** — manifest is push-ready but no push wiring this sprint (LINE is the channel; see 5a notifier).
- **PWA for the HR dashboard** — internal console stays a normal web app.
- **iOS install banner** — iOS has no `beforeinstallprompt`; rely on Safari "Add to Home Screen" (documented).
- **Custom splash screen images per device** — theme color + maskable icon only.
- **Caching the apply/status POST responses** — explicitly excluded.

---

## Step-by-Step Tasks

### Task 1: Add Serwist + manifest
- **ACTION**: `pnpm add @serwist/next serwist`. Create `app/manifest.ts` (MANIFEST). Wrap `next.config.ts` with `withSerwist` (SERWIST_CONFIG, `disable` in dev).
- **MIRROR**: MANIFEST, SERWIST_CONFIG.
- **GOTCHA**: keep the existing (empty) NextConfig object passed through `withSerwist(...)`. theme_color must match `--primary` green; background_color the warm bg.
- **VALIDATE**: `pnpm build` succeeds; `/manifest.webmanifest` is generated/served.

### Task 2: Service worker + offline fallback
- **ACTION**: `app/sw.ts` (SERWIST_SW) — precache `self.__SW_MANIFEST`, `defaultCache` runtime + a NetworkFirst rule for `GET ${NEXT_PUBLIC_API_URL}/api/v1/public/*`, document fallback → `/offline`. Create `app/offline/page.tsx` using `PortalShell` ("คุณกำลังออฟไลน์…").
- **MIRROR**: SERWIST_SW; PortalShell.
- **GOTCHA**: exclude `POST /apply` from runtime caching (only cache GETs). `app/sw.ts` needs webworker types (`tsconfig` lib or `/// <reference lib="webworker" />`).
- **VALIDATE**: `pnpm build && pnpm start`; DevTools → Application shows SW active + manifest; toggle offline → `/jobs` shows cached shell, `/offline` for uncached nav.

### Task 3: Layout metadata + icons
- **ACTION**: Extend `layout.tsx` metadata (LAYOUT_METADATA). Create `public/` with `icon-192/512`, `icon-maskable-512`, `apple-touch-icon`(180), `favicon.ico` — brand "N" on `--primary` green.
- **MIRROR**: LAYOUT_METADATA; the "N" mark in `PortalShell.tsx`.
- **GOTCHA**: maskable icon needs safe-zone padding (icon within center 80%). Commit binary PNGs.
- **VALIDATE**: built HTML links manifest + apple-touch-icon; icons resolve 200.

### Task 4: Install prompt
- **ACTION**: `components/InstallPrompt.tsx` — capture `beforeinstallprompt`, show a dismissible "เพิ่มลงหน้าจอหลัก" button (localStorage-dismiss), call `prompt()`. Mount once (jobs page or layout).
- **MIRROR**: existing `"use client"` component style (e.g. LineLoginButton).
- **GOTCHA**: only render when the event fired (Android/desktop Chrome); hide on iOS/installed. Reduced-motion respected (globals.css already does).
- **VALIDATE**: Chrome desktop/Android shows the prompt; dismiss persists; no crash on iOS/Safari.

### Task 5: gitignore + e2e + docs + Lighthouse
- **ACTION**: `.gitignore` add `public/sw.js` + `public/swe-worker-*.js`. `e2e/pwa.spec.ts`: assert `<link rel="manifest">` present, `/manifest.webmanifest` 200 + valid JSON, SW registers (prod), `/offline` renders. Update README with PWA build + `lighthouse` step.
- **MIRROR**: existing portal e2e structure (mobile projects).
- **GOTCHA**: SW only registers in prod build — the e2e PWA test must run against `pnpm build && pnpm start` (or guard the SW assertion). Generated `sw.js` must not be committed.
- **VALIDATE**: `pnpm exec playwright test pwa.spec.ts` (prod server) green; Lighthouse PWA category passes (installable + offline).

---

## Testing Strategy
### Priority (web, mobile-first)
1. **PWA installability** — manifest valid + linked, icons present, SW registers, start_url offline-200 (Lighthouse PWA).
2. **Offline behavior** — cached shell renders; `/offline` fallback for uncached nav; positions show last-known.
3. **No regression** — existing portal e2e (jobs/apply/status) still green; apply still hits the network (not cached).
4. **A11y/visual** — install prompt is dismissible, reduced-motion honored, screenshots at 320/375/768.

### Edge Cases Checklist
- [ ] Offline cold load → branded offline page (not browser error)
- [ ] Offline `/jobs` → last-cached positions or graceful empty
- [ ] Apply offline → clear "needs connection" notice (never silent-queue)
- [ ] iOS (no beforeinstallprompt) → no broken prompt; Safari add-to-home works
- [ ] SW update → new version activates (skipWaiting/clientsClaim) without stale shell
- [ ] Generated `sw.js` not committed; dev build has SW disabled

## Validation Commands
### Static + build
```bash
cd career-portal && pnpm lint && pnpm exec tsc --noEmit && pnpm build
```
### PWA / offline (prod server)
```bash
cd career-portal && pnpm build && pnpm start &   # :3001
curl -s http://localhost:3001/manifest.webmanifest | python3 -m json.tool   # valid manifest
pnpm exec playwright test pwa.spec.ts
# Lighthouse (Chrome): PWA category — installable + offline pass
```
### Regression
```bash
cd career-portal && pnpm exec playwright test portal.spec.ts
```

## Acceptance Criteria
- [ ] `career-portal` is installable: valid manifest (name/icons/theme/display=standalone) + registered service worker.
- [ ] Offline: cached app shell renders; branded `/offline` fallback; positions show last-known; apply shows an offline notice (never cached/replayed).
- [ ] Install prompt offered (Android/desktop Chrome), dismissible; iOS degrades gracefully.
- [ ] Lighthouse PWA category passes; existing portal e2e still green; generated `sw.js` gitignored.
- [ ] lint/tsc/build clean; screenshots at 320/375/768.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Caching the apply POST / stale PII | Low | High | cache GETs only; explicitly exclude apply; no background sync |
| Stale shell after deploy | Med | Med | skipWaiting + clientsClaim; versioned precache via Serwist manifest |
| LINE in-app browser SW quirks | Med | Med | graceful degradation (works as normal web app if SW unsupported); test in LINE |
| 6a CSP blocks SW/manifest | Med | Med | coordinate `worker-src`/`manifest-src 'self'` if 6a merged first |
| Icon quality/maskable safe-zone | Low | Low | generate from brand mark with padding; verify in Lighthouse |

## Notes
- Independent of 6a/6b — any merge order. If 6a lands first, add `worker-src 'self'` + `manifest-src 'self'` to the portal CSP.
- This completes the candidate-experience track; combined with 6a (security) + 6b (E2E), Sprint 6 leaves the product installable, hardened, and regression-guarded heading into S7/S8.
