# Code Review: ATS 3.9 — Reports

**Reviewed**: 2026-06-19
**Branch**: feat/ats-reports → main
**Mode**: Local (uncommitted, pre-PR)
**Decision**: APPROVE (1 finding fixed during review)

## Summary
Read-only aggregation slice extending `internal/reports` with RBAC-scoped, date-ranged hiring metrics + CSV, plus a new `/reports` dashboard page. Faithful to existing patterns (reports `*Repo` query style, `rbac.Scope`, dashboard panel/CSS conventions, `downloadFile` export). The headline risk — scope arg-index integration in the SQL — was traced per query and is correct. No CRITICAL/HIGH remain.

## Findings

### CRITICAL / HIGH
None.

### MEDIUM
- **Infinite skeleton on fetch error** — the page rendered `isLoading || !data ? <Skeleton>`, so a backend error (data undefined) showed the skeleton forever. **Fixed** → added `isError` branch rendering a `role="alert"` panel (`reports.loadFailed`, added to th/en).

### LOW
- **Wasted fetch for disallowed roles** — `useAtsReport` has no `enabled` flag, so a non-permitted role fires one request (→403→null) before the `notAvailable` gate renders. Harmless (graceful 403→null); left as-is to keep the hook signature simple.
- **Empty `ONBOARDING_REQUIRED_DOCS`** → completion forced to 0 (guarded in repo). Operator misconfig only.

## Verification notes (traced, correct)
- **Scope arg indices**: funnel/timing/offers/quality use fixed `$1,$2` (dates) + scope from `$3`; onboarding completion uses `$1,$2,$3(reqCount),$4(reqDocs ANY)` + scope from `$5`. `KindAll`→empty clause (no extra arg); `KindStore` nil-store→`1=0` (no arg); subregion/store→one arg. All placeholder counts match the appended args.
- **`assigned_store_id` unqualified** in the clause is unambiguous in every query (only `applications` has that column, even in the offers/onboarding/approval/interview JOINs).
- **Divide-by-zero** guarded everywhere via `pct()` (total 0 → 0); null aggregates `COALESCE(...,0)`; days/percent rounded 1dp.
- **Funnel = event-flag reached** (screened=scored, interview=appointment exists, offer=sent_at exists, hired=hired_at) — monotonic, documented.
- **`react-hooks/purity`**: date defaults moved into lazy `useState` initializers (no impure Date in render).

## Validation Results

| Check | Result |
|---|---|
| Go build / vet / gofmt | Pass |
| Go tests (full suite + 8 new) | Pass |
| Dashboard tsc / eslint / next build | Pass (`/reports` route built) |
| i18n parity (175 keys th/en) | Pass |
| DB aggregation SQL | Skipped (Docker disk-full) — inspection + handler/pure tests; operator validates on staging |

## Files Reviewed
Created: `internal/reports/ats_report.go`, `ats_report_csv.go`, `ats_report_handler.go`, `ats_report_test.go`; `frontend/app/(app)/reports/page.tsx`, `frontend/components/reports/ReportSections.tsx`.
Modified: `cmd/api/main.go`; `frontend/components/shell/nav-config.tsx`, `lib/queries.ts`, `lib/roles.ts`, `lib/types.ts`, `messages/{en,th}.json`.
