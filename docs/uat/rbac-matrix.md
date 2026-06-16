# RBAC Access Matrix — UAT checklist

Verifies each role sees only what it should, end-to-end (API + dashboard). The
SQL-scope logic is unit-tested in `internal/rbac/scope_test.go`; this is the
human/E2E sign-off layer.

## Roles → data scope (source: `internal/rbac/scope.go`)
| Role | Scope (Kind) | Sees |
|---|---|---|
| super_admin | all | every store/subregion |
| regional_director | all | every store/subregion |
| auditor | all (read-only) | every store/subregion; **no mutations** |
| operation_director | subregion | only their `subregion` |
| sgm | store | only their `store_id` |
| hr_manager | store | only their `store_id` |
| hr_staff | store | only their `store_id` |
| (store role, no store assigned) | fail-closed | **nothing** (`1=0`) |

## Feature-action gates (who can write)
| Action | Allowed roles | Source |
|---|---|---|
| Bulk CV upload | super_admin, hr_manager, sgm, hr_staff | `bulk_handler.go` |
| Record interview feedback | super_admin, hr_manager, sgm | `feedback_handler.go` |
| Member management | super_admin, hr_manager | `members/handler.go` |
| PDPA erase (anonymize) | super_admin | `members/handler.go` |
| Admin (settings, HR user CRUD) | super_admin | `settings`, `hrauth` |

## E2E checklist (run per role with a real login)
For each role below, log in (Entra or password account) and confirm:

- [ ] **super_admin** — sees all stores' candidates/applications; Admin + Members + Bulk nav visible.
- [ ] **regional_director** — sees all; no Admin/Members nav; can act within funnel.
- [ ] **auditor** — sees all; **mutation attempts blocked** (status change/bulk → 403); read-only.
- [ ] **operation_director** — inbox/candidates limited to their subregion only; another subregion's candidate → not found.
- [ ] **sgm** — only their store's candidates; Bulk + interview-feedback allowed; Members/Admin hidden.
- [ ] **hr_manager** — only their store; Members nav visible; Bulk + feedback allowed.
- [ ] **hr_staff** — only their store; Bulk allowed; **interview-feedback blocked (403)**; Members/Admin hidden.
- [ ] **store role with no store assigned** — sees no candidates (empty inbox), not an error.

## Negative checks (security-critical)
- [ ] A store-scoped user requesting another store's application by id → **404** (per-record `ExistsInScope`).
- [ ] A store-scoped user's list/search never returns out-of-scope rows.
- [ ] Unauthenticated request to any `/api/v1/*` (non-public) → **401**.
- [ ] Cross-origin state-changing request (foreign Origin) → **403** (EnforceOrigin).

Record pass/fail per box; any fail → security ticket (blocker).
