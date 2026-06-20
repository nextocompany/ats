---
phase: redesign
slug: executive-dashboard
status: approved
shadcn_initialized: true
preset: base-nova (existing)
created: 2026-06-21
---

# Executive Dashboard Redesign - UI Design Contract

> Visual and interaction contract for the redesigned Executive Overview at `frontend/app/(app)/executive/page.tsx`.
> Goal: "ดูง่าย เข้าใจง่าย เหมาะกับผู้บริหาร" - a board-report / print-style leadership pack, section-led, tabbed, with a dignified "pending HRIS" state for budget-dependent metrics.
> This is a REFINEMENT of the existing institutional "Ledger" system (CP Axtra navy + hairline rings + tabular-nums), NOT a rebrand.
> HARD RULE: never use the em dash (U+2014) or en dash (U+2013) anywhere (labels, copy, code comments). Use "-", ":", or a comma.

---

## 0. Design Goal & Direction (locked)

| Decision | Value |
|----------|-------|
| Visual direction | Board-report / print pack. Section-led, clear headings, summary tables, restrained, export/print friendly. |
| Design language | Existing "Ledger" (CP Axtra blue brand, hairline rules, `tabular-nums`, Anuphan display + IBM Plex Sans Thai Looped body). Refinement only. |
| Information architecture | TABS, not one dominating hero. Tabs: Top Shortage, Headcount/Vacancy, Pipeline, Sourcing. A persistent company summary band sits ABOVE the tabs. |
| Pending-HRIS metrics | Budget vs Actual, Fill Rate, Top Shortage ranking render with an explicit, dignified "รอเชื่อม HRIS / pending HRIS" treatment. Do NOT hide. |
| Bilingual | TH/EN via next-intl, default `th`. Every label has an i18n key in the `executive` namespace. |

---

## Design System

| Property | Value |
|----------|-------|
| Tool | shadcn (already initialized: `frontend/components.json`) |
| Preset/style | `base-nova`, baseColor `neutral`, cssVariables true |
| Component library | shadcn primitives only. Radix is NOT a dependency. Tabs MUST be hand-built (ARIA roving-tabindex) or via `shadcn add tabs` only if it resolves to a non-Radix `base-nova` Tabs. Default plan: hand-built `ExecutiveTabs` (see Section 5). |
| Icon library | `lucide-react` (already used: `Store`, `Briefcase`, `ArrowUpRight`). Add `Building2`, `Target`, `Radio`/`Share2` for tab icons. `strokeWidth={1.75}`, `size-4`. |
| Font | Body/UI: IBM Plex Sans Thai Looped (`--font-body`). Display/headings + stat numerals: Anuphan (`--font-heading` / `--font-display`, applied via `.num`). |
| State/data | Existing `useExecutiveOverview(enabled = true)` + `useMe()` TanStack Query hooks (pass the role-gate boolean as `enabled`). Tab state lives in URL search param `?tab=` (shareable, see Section 6). |
| Registry | shadcn official only. No third-party registries. Registry safety gate: not applicable. |

---

## 1. Layout & Spacing

### Spacing scale (existing tokens, multiples of 4)

| Token | Value | Usage in this phase |
|-------|-------|---------------------|
| xs | 4px (`gap-1`, `p-1`) | Icon-to-label gaps, inline pill padding |
| sm | 8px (`gap-2`, `py-2`) | Compact row spacing, pill gaps |
| md | 16px (`gap-4`, `p-4`) | Default element spacing, summary cell padding |
| lg | 24px (`gap-6`, `p-6`) | Card/section padding (the standard panel inset, matches existing panels) |
| xl | 32px (`gap-8`, `space-y-8`) | Vertical rhythm between page bands (matches current `space-y-8`) |
| 2xl | 48px (`py-12`) | Empty-state vertical padding (matches existing `EmptyState`) |
| 3xl | 64px | Reserved; not used on this dense exec surface |

