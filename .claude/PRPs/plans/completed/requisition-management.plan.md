# Plan: Requisition / Vacancy Management

## Summary
Give HR and leadership a dashboard surface to **open, approve, edit and close position openings (requisitions)** instead of relying solely on PeopleSoft-imported vacancies. Built directly on the existing `vacancies` table so an approved requisition is **immediately reflected** in branch assignment, the executive overview, the careers portal, and reports — all of which already read `vacancies WHERE status = 'open'`. A new `pending_approval` status gives a free approval gate (downstream consumers ignore non-`open` rows until approved).

## User Story
As an **HR manager or store GM**, I want to open a new requisition for a position at my store and have it approved by a regional/operation director, so that the opening flows into candidate branch-assignment, the careers portal, and the executive vacancy counts without waiting on a PeopleSoft sync.

## Problem → Solution
**Current:** `vacancies` rows are written ONLY by the PeopleSoft webhook (`internal/peoplesoft/webhook.go`, keyed on `ps_vacancy_id`). There is no HR-facing create/edit/close. Manual openings are impossible.
**Desired:** A `/requisitions` dashboard page + `/api/v1/requisitions` CRUD that writes manual `vacancies` rows (source=`manual`, `ps_vacancy_id` NULL) through a `draft/pending_approval → open → closed/cancelled` lifecycle, RBAC-scoped (store/subregion/all), with an approval step before a requisition goes `open`.

## Metadata
- **Complexity**: Large
- **Source PRD**: N/A (free-form feature request)
- **PRD Phase**: N/A
- **Estimated Files**: ~16 (7 backend new/edit, 1 migration, ~8 frontend new/edit)

---

## Key Design Decisions

1. **Extend `vacancies`, do NOT create a new `requisitions` table.** Every downstream consumer already queries `vacancies WHERE status='open'` (branch assigner `internal/vacancies/model.go:57-82`, executive `internal/executive/live.go:90-161`, portal `internal/positions/model.go:151-177`, reports `internal/reports/reports.go:121-150`). Reusing the table means an approved manual requisition is live everywhere with zero changes to those consumers. A separate table would require duplication or sync.
2. **A new `internal/requisitions` package** owns the manual write/list path; the existing `internal/vacancies` (PeopleSoft read + upsert) stays **untouched** so the PS webhook path cannot regress. Both operate on the same table.
3. **Approval gate via status, not a new workflow engine.** Lifecycle: `pending_approval` → (approve) → `open` → (close) → `closed`; plus `cancelled`. Because consumers only match `status='open'`, a `pending_approval` requisition is invisible until approved — the gate is implicit and free. (Do NOT reuse the heavier `internal/applications` approval workflow — that is candidate-offer approval, a different domain.)
4. **Two new permission keys** under the `hiring` category: `requisition.manage` (create/edit/close) and `requisition.approve` (approve pending → open). Manage ⇒ super_admin, regional_director, operation_director, sgm, hr_manager. Approve ⇒ super_admin, regional_director, operation_director (store-level roles create, leadership approves).
5. **RBAC scope** reuses the existing `rbac.Scope` machinery: add a `VacanciesClause(argStart)` to `internal/rbac/scope.go` keyed on `store_id` (store → `store_id = $n`; subregion → `store_id IN (SELECT store_no FROM stores WHERE subregion = $n)`; all → no filter), mirroring `ApplicationsClause` exactly.
6. **No PeopleSoft dependency.** Manual rows carry `source='manual'`, `ps_vacancy_id NULL`. Postgres allows multiple NULLs in a UNIQUE column, so the PS `ON CONFLICT (ps_vacancy_id)` upsert never collides with manual rows.

---

## UX Design

### Before
```
Vacancies exist only via PeopleSoft sync. HR has no way to open a role.
No /requisitions nav item. Executive "Vacancy" = whatever PS pushed.
```

