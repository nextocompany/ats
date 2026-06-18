# Implementation Report: ATS 3.9 — Reports

## Summary
Added an HR-facing **ATS Reports** surface: RBAC-scoped, date-ranged recruitment-funnel metrics over the Module-3 lifecycle (reached-funnel + conversion, time-to-hire & stage timing, offer + onboarding outcomes, interview + approval quality) with synchronous CSV export. Backend-only read aggregation (NO migration) over existing tables, extending `internal/reports`; one new dashboard page + nav + i18n. The last Module-3 slice.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 8/10 | Single-pass; only deviation = dropped `parsed_at` (column absent) |
| Files Changed | ~14 | 13 (6 created, 7 modified) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Confirm schema columns | ✅ | `parsed_at` absent → used time-to-offer instead of time-to-screen |
| 2-3 | `ats_report.go` types + `*Repo` aggregation | ✅ | funnel (event-flag reached), timing (percentile_cont median), offers, onboarding (vs config required set), quality; all scope-clause applied |
| 4 | `ats_report_csv.go` flatten | ✅ | `section,metric,value` |
| 5 | `ats_report_handler.go` | ✅ | role gate + scope build + parseRange + JSON & CSV + scopeLabel |
| 6 | Wire `cmd/api/main.go` | ✅ | reuses `reportRepo`, passes `cfg.OnboardingRequiredDocs()` |
| 7 | `ats_report_test.go` | ✅ | 8 tests (handler + pure) |
| 8 | FE types + `useAtsReport` | ✅ | 403→null |
| 9 | `/reports` page + `ReportSections` | ✅ | 4 panels, date range, CSV button, role gate |
| 10 | roles + nav | ✅ | `canViewReports`, `REPORTS_NAV` |
| 11 | i18n `reports` namespace + `nav.reports` | ✅ | parity green |
| 12 | Validation sweep | ✅ | see below |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go build`/`vet`/`gofmt -l` clean; dashboard `tsc` + `eslint` clean (fixed `react-hooks/purity` Date-in-render via lazy `useState` initializers) |
| Unit Tests | ✅ Pass | full `go test ./...` green; +8 reports tests |
| Build | ✅ Pass | dashboard `next build` exit 0 (`/reports` route present) |
| Integration | N/A | aggregation SQL not runnable locally (Docker disk-full); validated by inspection + handler/pure tests; operator validates on staging |
| Edge Cases | ✅ Pass | role 403, bad/inverted date 400, scope-from-user, CSV content-type, pct divide-by-zero, parseRange defaults |
| i18n parity | ✅ Pass | frontend 174 keys th/en in parity |

## Files Changed

### Created (6)
`backend/internal/reports/ats_report.go` (+260), `ats_report_csv.go` (+65), `ats_report_handler.go` (+135), `ats_report_test.go` (+220); `frontend/app/(app)/reports/page.tsx` (+110), `frontend/components/reports/ReportSections.tsx` (+150).

### Modified (7)
`backend/cmd/api/main.go`; `frontend/components/shell/nav-config.tsx`, `lib/queries.ts`, `lib/roles.ts`, `lib/types.ts`, `messages/{en,th}.json`.

## Deviations from Plan
- **Dropped time-to-screen metric** — `applications` has no `parsed_at`/`scored_at` column; replaced with **avg time-to-offer** (`offers.sent_at − applications.created_at`), which uses confirmed columns. Timing section = avg/median time-to-hire + time-to-offer + offer-response.
- **eslint react-hooks/purity** flagged `new Date()`/`Date.now()` in render — moved date defaults into **lazy `useState` initializers** (`today`/`from`/`to`).

## Issues Encountered
- `gofmt` realigned the new files (single-line helper funcs + struct comments) → `gofmt -w` applied.
- Confirmed `rbac.Scope.ApplicationsClause` returns a bare unqualified `assigned_store_id` condition — safe in every query (only `applications` has that column, even in JOINs).

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/reports/ats_report_test.go` | 8 | role gate 403, allowed JSON shape, scope-from-user (store 7), bad date 400, inverted range 400, CSV (text/csv + header), `pct` divide-by-zero table, `parseRange` defaults/date-only/errors, `EncodeATSCSV` rows |

## Next Steps
- [ ] Code review (`/code-review`)
- [ ] PR (`/prp-pr`) — fresh off main, no stacking
- [ ] Operator: rebuild/roll **api** + **dashboard** (5 Entra build-args). **No migration, no worker/scheduler, no career-portal.** Works on existing data immediately.
- [ ] Human UAT: `/reports` visible to HR roles, hidden otherwise; date range changes numbers; store HR scoped to their store; CSV download matches.
- [ ] **Module-3 complete** after this slice (3.4/3.1, 3.5, 3.6, 3.3, 3.8, 3.9).