Exceptions: hairline grids use a 1px gap over a `bg-hairline` background (`gap-px`) to draw cell separators without per-cell border math, exactly as `SummaryStrip` and `HeadcountBand` already do. The 1px gap is a deliberate hairline, not a spacing token.

### Page structure (component tree)

```
ExecutivePage (settle, space-y-8)
├── PageHeader (eyebrow / title / meta / actions=DataSourceBadge)   ← reuse shell/PageHeader
├── CompanySummaryBand  (persistent, ABOVE tabs)                    ← refactor of HeadcountBand
│     Actual headcount (lead) | Budgeted* | Vacancy | Fill rate*
│     (* = pending-HRIS treatment when company.budget_available === false)
├── ExecutiveTabs  (tablist: Top Shortage / Headcount / Pipeline / Sourcing)
│     ├── role="tablist"  (4 triggers, keyboard arrow nav)
│     └── role="tabpanel" (one active panel, others hidden)
│           ├── tab=shortage  → ShortStaffedPanel   (full-width board table)
│           ├── tab=headcount → HeadcountVacancyPanel(subregion/store table)
│           ├── tab=pipeline  → PipelinePanel        (funnel-by-position table)
│           └── tab=sourcing  → SourcesChart         (channel performance)
└── (print footer: generated_at timestamp + data-source line)
```

ASCII - desktop ≥1024 (board-report frame):

```
┌──────────────────────────────────────────────────────────────────────┐
│ LEADERSHIP                                          [Budget: pending HRIS]│
│ Executive Overview                                                      │
│ Company headcount, vacancies, pipeline & sourcing across all stores.    │
├──────────────────────────────────────────────────────────────────────┤  ← hairline (PageHeader border-b)
│ ┌───────────────┬──────────────┬──────────────┬──────────────┐         │
│ │ ACTUAL        │ BUDGETED     │ VACANCY      │ FILL RATE    │  Company │
│ │ 1,284  (lead) │  -  pending  │   142        │  -  pending  │  summary │
│ │ navy fill     │  HRIS        │  open vac.   │  HRIS        │  band    │
│ └───────────────┴──────────────┴──────────────┴──────────────┘         │
│                                                                          │
│ ┌ Top Shortage ┐ Headcount  Pipeline  Sourcing      ← tablist (active   │
│ └──────────────┘──────────────────────────────────    underlined brand) │
│ ┌──────────────────────────────────────────────────────────────────┐   │
│ │ STAFFING · สาขาขาดคน                              23 stores         │   │
│ │ (active tabpanel - full width board table)                         │   │
│ │ Rank  Branch              Subregion   Short   Fill                  │   │
│ │  1    Lotus Rama III       BKK South   -18    62%  ▓▓▓▓▓░░░         │   │
│ │  ...                                                                │   │
│ └──────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────┘
```

ASCII - mobile (≤768): the company summary band stacks to 2 columns (`grid-cols-2`), the tablist becomes a horizontally scrollable row (`overflow-x-auto`, no wrap), and the active panel is full-width single column.

### Grid rules

- Page container: existing `.settle space-y-8`. No change to outer shell.
- Company summary band: `grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline` with `grid-cols-2 sm:grid-cols-4` (board-report: a flat 4-up ledger strip, NOT the old asymmetric `1.35fr_2fr` hero split - the redesign demotes the hero to fit a leadership "scoreboard" read). Lead cell (Actual headcount) keeps a brass/blue left keyline via `accent`.
- Tab panels: each panel is ONE full-width `section` (`rounded-xl bg-card p-6 ring-1 ring-hairline`). The old `lg:grid-cols-2` side-by-side of Short-staffed + Pipeline is REPLACED by tabs - one focused board view at a time. This is the core "ดูง่าย" change.

---

## 2. Typography

Reuse the existing Swiss/Ledger scale. Declare exactly these roles for this surface:

