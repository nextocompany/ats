# Implementation Report: Sprint 6c — Career Portal PWA

## Summary
Turned `career-portal/` into an installable, offline-capable PWA using **Serwist**
(`@serwist/next` 9.5.11): App Router web manifest, a service worker that precaches
the shell + does NetworkFirst for public GET APIs with an `/offline` document
fallback, brand icons (white "N" on the brand green), a dismissible install
prompt, and a PWA e2e suite.

While validating against a **production** build (required for SW/Lighthouse), found
a pre-existing **Sprint 6a CSP defect**: `script-src 'self'` blocks Next.js's own
inline hydration/streaming scripts, so prod builds of **both** frontends render but
never hydrate. We first tried the framework-recommended per-request **nonce CSP via
middleware**, but confirmed it does **not** work for Next's statically-prerendered
pages (Next emits `nonce=undefined` for them — verified on both apps). Per the
user's decision, the fix is a header-based **`script-src 'self' 'unsafe-inline'`**
(Next's inline scripts are framework-generated, same-origin), applied to the portal
and the HR dashboard. This keeps pages static/CDN-friendly and lets prod hydrate.

**Repo note:** `ats` is a single git repo (`github.com/nextocompany/ats`); branch
`feat/sprint-6c-career-portal-pwa` holds all changes (both `career-portal/` and the
one-line `frontend/` CSP change).

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Add Serwist + manifest | ✅ | `app/manifest.ts`; `next.config.ts` wrapped with `withSerwist` |
| 2 | Service worker + offline fallback | ✅ | `app/sw.ts` (precache + NetworkFirst public GETs + `/offline`); `app/offline/page.tsx` |
| 3 | Layout metadata + icons | ✅ | PWA metadata + brand-green themeColor; 5 brand icons |
| 4 | Install prompt | ✅ | `components/InstallPrompt.tsx`, mounted on `/jobs` |
| 5 | gitignore + e2e + docs | ✅ | generated `sw.js` ignored; `e2e/pwa.spec.ts`; README PWA + Lighthouse |
| + | CSP fix (deviation) | ✅ | `script-src 'self' 'unsafe-inline'` in both apps' `next.config.ts` headers |

## Validation Results

| Check | Portal (`career-portal`, webpack) | Dashboard (`frontend`, turbopack) |
|---|---|---|
| ESLint | ✅ clean | ✅ clean |
| tsc --noEmit | ✅ clean | ✅ clean |
| Build | ✅ `next build --webpack` (Serwist) | ✅ `next build` |
| Prod CSP | ✅ `script-src 'self' 'unsafe-inline'`, **0 violations** | ✅ same, **0 violations** |
| Hydration | ✅ `/status` hydrates; SW registers | ✅ `/login` hydrates (8 els); auth gate → `/login` intact |
| E2E | ✅ **24/24** Playwright (PWA + portal + apply) 320/375/768, prod + live API | smoke-verified |

### Key evidence (portal)
- `/manifest.webmanifest` 200 + valid (name/start_url=`/jobs`/display=standalone/theme=#1f9d57/maskable icon).
- `/sw.js` 200; `navigator.serviceWorker.getRegistration()` resolves in prod.
- All 5 icons + `/offline` 200; `<link rel="manifest">` + apple-touch in `<head>`.
- Apply still hits the network (POST structurally excluded from SW caching).

## Files Changed (branch `feat/sprint-6c-career-portal-pwa`)

### `career-portal/`
| File | Action |
|---|---|
| `package.json` | UPDATED (deps + dev/build → `--webpack` for Serwist) |
| `pnpm-lock.yaml` | UPDATED |
| `next.config.ts` | UPDATED (`withSerwist`; CSP `script-src` → `'self' 'unsafe-inline'`) |
| `app/sw.ts` | CREATED |
| `app/manifest.ts` | CREATED |
| `app/layout.tsx` | UPDATED (PWA metadata + themeColor) |
| `app/offline/page.tsx` | CREATED |
| `components/InstallPrompt.tsx` | CREATED |
| `app/jobs/page.tsx` | UPDATED (mount InstallPrompt) |
| `public/{icon-192,icon-512,icon-maskable-512}.png, apple-touch-icon.png, favicon.ico` | CREATED (5) |
| `.gitignore` | UPDATED (ignore generated `sw.js`) |
| `eslint.config.mjs` | UPDATED (ignore generated `public/sw.js`) |
| `e2e/pwa.spec.ts` | CREATED |
| `README.md` | UPDATED (PWA + Lighthouse + CSP note) |

### `frontend/` (HR dashboard)
| File | Action |
|---|---|
| `next.config.ts` | UPDATED (one line: `script-src` → `'self' 'unsafe-inline'`) |

(The dashboard's `middleware.ts` is unchanged — it stays the auth gate; no PWA there.)

## Deviations from Plan
1. **CSP fix → `script-src 'self' 'unsafe-inline'` (user-approved).** The plan only
   anticipated "6a CSP must allow the SW/manifest" (it did). Prod validation showed
   6a's `script-src 'self'` also blocked Next's inline hydration scripts, breaking
   hydration in production for both frontends. The per-request nonce approach was
   tried first but **does not work for statically-prerendered pages** (Next emits
   `nonce=undefined`; verified on both apps via the served HTML/RSC payload). The
   user chose `'unsafe-inline'` — the reliable fix that keeps pages static. Applied
   as a header CSP in each app's `next.config.ts`.
2. **Portal build/dev → `--webpack`.** `@serwist/next` 9 doesn't support Next 16's
   default Turbopack (build errored). Portal `dev`/`build` use `--webpack`; SW stays
   disabled in dev. The dashboard has no Serwist and stays on Turbopack.
3. **eslint ignore for generated SW.** Bare `eslint` linted the generated
   `public/sw.js`; added it to the portal `eslint.config.mjs` `globalIgnores`.

## Issues Encountered
- **Nonce CSP dead-end** → spent effort on a per-request nonce middleware before
  confirming Next can't nonce statically-prerendered pages; reverted to the
  header-based `'unsafe-inline'` fix. (Net: portal `middleware.ts` removed; dashboard
  `middleware.ts` restored to its original auth-only form.)
- **Stale `next start` servers / parallel races** produced misleading smoke results
  mid-investigation; resolved by killing listeners by port and verifying sequentially.
- **Stale `.next` after bundler switch** → `rm -rf .next` + rebuild.
- **InstallPrompt setState-in-effect lint** → gated the dismissal check in the event handler.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `career-portal/e2e/pwa.spec.ts` | 4 (×3 viewports = 12) | manifest linked+served+valid, icons/apple-touch 200, `/offline` shell, SW registers in prod |

## Next Steps
- [ ] `/code-review` (covers the security-sensitive CSP change; also a good moment for the deferred 6a/6b independent pass)
- [ ] `/prp-pr` → squash-merge per the sprint cadence
- [ ] Optional: `lighthouse http://localhost:3001/jobs --only-categories=pwa`
- [ ] Future hardening (not S6): tighten `script-src` beyond `'unsafe-inline'` (hashes, or nonce + forced dynamic rendering) if the static/CDN trade-off becomes acceptable.
