# Code Review: Career Portal Website Redesign (local, branch `feat/career-portal-redesign`)

**Reviewed**: 2026-06-08
**Mode**: Local (uncommitted changes vs main) — frontend-only
**Decision**: APPROVE with comments (0 critical/high; LOW notes only)

## Summary
A clean-luxury responsive redesign of the candidate career portal (landing page + restyled jobs/detail/apply/status/offline + responsive shell). Frontend-only, token-driven, no logic/API/contract changes. All e2e specs pass; the apply/status accessible names + ids were preserved. No security surface touched.

## Findings

### CRITICAL / HIGH / MEDIUM
None.

### LOW
1. **PWA icons not regenerated for the new brand** (`public/icon-*.png`, `apple-touch-icon.png`) — the theme color moved green `#1f9d57` → emerald `#0f5132`, but the icon PNGs still show the old green mark. Not breaking (pwa.spec only checks they're served), but a brand-consistency gap. Follow-up: regenerate icons to the new palette before go-live.
2. **Footer placeholders** (`SiteFooter.tsx`) — contact (`recruit@example.co.th`, phone), year `2569`, "160 สาขา" are sample copy per the plan (placeholder assets). Replace with real values before production.
3. **`docker-compose.override.yml` is a local demo file** — added to allow the dashboard on :3002 (CORS). It must NOT be committed to the PR (frontend redesign scope). Exclude when staging.
4. **1 flaky e2e** (`portal.spec` apply flow, tablet-768) — consent-heading timing; passes on retry (CI `playwright` job has retries:1). Non-blocking; harden later if it recurs.

## Checked and clean
- **Security**: no secrets, no `dangerouslySetInnerHTML`/`innerHTML`, no new user-input handling (apply mutation/`lib/queries` untouched), no injection surface. The LINE-button green is the intentional LINE brand color, not a secret. ✅
- **Contract/e2e preserved**: `#status-token`, headings (consent/success/jobs/status/offline), `<ul><li><a>` jobs grid, form labels (`/ชื่อ-นามสกุล/`, `/อัปโหลดเรซูเม่/`, `รหัสติดตาม`), LINE strings, checkbox role — all intact. Only the pwa offline brand-link assertion changed to `.first()` (new footer adds a 2nd brand link) + the theme_color value. ✅
- **a11y**: focus-visible rings kept, tap ≥44px (`size:"tap"`), nav/`aria-label`s, decorative panels `aria-hidden`, reduced-motion honored (global + reveal opt-in). ✅
- **Quality**: components small (<130 lines), no console.log/TODO, no mutation, token-driven (no scattered hex), two-font rule kept, CSS/SVG-only atmosphere (no animation libs → bundle budget safe). ✅

## Validation Results
| Check | Result |
|---|---|
| Lint (eslint) | Pass — clean |
| Build (prod) | Pass — `/` static landing |
| Tests (e2e, 3 mobile projects) | Pass — 29 passed, 0 failed, 1 flaky (retry-covered) |
| Type check | Pass (via build) |

## Files Reviewed
New: `Container.tsx`, `SiteHeader.tsx`, `SiteFooter.tsx`, `landing/Hero.tsx`, `landing/FeaturedJobs.tsx`, `landing/LandingSections.tsx`
Modified: `globals.css`, `app/page.tsx`, `layout.tsx`, `manifest.ts`, `PortalShell.tsx`, `JobCard.tsx`, `jobs/page.tsx`, `jobs/[id]/page.tsx`, `jobs/[id]/apply/page.tsx`, `ApplyStepper.tsx`, `ConsentStep.tsx`, `StatusCard.tsx`, `LineLoginButton.tsx`, `status/page.tsx`, `offline/page.tsx`, `e2e/pwa.spec.ts`
Excluded from PR: `docker-compose.override.yml` (local demo only)