| Role | Size token | Weight | Line height | Font | Usage |
|------|-----------|--------|-------------|------|-------|
| Display stat | `--text-stat` (2.75rem) via `.num` | 600 | 1 (`leading-none`) | Anuphan (`--font-display`) | The single lead figure (Actual headcount) in the summary band |
| Section heading | `text-lg` (1.125rem) | 600 | 1.14 | Anuphan (`--font-heading`) | Each tab panel `<h2>` title |
| Page title | `text-3xl` (1.875rem) | 600 | 1.14 | Anuphan | PageHeader `<h1>` (unchanged) |
| Supporting stat | `text-3xl` (1.875rem) | 600 | 1 (`leading-none`) | Anuphan via `.num`/`tabular-nums` | Budgeted / Vacancy / Fill-rate figures in summary band |
| Body | `text-sm` (0.875rem) | 400 | 1.5 | IBM Plex Sans Thai Looped | Row labels, branch names, position titles, hints |
| Micro / label | `text-xs` (0.75rem) | 400-500 | normal | body | Hints, counts, conversion deltas, subregion tags |
| Eyebrow | `0.6875rem` (`.eyebrow`) | 600 | normal | body, `uppercase tracking-[0.16em]` | Section eyebrows ("STAFFING", "HIRING"), tab-section kickers |

Rules:
- Weights used: 400 (body) and 600 (semibold). No other weights. `font-medium` (500) is permitted only for row-label emphasis, consistent with existing panels.
- All numerals that sit in columns use `tabular-nums` (ledger reads in columns). Big figures use `.num`.
- Headings (`h1`-`h4`) inherit the global `font-heading`, `letter-spacing: -0.015em`, `line-height: 1.14` from `globals.css` - do not override.
- Thai must never clip: rely on the global heading `line-height: 1.14`; do not set `leading-none` on any element containing Thai prose.
- Size discipline: the surface uses exactly 4 heading/stat sizes (`--text-stat`, `text-3xl`, `text-lg`, `text-sm`). `text-xs` and the `0.6875rem` eyebrow are the existing system label tier, NOT new heading steps - do not introduce additional sizes.

---

## 3. Color & Semantics

60/30/10 split (existing Ledger tokens):

| Role | Token | Value | Usage |
|------|-------|-------|-------|
| Dominant (60%) | `--paper` / `--background` | `oklch(98.6% 0.004 250)` near-white | App background, board "paper" |
| Surface | `--card` | `oklch(100% 0 0)` white | Summary cells, tab panels, tables |
| Secondary (30%) | `--secondary` / `--muted` | `oklch(96% 0.006 250)` | Bar tracks (`bg-muted`), ledger header tint, inactive tab text |
| Hairline | `--hairline` | `oklch(91% 0.006 255)` | All rules, ring-1, grid gaps, tab underline track |
| Accent (10%) | `--brand` | `oklch(46% 0.18 264)` CP Axtra blue | See reserved list below |
| Brass (= brand) | `--brass` | repointed to `--brand` (no yellow) | Lead-figure left keyline, eyebrow underline. Reads as the single blue emphasis. |

Accent (`--brand`) reserved EXCLUSIVELY for:
1. The summary band lead panel fill (Actual headcount) - `bg-brand text-brand-foreground`.
2. The active tab indicator (underline + label color).
3. The lead-figure left keyline (`--brass`, which equals brand) and eyebrow `brass-underline`.
4. The "hired" terminal stage in the pipeline funnel/row (`text-brand` strong).
5. The best-converting channel marker dot in Sourcing.
6. Focus rings (`--ring` = brand).
Never use brand as a generic fill on every interactive element.

Semantic fill-rate ramp (existing `fillShade`, keep exactly):

| Condition | Token | Meaning |
|-----------|-------|---------|
| `pct < 70` | `--score-low` `oklch(58% 0.18 27)` clay | Critically short-staffed (worst, eye lands here first) |
| `70 ≤ pct < 85` | `--score-mid` `oklch(72% 0.15 75)` amber | Watch / review |
| `pct ≥ 85` | `--brand` blue | Healthy |

