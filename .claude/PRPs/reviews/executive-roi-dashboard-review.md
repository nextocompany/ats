# Code Review: Executive ROI & Performance dashboard

**Reviewed**: 2026-06-26
**Branch**: `feat/executive-roi-dashboard` → `main` (3 commits, 22 files, +2035/-60)
**Reviewer**: self-review (go-reviewer + typescript-reviewer agents stalled on watchdog; review completed manually with full-file reads)
**Decision**: APPROVE

## Summary

Recruitment ROI & Performance dashboard replacing the dormant budget-dependent
exec 4-tab. Backend SQL aggregations are fully parameterized, RBAC is correct
(read = executive.view, cost-config write = settings.admin), period/dimension are
whitelisted, and all division paths are guarded. Frontend types mirror the Go JSON
exactly, i18n has full en/th parity (124=124), and the cost editor validates
client-side. No CRITICAL or HIGH issues. Validation all green.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
None.

### LOW
- **Conversion can exceed 100%** (`roi.go` success(), `SuccessRow.Conversion`): a row's
  hires count over the hired_at window while applications count over the created_at
  window, so a branch that hired from an older cohort can show >100%. This is a
  documented, deliberate tradeoff to keep Σ success.hires == headline Hires. Acceptable;
  noted for awareness (executives may find >100% conversion unintuitive).
- **CostConfigDialog parse()** (`CostConfigDialog.tsx:79`): a non-numeric entry silently
  becomes null (unset) rather than surfacing a validation error. `type="number"` inputs
  mostly prevent this. Minor UX.
- **SuccessByDimension React key** (`SuccessByDimension.tsx:52`): `r.key` is the SQL
  GROUP BY column-1 value; effectively unique per row. No practical collision risk.

## Validation Results

| Check | Result |
|---|---|
| go build ./... | Pass |
| go vet ./internal/executive/ | Pass |
| go unit test | Pass |
| go integration test (PG16, -tags=integration) | Pass |
| tsc --noEmit | Pass |
| eslint (changed exec files) | Pass |
| migration 000049 up/down reversible | Pass (clean DROP) |
| i18n en/th parity | Pass (124=124, no orphans) |

## Files Reviewed (all read in full)

Backend: handler.go (M), roi.go (A), roi_mock.go (A), service.go (M), types.go (M),
roi_integration_test.go (A), roi_mock_test.go (A), migrations 000049 up/down (A).
Frontend: executive/page.tsx (M), CostConfigDialog/ExecFilters/FunnelVolume/RoiBand/
SuccessByDimension/TimeToHirePanel (A), lib/{format,queries,roles,types}.ts (M),
messages/{en,th}.json (M).
