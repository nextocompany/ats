# Plan: Executive Dashboard Redesign (board-report / tabbed)

## Summary
Redesign the Executive Overview into a board-report / print-style leadership pack: a persistent company summary band above a 4-tab layout (Top Shortage / Headcount / Pipeline / Sourcing), with a dignified "pending HRIS" state for the budget-dependent metrics and a print/export path. Frontend-only refinement of the existing institutional "Ledger" design language; no backend, no new data, no migration. Implements the approved contract at `docs/executive-dashboard-UI-SPEC.md`.

## User Story
As a **senior executive (regional/operation director, super_admin, auditor)**, I want the executive overview organized as a clear, tabbed board report I can read at a glance and print, so that I can see staffing shortages, headcount, pipeline, and sourcing without wading through a dense two-column dashboard, and understand which figures are live vs awaiting HRIS.

## Problem → Solution
**Current** (`app/(app)/executive/page.tsx`): a `HeadcountBand` hero + a `lg:grid-cols-2` side-by-side of `ShortStaffedPanel` + `PipelinePanel` + `SourcesChart` below. Dense, no hierarchy for "what needs attention", budget-pending metrics show bare dashes, no print path.
**Desired**: PageHeader + a flat 4-up `CompanySummaryBand` + an accessible `ExecutiveTabs` (one focused board table per tab, URL-persisted `?tab=`) + explicit pending-HRIS treatment + `@media print` board pack.

## Metadata
- **Complexity**: Large (frontend; new accessible tabs + 2 new panels + print CSS + ~22 i18n keys)
- **Source PRD**: N/A — implements `docs/executive-dashboard-UI-SPEC.md` (approved 2026-06-21)
- **PRD Phase**: N/A
- **Estimated Files**: ~8 (3 new components, 2 modified components, page, globals.css, 2 message catalogs)

---

## UX Design

### Before
```
[ Executive Overview ]              [Budget: pending HRIS]
┌ Actual headcount HERO (1.35fr) ┬ Budgeted | Vacancy | Fill ┐
└────────────────────────────────┴───────────────────────────┘
┌ Short-staffed (1/2) ┐ ┌ Pipeline (1/2) ┐
└─────────────────────┘ └────────────────┘
[ Sourcing chart full width ]
```