Destructive: `--destructive` `oklch(56% 0.2 27)`. There are NO destructive actions on this read-only dashboard, so destructive color is unused here.

Pending-HRIS color: do NOT use clay/red or amber for pending metrics (pending is not an error). Use the neutral `Pill tone="neutral"` (quiet ink on `bg-secondary`) plus a `-` placeholder in `text-muted-foreground`. See Section 5.4.

Dark mode: all tokens auto-swap via `.dark` in `globals.css`. No phase-specific dark overrides; both themes must read intentionally (board pack stays legible on the navy dark surface).

---

## 4. Copywriting Contract (bilingual TH/EN)

All keys live in the `executive` namespace of `messages/en.json` and `messages/th.json` (both already have the namespace; TH has 31 keys). Keep existing keys; ADD the keys below. NO em dash in any value.

### Existing keys reused (do not change)
`eyebrow, title, meta, budgetPending, notAvailable, notAvailableHint, actualHeadcount, budgeted, approvedHeadcount, pendingHris, vacancy, openPositions, openVacancies, fillRate, ofBudgetFilled, headcountBudgetLine, headcountNoBudgetLine, staffing, mostShortStaffed, storesCount, noStaffingData, noStaffingHint, hiring, pipelineByPosition, appliedToHired, noPipeline, noPipelineHint, openCount, headsShort, companyHeadcountAria, demoData`

### New keys to ADD

| Key | EN | TH |
|-----|----|----|
| `tabShortage` | Top shortage | สาขาขาดคน |
| `tabHeadcount` | Headcount | กำลังคน |
| `tabPipeline` | Pipeline | ไปป์ไลน์ |
| `tabSourcing` | Sourcing | ช่องทางสรรหา |
| `tabsAria` | Executive views | มุมมองผู้บริหาร |
| `headcountVacancyTitle` | Headcount & vacancy by area | กำลังคนและตำแหน่งว่างตามพื้นที่ |
| `colBranch` | Branch | สาขา |
| `colSubregion` | Area | พื้นที่ |
| `colShort` | Short | ขาด |
| `colFill` | Fill | เติมคน |
| `colActual` | Actual | ปัจจุบัน |
| `colBudget` | Budget | งบกำลังคน |
| `colVacancy` | Vacancy | ว่าง |
| `rank` | Rank | อันดับ |
| `pendingHrisTitle` | Waiting for HRIS data | รอข้อมูลจาก HRIS |
| `pendingHrisBody` | Budget figures arrive once PeopleSoft / HRIS is connected. Actual headcount and vacancies below are live. | ตัวเลขงบกำลังคนจะแสดงเมื่อเชื่อมต่อ PeopleSoft / HRIS ส่วนกำลังคนปัจจุบันและตำแหน่งว่างด้านล่างเป็นข้อมูลจริง |
| `pendingHrisShort` | Pending HRIS | รอเชื่อม HRIS |
| `printReport` | Print report | พิมพ์รายงาน |
| `asOf` | As of {date} | ข้อมูล ณ {date} |
| `sourcingTitle` | Channel performance | ประสิทธิภาพช่องทาง |
| `dataSourceLive` | Live data | ข้อมูลจริง |

### Copywriting elements

| Element | Copy (key) |
|---------|-----------|
| Primary action (header) | `printReport` -> "Print report" / "พิมพ์รายงาน" (opens browser print of the board pack) |
| Empty: staffing | `noStaffingData` + `noStaffingHint` (existing) |
| Empty: pipeline | `noPipeline` + `noPipelineHint` (existing) |
| Empty: sourcing | `sourcesEmptyTitle` + `sourcesEmptyHint` (existing `analytics` namespace, reused by SourcesChart) |
| Pending-HRIS state | `pendingHrisTitle` + `pendingHrisBody` (full panel), `pendingHrisShort` (inline pill/badge) |
| Error / fetch fail | Reuse query-error pattern: heading + "ลองใหม่อีกครั้ง / Try again" retry. Add keys `loadError` ("Could not load the overview" / "โหลดภาพรวมไม่สำเร็จ") + reuse a shared `retry` key if present. |
| Role-denied | `notAvailable` + `notAvailableHint` (existing) |
| Destructive confirmation | none (read-only surface) |

