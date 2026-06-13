# Code Review: Dashboard Clarity & De-noise Pass (local)

**Reviewed**: 2026-06-11
**Branch**: `feat/dashboard-clarity-redesign` → `main`
**Mode**: Local (uncommitted changes) + independent typescript-reviewer pass
**Decision**: ✅ APPROVE (2 findings fixed during review)

## Summary
A focused, presentational frontend change (4 files) replacing jargon with plain language,
removing misleading KPI delta arrows, making the funnel honestly proportional, and stripping
decorative noise. No security surface, no data/logic risk. Two reviewer findings (1 HIGH, 1
MEDIUM) were fixed in-review; all validation green.

## Findings

### CRITICAL
None.

### HIGH
1. **`PeopleBits.tsx` — `STATUS_LABELS` coverage gaps** *(FIXED)*
   Recognized statuses (`in_review`, `waiting`, `active`, `onboarded`, `passed`, `dropped`,
   `withdrawn`, `inactive`, `review`) fell through to raw capitalization — `in_review` → "In_review"
   reached users, contradicting the "one vocabulary" goal. **Fix**: added all real status values to
   the map; only `pass`/`fail` (internal tone tokens, never status field values) intentionally left
   to the harmless capitalized fallback.

### MEDIUM
1. **`candidates/page.tsx` — mobile province rendered twice** *(FIXED)*
   When `subregion` was empty, the mobile card showed `c.province` under the name AND again in the
   MapPin row. **Fix**: mirrored the desktop logic — show `subregion` only under the name; the
   MapPin row carries province exclusively. Now consistent across breakpoints.

### LOW (accepted, not changed)
- Funnel comment says "narrows monotonically"; strictly it's non-increasing (equal widths when two
  stages hit `FUNNEL_MIN_WIDTH`). Connector clip-path degrades gracefully to a rectangle — inert, no
  bug. Wording nit only.
- Stale code comment `// "oklch(53% 0.15 240)", // Passed AI` (non-user-facing) left for diff tightness.
- `status ?? ""` guard is formally redundant vs the `string` type but kept as correct defensive
  practice at the JSON boundary.

### Confirmed sound (reviewer-verified)
- Funnel `widths` reduce: no off-by-one, no div-by-zero (`Math.max(funnel.applied, 1)`), monotonic
  clamp via `Math.min(honest, acc[i-1])` correct.
- `ArrowDownRight` import cleanly removed; `ArrowUpRight` correctly retained (used by `SourcesChart`).
- `relative overflow-hidden` removed together with the `dot-cluster` it contained — no orphaned
  positioning context.
- No `any`, no unsafe casts, no console.log, no secrets, React auto-escapes all rendered text.

## Validation Results
| Check | Result |
|---|---|
| Type check (tsc) | ✅ Pass |
| Lint (eslint / next build) | ✅ Pass |
| Tests (e2e/dashboard.spec.ts) | ✅ Pass 5/5 |
| Build (next build) | ✅ Pass |

## Files Reviewed
- `frontend/components/people/PeopleBits.tsx` — Modified (statusLabel + expanded map)
- `frontend/components/analytics/Charts.tsx` — Modified (arrows/funnel/labels/corner-tick)
- `frontend/app/(app)/dashboard/page.tsx` — Modified (de-noise + copy)
- `frontend/app/(app)/candidates/page.tsx` — Modified (UUID→region, mobile dup fix)