### After
```
[ Executive Overview ]        [Pending HRIS] [Print report]
┌ ACTUAL ┬ BUDGETED ┬ VACANCY ┬ FILL RATE ┐   ← flat 4-up summary band
│ 1,284  │  -  pend │   142   │  -  pend  │      (persistent, above tabs)
└────────┴──────────┴─────────┴───────────┘
[ Top Shortage* ] Headcount  Pipeline  Sourcing  ← tablist (active underlined)
┌──────────────────────────────────────────────┐
│ STAFFING · สาขาขาดคน                23 stores  │   ← one full-width board
│ Rank Branch         Area      Short  Fill      │     table per tab
│  1   Lotus Rama III  BKK South  -18   62% ▓▓░  │
└──────────────────────────────────────────────┘
(print → all panels stacked under headings, light theme, footer "as of {date}")
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Layout | hero + 2-col grid | summary band + tabs (1 board/tab) | core "ดูง่าย" change |
| Navigation | scroll | tabs, URL `?tab=`, keyboard arrows | shareable, a11y |
| Pending budget | bare `-` | dignified "pending HRIS" pill + notice | proves live data flows |
| Export | none | Print report → board pack | `window.print()` + `@media print` |
| Headcount detail | (none) | new Headcount/Vacancy-by-area table | uses existing store fields |

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `docs/executive-dashboard-UI-SPEC.md` | all | The design contract — every decision, token, key, ASCII layout |
| P0 | `frontend/components/executive/ExecutiveSections.tsx` | all | HeadcountBand/ShortStaffedPanel/PipelinePanel/DemoBadge/EmptyState/Stage/fillShade to refactor/reuse |
| P0 | `frontend/app/(app)/executive/page.tsx` | all | Current composition + gate + skeleton to rebuild |
| P0 | `frontend/app/globals.css` | 76-78, 206-291 | `--text-stat`, `.num`, `.eyebrow`, `.ledger-head`, `.ledger-row`, `.brass-underline`, `.settle`; where to add `@media print` |
| P1 | `frontend/lib/types.ts` | ExecutiveCompany/StoreFill/PipelinePosition/Source/ExecutiveOverview | exact data fields available (no new data) |
| P1 | `frontend/components/people/PeopleBits.tsx` | 97-150 | `Pill` + `PillTone` (`neutral`/`pending`) for pending-HRIS + DataSourceBadge |
| P1 | `frontend/components/analytics/Charts.tsx` | 348+ | `SourcesChart({sources})` (Sourcing tab, used as-is) |
| P1 | `frontend/app/(app)/members/page.tsx` | 1-101, 351-357 | Suspense + `useSearchParams`/`router.replace` URL-state + RBAC gate + restricted-state pattern |
| P1 | `frontend/lib/queries.ts` | 263-268 | `useExecutiveOverview(enabled)` + `useMe()` |
| P2 | `frontend/components/ui/table.tsx` | all | shadcn table primitive (board tables may use it, styled with ledger classes) |
| P2 | `frontend/components/shell/PageHeader.tsx` | all | masthead props (`eyebrow/title/meta/actions`) |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| WAI-ARIA Tabs pattern | w3.org/WAI/ARIA/apg/patterns/tabs | roving tabindex (active=0 others=-1), Arrow/Home/End move+activate, `role=tab/tablist/tabpanel`, `aria-selected/controls/labelledby` |

No library research needed — Radix is NOT a dependency; `ExecutiveTabs` is hand-built per the WAI-ARIA pattern.

---

## Patterns to Mirror

### FILL_SHADE + STORE FIELDS (keep exactly)
```tsx
// SOURCE: components/executive/ExecutiveSections.tsx:19-23
function fillShade(pct: number): string {
  if (pct < 70) return "var(--score-low)";
  if (pct < 85) return "var(--score-mid)";
  return "var(--brand)";
}
// ExecutiveStoreFill: store_no, store_name, subregion, budget_headcount, actual_headcount, heads_short, fill_rate_pct
```

### LEDGER TABLE CLASSES (use for board tables)
```
// SOURCE: app/globals.css:252-291
.ledger-head      // tinted hairline header, uppercase 0.6875rem th labels
.ledger-head th
.ledger-row       // hover/focus blue left-rail via ::before scaleY transform
.brass-underline  // eyebrow underline (brand)
```
Board tables: `<table>` with `<thead className="ledger-head">` + `<tr className="ledger-row">`; numeric columns `tabular-nums` right-aligned; Fill column carries a `fillShade` mini-bar (mirror the existing bar markup in `ShortStaffedPanel`).

### PILL (pending-HRIS + data-source)
```tsx
// SOURCE: components/people/PeopleBits.tsx:97-115
export type PillTone = "pass" | "fail" | "pending" | "neutral";
// neutral: bg-secondary text-secondary-foreground  → pending-HRIS
// pending: amber-ish → mock/demo data
<Pill tone="neutral">{t("pendingHrisShort")}</Pill>
```

### i18n CLIENT PATTERN
```tsx
// SOURCE: existing executive/page.tsx:18, ExecutiveSections (post-i18n)
const t = useTranslations("executive");
const locale = useLocale(); // pick title_th vs title_en where needed
t("tabShortage")            // add keys to BOTH messages/{en,th}.json
t("asOf", { date })         // ICU arg
```

### URL TAB STATE + SUSPENSE (mirror members page)
```tsx
// SOURCE: app/(app)/members/page.tsx:68-101, 351-357
export default function ExecutivePage() {
  return (<Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}><ExecutiveInner/></Suspense>);
}
// inside ExecutiveInner: const params = useSearchParams(); const router = useRouter();
// const tab = TABS.includes(params.get("tab")) ? params.get("tab") : "shortage";
// setTab: router.replace(`?tab=${next}`, { scroll: false })
```

### RBAC GATE + RESTRICTED STATE (keep current)
```tsx
// SOURCE: current executive/page.tsx:19-35
const allowed = canViewExecutive(me);
const { data, isLoading } = useExecutiveOverview(me ? allowed : false);
if (me && !allowed) return <restricted PageHeader + notAvailable/notAvailableHint card>;
```

### EXISTING SUMMARY BAND HAIRLINE GRID (refactor source)
```tsx
// SOURCE: components/executive/ExecutiveSections.tsx HeadcountBand
// grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline
// lead cell: bg-brand text-brand-foreground, brass left keyline, .num stat
// → CompanySummaryBand flattens to grid-cols-2 sm:grid-cols-4 (4-up scoreboard)
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `frontend/messages/en.json` + `th.json` | UPDATE | +~22 keys in `executive` ns (tab labels, headcount cols, pendingHris*, printReport, asOf, sourcingTitle, dataSourceLive, loadError) — BOTH locales |
| `frontend/components/executive/ExecutiveTabs.tsx` | CREATE | Hand-built accessible tablist (roving tabindex, Arrow/Home/End, URL `?tab=`) |
| `frontend/components/executive/CompanySummaryBand.tsx` | CREATE | Flat 4-up ledger scoreboard (refactor of HeadcountBand) + pending-HRIS-aware Budgeted/Fill cells |
| `frontend/components/executive/HeadcountVacancyPanel.tsx` | CREATE | New board table: Area/Branch / Actual / Budget / Vacancy / Fill (pending-aware) |
| `frontend/components/executive/ExecutiveSections.tsx` | UPDATE | ShortStaffedPanel → Rank/Branch/Area/Short/Fill board table + pending; DemoBadge → DataSourceBadge (extend pending logic); remove HeadcountBand (moved to CompanySummaryBand); keep PipelinePanel/EmptyState/Stage/Sep/fillShade |
| `frontend/app/(app)/executive/page.tsx` | UPDATE | New composition: Suspense + PageHeader(actions=DataSourceBadge+Print) + CompanySummaryBand + ExecutiveTabs(4 panels) + print footer; keep RBAC gate + skeleton |
| `frontend/app/globals.css` | UPDATE | Add `@media print` board-report rules (force light, show all tabpanels, hide chrome, break-inside-avoid, footer) |

