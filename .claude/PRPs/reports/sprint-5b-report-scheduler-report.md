# Implementation Report: Sprint 5b — Report Scheduler

## Summary
Added a recurring **report scheduler**: a dedicated `cmd/scheduler` (single replica) enqueues a `report:export` task on `REPORT_SCHEDULE_CRON`; the worker computes a funnel/KPI/source **snapshot**, renders it to CSV+JSON, stores both in Blob, persists a `report_exports` row, and delivers a signed CSV link via the **5a notifier seam**. HR can also trigger an export on demand and list/download past exports from a new Analytics "Scheduled exports" card. Introduces the project's first asynq periodic dispatcher.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium–Large | Medium–Large (as predicted) |
| Confidence | 8/10 | High — all 5 levels green, live scheduler→worker→deliver verified |
| Files Changed | ~12 | 19 (10 created, 9 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Config — schedule + recipients | ✅ | `REPORT_SCHEDULE_CRON` (default `0 7 * * 1`), `REPORT_RECIPIENTS` + `ReportRecipientList()` |
| 2 | reports snapshot + encoders | ✅ | `Snapshot()`, `EncodeCSV` (labelled sections), `EncodeJSON` |
| 3 | Migration | ✅ | `000007_report_exports`; round-trips |
| 4 | exports repo + export service | ✅ | `RecordExport`/`ListExports`; `ExportService.Export` (blob-fail = fail, deliver-fail = delivered:false) |
| 5 | queue task + worker handler | ✅ | `report:export` type + `HandleExportReport` (derives ISO-week period) |
| 6 | scheduler binary + compose | ✅ | `cmd/scheduler` (asynq.Scheduler, 1 replica); compose `scheduler` service via `SVC` build-arg |
| 7 | API endpoints | ✅ | `GET /api/v1/reports/exports` (signed links) + `POST` (role-gated on-demand) |
| 8 | dashboard card | ✅ | `ScheduledExports` on Analytics: list + download links + "Export now" |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; `golangci-lint` 0 issues; frontend `eslint` + `tsc` clean |
| Unit Tests | ✅ Pass | encoders (2) + export service integration (2); `go test -race` clean |
| Build | ✅ Pass | api + worker + **scheduler** binaries; `next build` (frontend) |
| Integration | ✅ Pass | `-tags integration ./internal/reports/...` (export store + deliver + no-recipient) |
| Live e2e | ✅ Pass | on-demand POST → 201 delivered + signed links; **scheduler cron → worker → blob + deliver** (delivered=true) |

## Files Changed

| File | Action |
|---|---|
| `internal/reports/{export,exports_repo,export_service,worker}.go` | CREATED |
| `internal/reports/{export_test,export_integration_test}.go` | CREATED |
| `cmd/scheduler/main.go` | CREATED |
| `migrations/000007_report_exports.{up,down}.sql` | CREATED |
| `frontend/components/analytics/ScheduledExports.tsx` | CREATED |
| `internal/reports/handler.go` | UPDATED (exports endpoints) |
| `pkg/queue/tasks.go` | UPDATED (`report:export`) |
| `pkg/config/config.go` | UPDATED (schedule/recipients + helper) |
| `cmd/worker/main.go` | UPDATED (export svc + handler) |
| `cmd/api/main.go` | UPDATED (export svc + reports handler deps) |
| `docker-compose.yml` | UPDATED (scheduler service + worker/api `REPORT_RECIPIENTS`) |
| `frontend/lib/{types,queries}.ts`, `app/(app)/analytics/page.tsx` | UPDATED |

## Deviations from Plan
- **Makefile not changed** — the existing `build` target is `go build ./...`, which already compiles `cmd/scheduler`. No edit needed.
- **On-demand export runs synchronously** in the api (not enqueued) so the response returns the created export immediately; the scheduler path uses the queue. Both share one `ExportService`.
- **Worker `REPORT_RECIPIENTS`** — the live test caught that scheduled delivery happens in the *worker*, so added `REPORT_RECIPIENTS` to the worker (and api) compose env; removed it from the scheduler (which only enqueues).

## Issues Encountered
- **Duplicate `dsn()`** in `internal/reports` (existing `reports_integration_test.go` already defines it) — removed mine, reused the existing helper.
- **Live: scheduled export delivered=false** initially — the worker compose env lacked `REPORT_RECIPIENTS`. Fixed; re-verified delivered=true on the next cron tick.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/reports/export_test.go` | 2 | CSV sections + JSON round-trip |
| `internal/reports/export_integration_test.go` | 2 | full export (store+deliver) + no-recipient path |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr` → squash-merge to `main`
- [ ] 5c (candidate search) — independent, can go next