### After
```
┌── /requisitions ───────────────────────────────────────────┐
│ Requisitions                         [ + New requisition ]  │
│ Open 12 · Pending 3 · Filled 40   (summary strip)           │
│ ┌─ filters: status ▾  store ▾  position ▾ ─────────────────┐│
│ │ Position        Store         Heads  Status     Actions  ││
│ │ Forklift Driver CM Central    2      ● open      Close   ││
│ │ Cashier         Chiang Rai    1      ◷ pending   Approve ││  ← Approve only if can(approve)
│ │ Picker          Bangkok R9    3      ◷ pending   Edit    ││
│ └──────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
  + New requisition → Dialog: Position▾  Store▾  Headcount[ ]  → creates pending_approval
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Open a role | PeopleSoft only | `/requisitions` → New → pending → approve → open | RBAC-scoped |
| Branch assignment pool | PS vacancies | PS + approved manual requisitions | automatic (status='open') |
| Executive Vacancy/Pipeline | PS counts | includes approved manual reqs | automatic, EXECUTIVE_PROVIDER=real |
| Careers portal jobs | PS-backed | shows manual-opened positions too | automatic |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/vacancies/model.go` | 1-116 | Existing structs, repo idiom, the PS upsert NOT to touch; same table |
| P0 | `backend/internal/rbacadmin/handler.go` | 23-198 | Canonical CRUD handler: gate, BodyParser, validate, `writeErr` status mapping, `httpx` envelope |
| P0 | `backend/internal/rbac/repository.go` | 153-251 | Create (tx + RETURNING + 23505→sentinel), Update (dynamic SET builder), Delete (RowsAffected disambiguation) |
| P0 | `backend/internal/rbac/scope.go` | 9-63 | `ApplicationsClause` to mirror for `VacanciesClause`; KindAll/Subregion/Store |
| P0 | `backend/internal/rbac/permissions.go` | 10-48 | Permission catalog; add `PermRequisition*` + append to `AllPermissions` |
| P0 | `backend/migrations/000028_dynamic_rbac.up.sql` | 47-127 | Seed pattern for `rbac_permissions` + `rbac_role_permissions` (ON CONFLICT DO NOTHING) |
| P1 | `backend/internal/members/repository.go` | 125-210 | `buildWhere` placeholder-numbering + List+count pagination idiom |
| P1 | `backend/internal/applications/repository.go` | 493-507 | `ExistsInScope` per-row scope guard for detail/update/approve |
| P1 | `backend/internal/stores/handler.go` + `internal/positions/handler.go` | all | GET /stores, GET /positions for UI dropdowns (reuse as-is) |
| P1 | `backend/cmd/api/main.go` | 98-114, 236-254, 415-427 | RBAC authorizer bootstrap; storeRepo/positionRepo/vacancyRepo already built; where to register routes |
| P0 | `frontend/components/admin/RolesPermissions.tsx` | 72-381 | Create/edit Dialog + form + Select + mutateAsync(onSuccess:close) + inline error |
| P0 | `frontend/lib/queries.ts` | 117-183, 342-348, 639-657 | useRbacRoles CRUD trio + invalidation; usePositions/useStores/useMe; useMembers paginated |
| P1 | `frontend/app/(app)/members/page.tsx` | 68-344 | Page shell: gate, summary strip, list table, URL filters, restricted state |
| P1 | `frontend/lib/roles.ts` | 10-50 | PERMS mirror + `can(me,perm)` + `isMemberAdmin` helper shape |
| P1 | `frontend/components/shell/nav-config.tsx` | 28-84 | NavItem decl + `navForRole` gating + ALL_NAV |
| P1 | `frontend/components/admin/UserManagement.tsx` | 128-160, 346-366 | form-as-useState-object + live-query-fed Select (mirror for position/store dropdowns) |

## External Documentation
No external research needed — feature uses established internal patterns (pgx, Fiber, next-intl, TanStack Query, shadcn Dialog/Select).

---

## Patterns to Mirror