## NOT Building
- No backend/API/data changes — `ExecutiveOverview` already returns company/stores/pipeline/sourcing; the redesign only re-presents it.
- No PeopleSoft/HRIS integration — budget stays `budget_available=false`; this plan only renders the pending-HRIS state (unlocking real budget is a separate future effort).
- No new RBAC/permissions — same `canViewExecutive` gate.
- No charting library — reuse `SourcesChart`; no new deps; no Radix.
- No automated visual-regression harness (repo has none) — verification is build + manual responsive/print/keyboard check.
- No dark-mode-specific overrides — tokens auto-swap; just verify legibility.

---

## Step-by-Step Tasks

### Task 1: i18n keys (both locales)
- **ACTION**: Add the ~22 new keys from UI-SPEC §4 to the `executive` namespace in `messages/en.json` AND `messages/th.json`, plus `loadError` (EN "Could not load the overview" / TH "โหลดภาพรวมไม่สำเร็จ").
- **IMPLEMENT**: keys: `tabShortage, tabHeadcount, tabPipeline, tabSourcing, tabsAria, headcountVacancyTitle, colBranch, colSubregion, colShort, colFill, colActual, colBudget, colVacancy, rank, pendingHrisTitle, pendingHrisBody, pendingHrisShort, printReport, asOf, sourcingTitle, dataSourceLive, loadError` (exact EN/TH from UI-SPEC §4 table).
- **MIRROR**: prior catalog merges this session (throwaway Node script preserving 2-space indent), or edit JSON directly.
- **GOTCHA**: add to BOTH files or `node scripts/check-i18n-parity.mjs` fails. NO em dash in any value.
- **VALIDATE**: `node scripts/check-i18n-parity.mjs` → "th/en in parity".

