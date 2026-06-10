# Implementation Report: Career Portal — Proper Website Redesign (Clean-Luxury, Responsive)

## Summary
Transformed the career portal from a mobile-only single-column app into a responsive clean-luxury website: a real landing page (hero, value props, live featured jobs, 3-step band, CTA, multi-column footer), a responsive shell (desktop nav + footer, fluid 320→1920 within a 1200px container), and a visual overhaul of jobs/detail/apply/status/offline. All apply/status behavior, accessible names, ids, and the backend contract preserved.

## Assessment vs Reality
| Metric | Predicted | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 8/10 | 9/10 — single pass; one e2e strict-mode fix |
| Files Changed | ~16 | 18 (6 new, 12 updated) |

## Tasks Completed
| # | Task | Status |
|---|---|---|
| 1 | Luxury tokens (globals.css palette + scale) | ✅ |
| 2 | Container / SiteHeader / SiteFooter | ✅ |
| 3 | Recompose PortalShell (backHref + narrow) | ✅ |
| 4 | Landing page (Hero + FeaturedJobs + LandingSections) | ✅ |
| 5 | Jobs responsive grid + luxe JobCard | ✅ |
| 6 | Job detail 2-col desktop + sticky apply card | ✅ |
| 7 | Apply/Consent/LINE restyle (contract preserved) | ✅ |
| 8 | Status / StatusCard / Offline restyle | ✅ |
| 9 | PWA color sync (manifest + viewport + pwa.spec) | ✅ |
| 10 | Responsive + a11y + perf sweep | ✅ |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Lint | ✅ Pass | eslint clean |
| Build | ✅ Pass | prod build; `/` now static landing |
| E2E (prod, 3 mobile projects) | ✅ Pass | **29 passed, 0 failed**, 1 flaky (tablet apply, passes on retry) |
| Visual | ✅ | landing/jobs/dashboard headless screenshots confirm clean-luxury + data |

## Files Changed
**New (6):** `components/Container.tsx`, `SiteHeader.tsx`, `SiteFooter.tsx`, `landing/Hero.tsx`, `landing/FeaturedJobs.tsx`, `landing/LandingSections.tsx`
**Updated (12):** `app/globals.css`, `app/page.tsx` (redirect→landing), `app/layout.tsx` (themeColor), `app/manifest.ts` (theme/bg), `components/PortalShell.tsx`, `JobCard.tsx`, `app/jobs/page.tsx`, `app/jobs/[id]/page.tsx`, `app/jobs/[id]/apply/page.tsx`, `components/ApplyStepper.tsx`, `ConsentStep.tsx`, `StatusCard.tsx`, `LineLoginButton.tsx`, `app/status/page.tsx`, `app/offline/page.tsx`, `e2e/pwa.spec.ts`

## Deviations from Plan
- **`e2e/pwa.spec.ts` offline test** updated beyond just the theme_color assertion: the new footer adds a second `ร่วมงานกับเรา` brand link, so `getByRole("link", {name})` hit a strict-mode (2 matches) — changed to `.first()`. Expected consequence of adding a branded footer; assertion intent preserved.

## Design decisions in implementation
- **Brand color:** deep emerald `#0f5132` (theme_color/viewport/pwa.spec), ivory `#fbfaf7` bg; primary CTA = near-black ink, accent = emerald, gold hairline detail.
- **Contract preserved:** all e2e labels/ids/structure intact (`#status-token`, consent/success headings, `<ul><li><a>` jobs grid, form labels, LINE strings). No `lib/queries`/API changes.
- Two-font rule kept (Noto Sans Thai + Inter); CSS/SVG-only atmosphere (no animation libs); reveal animation compositor-only + reduced-motion honored.

## Issues Encountered
- pwa offline strict-mode (fixed, above).
- 1 flaky: tablet-768 apply flow consent-heading timing — passes on retry; CI `playwright` job has retries:1. Noted, not blocking.

## Next Steps
- [ ] `/code-review`
- [ ] `/prp-pr` (branch `feat/career-portal-redesign`)
- [ ] Restore ananta: `docker start ananta-postgres-1 ananta-keycloak-1` when demo done
- [ ] (separate slice) dev-CSP fix so `pnpm dev` works
