# Code Review: Executive Dashboard Redesign (feat/executive-dashboard-redesign)

**Reviewed**: 2026-06-21
**Branch**: feat/executive-dashboard-redesign → main
**Mode**: Local (branch diff vs main; not yet pushed)
**Decision**: APPROVE (after fixes applied in-review)

## Summary
Self-reviewed the board-report executive redesign (8 files + 2 shell tweaks). No security/correctness/HIGH issues. Found 3 MEDIUM/LOW quality issues (print chrome, dead i18n keys) — all fixed in-review. Hand-built tabs a11y verified against the WAI-ARIA pattern. Validation green.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
1. **Print pack would include the app sidebar + top header** — `app/(app)/layout.tsx` renders `SideNav` (`<aside bg-sidebar>`) + `AppHeader` (sticky top bar). The new `@media print` revealed the board sections but did not hide the shell chrome, so a printed board pack carried the navy sidebar + header. **FIXED**: added `print:hidden` to the `SideNav` `<aside>` and the `AppHeader` root (additive, print-only, zero screen change, benefits any future print).
2. **`sourcingTitle` i18n key was dead** — the Sourcing tab reuses `SourcesChart`, which renders its own `analytics`-namespace heading (`sourcesEyebrow`/`sourcesTitle`), so the added `executive.sourcingTitle` was never referenced. The tab is still visually consistent (eyebrow + h2). **FIXED**: removed `sourcingTitle` from both catalogs.

### LOW
3. **`retry` i18n key was dead** — the error state is a simple notice (a reload suffices on a read-only dashboard); no retry button consumes the key. **FIXED**: removed `retry` from both catalogs.

## Review Notes (verified OK)
- **Tabs a11y**: WAI-ARIA Tabs pattern correct — `role=tablist/tab/tabpanel`, `aria-selected/controls/labelledby`, roving tabindex (active 0 / others -1), Arrow/Home/End move+activate (automatic activation), focus moved to the new trigger via rAF. Tablist hidden in print; panels mounted-but-`hidden` so print reveals all.
- **Print correctness**: `[role="tabpanel"][hidden]{display:block !important}` overrides the UA `[hidden]` default (higher specificity + !important); `.dark` core tokens remapped to light under `@media print`; `print-color-adjust: exact` keeps brand fills/bars; `break-inside: avoid` on rows.
- **Pending-HRIS**: driven solely by `company.budget_available`; neutral pill + `-` placeholder, never red/amber; branch names stay real (dignified, not "broken"). Real reads (Actual/Vacancy/Pipeline/Sourcing) always render.
- **RBAC gate preserved** (`canViewExecutive`, no fetch when denied); Suspense wraps `useSearchParams`; loading skeletons + empty states + error notice present.
- **No security surface** (read-only client dashboard, no user input, no dangerouslySetInnerHTML, `window.print()` is safe). Functions <50 lines; no console.log/TODO; immutable (`[...sources].sort` in SourcesChart untouched).
- **Pre-existing eslint error** (`AppHeader.tsx:20` setState-in-effect) exists on `main`, unrelated to this change (my edit is line 36 className).

## Validation Results
| Check | Result |
|---|---|
| Type check (tsc) | Pass |
| Lint (eslint) | Pass (1 pre-existing AppHeader error on main, unrelated) |
| Tests | N/A (no dashboard component test harness) |
| Build (next build) | Pass (`/executive` emitted) |
| i18n parity | Pass (859 keys after dead-key removal) |
| No em dash | Pass (0) |

## Files Reviewed
Added: `components/executive/{ExecutiveTabs,CompanySummaryBand,HeadcountVacancyPanel}.tsx`.
Modified: `components/executive/ExecutiveSections.tsx`, `app/(app)/executive/page.tsx`, `app/globals.css`, `messages/{en,th}.json`, `components/shell/{SideNav,AppHeader}.tsx` (print:hidden).