### Task 2: ExecutiveTabs (hand-built accessible)
- **ACTION**: Create `components/executive/ExecutiveTabs.tsx`.
- **IMPLEMENT**: props `{ tabs: {key,label,icon}[]; active: string; onChange:(k)=>void; children: ReactNode (active panel) }` OR render-children-by-key. `role="tablist"` + `aria-label={t("tabsAria")}`; each trigger `<button role="tab" id aria-selected aria-controls tabIndex={active?0:-1}>` with lucide icon (`size-4 strokeWidth={1.75}`) + label; active = `border-b-2 border-brand text-foreground font-semibold`, inactive `text-muted-foreground font-medium hover:text-foreground`; tablist on `border-b border-hairline`; mobile `flex overflow-x-auto`. Keyboard: ArrowLeft/Right move+activate, Home/End first/last (WAI-ARIA). One `<div role="tabpanel" aria-labelledby tabIndex={0}>` for the active tab; inactive `hidden` (for print, see Task 7).
- **MIRROR**: WAI-ARIA tabs pattern (External Documentation); icon usage from ExecutiveSections (`Store`/`Briefcase` `strokeWidth={1.75}`).
- **IMPORTS**: `useTranslations` (next-intl), lucide `Store, Building2, Briefcase, Share2`.
- **GOTCHA**: focus-visible ring via global `--ring`; do not add Radix. Keep panels mounted-but-`hidden` (not unmounted) so `@media print` can reveal all.
- **VALIDATE**: keyboard arrows cycle tabs; `aria-selected` flips; `tsc` clean.

### Task 3: CompanySummaryBand (refactor HeadcountBand)
- **ACTION**: Create `components/executive/CompanySummaryBand.tsx`; remove `HeadcountBand` from ExecutiveSections (Task 5).
- **IMPLEMENT**: `{ company: ExecutiveCompany }`. `grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline grid-cols-2 sm:grid-cols-4`, `aria-label={t("companyHeadcountAria")}`. Cell 1 lead: Actual headcount, `bg-brand text-brand-foreground`, brass left keyline, `.num` `--text-stat`, sub-line `headcountBudgetLine`(budget) / `headcountNoBudgetLine`(pending). Cells 2-4: Budgeted, Vacancy (always real), Fill rate. Budgeted + Fill use the pending-HRIS inline treatment (Task 6 helper) when `!company.budget_available`.
- **MIRROR**: HeadcountBand markup (ExecutiveSections) for the lead cell + supporting cells; `fmt = new Intl.NumberFormat("en-US")`.
- **GOTCHA**: do not put `leading-none` on Thai prose lines; `.num` only on numerals.
- **VALIDATE**: renders 4 cells; pending cells show `-` + neutral pill, real cells show figures.

### Task 4: HeadcountVacancyPanel (new board table)
- **ACTION**: Create `components/executive/HeadcountVacancyPanel.tsx`.
- **IMPLEMENT**: `{ stores: ExecutiveStoreFill[] }`. Section `rounded-xl bg-card p-6 ring-1 ring-hairline` with `<h2>` eyebrow `headcountVacancyTitle`. Board `<table>` `.ledger-head`/`.ledger-row`: columns Branch (`store_name`) / Area (`subregion`) / Actual (`actual_headcount`) / Budget (`budget_headcount`) / Vacancy (derive or `heads_short`) / Fill (`fill_rate_pct` + `fillShade` bar). Budget + Fill columns pending-aware (`-` + header `pendingHrisShort` pill) when budget unavailable; Actual always real. Empty → reuse `EmptyState`.
- **MIRROR**: ShortStaffedPanel table/bar markup; `fillShade`; ledger classes.
- **GOTCHA**: `tabular-nums` right-aligned numeric columns; `colSubregion`/`colActual`/etc keys.
- **VALIDATE**: table renders real Actual/Vacancy; Budget/Fill show pending state when applicable.