Microcopy rules:
- Never echo raw tokens (UUIDs, "scored"). Branch and position names come from real fields (`store_name`, `title`).
- Pending placeholder is `-` (single hyphen) in `text-muted-foreground`, never blank, never "N/A", never a long dash.
- `headsShort` stays `-{n} heads` (hyphen prefix is the minus sign, allowed).

---

## 5. Design System / Component Contracts

### 5.1 Reused components (no change)
- `shell/PageHeader` - masthead. Pass `actions={<HeaderActions/>}` (DataSourceBadge + Print button).
- `components/people/PeopleBits` -> `Pill` (tones: `pass`/`fail`/`pending`/`neutral`). Pending-HRIS uses `tone="neutral"`; demo/mock uses `tone="pending"`.
- `analytics/Charts` -> `SourcesChart` (Sourcing tab, used as-is) and `FunnelChart` pattern for reference.
- `components/ui/skeleton` -> loading.
- `components/ui/table` (shadcn) -> the board tables for Headcount/Vacancy and Top Shortage MAY use this primitive; styled with the existing `.ledger-head` / `.ledger-row` classes from `globals.css` so they read as part of the system.

### 5.2 NEW: `ExecutiveTabs` (hand-built, accessible)
Path: `frontend/components/executive/ExecutiveTabs.tsx`.

Contract:
- `role="tablist"` with `aria-label={t("tabsAria")}`. Four `<button role="tab">` triggers, each with `id`, `aria-selected`, `aria-controls`, and roving `tabIndex` (active = 0, others = -1).
- One `<div role="tabpanel">` per tab with `aria-labelledby`, `tabIndex={0}`; inactive panels unmounted or `hidden`.
- Active indicator: brand-colored bottom border (`border-b-2 border-brand`) + `text-foreground font-semibold`; inactive triggers `text-muted-foreground font-medium`, hover `text-foreground`. Tablist sits on a hairline track (`border-b border-hairline`).
- Each trigger: lucide icon (`size-4`, `strokeWidth={1.75}`) + label. Icons: Top Shortage `Store`, Headcount `Building2`, Pipeline `Briefcase`, Sourcing `Share2`.
- Mobile: `flex overflow-x-auto` (horizontal scroll), no wrap, snap optional.
- Keyboard: ArrowLeft/ArrowRight move focus + activate (or focus-then-Enter), Home/End jump to first/last, per WAI-ARIA tabs pattern.
- Active tab persisted in URL `?tab=shortage|headcount|pipeline|sourcing` (default `shortage`).

### 5.3 NEW: `CompanySummaryBand` (refactor of `HeadcountBand`)
- Flat 4-up ledger strip (`grid-cols-2 sm:grid-cols-4`, `gap-px`, hairline grid), board-report scoreboard.
- Cell 1 (lead): Actual headcount, `bg-brand text-brand-foreground`, brass left keyline, `.num` stat figure, sub-line `headcountBudgetLine` (when budget) or `headcountNoBudgetLine` (when pending).
- Cells 2-4: Budgeted, Vacancy, Fill rate. Budgeted and Fill-rate are pending-HRIS-aware (see 5.4). Vacancy is always real.
- Persists ABOVE the tabs on every tab.

### 5.4 NEW: Pending-HRIS state (the key spec)
Two presentations, both dignified, neither "broken":

