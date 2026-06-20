# Implementation Report: Dynamic RBAC (UAT #6) — all phases complete

> Originally written for Phase 1 (backend foundation); task table + next-steps
> updated as the remaining phases landed. Feature is code-complete (PRs #113–#117,
> main `c8fc69b`); only deploy + UAT remain.

## Summary
Implemented the **foundation** of the dynamic RBAC plan: migration `000028` (roles / permissions / role_permissions tables + a seed reproducing the exact current matrix) and a new `internal/rbac` package (permission-key catalog, role/permission repository, and a TTL-cached `Authorizer` with super_admin code-bypass and fail-closed unknown-role handling). This phase is **purely additive — zero behavior change**: the existing hardcoded allowlists still enforce; nothing is wired in yet. A **parity test** proves the seeded matrix equals the legacy allowlists, de-risking the later cutover.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual (this phase) |
|---|---|---|
| Complexity | XL (whole feature) | Foundation slice = M |
| Confidence | 8/10 | Phase delivered green |
| Files Changed | ~30 (whole feature) | 7 created (this phase) |
| Phases done | 8 tasks total | **All 8 tasks ✅ — feature code-complete (PRs #113–#117)** |

## Tasks Completed (this phase)
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration `000028_dynamic_rbac` (+seed) | ✅ Complete | up/down; seeds 21 perms, 7 built-in roles, exact matrix. **NOT yet applied to any DB** (operator-run per runbook) |
| 2 | rbac package (permissions/model/repository/authorizer) | ✅ Complete | pgxpool repo mirrors hrauth; authorizer TTL cache + super_admin bypass |
| 8 | rbac unit tests incl. **seed-matrix parity** | ✅ Complete | 7 tests, race-clean; parity + legacy-scope parity + bypass + fail-closed + reload |
| 3 | Cutover: scope.go + 16 handler swaps | ✅ Complete | PR #114 — singleton + legacy fallback; all 16 gates → rbac.Can; main wires DB authorizer (guarded); build/vet/test green |
| 4 | Admin API + `/me` permissions | ✅ Complete | PR #115 — new `internal/rbacadmin` pkg (dodges import cycle); `/api/v1/admin/rbac/*` CRUD gated by rbac.admin; super_admin/builtin/in-use delete guards; `/users/me` returns permissions[]+scope; hrauth role validator |
| 5 | Frontend Me/can()/queries/nav | ✅ Complete | PR #116 — Me.permissions+scope; lib/roles.ts → `can(me,perm)`; ~18 call sites pass me; navForRole(me); rbac query hooks; tsc/eslint/build/parity green |
| 6 | Frontend Admin Roles & Permissions UI | ✅ Complete | PR #117 — RolesPermissions.tsx matrix editor (grouped checkboxes + scope select + new-role form; super_admin locked); UserManagement RoleSelect → useRbacRoles; mounted on Admin page; admin i18n keys (catalog 743) |
| 7 | Remove dead allowlists + role-list consts | ✅ Complete | PR #117 — backend allowlists already removed in #114; deleted frontend HR_ROLES transitional const |

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

## Next Steps (remaining)
All 8 implementation tasks are complete and merged (PRs #113–#117, main `c8fc69b`).
The plan is archived to `PRPs/plans/completed/`. Only deploy + UAT remain:
- [ ] Apply migration `000028` on staging/prod (operator, manual) — then **roll api + dashboard** together (dashboard needs the 4 Entra build-args incl. `AUTHORITY=/organizations`). Safe in either order: the `legacy.go` fallback keeps pre-RBAC behavior until the DB authorizer is installed.
- [ ] Run the seed-matrix **parity test against the real seeded DB** after applying `000028`.
- [ ] **Human browser UAT** — create a custom role, toggle permissions, assign to a user, verify scope + gating; confirm the super_admin row is locked.