### Task 5: ExecutiveSections refactor
- **ACTION**: Modify `components/executive/ExecutiveSections.tsx`.
- **IMPLEMENT**: (a) `ShortStaffedPanel` → board table Rank / Branch / Area / Short / Fill (`.ledger-head`/`.ledger-row`), Rank = index+1, Short = `headsShort`, Fill = `fill_rate_pct` + `fillShade` bar; when `budget_available===false` render branch names with Short/Fill columns as `-` + `pendingHrisShort` (per UI-SPEC §5.4.B decision: show real names, pending columns). (b) Rename `DemoBadge` → `DataSourceBadge`: `mock`→`Pill tone="pending"` `demoData`; `live && !budget_available`→`Pill tone="neutral"` `pendingHrisShort`; else optional `dataSourceLive`. (c) Delete `HeadcountBand` (moved to CompanySummaryBand). (d) Keep `PipelinePanel`, `EmptyState`, `Stage`, `Sep`, `fillShade`.
- **MIRROR**: existing ShortStaffedPanel + DemoBadge in this file.
- **GOTCHA**: update the export name `DemoBadge`→`DataSourceBadge` and its single call site in page.tsx. ShortStaffedPanel now takes `budgetAvailable` (or read from a passed company flag) to drive pending columns.
- **VALIDATE**: `tsc` clean; no orphan import of HeadcountBand.

### Task 6: Page composition + pending helper
- **ACTION**: Rewrite `app/(app)/executive/page.tsx`.
- **IMPLEMENT**: `Suspense`-wrapped inner (reads `useSearchParams`). Keep RBAC gate + restricted state. Compose: `PageHeader(eyebrow/title/meta, actions={<><DataSourceBadge.../> <PrintButton/></>})`, `CompanySummaryBand`, `ExecutiveTabs` with 4 panels (shortage→ShortStaffedPanel, headcount→HeadcountVacancyPanel, pipeline→PipelinePanel, sourcing→SourcesChart), tab from `?tab=` default `shortage`, `router.replace("?tab="+k,{scroll:false})`. Print footer: `asOf` + data-source line (uses `data.generated_at`). Loading: skeletons per UI-SPEC §6. Error: `loadError` + retry. A small `pendingInline` helper (figure→`-`+`Pill neutral` when `!budget_available`) shared by CompanySummaryBand/panels (put in a tiny `executive/pending.tsx` or inline).
- **MIRROR**: members page Suspense+URL pattern; current executive gate/skeleton.
- **IMPORTS**: `Suspense, useState?`, `useSearchParams, useRouter`, `useTranslations`, `useMe, useExecutiveOverview`, `canViewExecutive`, the new components, lucide `Printer`.
- **GOTCHA**: `window.print()` only client-side (button onClick). `?tab=` invalid → default shortage. Keep `data.generated_at` for the footer (exists on ExecutiveOverview).
- **VALIDATE**: `tsc` + `next build`; page renders, tabs switch, URL updates.

### Task 7: Print stylesheet
- **ACTION**: Add `@media print` block to `app/globals.css`.
- **IMPLEMENT**: under `@media print`: force light tokens on the print container (board packs print on white even if `.dark`); reveal all panels — `[role="tabpanel"][hidden]{ display:block !important; }`; `print:hidden` (and explicit selectors) hide tablist, header action buttons, app sidebar/nav chrome; `table, .ledger-row { break-inside: avoid; }`; ensure the print footer (`asOf` + source) shows. Each tab panel keeps its `<h2>` heading so the stacked pack reads section-by-section in order: summary, shortage, headcount, pipeline, sourcing.
- **MIRROR**: existing global media-query rules (reduced-motion block) for structure.
- **GOTCHA**: don't break screen layout — scope everything inside `@media print`. Test with browser print preview.
- **VALIDATE**: print preview shows all sections stacked, light, no nav, footer present.

---

## Testing Strategy
The repo has no visual-regression/unit harness for dashboard components; verification is static + build + manual (consistent with prior executive/i18n work).

### Static + Build
| Check | Expected |
|---|---|
| `node scripts/check-i18n-parity.mjs` | th/en parity (new keys both locales) |
| `pnpm exec tsc --noEmit` | zero type errors |
| `pnpm exec eslint app components lib` | clean (ignore pre-existing AppHeader/LocaleSwitcher) |
| `pnpm exec next build` | green, `/executive` route emitted |
| `grep -c "—" docs + edited files` | 0 em dashes |