### REPOSITORY_CONSTRUCTOR + SENTINELS
```go
// SOURCE: internal/members/repository.go:9-35
const ( uniqueViolation = "23505"; foreignKeyViolation = "23503" )
func isUnique(err error) bool { var pgErr *pgconn.PgError; return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation }
var ErrNotFound = errors.New("requisitions: not found")
type pgRepository struct { pool *pgxpool.Pool }
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }
```

### CREATE (tx + RETURNING + 23505)
```go
// SOURCE: internal/rbac/repository.go:153-179 (shape to mirror)
row := tx.QueryRow(ctx, `INSERT INTO vacancies (position_id, store_id, headcount, status, source, created_by, created_at, updated_at)
    VALUES ($1,$2,$3,'pending_approval','manual',$4, now(), now()) RETURNING `+vacancyColumns, ...)
// on pgErr.Code == uniqueViolation → return ErrRequisitionExists
```

### UPDATE (dynamic SET builder)
```go
// SOURCE: internal/rbac/repository.go:181-232
set := []string{}; args := []any{}
add := func(expr string, val any){ args = append(args, val); set = append(set, fmt.Sprintf("%s = $%d", expr, len(args))) }
if in.Headcount != nil { add("headcount", *in.Headcount) }
if in.PositionID != nil { add("position_id", *in.PositionID) }
add("updated_at", "now()") // always bump; or set in SQL literal
args = append(args, id)
q := fmt.Sprintf(`UPDATE vacancies SET %s WHERE id = $%d AND source='manual'`, strings.Join(set, ", "), len(args))
ct, _ := r.pool.Exec(ctx, q, args...); if ct.RowsAffected()==0 { return ErrRequisitionNotFound }
```

### RBAC SCOPE CLAUSE (add a sibling to ApplicationsClause)
```go
// SOURCE: internal/rbac/scope.go:30-63 (mirror for the vacancies.store_id column)
func (s Scope) VacanciesClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("store_id IN (SELECT store_no FROM stores WHERE subregion = $%d)", argStart), []any{s.Subregion}
	case KindStore:
		if s.StoreID == nil { return "1=0", nil }
		return fmt.Sprintf("store_id = $%d", argStart), []any{*s.StoreID}
	default:
		return "", nil
	}
}
```

### HANDLER GATE + CREATE + ERROR MAPPING
```go
// SOURCE: internal/rbacadmin/handler.go:46-52, 78-110, 186-198
func (h *Handler) gate(c *fiber.Ctx, perm string) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" { return middleware.DevUser{}, false }
	return u, rbac.Can(u.Role, perm)
}
func (h *Handler) writeErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrRequisitionNotFound): return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	case errors.Is(err, ErrRequisitionExists):   return httpx.Fail(c, fiber.StatusConflict, "duplicate open requisition")
	case errors.Is(err, ErrBadState):            return httpx.Fail(c, fiber.StatusConflict, "requisition not in an approvable state")
	default: log.Error().Err(err).Msg("requisitions: write failed"); return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	}
}
```

### ROUTES (static before param)
```go
// SOURCE: internal/rbacadmin/handler.go:36-43 + members/routes.go:9-12 ordering rule
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/requisitions")
	g.Get("/", h.List)
	g.Post("/", h.Create)
	g.Patch("/:id", h.Update)
	g.Post("/:id/approve", h.Approve)
	g.Post("/:id/close", h.Close)
	g.Delete("/:id", h.Delete)
}
```

### MIGRATION SEED (permissions + grants)
```sql
-- SOURCE: migrations/000028_dynamic_rbac.up.sql:47-127
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('requisition.manage',  'Manage requisitions', 'จัดการการเปิดรับ',  'hiring', 60),
    ('requisition.approve', 'Approve requisitions','อนุมัติการเปิดรับ', 'hiring', 61)
ON CONFLICT (key) DO NOTHING;
INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('regional_director','requisition.manage'), ('regional_director','requisition.approve'),
    ('operation_director','requisition.manage'),('operation_director','requisition.approve'),
    ('sgm','requisition.manage'), ('hr_manager','requisition.manage')
ON CONFLICT DO NOTHING;
INSERT INTO rbac_role_permissions (role_key, permission)
    SELECT 'super_admin', key FROM rbac_permissions WHERE key LIKE 'requisition.%'
ON CONFLICT DO NOTHING;
```

