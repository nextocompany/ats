# Implementation Report: Bilingual UI (i18n TH/EN) — foundation + first surfaces

## Summary
Wired **next-intl** into both frontends with a working **TH/EN language switcher**
(cookie-based, default Thai) and translated the highest-value shared surfaces. The
i18n foundation is complete and building on both apps; the remaining per-surface
string extraction (the long tail) is a tracked mechanical follow-up.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (foundation done; full extraction phased) |
| Confidence | 6.5/10 | held |
| Files Changed | framework + ~100 surfaces | foundation (16) + login/nav/header surfaces; rest = follow-up |

## Key deviation (deliberate, risk-reduction)
**Career-portal uses cookie-based locale (no `[locale]` URL routing) in v1**, same as
the dashboard — NOT the plan's locale-prefixed routing. Why: moving the public portal's
10 routes under `app/[locale]/` risks breaking the PWA manifest, OAuth/LINE callbacks,
and the backend deep links (`${PORTAL_BASE_URL}/interview`, `/status`) — none of which
are browser-verifiable here. Cookie mode delivers the working bilingual switcher with
zero routing risk. **URL-prefix + SEO is a clean follow-up** (flip `localePrefix` +
move routes) once verified in staging. Tradeoff: portal pages are now server-dynamic
(cookie read) instead of static — acceptable for v1.

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | next-intl config (portal) | ✅ | cookie `i18n/request.ts` + plugin (composed with withSerwist) |
| 2 | Portal routing/layout | ⚠️ Deviated | cookie mode (no `[locale]` move) — see deviation |
| 3 | Portal switcher + strings | ◑ Partial | `LocaleSwitcher` + `SiteHeader` (chrome on every page) translated; landing/jobs/apply copy = follow-up |
| 4 | next-intl config (dashboard) | ✅ | cookie mode |
| 5 | Dashboard provider + middleware | ✅ | `NextIntlClientProvider` + dynamic `<html lang>` (auth middleware unchanged/compatible) |
| 6 | Dashboard switcher + strings | ◑ Partial | login page fully + nav (SideNav/MobileBar/AppHeader) translated + switcher in AppHeader; deep surfaces = follow-up |
| 7 | Switcher persistence + lang | ✅ | cookie + `router.refresh()`; `<html lang>` reflects locale both apps |
| 8 | Catalog parity test | ✅ | `scripts/check-i18n-parity.mjs` (dependency-free) — both apps in parity |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| TypeScript (both apps) | ✅ Pass | `tsc --noEmit` |
| Build (both apps) | ✅ Pass | `next build` (dashboard) + `next build --webpack` (portal); manifest still static, deep-link paths unchanged |
| Catalog parity | ✅ Pass | frontend 32 keys, portal 7 keys, th/en identical |
| Switcher | ✅ | cookie write + refresh; verified via build (not browser) |

## Files Changed (foundation + first surfaces)
- **Both apps**: `i18n/request.ts`, `next.config.ts` (plugin), `messages/{th,en}.json`, `components/LocaleSwitcher.tsx`, `app/layout.tsx` (provider + lang); `next-intl` dep.
- **Dashboard**: `login/page.tsx` (full), `shell/nav-config.tsx` (+key), `shell/{SideNav,MobileBar,AppHeader}.tsx` (translate + switcher mount).
- **Portal**: `SiteHeader.tsx` (async + translate + switcher).
- **Repo**: `scripts/check-i18n-parity.mjs`.

## Remaining (tracked follow-up — mechanical extraction)
Translate the per-surface copy into the catalogs (pattern is established):
- **Dashboard**: inbox/applications list, application detail (AiSummary, InterviewPanel, InterviewFeedback, Bulk, Fit), candidates, search, analytics, admin, members.
- **Portal**: landing (Hero/LandingSections), jobs list + detail, apply stepper, signup, status, account, interview chat, offline.
Each: replace hardcoded strings with `t(...)`, add keys to both `th.json`/`en.json` (parity script guards drift).

## Out of scope (unchanged, by design)
- AI/LLM output (scores' strengths/summaries, fit) and candidate notifications stay Thai (server-generated data).
- URL-prefix/SEO routing for the portal (follow-up).

## Issues Encountered
- `SiteHeader` was a sync server component → made it `async` + `getTranslations` (server). No client conversion needed.

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Commit + PR (+ optional deploy: additive + non-breaking — switcher live, untranslated surfaces still render their existing text)
- [ ] Follow-up: extract remaining per-surface strings (parity script gates it)
- [ ] Later: portal URL-prefix routing for SEO once staging-verified
