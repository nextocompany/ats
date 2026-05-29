# Implementation Report: Sprint 5a — Re-engagement + Notifier seam

## Summary
Added a mock-default **Notifier seam** (`internal/notify`) and a **candidate re-engagement** flow (`internal/reengage`): when a vacancy opens (PeopleSoft webhook) or HR triggers it manually, an asynq job finds matching talent-pool / previously-rejected candidates for the position, notifies each once (suppressed via a `reengagement_contacts` table), and records an audit entry. Closes the long-deferred "Sprint 5" notification gap. The notifier seam is the dependency 5b will reuse for report delivery.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | High — all 5 levels green, live e2e verified |
| Files Changed | ~14 + migration pair | 20 (13 created, 7 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Config — notify toggle | ✅ Complete | `NOTIFY_PROVIDER`, `NOTIFY_LINE_TOKEN`, `NOTIFY_EMAIL_FROM`, `PORTAL_BASE_URL`, `UsesRealNotify()` |
| 2 | Notifier seam | ✅ Complete | interface + mock (log) + real LINE-push/email REST; mirrors peoplesoft seam |
| 3 | Migration | ✅ Complete | `000006_reengagement` — `reengagement_contacts` unique(candidate_id, position_id); round-trips |
| 4 | reengage repository | ✅ Complete | `MatchingCandidates` + `RecordContact` (ON CONFLICT DO NOTHING RETURNING) |
| 5 | reengage service | ✅ Complete | record-before-send; notify-failure-safe; audit `reengage` |
| 6 | queue task + worker | ✅ Complete | `vacancy:reengage` type + handler registered on the worker mux |
| 7 | webhook trigger + admin endpoint + wiring | ✅ Complete | `ReengageTrigger` iface fired on vacancy-open; `POST /api/v1/positions/:id/reengage` (RBAC) |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; `golangci-lint` 0 issues |
| Unit Tests | ✅ Pass | notify (3), reengage service (3), peoplesoft webhook (+2 new) — `go test -race ./...` all pass |
| Build | ✅ Pass | `go build ./...` incl. api + worker |
| Integration | ✅ Pass | `-tags integration ./internal/reengage/...` — matching + suppression against live PG |
| Edge Cases / live e2e | ✅ Pass | trigger→worker→mock-notify; 2nd run suppressed (sent=0, 1 row); audit recorded |

## Files Changed

| File | Action |
|---|---|
| `internal/notify/{notify,mock,rest,notify_test}.go` | CREATED |
| `internal/reengage/{repository,service,worker,trigger,handler}.go` | CREATED |
| `internal/reengage/{service_test,repository_integration_test}.go` | CREATED |
| `migrations/000006_reengagement.{up,down}.sql` | CREATED |
| `pkg/config/config.go` | UPDATED (+notify/portal config) |
| `pkg/queue/tasks.go` | UPDATED (+`vacancy:reengage`) |
| `internal/activity/activity.go` | UPDATED (+`ActionReengage`) |
| `cmd/worker/main.go` | UPDATED (build notifier+svc, register handler) |
| `cmd/api/main.go` | UPDATED (trigger wiring + reengage routes) |
| `internal/peoplesoft/webhook.go` | UPDATED (`ReengageTrigger` fired on vacancy-open) |
| `internal/peoplesoft/webhook_test.go` | UPDATED (signature + 2 trigger tests) |

## Deviations from Plan
- **Added `PORTAL_BASE_URL` config** (not in the plan's config task) — needed to build the apply link in notification bodies; reusable by 5b. Minor, additive.
- **Channel selection**: service prefers email, falls back to LINE-via-phone. Documented that real LINE push needs a stored LINE user id (deploy-time) — mock logs regardless.

## Issues Encountered
- **Integration tests truncate seed data** (existing project pattern — `applications/list_integration_test` and the new reengage test `TRUNCATE positions/vacancies/...`). After running them, re-ran `make seed` before the live e2e. Noted for the next sprint's e2e ordering.
- The existing `peoplesoft.NewHandler` callers in `webhook_test.go` needed the new 5th arg — updated to pass `nil` (re-engagement disabled), plus 2 new tests asserting the trigger fires only when the position maps.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/notify/notify_test.go` | 3 | factory selection + mock send |
| `internal/reengage/service_test.go` | 3 | send, suppression, notify-failure-safe |
| `internal/reengage/repository_integration_test.go` | 2 | matching set + contact suppression (live PG) |
| `internal/peoplesoft/webhook_test.go` | +2 | trigger fires on mapped / skips unmapped |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr` → squash-merge to `main`
- [ ] Then 5b (report scheduler) — reuses this notifier seam
