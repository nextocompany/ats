# Code Review: Requisition / Vacancy Management (feat/requisition-management)

**Reviewed**: 2026-06-21
**Branch**: feat/requisition-management → main
**Mode**: Local (branch diff vs main; not yet pushed)
**Decision**: APPROVE (after fixes applied in-review)

## Summary
Self-reviewed the 20-file requisition feature. Found one HIGH correctness bug (FK
on actor columns breaks Entra-SSO users) and two minor issues; all fixed in-review.
Validation green across backend + frontend. No security issues.

## Findings

### CRITICAL
None.

### HIGH
1. **`created_by`/`approved_by` FK to `users(id)` breaks Entra-SSO actors** —
   `migrations/000029_requisitions.up.sql`. Entra users carry `DevUser.ID = OID` and
   are never inserted into `users` (that table is local password-login only). A
   manager/approver logged in via Microsoft SSO would hit a `23503` FK violation →
   500 on create/approve. **FIXED**: dropped `REFERENCES users(id)` — the columns are
   now plain nullable UUID audit pointers (the actor id is recorded, not constrained).

### MEDIUM
None.

### LOW
2. **Create dialog retained field values across reopens** —
   `app/(app)/requisitions/page.tsx`. The create dialog stayed mounted, so reopening
   showed the previously-typed position/store/headcount. **FIXED**: keyed the create
   dialog on open state so it remounts fresh.
3. **`err == pgx.ErrNoRows` instead of `errors.Is`** —
   `internal/requisitions/repository.go` `getByID`. Works today (pgx returns the
   sentinel directly) but the codebase convention is `errors.Is`. **FIXED**.

## Review Notes (verified OK)
- **Auth/scope**: every handler gates on `rbac.Can` (manage vs approve correctly
  split); list/detail bounded by `scope.VacanciesClause`; out-of-scope → 404 (no
  existence leak). super_admin bypass intact.
- **SQL**: parameterised throughout (no injection); scope-clause placeholder numbering
  (`len(args)+1` then append) is correct; PS rows immutable here via `source='manual'`
  + status guards (approve/close/update/delete RowsAffected→409).
- **No N+1**: List is COUNT + one JOINed SELECT, indexed by `idx_vacancies_source_status`.
- **Downstream**: approved (status='open') manual rows are picked up by the branch
  assigner / executive(real) / portal / reports automatically — no consumer change.
- Functions all <50 lines; no secrets, no console.log, no TODO; i18n both locales.

## Validation Results
| Check | Result |
|---|---|
| Type check (tsc) | Pass |
| Lint (eslint + gofmt + go vet) | Pass |
| Tests (go test ./... 32 pkgs; 7 requisition funcs) | Pass |
| Build (go build ./... + next build) | Pass |
| i18n parity | Pass (838 keys) |

## Files Reviewed
Added: `internal/requisitions/{model,repository,handler,handler_test}.go`,
`migrations/000029_requisitions.{up,down}.sql`, `app/(app)/requisitions/page.tsx`,
`components/requisitions/{RequisitionDialog,RequisitionTable}.tsx`.
Modified: `internal/rbac/{permissions,scope,authorizer_test}.go`, `cmd/api/main.go`,
`lib/{queries,roles,types}.ts`, `components/shell/nav-config.tsx`, `messages/{en,th}.json`.
