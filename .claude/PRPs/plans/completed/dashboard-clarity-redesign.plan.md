# Plan: Dashboard Clarity & De-noise Pass

## Summary
The HR dashboard app (Overview, Analytics, Inbox, Candidates, Search) reads as hard
and confusing because the GAN redesign optimized for editorial originality: jargon
labels ("Command center", "Pass-through", "Passed AI gate"), a **misleading up/down
delta arrow** on KPI metrics (implies a trend with no time comparison), the same
pass-rate repeated in 3 places, and decorative competition (dot-cluster, radial
glows, brass corner-ticks, an engineered funnel taper that hides real proportion).
This plan keeps each surface's structure but applies one **plain-language vocabulary**
and strips the decorative noise so the data reads at a glance.

## User Story
As an HR operator at CP Axtra, I want the dashboard to tell me plainly what the
numbers mean and what needs my attention, so that I can act without decoding jargon
or mistaking a decorative arrow for a performance trend.

## Problem → Solution
Metric-dense, jargon-labeled, visually noisy screens where the same number appears
three ways and a "↓" looks like bad news → calm, plain-language screens with one
canonical figure per concept, no misleading viz, and one disciplined brass accent.

## Metadata
- **Complexity**: Large (leaning XL) — 5 pages + 3 shared components + 1 CSS file + tests
- **Source PRD**: N/A (free-form request via /prp-plan)
- **PRD Phase**: N/A
- **Estimated Files**: ~9 (2 shared components, 5 pages, 1 CSS, 1 test) + 1 new copy module
- **Direction (locked with user)**: Clarity + reduce visual noise; KEEP existing layout/structure
- **Scope (locked with user)**: Whole dashboard app, applied consistently
- **Label language (decided)**: Plain **English**, to match the just-shipped inbox
  ("Meets requirements", "Strong fit"). Thai localization is explicitly out of scope here.

---

## UX Design

### Before (Overview `/dashboard`)
```
┌────────────────────────────────────────────────────────────┐
│ COMMAND CENTER ·                              ·:· (dots)    │
│ Overview                              [ Open ranked inbox → ]│
│ Live read of the national recruitment pipeline…            │
├────────────────────────────────────────────────────────────┤
│ ┌────────────┐ Passed AI gate  Onboarded   Awaiting review │
│ │TOTAL APPS  │   1,240 ↑68%cv    312 ↓41%cv    96          │  ← arrows mislead
│ │  ▌ 1,820   │                                              │
│ └────────────┘                                              │
├──────────────────────────┬─────────────────────────────────┤
│ Recruitment Funnel        │ OPERATOR FOCUS    [96 open]     │
│ (engineered taper)        │ Where to act                   │
│   Applied ▆▆▆▆ 1820       │  ▸ Review scored applications   │
│   Passed AI ▆▆ 1240       │  ▸ Top AI matches (≥75)         │
│   Reviewed ▆ 410          │  ▸ Flagged for manual review    │
│   Hired ▍ 96 ◆            │ ┌─ PASS-THROUGH ──────────────┐ │
│                           │ │ 68%  of applicants clear…   │ │ ← 3rd copy of pass-rate
│                           │ └─────────────────────────────┘ │
└──────────────────────────┴─────────────────────────────────┘
```