A. Inline (summary band cells: Budgeted, Fill rate):
   - Figure renders as `-` in `text-muted-foreground` (not the bold foreground figure).
   - Hint line shows `pendingHris` ("pending HRIS" / "pending HRIS") instead of the real hint.
   - A small `Pill tone="neutral"` with `pendingHrisShort` may sit in the cell corner. No red, no amber.

B. Full panel (Top Shortage tab when `budget_available === false`):
   - The branch ranking depends on budget-derived fill-rate/heads_short. When pending, render a centered, calm notice card (same `EmptyState` shell as existing, but neutral, not "no data"):
     - Icon: `Building2` in `bg-brand-soft text-brand` rounded tile (matches existing empty-state visual).
     - Title: `pendingHrisTitle`.
     - Body: `pendingHrisBody` (states clearly that actual headcount + vacancy below ARE live, so the exec knows the system works).
   - If the backend still returns store rows with names but zeroed budget fields, prefer showing the branch list with fill columns replaced by `-` + the `pendingHrisShort` pill in the header, rather than hiding the table. Decision: show branch names (real) with pending fill columns; this is more dignified than an empty card and proves data is flowing.

Header badge logic (existing `DemoBadge`, rename to `DataSourceBadge`):
   - `data_source === "mock"` -> `Pill tone="pending"` `demoData`.
   - `data_source === "live" && budget_available === false` -> `Pill tone="neutral"` `pendingHrisShort` (or existing `budgetPending`).
   - else -> small `dataSourceLive` neutral marker (optional) or nothing.

### 5.5 Tab panel specs

| Tab | Lead component | Data fields | Pending behavior |
|-----|---------------|-------------|------------------|
| `shortage` | `ShortStaffedPanel` as a board table (Rank / Branch / Area / Short / Fill bar) | `ExecutiveStoreFill[]` sorted asc by `fill_rate_pct`: `store_name, subregion, heads_short, fill_rate_pct` | Fill/Short columns -> `-` + `pendingHrisShort` when `budget_available===false`; branch names still shown |
| `headcount` | NEW `HeadcountVacancyPanel` table (Area or Branch / Actual / Budget / Vacancy / Fill) | `ExecutiveStoreFill[]`: `actual_headcount, budget_headcount, heads_short, fill_rate_pct` + company totals | Budget/Fill columns pending-aware; Actual + Vacancy always real |
| `pipeline` | `PipelinePanel` (position rows with funnel applied -> hired) | `ExecutivePipelinePosition[]`: `title, openings, applied, screening, interview, offer, hired` | Always real; no pending state |
| `sourcing` | `SourcesChart` (channel bars + conversion, brass-best) | `Source[]`: `channel, applied, hired, conversion` | Always real; empty-state if zero channels |

Board-table styling: use `.ledger-head` (tinted hairline header, uppercase `0.6875rem` labels) and `.ledger-row` (hover blue left-rail via `scaleY` transform) from `globals.css`. Numeric columns `tabular-nums`, right-aligned. Fill column carries the existing `fillShade` mini-bar.

---

## 6. Interaction & States

### Tab switching
- Click or keyboard activates a tab; `?tab=` updates via `router.replace` (shallow, no scroll jump). Reload/share restores the tab. Invalid/absent param -> `shortage`.
- Inactive panels are `hidden` (or unmounted); only the active panel is in the a11y tree.
- Transition: panel content uses the existing `.settle` entrance (opacity + 6px translateY, `--duration-normal` 280ms, `--ease-out`). No layout-bound animation. Respects reduced motion (global media query already zeroes it).

### Loading (skeletons)
- Before `data`: render `PageHeader` (static), then a `Skeleton h-28 rounded-xl` for the summary band, the tablist (static, disabled), and a `Skeleton h-72 rounded-xl` for the active panel. Mirrors current skeleton heights.
- `isLoading` true but no data -> skeletons; query settled with empty arrays -> per-panel EmptyState.