### FRONTEND CRUD QUERIES
```ts
// SOURCE: frontend/lib/queries.ts:117-157 (mirror exactly, new key namespace)
export function useRequisitions(filter: RequisitionFilter, enabled = true) {
  return useQuery({ queryKey: ["requisitions", filter],
    queryFn: () => api.get<Wrapped<Requisition[]>>(`/api/v1/requisitions${buildQuery(filter)}`),  // keep wrapper for meta.total
    enabled });
}
function invalidateReq(qc) { qc.invalidateQueries({ queryKey: ["requisitions"] }); }
export function useCreateRequisition(){ const qc=useQueryClient(); return useMutation({
  mutationFn:(i:RequisitionInput)=>api.post<Requisition>("/api/v1/requisitions", i).then(r=>r.data),
  onSuccess:()=>invalidateReq(qc) }); }
// useUpdateRequisition (patch :id), useApproveRequisition (post :id/approve), useCloseRequisition (post :id/close) — same shape
```

### FRONTEND DIALOG + LIVE-QUERY SELECT
```tsx
// SOURCE: components/admin/UserManagement.tsx:346-366 + RolesPermissions.tsx:266-316
function PositionSelect({ value, onChange }: { value: string; onChange: (v:string)=>void }) {
  const locale = useLocale(); const { data: positions } = usePositions();
  return (<Select value={value} onValueChange={(v)=>onChange(v ?? value)}>
    <SelectTrigger className="w-full"><SelectValue/></SelectTrigger>
    <SelectContent>{(positions ?? []).map(p=>(
      <SelectItem key={p.id} value={p.id}>{locale==="th"?p.title_th:p.title_en}</SelectItem>))}</SelectContent>
  </Select>);
}
// StoreSelect mirrors with useStores(): <SelectItem value={String(s.store_no)}>{s.store_name}</SelectItem>
```

### FRONTEND RBAC GATE + NAV
```ts
// SOURCE: lib/roles.ts:10-50 + nav-config.tsx:64-84
export const PERMS = { /* ...existing... */, requisitionManage: "requisition.manage", requisitionApprove: "requisition.approve" } as const;
export function canManageRequisitions(me: PermHolder){ return can(me, PERMS.requisitionManage); }
export function canApproveRequisitions(me: PermHolder){ return can(me, PERMS.requisitionApprove); }
// nav-config.tsx:
export const REQUISITIONS_NAV: NavItem = { href:"/requisitions", label:"Requisitions", key:"requisitions", icon: ClipboardList };
// in navForRole(me): if (canManageRequisitions(me)) items.push(REQUISITIONS_NAV);  + append to ALL_NAV
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000029_requisitions.up.sql` | CREATE | ALTER vacancies (source, created_by, approved_by, approved_at, created_at, updated_at) + seed 2 perms + grants |
| `backend/migrations/000029_requisitions.down.sql` | CREATE | DROP columns + DELETE seeded perm/grant rows |
| `backend/internal/requisitions/model.go` | CREATE | `Requisition` struct (id, position+title, store+name, headcount, status, source, created/approved by/at), `ListFilter`, `CreateInput`, `UpdateInput`, `Repository` interface |
| `backend/internal/requisitions/repository.go` | CREATE | List (scoped + filters + count), Create (manual→pending), Update, Approve, Close, Delete, ExistsInScope — pgxpool idiom |
| `backend/internal/requisitions/handler.go` | CREATE | gate(perm), BodyParser, validate, scope guard, httpx envelope, writeErr mapping |
| `backend/internal/requisitions/routes.go` | CREATE | RegisterRoutes (static before :id) |
| `backend/internal/requisitions/handler_test.go` | CREATE | stub repo + role-injecting app; gate 403/200 per role; approve state machine |
| `backend/internal/requisitions/repository_test.go` | CREATE | table-driven where/validate unit tests (no DB) |
| `backend/internal/rbac/permissions.go` | UPDATE | add `PermRequisitionManage`/`PermRequisitionApprove` + append to `AllPermissions` |
| `backend/internal/rbac/scope.go` | UPDATE | add `VacanciesClause(argStart)` |
| `backend/internal/rbac/permissions_test.go` (or parity test) | UPDATE | keep `AllPermissions` ↔ seed parity assertion green |
| `backend/cmd/api/main.go` | UPDATE | construct + register requisitions handler (~line 416, reuse pool/scope) |
| `frontend/lib/types.ts` | UPDATE | `Requisition`, `RequisitionInput`, `RequisitionFilter` |
| `frontend/lib/queries.ts` | UPDATE | useRequisitions + create/update/approve/close mutations + invalidation |
| `frontend/lib/roles.ts` | UPDATE | PERMS keys + canManage/canApproveRequisitions |
| `frontend/components/shell/nav-config.tsx` | UPDATE | REQUISITIONS_NAV + navForRole gate + ALL_NAV |
| `frontend/app/(app)/requisitions/page.tsx` | CREATE | gated list page + summary + filters + New button |
| `frontend/components/requisitions/RequisitionDialog.tsx` | CREATE | create/edit dialog (position+store Select, headcount) |
| `frontend/components/requisitions/RequisitionTable.tsx` | CREATE | rows + status badge + Approve/Close/Edit actions (action visibility by can()) |
| `frontend/messages/en.json` + `th.json` | UPDATE | new `requisitions` namespace + `nav.requisitions` (BOTH files) |