### After (Overview `/dashboard`)
```
┌────────────────────────────────────────────────────────────┐
│ Overview                              [ Open inbox → ]      │  ← drop "Command center" eyebrow + dot-cluster
│ A live read of recruitment — intake, screening, onboarding.│
├────────────────────────────────────────────────────────────┤
│ ┌────────────┐ Passed screening  Onboarded   Waiting for you│
│ │TOTAL APPS  │   1,240            312          96           │  ← no arrows; plain share hint below
│ │  ▌ 1,820   │   68% of applied   25% of passed  needs you  │
│ └────────────┘                                              │
├──────────────────────────┬─────────────────────────────────┤
│ Recruitment Funnel        │ Needs your attention   [96]     │  ← plainer heading
│ (honest proportion)       │  ▸ Review screened candidates   │
│   Applied ▆▆▆▆ 1820       │  ▸ Best-fit candidates (75+)    │
│   Passed screening ▆▆ 1240│  ▸ Needs a human check          │
│   Reviewed ▆ 410          │ ┌─ Screening pass rate ───────┐ │
│   Hired ▍ 96              │ │ 68%  clear screening.       │ │ ← single canonical pass-rate home
│                           │ │      312 onboarded this cycle│ │
│                           │ └─────────────────────────────┘ │
└──────────────────────────┴─────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| KPI supporting metric | `1,240 ↑68% conversion` | `1,240` + hint `68% of applied` | Remove arrow; arrow implied trend that doesn't exist |
| Pass-rate | shown in KPI hint + Pass-through card + funnel step (3×) | one canonical home (Pass-rate card on Overview; KPI lead bar on Analytics) | De-duplicate |
| "Passed AI gate" | jargon | "Passed screening" | Plain language, app-wide |
| Status pills | raw capitalized status ("Scored", "Parsed") | plain labels via `statusLabel()` | Consistent across Inbox/Candidates/Search |
| Candidate UUID | `a3f9c2e1` mono under name | subregion/province instead | Same fix already applied to Inbox |
| Decorative | dot-cluster, radial glows, corner-ticks, brass corner-dot | removed; keep one brass keyline per surface | Reduce competition |
| Funnel widths | engineered taper ceiling | honest proportion (min floor + monotonic clamp) | Width now means something |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `frontend/components/analytics/Charts.tsx` | 21-356 | KpiCards (hero+reporting), FunnelChart — the worst offenders for arrows/jargon/taper |
| P0 | `frontend/app/(app)/dashboard/page.tsx` | 1-163 | Overview: eyebrow/headings, Pass-through card, QuickAction copy |
| P0 | `frontend/components/people/PeopleBits.tsx` | (StatusPill + toneForStatus) | Where the shared `statusLabel()` is added; Pill system to reuse |
| P1 | `frontend/app/(app)/candidates/page.tsx` | all | UUID-under-name to remove; eyebrow/meta copy |
| P1 | `frontend/app/(app)/analytics/page.tsx` | all | Consumes KpiCards `variant="reporting"` + FunnelChart + SourcesChart |
| P1 | `frontend/app/(app)/applications/page.tsx` | all | Already humanized (PR #42) — the reference for tone/labels; reuse `Requirements`/`FitLabel` vocabulary |
| P1 | `frontend/components/shell/PageHeader.tsx` | all | `eyebrow` + `brass-underline` pattern every page uses |
| P1 | `frontend/components/shell/SummaryStrip.tsx` | all | Shared stat strip; label conventions |
| P2 | `frontend/app/globals.css` | 228-354 | `.eyebrow`, `.dot-cluster`, `.dot-rule`, `.brass-underline`, `.ledger-*`, `--text-stat`, `--ease-out` — the decorative layer to dial back |
| P2 | `frontend/components/shell/nav-config.tsx` | all | `BrandMark` brass corner-dot (a de-noise candidate); NAV labels (keep) |
| P2 | `frontend/e2e/dashboard.spec.ts` | all | E2E asserts heading text "Analytics", "Candidates", `getByText("Recruitment Funnel")` — keep these stable or update |
| P2 | `frontend/app/(app)/search/page.tsx` | all | Lightest touch; mostly fine |

## External Documentation
No external research needed — feature uses established internal patterns (Tailwind v4
utility classes, CSS custom properties in `globals.css`, lucide-react icons, the
existing Pill/SummaryStrip/PageHeader system). No new dependencies.

---

## Patterns to Mirror

### NAMING_CONVENTION
```tsx
// SOURCE: frontend/components/people/PeopleBits.tsx (toneForStatus / StatusPill)
const POSITIVE = new Set(["available", "active", "hired", "shortlisted", "interview", "onboarded", "pass", "passed"]);
export function toneForStatus(status: string): PillTone { /* maps status → tone */ }
export function StatusPill({ status }: { status: string }) {
  const tone = toneForStatus(status);
  const label = status ? status[0].toUpperCase() + status.slice(1) : "—";  // ← TECHNICAL: raw status shown
  return <Pill tone={tone}>{label}</Pill>;
}
```
Mirror this exact shape for the NEW `statusLabel()` — a `Record`/switch keyed by lowercased status, sibling to `toneForStatus`, returning plain words. `StatusPill` calls it instead of capitalizing.

### PLAIN_LANGUAGE_VOCABULARY (already shipped — match it)
```tsx
// SOURCE: frontend/app/(app)/applications/page.tsx (Requirements + FitLabel usage)
// "Meets requirements" / "Missing requirements" / "Pending"  ← the tone to match
// FitLabel: "Strong fit" / "Possible fit" / "Weak fit"
```
New copy must read like this — concrete, plain, no "AI gate"/"operator"/"pass-through".

### DERIVED_PRESENTATIONAL_METRIC (keep the derivation, drop the arrow)
```tsx
// SOURCE: frontend/components/analytics/Charts.tsx:29-37
const passRate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
const supporting: Metric[] = [
  { label: "Passed AI gate", value: kpi.passed, hint: `${passRate}% of applied`, delta: passRate }, // ← drop delta
  ...
];
```

### SECTION_CARD
```tsx
// SOURCE: frontend/app/(app)/dashboard/page.tsx:52-63
<section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
  <header className="flex items-baseline justify-between">
    <div><p className="eyebrow">Operator focus</p>
    <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">Where to act</h2></div>
  </header>
