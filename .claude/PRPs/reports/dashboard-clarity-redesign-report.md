# Implementation Report: Dashboard Clarity & De-noise Pass

## Summary
Applied a plain-language vocabulary and removed decorative/misleading noise across the
HR dashboard so data reads at a glance. Key wins: removed the misleading KPI up/down
delta arrows (they implied a trend with no time comparison), de-duplicated the pass-rate
to one canonical home, made the funnel widths honestly proportional, replaced jargon
("Passed AI gate", "Pass-through", "Operator focus", "Command center") with plain English,
removed the candidate hex-UUID, and stripped competing brass marks (dot-cluster, radial
glow, KPI corner-tick) down to one keyline per surface.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large (→XL) | Large — landed smaller than feared (4 files) |
| Confidence | 8/10 | 9/10 — single pass, no rework |
| Files Changed | ~9 | 4 (shared changes covered most surfaces) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Shared `statusLabel()` | ✅ Complete | Plain status words; StatusPill uses it (Inbox/Candidates/Search) |
| 2 | KpiCards — remove arrows, relabel | ✅ Complete | Deleted delta arrows + `ArrowDownRight` import + `delta` field |
| 3 | Funnel — honest widths | ✅ Complete | Dropped `FUNNEL_TAPER`; proportional + monotonic clamp |
| 4 | Overview — copy + de-noise | ✅ Complete | Removed dot-cluster + glow; plain copy; single pass-rate home |
| 5 | Candidates — drop UUID | ✅ Complete | Region replaces hex id (mobile + desktop); plainer copy |
| 6 | Analytics + Search + sweep | ✅ Complete | Fixed 2 user-facing jargon spots in Charts; pages else unchanged |
| 7 | BrandMark brass dot | ⏭️ Skipped | Per task guard — it's the brand-logo signature, not data noise |
| 8 | Tests + build | ✅ Complete | Build green; E2E 5/5; jargon sweep clean |
| + | KPI hero corner-tick | ✅ Complete | Extra de-noise found during visual verify (was in plan's noise list) |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis (tsc) | ✅ Pass | Zero type errors after every change |
| Lint (eslint/next build) | ✅ Pass | No unused imports (kept `ArrowUpRight` for SourcesChart) |
| Unit Tests | ⚠️ N/A | No unit harness in project (Playwright only); `statusLabel` validated via E2E + types |
| Build (next build) | ✅ Pass | Turbopack, all 10 routes generated |
| Integration (Playwright) | ✅ Pass | `e2e/dashboard.spec.ts` 5/5 passed |
| Edge Cases | ✅ Pass | applied=0 guard, funnel floor, empty name fallback, unknown status fallback — verified by reasoning + live seeded data |

## Files Changed

| File | Action | Change |
|---|---|---|
| `frontend/components/people/PeopleBits.tsx` | UPDATED | +`statusLabel()` map + StatusPill uses it |
| `frontend/components/analytics/Charts.tsx` | UPDATED | Arrows removed, labels, honest funnel, corner-tick removed, jargon fixed |
| `frontend/app/(app)/dashboard/page.tsx` | UPDATED | dot-cluster + glow removed, plain copy, single pass-rate |
| `frontend/app/(app)/candidates/page.tsx` | UPDATED | UUID→region, plainer eyebrow/meta/strip |

## Deviations from Plan
1. **Task 7 skipped** — the BrandMark brass corner-dot is the brand-logo signature in the
   sidebar (away from data), not clarity noise. Per the task's own guard, left untouched.
2. **Analytics & Search pages not edited** — the plan allowed "copy only / likely none".
   Their jargon lived in the shared `Charts.tsx` (fixed there: funnel label "Passed AI"→
   "Passed screening", KpiStrip "clear the AI gate"→"clear screening"), so the pages
   inherited the fix with no per-page edits needed.
3. **No unit test added** — project has no unit-test runner (only Playwright E2E). Adding
   vitest for one pure function would violate the plan's "NOT building"; validated via E2E.
4. **Net 4 files** vs ~9 predicted — shared-component changes (Task 1-3) covered Inbox,
   Candidates, Search, Overview, and Analytics at once.

## Issues Encountered
- **Stale `.next` dev cache** threw a `__webpack_modules__[moduleId] is not a function`
  runtime error on first dev-server screenshot (reported Next 15.5.19/Webpack while the
  build is 16.2.6/Turbopack). Resolved by `rm -rf .next` + freeing port 3000 and restarting.
  Not a code defect — production build was green throughout.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| (none added) | — | No unit harness; `e2e/dashboard.spec.ts` (existing, 5 tests) covers the surfaces and passed |

## Visual Verification
Screenshots at 1440 + 390 for /dashboard, /analytics, /candidates (`/tmp/clarity-shots/`):
no arrows, honest funnel (15→11→5→2), plain labels, region instead of UUID, one brass
keyline per surface, no overflow at 390.

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr`
- [ ] Deploy: **frontend-only** change — rebuild dashboard image only (4 NEXT_PUBLIC_AZURE_AD_* args); API does NOT need a roll