## NOT Building
- No new heavyweight approval-workflow engine (reuse status field; the `internal/applications` multi-level approval is candidate-offer, out of scope).
- No PeopleSoft write-back (manual requisitions stay local; PS sync path untouched).
- No editing/deleting PeopleSoft-sourced rows from this UI (guard `source='manual'` on update/delete).
- No `filled_at` auto-tracking or per-requisition candidate linking (close is a manual status flip).
- No executive/portal/assigner code changes (they consume `status='open'` automatically).
- No bulk requisition import/CSV.
- No notifications/email on approve in MVP (can add `SetNotifier` later, mirroring offerHandler).

---

## Step-by-Step Tasks

### Task 1: Migration 000029 (schema + RBAC seed)
- **ACTION**: Create `backend/migrations/000029_requisitions.up.sql` and `.down.sql`.
- **IMPLEMENT**: `ALTER TABLE vacancies ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'peoplesoft', ADD COLUMN created_by UUID REFERENCES users(id), ADD COLUMN approved_by UUID REFERENCES users(id), ADD COLUMN approved_at TIMESTAMPTZ, ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now(), ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();` then the permission + grant seed (see MIGRATION SEED pattern). Add index `CREATE INDEX idx_vacancies_source_status ON vacancies (source, status);`. Down: `ALTER TABLE vacancies DROP COLUMN ...` (6 cols) + `DELETE FROM rbac_role_permissions WHERE permission LIKE 'requisition.%'; DELETE FROM rbac_permissions WHERE key LIKE 'requisition.%';` + drop index.
- **MIRROR**: `migrations/000028_dynamic_rbac.up.sql:47-127`.
- **GOTCHA**: existing PS rows get `source='peoplesoft'` by DEFAULT — correct. No CHECK on status (stays free-form, app-enforced). Confirm `users` table is the FK target (it is — hrauth users).
- **VALIDATE**: `migrate up` then `\d vacancies` shows 6 new cols; `SELECT count(*) FROM rbac_permissions WHERE key LIKE 'requisition.%'` = 2.

