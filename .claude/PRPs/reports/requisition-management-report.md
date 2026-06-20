# Implementation Report: Requisition / Vacancy Management

## Summary
Added an HR/leadership surface to open, approve, edit and close position openings
(requisitions) as manual rows in the existing `vacancies` table. Approved
requisitions flow into branch assignment, the executive overview, the careers
portal, and reports automatically (they read `vacancies WHERE status='open'`). A
`pending_approval` status gives a free approval gate. RBAC-scoped (store/subregion/
all) via two new permission keys.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 9/10 | Single-pass, no blockers |
| Files Changed | ~16 + migration | 20 (incl. plan + tests) |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000029 (schema + RBAC seed) | Complete | ALTER vacancies +6 cols, +index, 2 perms, grants |
| 2 | RBAC catalog + scope | Complete | PermRequisitionManage/Approve, VacanciesClause; parity test updated |
| 3 | requisitions model + repository | Complete | List(scoped)+Create+Update+Approve+Close+Delete+ExistsInScope |
| 4 | requisitions handler + routes | Complete | gate(perm), validate, ExistsInScope→404, writeErr |
| 5 | Wire into cmd/api/main.go | Complete | registered after stores; import added |
| 6 | Backend tests | Complete | 7 test funcs (gating, validation, approve state machine, scope, normalize) |
| 7 | Frontend types + queries + roles | Complete | Requisition types, 4 query hooks, PERMS + can helpers |
| 8 | Frontend page + dialog + table + nav + i18n | Complete | /requisitions page, dialog, table, REQUISITIONS_NAV, requisitions namespace (838 keys) |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | Pass | gofmt + go vet clean; tsc + eslint clean |
| Unit Tests | Pass | backend `go test ./...` 32 pkgs ok; requisitions 7 funcs |
| Build | Pass | `go build ./...` + `next build` green (/requisitions route emitted) |
| Integration | N/A | migration applied by operator on deploy (no local DB — Docker disk-full) |
| Edge Cases | Pass | approve non-pending → 409; out-of-scope → 404; manage≠approve gating; headcount bounds |

## Files Changed
| File | Action | Lines |
|---|---|---|
| backend/migrations/000029_requisitions.{up,down}.sql | CREATE | +58 |
| backend/internal/requisitions/{model,repository,handler,handler_test}.go | CREATE | +751 |
| backend/internal/rbac/{permissions,scope}.go | UPDATE | +~30 |
| backend/internal/rbac/authorizer_test.go | UPDATE | skip 2 net-new perms in parity |
| backend/cmd/api/main.go | UPDATE | +5 (import + register) |
| frontend/app/(app)/requisitions/page.tsx | CREATE | +121 |
| frontend/components/requisitions/{RequisitionDialog,RequisitionTable}.tsx | CREATE | +268 |
| frontend/components/shell/nav-config.tsx | UPDATE | +10 |
| frontend/lib/{queries,roles,types}.ts | UPDATE | +109 |
| frontend/messages/{en,th}.json | UPDATE | +90 (requisitions ns + nav.requisitions) |

## Deviations from Plan
- **Routes live in handler.go (not a separate routes.go).** Matched the `rbacadmin`
  reference (RegisterRoutes in handler.go) rather than the `members` split — one fewer
  file, consistent with the closest CRUD analogue.
- **No 3-count summary strip on the page.** The plan sketched an open/pending/closed
  summary, but accurate cross-page counts need a stats endpoint (out of MVP scope). The
  page shows a status filter + paginated total instead — honest over page-local counts.
  A `/requisitions/stats` endpoint is an easy fast-follow if desired.
- **Filters in local state, not URL.** Avoided the `useSearchParams` + `Suspense`
  wrapper the members page needs; acceptable for a single status filter.

## Issues Encountered
- New permissions have no legacy allowlist, so `rbac.Can` returns false under the
  legacy fallback. Backend tests seed a dynamic authorizer via the exported
  `rbac.RoleReader` + `rbac.SetDefault` so gating resolves in-process. Resolved.
- `TestSeedMatrixParity` iterates `AllPermissions` against legacy allowlists; added the
  2 net-new keys to its skip list (same treatment as `PermRBACAdmin`).

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| internal/requisitions/handler_test.go | 7 funcs | list gating per role, create validation, create forbidden for staff, approve permission split, approve bad-state 409, out-of-scope 404, ListFilter.normalize |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr`
- [ ] Deploy: apply migration 000029 → restart api (pick up new perms) → roll dashboard (4 Entra args). worker/scheduler/portal need no rebuild.
- [ ] Human UAT: hr_manager opens a requisition → regional_director approves → confirm it appears in executive(real) Vacancy + portal jobs.
