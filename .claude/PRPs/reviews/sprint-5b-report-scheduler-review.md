# Code Review: Sprint 5b — Report Scheduler (pre-PR)

**Reviewed**: 2026-05-30
**Branch**: `feat/sprint-5b-report-scheduler`
**Reviewer**: go-reviewer agent + maintainer triage
**Decision**: APPROVE (idempotency + correctness findings fixed; rest documented)

## Summary
Independent Go review surfaced 13 findings, including two CRITICAL idempotency gaps for
a scheduled/retried job. Nine were fixed; four documented with rationale. Re-validated:
static, race, integration, and a live double-POST idempotency check all pass.

## Findings & Disposition

| # | Sev | Finding | Disposition |
|---|---|---|---|
| 1 | CRIT | No uniqueness on `(kind,period)` → duplicate rows on asynq retry | **Fixed** — `UNIQUE (kind, period)` + `RecordExport` upsert (`ON CONFLICT DO UPDATE`). Verified live: 2× on-demand → 1 row |
| 2 | CRIT | No enqueue dedup → double-enqueue during rolling deploy | **Fixed** — `asynq.Unique(1h)` on the task (payload-scoped; distinct periods never collide) |
| 3 | HIGH | Deliver fired before the row was persisted (email without record; double email on retry) | **Fixed** — record(delivered=false) → deliver → `MarkDelivered` (non-fatal); retry only before delivery |
| 4 | HIGH | `ListExports` signing errors silently dropped | **Fixed** — `log.Warn` on sign failure |
| 5 | HIGH | Redundant `Flush`+dead error check after `WriteAll` | **Fixed** — single error check |
| 6 | HIGH | `downloadSignedTTL` 1h too short for report links | **Fixed** — raised to 7 days |
| 11 | MED | Export errors returned unwrapped | **Fixed** — `fmt.Errorf("reports: …: %w")` on snapshot/encode/upload |
| 12 | LOW | `VARCHAR(512)` blob cols vs codebase `TEXT` convention | **Fixed** — `TEXT` |
| — | — | stray `backend/scheduler` build binary | **Fixed** — removed + added `backend/.gitignore` |
| 10 | MED | "Set period at enqueue time" | **Won't apply** — `asynq.Scheduler.Register` enqueues a *fixed* task copy each tick; baking period at registration would freeze it to startup. Handler-time ISO-week derivation is correct here; documented in `cmd/scheduler/main.go`. |
| 7 | MED | Scheduler requires DB/blob creds it never uses | **Documented** — `config.Load` requires them; noted in `cmd/scheduler/main.go`. Splitting config is out of scope. |
| 8 | MED | Three DB round-trips, no sub-deadline | **Accepted** — covered by `asynq.Timeout(90s)`; no churn. |
| 13 | LOW | Package-level role map | **Won't change** — matches the 5a `reengage` pattern (consistency). |
| 4-auth | HIGH | POST role gate reads dev user | **Documented** — codebase-wide posture; fails closed in prod (same as 5a). |

## Validation (post-fix)

| Check | Result |
|---|---|
| `go vet` / `golangci-lint` | Pass (0 issues) |
| `go test -race` (reports, queue, config) | Pass |
| `go test -tags integration ./internal/reports/...` | Pass |
| Migration round-trip (TEXT + unique(kind,period)) | Pass |
| Live: 2× on-demand POST same week → 1 upserted row, delivered=true | Pass |
| Frontend lint / tsc / build | Pass |

## Files Reviewed
10 created + 9 updated, plus 4 review fixes touching `exports_repo.go`, `export_service.go`,
`export.go`, `handler.go`, `tasks.go`, `cmd/scheduler/main.go`, the migration, and a new
`backend/.gitignore`.