### Task 2: RBAC catalog + scope
- **ACTION**: Edit `internal/rbac/permissions.go` and `internal/rbac/scope.go`.
- **IMPLEMENT**: add `PermRequisitionManage = "requisition.manage"`, `PermRequisitionApprove = "requisition.approve"` constants; append both to `AllPermissions`. Add `VacanciesClause(argStart int)` to scope.go (see RBAC SCOPE CLAUSE pattern).
- **MIRROR**: `permissions.go:10-48`, `scope.go:30-63`.
- **GOTCHA**: the `AllPermissions` ↔ `rbac_permissions` parity test will FAIL until Task 1's seed is applied in the test DB; keep the keys spelled identically (`requisition.manage`, not `requisitions.manage`).
- **VALIDATE**: `go build ./...`; parity test green after migration.

### Task 3: Requisitions model + repository
- **ACTION**: Create `internal/requisitions/model.go` + `repository.go`.
- **IMPLEMENT**: `Requisition` struct with `ID uuid.UUID; PositionID *uuid.UUID; PositionTitle string; StoreID *int; StoreName string; Subregion string; Headcount int; Status string; Source string; CreatedBy *uuid.UUID; ApprovedBy *uuid.UUID; CreatedAt, UpdatedAt time.Time`. `ListFilter{Status, StoreID, PositionID, Page, Limit}` + `normalize()`. `CreateInput{PositionID uuid.UUID; StoreID int; Headcount int}`. `UpdateInput{PositionID *uuid.UUID; StoreID *int; Headcount *int}`. Repository interface: `List(ctx, ListFilter, rbac.Scope) ([]Requisition,int,error)`, `Create`, `Update`, `Approve(ctx,id,approverID)`, `Close`, `Delete`, `ExistsInScope`. List SELECT joins positions + stores for display names, filters `source='manual'` OR include PS? → **List shows ALL vacancies but flags source**; scope clause via `scope.VacanciesClause`. Create inserts `status='pending_approval', source='manual'`. Approve: `UPDATE ... SET status='open', approved_by=$2, approved_at=now() WHERE id=$1 AND status='pending_approval'` → RowsAffected 0 ⇒ `ErrBadState`. Close: `status='closed' WHERE id=$1 AND status='open'`.
- **MIRROR**: `internal/rbac/repository.go:153-251` (create/update/delete), `internal/members/repository.go:125-210` (List+count+buildWhere), `internal/applications/repository.go:493-507` (ExistsInScope using `VacanciesClause`).
- **IMPORTS**: `github.com/jackc/pgx/v5`, `pgconn`, `pgxpool`, `github.com/google/uuid`, `internal/rbac`.
- **GOTCHA**: do NOT reuse `vacancies.Upsert` (it requires non-null `ps_vacancy_id`). Manual insert leaves `ps_vacancy_id` NULL. Guard Update/Delete with `AND source='manual'` so PS rows are immutable here.
- **VALIDATE**: `go build ./internal/requisitions/`.

