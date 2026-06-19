# Implementation Report: Dynamic RBAC — Phase 1 (backend foundation)

## Summary
Implemented the **foundation** of the dynamic RBAC plan: migration `000028` (roles / permissions / role_permissions tables + a seed reproducing the exact current matrix) and a new `internal/rbac` package (permission-key catalog, role/permission repository, and a TTL-cached `Authorizer` with super_admin code-bypass and fail-closed unknown-role handling). This phase is **purely additive — zero behavior change**: the existing hardcoded allowlists still enforce; nothing is wired in yet. A **parity test** proves the seeded matrix equals the legacy allowlists, de-risking the later cutover.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual (this phase) |
|---|---|---|
| Complexity | XL (whole feature) | Foundation slice = M |
| Confidence | 8/10 | Phase delivered green |
| Files Changed | ~30 (whole feature) | 7 created (this phase) |
| Phases done | 8 tasks total | Tasks 1, 2, 8(core) ✅ — Tasks 3–7 pending |

## Tasks Completed (this phase)
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration `000028_dynamic_rbac` (+seed) | ✅ Complete | up/down; seeds 21 perms, 7 built-in roles, exact matrix. **NOT yet applied to any DB** (operator-run per runbook) |
| 2 | rbac package (permissions/model/repository/authorizer) | ✅ Complete | pgxpool repo mirrors hrauth; authorizer TTL cache + super_admin bypass |
| 8 | rbac unit tests incl. **seed-matrix parity** | ✅ Complete | 7 tests, race-clean; parity + legacy-scope parity + bypass + fail-closed + reload |
| 3 | Cutover: scope.go + 16 handler swaps | ⏳ Pending | next phase |
| 4 | Admin API + `/me` permissions | ⏳ Pending | next phase |
| 5 | Frontend Me/can()/queries/nav | ⏳ Pending | next phase |
| 6 | Frontend Admin Roles & Permissions UI | ⏳ Pending | next phase |
| 7 | Remove dead allowlists + role-list consts | ⏳ Pending | next phase |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static (go build) | ✅ Pass | whole backend builds |
| Static (go vet) | ✅ Pass | `./...` clean |
| Unit Tests | ✅ Pass | `go test -race ./internal/rbac/...` ok; full `go test ./...` no regressions |
| Build | ✅ Pass | (Go build is the build) |
| Migration apply | ⚪ Deferred | Docker postgres disk-full locally (session norm); SQL inspected, additive + idempotent; operator applies on staging/prod per `docs/module-3-deploy-runbook.md` |

## Files Changed
| File | Action | Lines |
|---|---|---|
| `backend/migrations/000028_dynamic_rbac.up.sql` | CREATED | +~120 |
| `backend/migrations/000028_dynamic_rbac.down.sql` | CREATED | +4 |
| `backend/internal/rbac/permissions.go` | CREATED | +75 |
| `backend/internal/rbac/model.go` | CREATED | +60 |
| `backend/internal/rbac/repository.go` | CREATED | +260 |
| `backend/internal/rbac/authorizer.go` | CREATED | +135 |
| `backend/internal/rbac/authorizer_test.go` | CREATED | +210 |

## Deviations from Plan
- **Scoped to the foundation phase, not the whole plan.** The plan is XL and explicitly says "ship phase-by-phase / one PR per phase." Delivering all 8 tasks (16 handler edits + full frontend + admin UI) in one pass would risk a broken half-state across the codebase. This PR lands the safe, fully-tested, zero-behavior-change base; the cutover and frontend follow as their own PRs/phases. The plan is **kept active (not archived)**.
- `internal/rbac/scope.go` intentionally **untouched** this phase (its change is part of the Task 3 cutover) — keeps behavior identical.
- Repository uses an explicit `tx.Begin/Commit` for the multi-statement role+permissions writes (the matrix replace must be atomic). Single-statement reads/deletes stay on the pool, matching the hrauth style.

## Issues Encountered
None. Build/vet/test green first pass after writing.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `internal/rbac/authorizer_test.go` | 7 | seed↔legacy parity, scope parity, super_admin bypass, unknown/empty fail-closed, permissions list, reload propagation |

## Next Steps (remaining phases)
- [ ] **Phase 3 (cutover)** — `scope.go` consults the authorizer (package singleton); swap the 16 handler allowlists for `authz.Can(role, Perm…)`; wire authorizer in `cmd/api/main.go`. Parity test guards behavior.
- [ ] **Phase 4** — rbac admin API (`/api/v1/admin/rbac/*`) + `/users/me` returns `permissions[]`+`scope`; `hrauth` role validation via repo.
- [ ] **Phase 5–6** — frontend `can()` + data-driven role select + Admin Roles & Permissions matrix UI (+ i18n).
- [ ] **Phase 7** — delete dead allowlists / `*_ROLES` constants.
- [ ] Apply migration `000028` on staging, run the parity test against the real seeded DB, then deploy api+dashboard together.