### Pending vs real data
- Driven solely by `data.company.budget_available` and `data.data_source`. The 3 budget-dependent reads (Budget vs Actual, Fill Rate, Top Shortage ranking) follow Section 5.4. Real reads (Vacancy, Channel Performance, Pipeline, Actual headcount, branch names) always render fully.

### Print / export (board-report)
- `printReport` action triggers `window.print()`.
- Add a print stylesheet (scoped `@media print` in `globals.css` or a `print:` Tailwind variant):
  - Force light theme (board packs print on white): print container uses paper/ink tokens regardless of `.dark`.
  - Show ALL tab panels stacked (override `hidden` for `role="tabpanel"` under `@media print`) so the printed pack contains every section, each under its heading, in this order: summary band, Top Shortage, Headcount/Vacancy, Pipeline, Sourcing.
  - Hide the tablist, header action buttons, sidebar/nav chrome (`print:hidden`).
  - Print the `generated_at` timestamp (`asOf {date}`) and the data-source line in a print footer.
  - Avoid page breaks inside a table row (`break-inside-avoid`).

### Responsive
| Breakpoint | Behavior |
|-----------|----------|
| 320 | Summary band `grid-cols-2`; tablist horizontal-scroll; tables collapse to label-stacked rows or a 2-col mini layout; no horizontal page overflow. Fill bars keep min 4% width so tiny values stay visible. |
| 375 | Same as 320 with slightly more breathing room. |
| 768 | Summary band `grid-cols-2` -> begins `sm:grid-cols-4`; tablist fits inline; tables show core columns. |
| 1024 | Full 4-up summary band; full board tables (all columns); tab panels full-width. |
| 1440 | Max content width respected by the app shell; no stretched line lengths; generous gutters. |

### Reduced motion
- All entrance/transition animations honor `prefers-reduced-motion: reduce` via the existing global rule (durations -> 0.01ms). Fill bars, tab indicator, and `.settle` all degrade to instant.

### Keyboard & focus / a11y
- Tablist implements the WAI-ARIA Tabs pattern (roving tabindex, Arrow/Home/End).
- Focus-visible ring uses `--ring` (brand) via global `outline-ring/50`. Every interactive element (tabs, print button) is keyboard reachable and shows a visible focus ring.
- Color is never the sole signal: pending uses a `-` + text label, fill-rate severity pairs the bar color with the numeric `fill_rate_pct` and `headsShort` text, best-channel pairs the brass dot with a "best" text suffix.
- Contrast: navy ink on white and brand-foreground on brand fill meet AA. Pending muted text on white must stay >= AA (use `--muted-foreground` `oklch(47% ...)`, which passes on white card).
- Each tab panel `<section>` has an accessible heading (`<h2>`); the summary band keeps `aria-label={companyHeadcountAria}`. Tables use `<caption>` or an `aria-label` naming the view.
- Live region: when a fetch error toast appears, announce via `sonner` (already present) `aria-live`.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | table, skeleton, badge, button (existing) + optional `tabs` if non-Radix | not required |
| third-party | none | not applicable |

If `npx shadcn add tabs` is run and resolves to a Radix-based implementation, do NOT add Radix as a dependency for this `base-nova` project; hand-build `ExecutiveTabs` per Section 5.2 instead.

---

## Checker Sign-Off

- [x] Dimension 1 Copywriting: bilingual keys added, no em dash, pending state worded with dignity
- [x] Dimension 2 Visuals: board-report frame, tabs, ledger tables, persistent summary band
- [x] Dimension 3 Color: 60/30/10 with brand accent reserved-list, semantic fill ramp, pending = neutral
- [x] Dimension 4 Typography: 2 weights, Anuphan display + Plex body, tabular-nums in columns
- [x] Dimension 5 Spacing: existing 4px scale + hairline gap-px grids
- [x] Dimension 6 Registry Safety: shadcn official only, no third-party

**Approval:** approved 2026-06-21