### Task 4: Requisitions handler + routes
- **ACTION**: Create `internal/requisitions/handler.go` + `routes.go`.
- **IMPLEMENT**: `NewHandler(repo Repository) *Handler`. `List` (gate manage, build `rbac.Scope` from DevUser, parse filters, return `httpx.Envelope` with `Meta`). `Create` (gate manage, BodyParser, validate position/store present + headcount 1..N, repo.Create, `httpx.Created`). `Update` (gate manage, ExistsInScope→404, repo.Update). `Approve` (gate **approve**, ExistsInScope, repo.Approve with `u.ID`). `Close` (gate manage, ExistsInScope, repo.Close). `Delete` (gate manage, only pending/draft manual). Centralized `writeErr`.
- **MIRROR**: `internal/rbacadmin/handler.go:46-198`. `scopeFrom(c)` builds `rbac.New(u.Role, u.StoreID, u.Subregion)`.
- **GOTCHA**: register static `/` POST before `/:id` routes (Fiber param capture). Use 404 not 403 on out-of-scope rows (don't leak existence — `applications/handler.go:187`).
- **VALIDATE**: `go build ./...`; routes return 401 unauthenticated.

### Task 5: Wire into cmd/api/main.go
- **ACTION**: Edit `cmd/api/main.go` near line 416.
- **IMPLEMENT**: `requisitions.RegisterRoutes(app, requisitions.NewHandler(requisitions.NewRepository(pool)))`.
- **MIRROR**: the `stores.RegisterRoutes`/`rbacadmin.RegisterRoutes` wiring (main.go:416, 287).
- **GOTCHA**: place after `app.Use(authMW)` so the handler reads DevUser locals. No new middleware.
- **VALIDATE**: `go build ./cmd/api`; boot logs no error.

### Task 6: Backend tests
- **ACTION**: Create `handler_test.go` + `repository_test.go`.
- **IMPLEMENT**: stub `Repository`; `appWithRole(role)` injecting `middleware.DevUser`. Assert: hr_staff → 403 on all; hr_manager → 200 list/create, 403 approve; regional_director → 200 approve; approve on non-pending → 409; out-of-scope id → 404. Unit-test `ListFilter.normalize()` + validation.
- **MIRROR**: `internal/members/handler_test.go:18-154`, `internal/rbacadmin/handler_test.go`.
- **VALIDATE**: `go test ./internal/requisitions/ ./internal/rbac/`.

### Task 7: Frontend types + queries + roles
- **ACTION**: Edit `lib/types.ts`, `lib/queries.ts`, `lib/roles.ts`.
- **IMPLEMENT**: `Requisition`/`RequisitionInput`/`RequisitionFilter` types; `useRequisitions` (keep wrapper for meta), `useCreate/Update/Approve/CloseRequisition` + `invalidateReq`; PERMS keys + `canManage/canApproveRequisitions`.
- **MIRROR**: `lib/queries.ts:117-157`, `lib/types.ts:484-500`, `lib/roles.ts:10-50`.
- **GOTCHA**: list query keeps `{data,meta}` wrapper (no `.then(r=>r.data)`) so the page reads `meta.total`; mutations unwrap with `.then(r=>r.data)`.
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 8: Frontend page + dialog + table + nav + i18n
- **ACTION**: Create `app/(app)/requisitions/page.tsx`, `components/requisitions/RequisitionDialog.tsx`, `RequisitionTable.tsx`; edit `nav-config.tsx`, `messages/{en,th}.json`.
- **IMPLEMENT**: page gated by `canManageRequisitions(me)` (restricted state mirror), summary strip (open/pending/filled), status+store+position filters in URL, "New requisition" button → dialog. Dialog: PositionSelect + StoreSelect (live queries) + headcount input → create/update. Table: status badge, Approve button only when `canApproveRequisitions(me)` && status pending, Close when open. `REQUISITIONS_NAV` in navForRole + ALL_NAV. New `requisitions` i18n namespace + `nav.requisitions` in BOTH locales.
- **MIRROR**: `app/(app)/members/page.tsx:68-344`, `components/admin/RolesPermissions.tsx:266-381`, `UserManagement.tsx:346-366`.
- **GOTCHA**: add every key to BOTH `en.json` and `th.json`. `useTranslations("requisitions")`. Status badge can reuse a small local map (statuses: pending_approval/open/closed/cancelled).
- **VALIDATE**: `node scripts/check-i18n-parity.mjs` (repo root), `pnpm exec tsc --noEmit`, `pnpm exec eslint`, `pnpm exec next build`.

---

## Testing Strategy

### Unit Tests (backend)
| Test | Input | Expected | Edge? |
|---|---|---|---|
| List gate | hr_staff | 403 | yes |
| List gate | hr_manager | 200 | no |
| Create | valid pos+store+heads | 201, status=pending_approval | no |
| Create | headcount 0 / missing position | 400 | yes |
| Approve | hr_manager (no approve perm) | 403 | yes |
| Approve | regional_director on pending | 200, status=open | no |
| Approve | on already-open | 409 ErrBadState | yes |
| Update/Delete | PS-sourced row (source!=manual) | 404/no-op | yes |
| Scope | store-scoped user, other store's req id | 404 | yes |

### Edge Cases Checklist
- [ ] store-scoped user with no store → sees nothing (`1=0` clause)
- [ ] approve a cancelled/closed requisition → 409
- [ ] manual requisition with NULL ps_vacancy_id does not collide with PS upsert
- [ ] duplicate open (position+store) — decide allow (SUM headcount) vs block; MVP allows (matches current SUM semantics), note in UI
- [ ] permission denied (403) vs out-of-scope (404)

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l internal/requisitions && go vet ./... 
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib
```
EXPECT: clean

### Unit Tests
```bash
cd backend && go test ./internal/requisitions/ ./internal/rbac/ ./internal/middleware/
```
EXPECT: all pass

### Full Backend Suite
```bash
cd backend && go build ./... && go test ./...
```
EXPECT: no regressions

### i18n + Build
```bash
node scripts/check-i18n-parity.mjs           # repo root — NOT in package.json, run directly
cd frontend && pnpm exec next build
```
EXPECT: "th/en in parity" + green build

### Migration
```bash
# applied by operator on prod (Option B az): migrate up → schema v29
psql "$DB_URL" -c "\d vacancies"   # 6 new columns
psql "$DB_URL" -c "SELECT key FROM rbac_permissions WHERE key LIKE 'requisition.%'"
```
EXPECT: schema v29, 2 permission rows

### Manual Validation
- [ ] Log in (hr_manager) → /requisitions → New → pick position+store+headcount → row appears `pending_approval`
- [ ] Log in (regional_director) → Approve → status `open`
- [ ] Executive (EXECUTIVE_PROVIDER=real) Vacancy count increases by headcount; portal /jobs shows the position
- [ ] store-scoped sgm sees only their store's requisitions

---

## Acceptance Criteria
- [ ] All tasks completed
- [ ] All validation commands pass
- [ ] Tests written and passing (gate + state machine + scope)
- [ ] No type/lint errors; i18n parity green
- [ ] Approved manual requisition appears in branch assignment + executive(real) + portal
- [ ] PS webhook path unchanged (vacancies.Upsert untouched)

## Completion Checklist
- [ ] Code follows discovered patterns (rbacadmin handler, rbac repo, members page)
- [ ] Error handling via `writeErr` + httpx envelope
- [ ] RBAC gating per-handler + scope clause on list/detail
- [ ] Tests mirror members/rbacadmin style
- [ ] No hardcoded strings (i18n both locales)
- [ ] `source='manual'` guard on update/delete; PS rows immutable here
- [ ] Self-contained — no further codebase search needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Manual + PS rows mixed in one table cause confusing executive counts | Med | Med | `source` column + UI badge; executive intentionally sums all open — document as expected |
| Approval gate bypassed if a role has both manage+approve | Med | Low | Acceptable (leadership self-approve); enforce approve as separate perm so it's auditable via approved_by |
| `AllPermissions` parity test fails before migration applied in CI | Med | Low | Apply 000029 in test setup; keys spelled exactly |
| Duplicate open requisitions inflate headcount | Low | Med | MVP allows (matches SUM semantics); add optional UNIQUE later if desired |
| Operator forgets schema migration on deploy → boot legacy fallback | Low | Med | Deploy runbook: apply 000029 BEFORE rolling api; RBAC authorizer reload picks new perms |

## Notes
- Deploy mirrors prior Module-3/RBAC rolls: apply migration 000029 → roll **api** (new routes + perms) → roll **dashboard** (new page, 4 Entra build-args). worker/scheduler/portal need NO rebuild (portal reads the same `status='open'` automatically). RBAC authorizer reloads roles on its TTL ticker, but to surface the 2 new permission keys immediately, a **restart** of api after migration is safest (same gotcha as 000028).
- `EXECUTIVE_PROVIDER=real` (now set on prod) means approved manual requisitions show in the executive Vacancy/Pipeline immediately — this feature pairs naturally with that switch.
- i18n parity script lives at repo-root `scripts/check-i18n-parity.mjs` (run with `node`, it is NOT a package.json script).
- Confidence: this plan reuses 4 fully-mapped internal patterns (rbacadmin CRUD, rbac repo/scope/seed, members page, queries) with quoted sources — single-pass implementable.
```
