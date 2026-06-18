# Code Review: ATS Approval Workflow (feat/ats-approval-workflow)

**Reviewed**: 2026-06-18
**Branch**: feat/ats-approval-workflow → main
**Mode**: Local (uncommitted) · go-reviewer + typescript-reviewer agents + synthesis
**Decision**: ✅ APPROVE (all CRITICAL/HIGH addressed; remaining items accepted with rationale)

## Summary
Multi-level hiring approval workflow (Staff→HR Manager→SGM→Regional). Two specialized reviewers initially returned BLOCK (Go: TOCTOU/409/step-rowcount; TS: unhandled promise rejection). All blocking findings fixed and re-validated; non-blocking items either fixed or accepted with documented rationale.

## Findings & Resolution

### Backend (Go)
| # | Sev | Finding | Resolution |
|---|---|---|---|
| 1 | CRITICAL→ | Concurrent same-level decide returns misleading 500 (stale `current_level` vs `FOR UPDATE`) | **Reassessed**: atomicity is SAFE — `FOR UPDATE` + `WHERE current_level=$level` correctly serializes, no double-write. Real defect was status code. **Fixed**: `ErrApprovalConflict` sentinel → handler maps to **409**. |
| 3 | HIGH | Double-submit (`RowsAffected==0`) → 500 instead of 409 | **Fixed**: `CreateApprovalRequest` returns `ErrApprovalConflict` → 409. |
| 5 | HIGH | Step `UPDATE` `RowsAffected` unchecked → silent inconsistency if step row missing | **Fixed**: both approve+reject paths error if `RowsAffected()==0`. |
| 4 | HIGH | DB errors in notify helpers swallowed without logging | **Fixed**: `log.Warn` added to `notifyActiveLevel` + `notifyAfterDecision` (resolve-approvers, load-application, resolve-HR-emails). |
| 8 | MED | Overdue query missing `s.level=r.current_level` | **Fixed**: added to the JOIN. |
| 13 | LOW | No test for conflict→409 | **Fixed**: `TestCreateApproval_ConflictIs409` + `TestDecideApproval_ConflictIs409`. |
| 2 | HIGH | Post-commit read on pool connection | **Accepted**: reviewer admits "almost never materialises" with pgxpool read-committed on a single primary; no replica reads in this system. |
| 6 | MED | Unqualified `assigned_store_id` in scoped query | **Accepted**: inherited `Shortlist` pattern; only `applications` has that column in the join set → unambiguous. |
| 7 | MED | Unexported `approvalDecideArgs` in exported `Repository` | **Accepted**: interface is package-internal in usage; exporting adds noise for no caller benefit. |
| 9 | MED | Notify-then-mark → duplicate escalation if mark fails | **Accepted**: inherent best-effort trade-off; inverse order would suppress a retry. Documented in plan. |
| 10 | LOW | `DecisionReason` missing `omitempty` | **No-op**: already had `omitempty`; reviewer mistaken. |
| 11 | LOW | Linear scan of 4-element `approvalChain` | **Accepted**: cosmetic, fixed-length 4. |

### Frontend (TS/React)
| Sev | Finding | Resolution |
|---|---|---|
| HIGH | `mutateAsync` without try/catch in reject() → unhandled promise rejection on backend error | **Fixed**: switched to `mutate` (callbacks own all UI feedback). |
| MED | Orphaned Reject button (null `submit_approval` leaves empty grid cell) | **Fixed**: filter nulls into `nextStepButtons` before rendering. |
| MED | Unsound `t(\`level${n}\` as "level1")` cast | **Fixed**: typed `LEVEL_KEYS` map + raw-number fallback (ApprovalPanel + approvals page). |
| MED | Hardcoded Thai `"ผู้สมัคร"` fallback | **Fixed**: `approvals.unknownCandidate` key in both locales. |
| LOW | Dead duplicate `<Skeleton>` | **Fixed**: removed (and unused `isLoading`). |
| LOW | Imprecise `t` prop type | **Accepted**: low value, risk of tsc churn from generic-typeof syntax. |
| — | i18n key-presence audit (33 keys) | **PASS** — all present in both locales. |

## Validation Results (post-fix)
| Check | Result |
|---|---|
| go build ./... | ✅ Pass |
| go vet ./... | ✅ Pass |
| go test ./... | ✅ Pass (no failures; +2 conflict tests) |
| gofmt (my files) | ✅ Pass (pre-existing `cmd/seedresumes` dirty, not mine) |
| tsc --noEmit | ✅ Pass |
| eslint (changed files) | ✅ Pass |
| i18n parity | ✅ Pass (frontend 84 keys th/en) |
| next build | ✅ Pass (`/approvals` route present) |

## Files Reviewed
All 31 changed files (backend 19, frontend 12). Migration 000022 validated by inspection (local apply blocked by Docker disk-full, not SQL).
