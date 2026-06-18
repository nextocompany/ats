# Implementation Report: ATS Slice 2 — Multi-Level Hiring Approval Workflow (3.5)

## Summary
Implemented the four-level hiring approval chain (Staff → HR Manager → SGM → Regional Director). An interviewed candidate is submitted into a `pending_approval` state; each level approves or rejects-with-mandatory-reason; the final approval advances to `offer`, any reject ends in `rejected`. A scheduler/worker SLA sweep escalates steps left pending past their deadline. New `/approvals` dashboard queue + per-application `ApprovalPanel`, both role-gated, bilingual (TH/EN).

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 8.5/10 | Implemented single-pass, no design changes |
| Files Changed | ~22 | 31 (excl. plan/report) — split repo impl into its own file + 2 test files |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000022 (2 tables) | ✅ | Local apply blocked by Docker disk-full (env, not SQL); validated by inspection vs 000020/021 |
| 2 | Status const + state machine | ✅ | `pending_approval`; removed `interviewed→offer`; `CanRequestApproval` |
| 3 | Domain types `approval.go` | ✅ | + `roleForLevel`/`levelLabel` helpers |
| 4 | Repository methods | ✅ | Split into `approval_repository.go`; tx for create/decide; `FOR UPDATE` lock |
| 5 | HRDirectory `EmailsForRoleStore` | ✅ | store-scoped vs all-scope via `rbac.Kind()`; updated `fakeHRDir` stub |
| 6 | Notify builders | ✅ | Pending / Decided / Escalation HR builders |
| 7 | Approval handler + routes | ✅ | Create/Decide/GetForApplication/ListQueue; per-level gate |
| 8 | SLA sweep task + service | ✅ | `asynq.Unique`; best-effort dispatch |
| 9 | Config knobs | ✅ | `APPROVAL_SLA_ENABLED`/`_CRON`/`_HOURS` |
| 10 | Wire api/worker/scheduler | ✅ | gated cron registration |
| 11 | Frontend types | ✅ | |
| 12 | Roles + nav + statusMachine | ✅ | `APPROVALS_NAV`, `canAccessApprovals`, `roleLevel`, `canDecideApprovalLevel` |
| 13 | Frontend queries | ✅ | 4 hooks with key invalidation |
| 14 | Backend tests | ✅ | 13 approval/sla tests + transitions update |
| 15 | ApprovalPanel + /approvals page | ✅ | chain view + approve/reject; queue |
| 16 | Wire panel + drop Hire | ✅ | `submit_approval` renders nothing in AiSummaryPanel (panel owns it) |
| 17 | i18n catalogs | ✅ | 33-key `approvals.*` block + `nav.approvals` in both locales |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis (go build/vet) | ✅ Pass | `go build ./...` + `go vet ./...` clean |
| Static Analysis (tsc) | ✅ Pass | zero type errors |
| Lint (eslint) | ✅ Pass | my files clean; 2 pre-existing errors (LocaleSwitcher, AppHeader) + 1 pre-existing warning unchanged |
| gofmt | ✅ Pass | all new Go files formatted |
| Unit Tests (go) | ✅ Pass | full `go test ./...` green, no regressions |
| i18n parity | ✅ Pass | frontend 83 keys th/en in parity |
| Build (next build) | ✅ Pass | `/approvals` route present |
| Migration round-trip | ⚠️ Blocked | local Docker Postgres disk-full; operator runs on staging/prod |

## Files Changed
31 files (excluding plan + report): backend 19, frontend 12. +2535 / −14. See `git diff --stat`.

## Deviations from Plan
- **Repository impl in its own file** (`approval_repository.go`) instead of appending to `repository.go` — keeps `repository.go` focused; methods still on `pgRepository`, interface decls added to `repository.go`. Cosmetic.
- **`DecisionControls` reject UI is inline** (expanding form within the panel) rather than a separate `RejectDialog` instance — same mandatory-reason + `role="alert"` + spinner pattern, fewer moving parts, matches Scorecards' inline-form idiom.
- Added a `FOR UPDATE` row lock in `DecideApproval` (beyond the plan) to harden against concurrent double-decide of the same level.

## Issues Encountered
- `fakeHRDir` test stub failed to satisfy `HRDirectory` after adding `EmailsForRoleStore` — exactly the GOTCHA the plan flagged; added the stub method. Resolved.
- Local migration apply errored `No space left on device` — Docker VM disk, not the migration. Host has 42GB free; frontend build unaffected. Operator applies 000022 on staging/prod via the session migration recipe.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `approval_test.go` | 11 | Create gates (status/role/scope), Decide gates (level/reason/409/super_admin), advance/final/reject, queue level-filter |
| `approval_sla_test.go` | 2 | sweep escalates+marks overdue; no-overdue no-op |
| `transitions_test.go` (updated) | +1 | `interviewed→offer` now false, `pending_approval` sealed, `CanRequestApproval` |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr` (note the behavior change: one-click Hire removed → routes through approval)
- [ ] Operator: apply migration 000022 on staging/prod (migrate FIRST, then roll api/worker/scheduler); optionally set `APPROVAL_SLA_ENABLED=true` once HR is ready
- [ ] Human browser UAT of the chain on prod (login per role)
- [ ] Next ATS slice: 3.6 Offer Management