```
Keep this card shell; only swap copy. `rounded-xl bg-card ring-1 ring-hairline` is the universal surface.

### DECORATIVE_NOISE (the elements to remove)
```tsx
// SOURCE: dashboard/page.tsx:20-23 — dot-cluster atmosphere on hero
<div className="dot-cluster pointer-events-none absolute right-0 top-1 ..." aria-hidden />
// SOURCE: dashboard/page.tsx:94-98 — radial glow blob on Pass-through
<span aria-hidden className="... size-40 rounded-full opacity-25"
  style={{ background: "radial-gradient(circle, var(--brass) 0%, transparent 70%)" }} />
// SOURCE: Charts.tsx:60-64 — brass corner tick on KPI primary
<span aria-hidden className="... right-4 top-6 size-3 border-r border-t opacity-50 ..." />
// SOURCE: nav-config.tsx (BrandMark) — brass corner-dot on monogram
<span className="absolute -right-0.5 -top-0.5 size-2 rounded-full bg-brass" ... />
```
KEEP: the brass **left keyline** (`absolute inset-y-* left-0 w-[3px]`) — that is the single intentional accent per surface.

### TEST_STRUCTURE
```ts
// SOURCE: frontend/e2e/dashboard.spec.ts:43-53
test("analytics renders charts", async ({ page }) => {
  await page.goto("/analytics");
  await expect(page.getByRole("heading", { name: "Analytics" })).toBeVisible();
  await expect(page.getByText("Recruitment Funnel")).toBeVisible();   // ← keep this string stable
});
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `frontend/components/people/PeopleBits.tsx` | UPDATE | Add `statusLabel()`; `StatusPill` renders plain label |
| `frontend/components/analytics/Charts.tsx` | UPDATE | Remove delta arrows; relabel "Passed AI gate"→"Passed screening"; honest funnel widths; drop corner-tick |
| `frontend/app/(app)/dashboard/page.tsx` | UPDATE | Copy: eyebrow/headings/QuickActions/Pass-through; remove dot-cluster + glow; single pass-rate home |
| `frontend/app/(app)/candidates/page.tsx` | UPDATE | Remove UUID-under-name; plainer eyebrow/meta; summary-strip labels |
| `frontend/app/(app)/analytics/page.tsx` | UPDATE | Copy only (meta line); inherits Charts changes |
| `frontend/app/(app)/search/page.tsx` | UPDATE | Light copy polish only (eyebrow/meta) |
| `frontend/components/shell/nav-config.tsx` | UPDATE | Remove BrandMark brass corner-dot (de-noise) — optional, low-risk |
| `frontend/app/globals.css` | UPDATE (maybe) | Only if a decorative class becomes unused after removal; otherwise leave |
| `frontend/e2e/dashboard.spec.ts` | UPDATE (verify) | Confirm/adjust assertions if any asserted heading text changes |