### Edge Cases Checklist
- [ ] `budget_available === false` → Budgeted/Fill/Top-Shortage show pending-HRIS (not red, not blank); Actual/Vacancy/Pipeline/Sourcing real
- [ ] `data_source === "mock"` → DataSourceBadge shows demoData (pending tone)
- [ ] empty arrays (stores/pipeline/sourcing) → per-panel EmptyState
- [ ] `?tab=` absent/invalid → defaults to shortage; valid → restores on reload
- [ ] role without `executive.view` → restricted state (no fetch)
- [ ] keyboard: Tab to tablist, Arrow/Home/End navigate, focus ring visible
- [ ] responsive 320/768/1024/1440 → no horizontal overflow; summary band 2→4 cols; tablist scrolls on mobile
- [ ] print preview → all panels stacked, light theme, chrome hidden, footer with asOf
- [ ] reduced-motion → instant transitions

### Manual Validation
- [ ] `pnpm dev`, open `/executive` (or prod with super_admin) → switch all 4 tabs, print preview, toggle TH/EN, resize.

---

## Validation Commands
```bash
cd /Users/nex/Documents/SourceCode/ats
node scripts/check-i18n-parity.mjs                       # parity
cd frontend
pnpm exec tsc --noEmit                                   # types
pnpm exec eslint "app/(app)/executive/page.tsx" components/executive   # lint new/edited
pnpm exec next build                                     # build (/executive emitted)
grep -rc "—" app/(app)/executive components/executive docs/executive-dashboard-UI-SPEC.md  # expect 0
```
EXPECT: parity green, 0 type/lint errors, build green, 0 em dashes.

---

## Acceptance Criteria
- [ ] All tasks completed; matches `docs/executive-dashboard-UI-SPEC.md`
- [ ] 4 tabs (shortage/headcount/pipeline/sourcing) with accessible keyboard nav + URL `?tab=`
- [ ] Persistent CompanySummaryBand above tabs; pending-HRIS state on Budgeted/Fill/Top-Shortage
- [ ] Print report path (all sections stacked, light, footer)
- [ ] Bilingual TH/EN (parity green), no em dash
- [ ] tsc/eslint/build green
- [ ] All 6 leadership requirements present (Budget vs Actual, Vacancy, Channel Performance, Fill Rate, Pipeline, Top Shortage)

## Completion Checklist
- [ ] Reuses existing tokens/classes (ledger, Pill, fillShade, .num, .settle) — no invented values
- [ ] No Radix / no new deps
- [ ] RBAC gate + restricted state preserved
- [ ] Skeletons + EmptyState + error state per spec
- [ ] a11y: WAI-ARIA tabs, focus rings, color-not-sole-signal, headings per section
- [ ] Self-contained — UI-SPEC + this plan suffice

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Hand-built tabs miss a WAI-ARIA detail | Med | Med | Follow the apg pattern exactly (roving tabindex, Arrow/Home/End, aria-controls/labelledby); manual keyboard test |
| Print CSS leaks into screen layout | Med | Med | Scope strictly inside `@media print`; verify screen unchanged + print preview |
| Pending-HRIS reads as "broken" | Low | Med | Neutral tone + body copy stating live data flows (UI-SPEC §5.4) |
| Panels unmounted break print "show all" | Med | Low | Keep panels mounted + `hidden`; print overrides `[hidden]` to `display:block` |
| i18n drift (key in one locale only) | Low | Low | parity script in validation |

## Notes
- Deploy mirrors prior dashboard rolls: **dashboard-only** (no api/migration). Build dashboard image with the **4 Entra build-args** (AUTHORITY=/organizations) + roll `hrats-prod-dashboard`. api/worker/scheduler/portal unchanged.
- Pairs with `EXECUTIVE_PROVIDER=real` (already on prod): the real Vacancy/Channel/Pipeline render fully; budget-pending panels show the new HRIS state until PeopleSoft is wired.
- `ExecutiveOverview` already exposes `data_source`, `generated_at`, `company.budget_available` — all the flags the redesign needs.
- Confidence: high — frontend-only, every token/class/component/hook verified present, contract fully specified in the UI-SPEC.