## NOT Building
- **No layout restructure** — same grids, same card positions, same page order. (User chose "keep structure".)
- **No "what to do today" action-first redesign** — that was the rejected option.
- **No Thai localization** of labels — English plain language only this pass.
- **No new data/endpoints** — purely presentational; KPI/Funnel/Source shapes unchanged.
- **No changes to the Inbox `/applications`** beyond reusing its vocabulary — it was just humanized (PR #42) and is the reference, not a target.
- **No dark-mode rework**, no new charts, no animation overhaul.
- **No removal of the brass keyline** — keep one intentional accent per surface.

---

## Step-by-Step Tasks

### Task 1: Add shared `statusLabel()` and plain-language vocabulary
- **ACTION**: In `PeopleBits.tsx`, add an exported `statusLabel(status: string): string` next to `toneForStatus`, and make `StatusPill` use it.
- **IMPLEMENT**: A `Record<string,string>` (lowercased keys) → `{ pending:"Awaiting screening", parsed:"Profile ready", scored:"Screened", shortlisted:"Shortlisted", interview:"Interview", hired:"Hired", rejected:"Not selected", failed:"Could not process", available:"Available" }`. Fallback: capitalize raw (current behavior) for unknown keys. `StatusPill` label = `statusLabel(status)`.
- **MIRROR**: `toneForStatus` (same file) — same lookup shape, same lowercasing.
- **IMPORTS**: none new.
- **GOTCHA**: `toneForStatus` keys must still cover any new label semantics — do NOT change `toneForStatus`, only add `statusLabel`. Keep the `—` fallback for empty string.
- **VALIDATE**: `tsc --noEmit`; StatusPill on Inbox/Candidates/Search shows plain words.

### Task 2: KpiCards — remove misleading delta arrows, relabel
- **ACTION**: In `Charts.tsx` `KpiCards`, delete the `delta` field usage and the `ArrowUpRight/ArrowDownRight` block (lines ~89-106); relabel supporting metrics.
- **IMPLEMENT**: `supporting` becomes `{ label:"Passed screening", value:kpi.passed, hint:`${passRate}% of applied` }`, `{ label:"Onboarded", value:kpi.onboarded, hint:`${onboardRate}% of passed` }`, `{ label:"Waiting for you", value:kpi.waiting, hint:"needs an operator"→"awaiting your review" }`. Render only `value` + `hint` (no arrow, no "conversion" word).
- **MIRROR**: `DERIVED_PRESENTATIONAL_METRIC` snippet — keep `passRate`/`onboardRate` math, drop `delta`.
- **IMPORTS**: remove now-unused `ArrowUpRight, ArrowDownRight` from the top import IF SourcesChart no longer needs them (SourcesChart uses `ArrowUpRight` at line 401 — KEEP the import; only stop using it in KpiCards).
- **GOTCHA**: `ArrowUpRight` is still used by `SourcesChart` empty-state (Charts.tsx:401). Do not delete the import outright — verify usage first.
- **VALIDATE**: `tsc`; Overview + Analytics KPI rows show no arrows; `next build` lint passes (no unused import).

### Task 3: Funnel — honest proportional widths
- **ACTION**: In `FunnelChart`, drop the engineered `FUNNEL_TAPER` ceiling; width = honest proportion with a min floor, clamped so each stage ≤ previous (monotonic narrowing).
- **IMPLEMENT**: `widthFor(value, i, prevWidth)` → `clamp(max(FUNNEL_MIN_WIDTH, (value/max)*100), FUNNEL_MIN_WIDTH, prevWidth)`. First stage = `100`. Remove `FUNNEL_TAPER` const and its blend.
- **MIRROR**: existing `widthFor` (Charts.tsx:225-231) — same signature style, drop the ceiling term.
- **IMPORTS**: none.
- **GOTCHA**: connectors (`prevWidthPct`) must use the SAME computed widths; recompute prev from the clamped chain, not from `FUNNEL_TAPER`. Keep `FUNNEL_MIN_WIDTH` floor or a 2-of-1820 stage vanishes.
- **VALIDATE**: With seeded demo data, Hired band is visibly narrower than Reviewed which is narrower than Passed; widths track real ratios.

### Task 4: Overview page — copy + de-noise + single pass-rate
- **ACTION**: In `dashboard/page.tsx`: (a) remove the `dot-cluster` div (lines 20-23); (b) drop the "Command center" eyebrow (or change to none); (c) relabel "Operator focus"/"Where to act" → "Needs your attention", QuickActions copy (see table); (d) remove the radial-glow `<span>` (94-98) and keep only the brass keyline; (e) make the Pass-through card the single canonical pass-rate, relabel "Pass-through"→"Screening pass rate".
- **IMPLEMENT**: QuickActions: "Review scored applications"→"Review screened candidates"; "Top AI matches"/"Score ≥ 75 — fast-track" → "Best-fit candidates"/"Score 75+ — fast-track"; "Flagged for manual review"/"OCR / dedup edge cases" → "Needs a human check"/"Unclear scans or possible duplicates". Button "Open ranked inbox"→"Open inbox".
- **MIRROR**: `SECTION_CARD` shell; keep `QuickAction` component as-is (copy only).
- **IMPORTS**: none removed unless `dot-cluster` removal leaves nothing (it's a className, no import).
- **GOTCHA**: The Pass-through card already shows `{kpi.onboarded}`; keep that. Ensure pass-rate now lives ONLY here + the KPI hint `% of applied` — acceptable (hint is a sub-read, card is the headline). Do not also surface it as a 4th place.
- **VALIDATE**: Visual at 1440/768/320; no dot-cluster/glow; one bold brass keyline remains.

### Task 5: Candidates page — drop UUID, plainer copy
- **ACTION**: Remove the `c.id.slice(0, 8)` mono line under the name in BOTH mobile (line ~? `font-mono ... {c.id.slice(0,8)}`) and desktop rows; show subregion/province there instead. Relabel eyebrow "Talent records"→"Candidates", meta "the national roster"→"all candidates on file". SummaryStrip labels: "Active / shortlisted"→"Active", keep others.
- **IMPLEMENT**: Desktop secondary line already has `· {c.subregion}` — promote subregion/province to primary secondary text; delete the `{c.id.slice(0,8)}` span. Mobile: replace the mono id span with `c.subregion || c.province || ""`.
- **MIRROR**: the Inbox fix (PR #42) — name links to detail, no UUID surfaced.
- **IMPORTS**: none.
- **GOTCHA**: `InitialChip name={c.full_name}` already gives a handle; if `full_name` empty, keep a graceful fallback ("Unnamed candidate") consistent with Inbox.
- **VALIDATE**: `tsc`; no mono UUID visible; rows still link to `/candidates/:id`.

### Task 6: Analytics + Search — copy polish (inherit shared changes)
- **ACTION**: Analytics: update meta line if it references jargon (currently "Pipeline conversion, sourcing efficiency, and scheduled deliveries." — acceptable; leave). Confirm `KpiCards variant="reporting"` reads correctly post-Task 2. Search: eyebrow "Lookup" + meta are fine; no change needed beyond shared StatusPill/ScoreBadge.
- **IMPLEMENT**: Minimal — likely no edits to search; analytics inherits Charts. Verify "Passed AI gate" no longer appears anywhere (grep).
- **MIRROR**: n/a.
- **GOTCHA**: `KpiStrip` (reporting variant) has its own `"Passed AI gate"`? No — it reuses the `supporting` array from `KpiCards`, so Task 2 fixes it. Verify.
- **VALIDATE**: `grep -rn "Passed AI gate\|Pass-through\|Operator focus\|Command center" frontend/` returns nothing.

### Task 7: BrandMark de-noise (optional, low-risk)
- **ACTION**: In `nav-config.tsx` `BrandMark`, remove the brass corner-dot span. Keep the blue monogram + wordmark + brass "Recruitment" subtitle.
- **IMPLEMENT**: delete the `<span className="absolute -right-0.5 -top-0.5 size-2 rounded-full bg-brass" .../>`.
- **GOTCHA**: This is brand identity — confirm acceptable; if the brass dot is considered the logo signature, SKIP this task. Default: remove (it's one more competing brass mark).
- **VALIDATE**: visual; logo still legible.

### Task 8: Tests + grep sweep + build
- **ACTION**: Run the jargon grep sweep; update `e2e/dashboard.spec.ts` only if an asserted heading changed (Overview "Overview", Analytics "Analytics", Candidates "Candidates" all UNCHANGED; "Recruitment Funnel" UNCHANGED → no test edits expected). Update any unit assertions if present.
- **IMPLEMENT**: confirm no spec asserts removed strings ("Passed AI gate" etc.).
- **VALIDATE**: full build + (optional) local Playwright run.

---

## Testing Strategy

### Unit / Component Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `statusLabel("scored")` | "scored" | "Screened" | no |
| `statusLabel("PARSED")` | mixed case | "Profile ready" | case-insensitivity |
| `statusLabel("weird")` | unknown | "Weird" (capitalized fallback) | yes |
| `statusLabel("")` | empty | "—" | yes |
| KpiCards render | kpi with passed<50% of applied | no down-arrow shown | yes (the misleading case) |
| FunnelChart widths | hired=2, applied=1820 | hired band ≥ FUNNEL_MIN_WIDTH, < reviewed width | small-stage floor |
| FunnelChart monotonic | any funnel | each width ≤ previous | invariant |

> Note: project has no component-unit harness for these today (E2E + visual only). Add a
> minimal `statusLabel` unit test (pure function) — it's the one piece of testable logic.

### Edge Cases Checklist
- [ ] KPI with `applied === 0` (passRate guard → 0%, no NaN)
- [ ] Funnel all-zero (min floor keeps bands; no divide-by-zero — `max(…,1)`)
- [ ] Candidate with empty `full_name` → "Unnamed candidate" fallback
- [ ] Status value not in map → capitalized fallback
- [ ] 320px width — no dot-cluster reflow, no overflow

---

## Validation Commands

### Static Analysis
```bash
cd frontend && ./node_modules/.bin/tsc --noEmit -p tsconfig.json
```
EXPECT: Zero type errors

### Jargon sweep (must be empty)
```bash
cd frontend && grep -rn "Passed AI gate\|Pass-through\|Operator focus\|Command center\|ranked inbox" app components | grep -v "\.test\."
```
EXPECT: No matches (all replaced with plain language)

### Build (lint + compile)
```bash
cd frontend && ./node_modules/.bin/next build
```
EXPECT: Green; no unused-import lint errors (watch `ArrowUpRight/ArrowDownRight`)

### Browser / E2E (needs local stack)
```bash
# bring stack up (api :8080 mock+seeded), then dev server
cd /Users/nex/Documents/SourceCode/ats && docker compose up -d api
cd frontend && NEXT_PUBLIC_API_URL=http://localhost:8080 ./node_modules/.bin/next dev -p 3000
# in another shell, run the dashboard E2E
cd frontend && ./node_modules/.bin/playwright test e2e/dashboard.spec.ts
```
EXPECT: All dashboard.spec tests pass; screenshots in `e2e/__screens__/` look calm

### Manual Validation (screenshot at 320 / 768 / 1440)
- [ ] Overview: no dot-cluster, no radial glow, one brass keyline; pass-rate appears once (card) + as a KPI sub-hint only
- [ ] KPI band: no ↑/↓ arrows anywhere
- [ ] Funnel: Hired clearly narrower than Reviewed < Passed < Applied, proportional
- [ ] Candidates: no hex UUID under names; subregion/province shows instead
- [ ] Status pills read plainly ("Screened", "Not selected") across Inbox/Candidates/Search
- [ ] Both data and screens feel readable at a glance (the original complaint)

---

## Acceptance Criteria
- [ ] All tasks completed
- [ ] Jargon sweep empty; plain English vocabulary consistent app-wide
- [ ] No misleading delta arrows; pass-rate has one canonical home
- [ ] Funnel widths are honestly proportional + monotonic
- [ ] Decorative noise (dot-cluster, glows, corner-ticks) removed; one brass keyline kept per surface
- [ ] `tsc` clean, `next build` green, dashboard E2E passes
- [ ] Layout/structure unchanged (per locked direction)

## Completion Checklist
- [ ] Code follows discovered patterns (Pill system, SECTION_CARD, derived metrics)
- [ ] Error/empty states untouched and still work
- [ ] No hardcoded duplicate copy — shared `statusLabel()` is the single source for status words
- [ ] No unused imports (ArrowUpRight/Down)
- [ ] E2E assertions still valid
- [ ] No scope creep into action-first redesign or Thai localization
- [ ] Self-contained — implementable without further questions

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Removing arrows/`delta` leaves unused imports → lint fail | Med | Low | Task 2 GOTCHA: verify `ArrowUpRight` still used by SourcesChart before deleting import |
| Honest funnel makes near-equal stages look un-funnel-like | Med | Low | Keep `FUNNEL_MIN_WIDTH` floor + monotonic clamp; demo data has clear deltas |
| Changing a heading breaks `dashboard.spec.ts` | Low | Med | Headings kept stable (Overview/Analytics/Candidates/"Recruitment Funnel"); Task 8 greps specs first |
| Shared `statusLabel` shifts tone semantics | Low | Low | Only `statusLabel` added; `toneForStatus` (color) untouched |
| Coordinated frontend-only change deployed without rebuild | Low | Med | Frontend-only (no API change) → only dashboard image needs rebuild; see [[inbox-humanize-live]] deploy lesson |
| BrandMark dot removal seen as logo change | Low | Low | Task 7 is optional/flagged; skip if brass dot is the brand signature |

## Notes
- This is the **clarity counterpart** to the Inbox humanization (PR #42). Reuse its
  vocabulary and the `InitialChip`/`Pill`/`FitLabel` primitives — do not invent new ones.
- Deploy reminder (from [[inbox-humanize-live]] / [[cpaxtra-theme-live]]): this is a
  **frontend-only** change, so only the dashboard image must be rebuilt with the 4
  `NEXT_PUBLIC_AZURE_AD_*` build-args; the API image does NOT need a roll (no backend change).
- Consider splitting execution: **Phase A** = shared foundation (Tasks 1-3, the highest-leverage
  fixes touching every surface), **Phase B** = per-page copy/de-noise (Tasks 4-7), **Phase C** =
  test/build (Task 8). Each phase is independently shippable.
