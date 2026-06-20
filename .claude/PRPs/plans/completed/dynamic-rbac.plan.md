# Plan: Dynamic RBAC (admin-managed roles & permissions)

## Summary
Replace the ~18 hardcoded role-string allowlists scattered across the Go backend (and their mirror in the frontend) with a **data-driven role→permission model** that a super_admin can edit in the Admin UI. Permissions are a **fixed code catalog** (one key per gateable action — they map 1:1 to code call sites, so new capabilities still require code); **roles**, the **role→permission matrix**, and each role's **data scope** (all / subregion / store) become **dynamic** (DB-backed, CRUD-able). The current 7 roles + their exact current capability matrix are seeded so behavior is byte-for-byte unchanged on day one.

## User Story
As a **super_admin**, I want to create/edit roles and toggle which permissions and data-scope each role has from the Admin console, so that **"who can do/see what" can change without a code deploy** — and so I can add bespoke roles (e.g. "Regional HR read-only") without engineering.

## Problem → Solution
**Current:** Authorization = compile-time `map[string]bool` allowlists next to each handler (16 gates) + a role→scope `switch` in `internal/rbac/scope.go`, mirrored by ~15 `canX(role)` helpers in `frontend/lib/roles.ts` and `navForRole()`. Any access change = code edit + full deploy of api **and** dashboard. Roles are a fixed set of 7 validated in `internal/hrauth/model.go`.
**Desired:** A `internal/rbac` authorizer resolves `role → {permissions, scopeKind}` from new DB tables (seeded from today's matrix). Handlers call `authz.Can(role, perm)`; scope reads `role.scope_kind`. `/users/me` returns the caller's **effective permission set**, so the frontend gates on `me.permissions` instead of hardcoded role lists. A super_admin Admin page edits the matrix live.

## Metadata
- **Complexity**: XL (cross-cutting; ~16 backend gates + scope + new package + admin API + admin UI + frontend capability refactor + 1 migration). Built in phases; ship phase-by-phase.
- **Source PRD**: N/A (UAT backlog item #6, from `docs/uat-talent-pool.md` / memory `uat-feedback-fixes`)
- **PRD Phase**: standalone
- **Estimated Files**: ~30 (1 migration, ~6 new backend files, ~16 handler edits, ~6 new/edited frontend files)

---

## UX Design

### Before
```
Admin page (/admin, super_admin only)
┌──────────────────────────────────────────┐
│ Tenant access  [ allow all orgs  ⦿ off ]  │
│ User accounts                              │
│   ┌──────────────────────────────────┐    │
│   │ name · email │ Role(dropdown of 7 │    │
│   │              │ FIXED roles)       │    │
│   └──────────────────────────────────┘    │
└──────────────────────────────────────────┘
Role→capability is invisible & code-locked.
```

### After
```
Admin page (/admin, super_admin only)
┌───────────────────────────────────────────────────────────┐
│ Tenant access  [ allow all orgs  ⦿ off ]                    │
│                                                             │
│ Roles & permissions                          [ + New role ]│
│  ┌─────────────────────────────────────────────────────┐  │
│  │ Role            Scope        Permissions             │  │
│  │ Super admin     all   🔒     (all · locked)          │  │
│  │ HR manager      store ▾      [Edit ▸] 9 permissions  │  │
│  │ Store GM        store ▾      [Edit ▸] 7 permissions  │  │
│  │ Regional HR RO  all   ▾      [Edit ▸] 2  (custom) 🗑 │  │
│  └─────────────────────────────────────────────────────┘  │
│  Edit drawer: a permission matrix (grouped checkboxes) +   │
│  a scope selector (all / subregion / store).               │
│                                                             │
│ User accounts                                               │
│   Role dropdown now lists ALL roles (built-in + custom),   │
│   fetched from the API.                                     │
└───────────────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Assign role to user | Pick 1 of 7 hardcoded labels | Pick any role (built-in + custom) fetched from API | UserManagement role `<Select>` becomes data-driven |
| Change who can do X | Edit Go allowlist + deploy api+dashboard | Toggle a checkbox in Admin → effective within cache TTL | No deploy |
| Add a new role | Impossible | "New role" → name + scope + permission checkboxes | Built-ins not deletable; super_admin not editable |
| Frontend capability gates | hardcoded `canX(role)` role lists | `can(me, "perm")` against `me.permissions` | `lib/roles.ts` becomes a thin `can()` over the Me payload |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/rbac/scope.go` | 1-69 | The package the authorizer joins; the role→scope switch to make data-driven |
| P0 | `backend/internal/hrauth/model.go` | 30-66 | `allowedRoles` (the 7-role gate to replace) + User/Input models |
| P0 | `backend/internal/hrauth/repository.go` | 152-213 | The repo style to mirror (RETURNING, 23505 sentinel, dynamic UPDATE builder) |
| P0 | `backend/internal/members/handler.go` | 26-90 | The canonical allowlist + `authorized()` pattern every gate uses |
| P0 | `backend/internal/users/handler.go` | 1-30 | `/me` endpoint — where effective permissions get added to the payload |
| P0 | `backend/internal/middleware/mock_jwt.go` | 1-25 | `DevUser{ID,Email,Role,StoreID,Subregion}` + `UserContextKey` |
| P1 | `backend/internal/middleware/auth.go` | 59-114 | How role enters context (Entra claim / hr session); unauth bypass list |
| P1 | `backend/internal/hrauth/handler.go` | 78-135 | super_admin user-CRUD handler pattern to mirror for the rbac admin API |
| P1 | `backend/internal/hrauth/service.go` | 120-165 | Role validation + self-lockout guard (mirror for role delete/scope guards) |
| P1 | `backend/migrations/000018_hr_password_auth.up.sql` / `.down.sql` | all | Exact migration style to match (additive, `IF NOT EXISTS`, no explicit tx) |
| P1 | `backend/cmd/api/main.go` | 183-191, 250-423 | Where middleware is built and each module's `RegisterRoutes` is wired |
| P1 | `frontend/lib/roles.ts` | 1-168 | All ~15 capability helpers to convert to `can(me, perm)` |
| P1 | `frontend/components/shell/nav-config.tsx` | 17-70 | `navForRole()` — menu gating to convert to permission checks |
| P1 | `frontend/components/admin/UserManagement.tsx` | 1-130, 346-361 | Admin section + role `<Select>` (RoleSelect) to make data-driven |
| P1 | `frontend/lib/types.ts` | 463-469 | `Me` interface to extend with `permissions` |
| P2 | `backend/internal/applications/approval.go` | 41-58 | `approvalChain` + `approvalLevelRoles` (per-level → per-permission mapping) |
| P2 | `backend/internal/settings/handler.go` | 12, 47-62 | Smallest allowlist gate (good first swap) |
| P2 | `docs/module-3-deploy-runbook.md` | 89-126, 205-246 | How migrations are applied to prod manually |

## External Documentation
No external research needed — feature is greenfield over established internal patterns (pgxpool repos, Fiber handlers, next-intl/TanStack-Query frontend). RBAC is a well-understood pattern; no new library.

---

## Patterns to Mirror

### ALLOWLIST_GATE (the thing being replaced — and the seed source of truth)
```go
// SOURCE: backend/internal/members/handler.go:28,71-77
var memberAdminRoles = map[string]bool{"super_admin": true, "hr_manager": true}

func (h *Handler) authorized(c *fiber.Ctx) bool {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return false // no auth context → fail closed
	}
	return memberAdminRoles[u.Role]
}
```
→ After: `return h.authz.Can(u.Role, rbac.PermMembersAdmin)` (keep the fail-closed cast). The literal map is the **seed data** for that permission.

### ROLE_SCOPE_SWITCH (make data-driven, keep fail-closed default)
```go
// SOURCE: backend/internal/rbac/scope.go:29-38
func (s Scope) Kind() string {
	switch s.Role {
	case "super_admin", "regional_director", "auditor":
		return KindAll
	case "operation_director":
		return KindSubregion
	default: // sgm, hr_manager, hr_staff, unknown
		return KindStore
	}
}
```
→ After: `Kind()` consults the injected resolver (`role.ScopeKind`), still defaulting to `KindStore` for unknown/missing roles so a misconfigured role never widens visibility.

### REPO_DYNAMIC_UPDATE (mirror exactly for rbac repo writes)
```go
// SOURCE: backend/internal/hrauth/repository.go:167-205
set := []string{}
args := []any{}
add := func(expr string, val any) {
	args = append(args, val)
	set = append(set, fmt.Sprintf("%s = $%d", expr, len(args)))
}
if in.FullName != nil { add("full_name", *in.FullName) }
// ...
args = append(args, id)
q := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d RETURNING %s`,
	strings.Join(set, ", "), len(args), userColumns)
```

### REPO_INSERT_UNIQUE (mirror for role create)
```go
// SOURCE: backend/internal/hrauth/repository.go:152-165
u, err := scanUser(r.pool.QueryRow(ctx, q, ...))
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation { // "23505"
	return User{}, ErrEmailExists
}
if err != nil { return User{}, fmt.Errorf("hrauth: create user: %w", err) }
```

### SERVICE_SELF_LOCKOUT_GUARD (mirror for role delete / super_admin protection)
```go
// SOURCE: backend/internal/hrauth/service.go:154-165 (paraphrased intent)
// A super_admin may not demote/disable their own account — prevents lockout.
// MIRROR: a built-in role may not be deleted; super_admin role may not be
// scope-narrowed or have permissions removed (code bypass guarantees it anyway).
```

### MIGRATION_STYLE (additive, IF NOT EXISTS, heavy comments, no explicit tx)
```sql
-- SOURCE: backend/migrations/000018_hr_password_auth.up.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
CREATE TABLE IF NOT EXISTS hr_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ...
);
CREATE INDEX IF NOT EXISTS idx_hr_sessions_user ON hr_sessions (user_id);
-- down: DROP ... IF EXISTS in reverse order
```

### ME_PAYLOAD (where to attach effective permissions)
```go
// SOURCE: backend/internal/users/handler.go:24-30
func (h *Handler) Me(c *fiber.Ctx) error {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok { return fiber.NewError(fiber.StatusUnauthorized, "not authenticated") }
	return httpx.OK(c, u) // → wrap: {…u, permissions: authz.Permissions(u.Role), scope: authz.ScopeKind(u.Role)}
}
```

### FRONTEND_CAPABILITY (convert role lists → permission checks)
```ts
// SOURCE: frontend/lib/roles.ts:46-52 (before)
export const BULK_UPLOAD_ROLES = ["super_admin", "hr_manager", "sgm", "hr_staff"];
export function canBulkUpload(role?: string): boolean {
  return !!role && BULK_UPLOAD_ROLES.includes(role);
}
// After: gate on the Me payload's resolved permissions.
export function can(me: { permissions?: string[] } | undefined, perm: string): boolean {
  return !!me?.permissions?.includes(perm);
}
```

### ADMIN_API_HANDLER (mirror super_admin user-CRUD for the rbac admin endpoints)
```go
// SOURCE: backend/internal/hrauth/handler.go:78-135 — requireSuperAdmin(c) then
// repo CRUD, returning httpx envelopes; mounted via RegisterRoutes in cmd/api/main.go.
```

### FRONTEND_QUERY (TanStack Query hook style for the admin matrix)
```ts
// SOURCE: frontend/lib/queries.ts (useHRUsers/useCreateHRUser/useUpdateHRUser pattern)
// Mirror for useRbacRoles / useRbacPermissions / useCreateRole / useUpdateRole / useDeleteRole,
// invalidating the roles query (and "me") on success.
```

---

## The permission catalog (FIXED, code-defined) — `internal/rbac/permissions.go`

Each key maps 1:1 to a current allowlist (agent-confirmed). These are Go constants — the source of truth for *enforcement*. A `rbac_permissions` table mirrors them (key + en/th label + category) only for the admin UI.

| Permission key | Replaces allowlist | Current roles (seed) |
|---|---|---|
| `settings.admin` | `adminRolesAllowed` (settings) | super_admin |
| `users.admin` | `requireSuperAdmin` (hrauth) | super_admin |
| `rbac.admin` | *(new — gates the role/permission CRUD itself)* | super_admin |
| `executive.view` | `executiveRolesAllowed` | super_admin, regional_director, auditor |
| `reports.view` | `reportViewRoles` | all 7 |
| `reports.export` | `exportRolesAllowed` | super_admin, regional_director |
| `reengage.trigger` | `rolesAllowed` (reengage) | super_admin, regional_director, operation_director |
| `members.admin` | `memberAdminRoles` | super_admin, hr_manager |
| `members.erase` | `memberEraseRoles` | super_admin |
| `bulk.upload` | `bulkIntakeRoles` | super_admin, hr_manager, sgm, hr_staff |
| `assignment.write` | `assignmentRoles` | super_admin, hr_manager, sgm |
| `offer.write` | `offerWriteRoles` | super_admin, hr_manager |
| `onboarding.write` | `onboardingWriteRoles` | super_admin, hr_manager, hr_staff, sgm |
| `letter.write` | `letterWriteRoles` | super_admin, hr_manager, hr_staff, sgm |
| `scorecard.ta` | `taRecordRoles` | super_admin, hr_manager, hr_staff |
| `scorecard.lm` | `lmRecordRoles` | super_admin, sgm |
| `approval.submit` | `canSubmitApproval` | super_admin, hr_staff |
| `approval.decide.l1` | `approvalLevelRoles[1]` | super_admin, hr_staff |
| `approval.decide.l2` | `approvalLevelRoles[2]` | super_admin, hr_manager |
| `approval.decide.l3` | `approvalLevelRoles[3]` | super_admin, sgm |
| `approval.decide.l4` | `approvalLevelRoles[4]` | super_admin, regional_director |

Scope kind per built-in role (from `scope.go`): super_admin/regional_director/auditor = `all`; operation_director = `subregion`; sgm/hr_manager/hr_staff = `store`.

**super_admin is a hard code bypass:** `Can(super_admin, *) == true` always, and the super_admin row is `is_builtin`, non-deletable, scope `all`, all permissions checked+locked in the UI — guarantees no lockout regardless of DB edits.

**NOT gated by permissions:** `hrNotifyRoles` / `lineManagerRoles` (`internal/applications/hr_directory.go`) are notification-recipient routing, not access gates — left as-is.

---

## Files to Change

### Backend
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000028_dynamic_rbac.up.sql` / `.down.sql` | CREATE | `rbac_roles`, `rbac_permissions`, `rbac_role_permissions` + seed current matrix |
| `backend/internal/rbac/permissions.go` | CREATE | Fixed permission-key constants + catalog metadata |
| `backend/internal/rbac/authorizer.go` | CREATE | `Authorizer` interface + `Can`, `Permissions`, `ScopeKind`; in-memory cache w/ TTL reload from repo |
| `backend/internal/rbac/repository.go` | CREATE | pgxpool CRUD for roles/role_permissions/permissions (mirror hrauth repo) |
| `backend/internal/rbac/handler.go` | CREATE | super_admin admin API: list/create/update/delete roles, list permissions |
| `backend/internal/rbac/model.go` | CREATE | `Role`, `RoleInput`, `Permission` structs + DTOs |
| `backend/internal/rbac/scope.go` | UPDATE | `Scope.Kind()` consults the authorizer instead of the hardcoded switch |
| `backend/internal/users/handler.go` | UPDATE | `/me` returns `permissions` + `scope` from the authorizer |
| `backend/internal/hrauth/model.go` | UPDATE | `allowedRoles` validation → "role exists in rbac_roles" (via repo), keep 7 as seed |
| 16 handler files (settings, hrauth, executive, reports×2, reengage, members, applications: bulk/handler/offer/onboarding/letter/feedback/approval) | UPDATE | Swap each allowlist check for `authz.Can(role, Perm…)`; inject authorizer via constructor |
| `backend/cmd/api/main.go` | UPDATE | Build authorizer (load cache), pass into each handler ctor + `rbac.RegisterRoutes` |
| `backend/internal/rbac/*_test.go` | CREATE | Table-driven tests: seed-matrix parity, Can(), scope, super_admin bypass, lockout guards |

### Frontend
| File | Action | Justification |
|---|---|---|
| `frontend/lib/types.ts` | UPDATE | `Me` gains `permissions: string[]` + `scope?: string`; add `RbacRole`, `RbacPermission` types |
| `frontend/lib/roles.ts` | UPDATE | Replace ~15 role-list helpers with `can(me, perm)`; keep names as thin wrappers during transition |
| `frontend/lib/queries.ts` | UPDATE | `useRbacRoles`, `useRbacPermissions`, `useCreateRole`, `useUpdateRole`, `useDeleteRole` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | `navForRole(me)` → permission checks |
| `frontend/components/admin/RolesPermissions.tsx` | CREATE | The roles×permissions matrix editor + scope selector + create/delete |
| `frontend/components/admin/UserManagement.tsx` | UPDATE | `RoleSelect` fetches roles from API; role label from API |
| `frontend/app/(app)/admin/page.tsx` | UPDATE | Mount `<RolesPermissions />` |
| `frontend/messages/{en,th}.json` | UPDATE | i18n keys for the new admin section (follow established `admin` namespace pattern) |
| call sites of changed `canX` helpers (resume panels, pages) | UPDATE | Pass `me` instead of `me?.role` where signatures change |

## NOT Building
- **Runtime-editable permission *catalog***. New gateable actions are code call sites — adding a brand-new capability still needs a code change (+ a catalog entry + a migration seed row). Only the **role→permission matrix, roles, and scope** are dynamic.
- **Per-user permission overrides.** Permissions attach to roles only (a user has exactly one role, as today).
- **New scope semantics.** Keep `all`/`subregion`/`store`; only make the role→scope mapping data-driven. No record-level/field-level ACLs.
- **Notification recipient lists** (`hrNotifyRoles`/`lineManagerRoles`) — not access gates.
- **Candidate-portal / `candidateauth`** — unaffected.
- **Per-route authorization middleware** — keep authz inside handlers (no per-route gate exists today; introducing one is a larger refactor and not required).
- **Audit log UI for permission changes** beyond writing an `activity` record on mutations.

---

## Step-by-Step Tasks

> Phased so each phase is independently shippable and leaves the system behaving identically until the final cutover. Phases 1–2 add infra with **zero behavior change** (allowlists still enforce); Phase 3 swaps enforcement; Phases 4–6 add the admin surface + frontend.

### Task 1: Migration `000028_dynamic_rbac` (+ seed)
- **ACTION**: Create `backend/migrations/000028_dynamic_rbac.up.sql` + `.down.sql`.
- **IMPLEMENT**: Three tables — `rbac_permissions(key TEXT PRIMARY KEY, label_en TEXT, label_th TEXT, category TEXT, sort INT)`, `rbac_roles(key TEXT PRIMARY KEY, label_en TEXT, label_th TEXT, scope_kind TEXT NOT NULL DEFAULT 'store', is_builtin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`, `rbac_role_permissions(role_key TEXT NOT NULL REFERENCES rbac_roles(key) ON DELETE CASCADE, permission TEXT NOT NULL, PRIMARY KEY(role_key, permission))`. Seed all permission rows (the catalog table) and the 7 built-in roles with `is_builtin=TRUE` + correct `scope_kind`, then seed `rbac_role_permissions` to **exactly** the matrix above (INSERT … ON CONFLICT DO NOTHING).
- **MIRROR**: `MIGRATION_STYLE` (000018) — `CREATE TABLE IF NOT EXISTS`, heavy comments, no explicit tx; `.down.sql` drops in reverse.
- **IMPORTS**: n/a (SQL).
- **GOTCHA**: This is the ONE place the current matrix is committed to data — get it byte-for-byte right (cross-check against the catalog table above). `CHECK (scope_kind IN ('all','subregion','store'))` on `rbac_roles`. Do NOT add a FK from `users.role` → `rbac_roles.key` (legacy rows / Entra-claim roles may not exist as rows; validate in the service layer instead, fail-closed).
- **VALIDATE**: Apply locally to a scratch DB (or inspect-only if Docker disk-full, per session norm): `~/go/bin/migrate -path backend/migrations -database "$DBURL" up`; `SELECT role_key, count(*) FROM rbac_role_permissions GROUP BY 1;` matches the matrix counts.

### Task 2: Permission catalog + Authorizer + repo + model
- **ACTION**: Create `internal/rbac/permissions.go`, `model.go`, `repository.go`, `authorizer.go`.
- **IMPLEMENT**:
  - `permissions.go`: `const PermMembersAdmin = "members.admin"` … (all keys from the catalog); a slice `AllPermissions` with metadata for seeding/UI parity tests.
  - `model.go`: `Role{Key, LabelEn, LabelTh, ScopeKind, IsBuiltin, Permissions []string}`, `RoleInput`, `Permission{Key,LabelEn,LabelTh,Category}`.
  - `repository.go`: `ListRoles`, `GetRole`, `CreateRole`, `UpdateRole` (dynamic builder), `DeleteRole`, `SetRolePermissions`, `ListPermissions`, `UsersWithRole(key) int`. Mirror `REPO_*` patterns.
  - `authorizer.go`: `type Authorizer interface { Can(role, perm string) bool; Permissions(role string) []string; ScopeKind(role string) string; Reload(ctx) error }`. Impl holds `map[string]roleEntry` behind `sync.RWMutex`, loaded from repo; **super_admin bypass** in `Can`; unknown role → no perms + `store` scope. TTL refresh (configurable `RBAC_CACHE_TTL`, default 60s) via `loadedAt` checked in `Can`/lazy, plus explicit `Reload()` after writes.
- **MIRROR**: `REPO_DYNAMIC_UPDATE`, `REPO_INSERT_UNIQUE`; constructor `New(pool)` returns interface (Go DI pattern).
- **IMPORTS**: `context`, `fmt`, `errors`, `strings`, `sync`, `time`, `github.com/jackc/pgx/v5`, `pgconn`, `pgxpool`.
- **GOTCHA**: prod api may run >1 replica → an in-memory cache invalidated only on *local* write won't propagate. Use **TTL refresh** (every replica reloads within TTL) as the propagation mechanism; `Reload()` only makes the writer's own replica instant. Document this; 60s staleness on a permission tightening is acceptable (backend still the gate, just slightly delayed). Keep `Can` allocation-free on the hot path (read under RLock, no slice copies).
- **VALIDATE**: `go build ./... && go vet ./...`; unit test `Can` against the seeded matrix.

### Task 3: Wire authorizer into scope + every handler (the cutover)
- **ACTION**: Update `internal/rbac/scope.go` `Kind()` to consult the authorizer; replace each of the 16 allowlist checks with `authz.Can(u.Role, Perm…)`; inject the authorizer through each handler's constructor + `cmd/api/main.go`.
- **IMPLEMENT**: For each handler file (section "Files to Change"), add an `authz rbac.Authorizer` field + ctor param; replace `xxxRoles[u.Role]` with `h.authz.Can(u.Role, rbac.PermXxx)`. Keep the fail-closed `c.Locals(...).(DevUser)` cast and the `u.ID==""` guard. For approval, map level→`approval.decide.lN`. Delete the now-dead `var xxxRoles = map[...]` after each swap. `scope.go`: give `Scope` an `authz` reference (or change call sites to `rbac.NewWithAuthorizer(authz, role, store, subregion)`); `Kind()` returns `authz.ScopeKind(s.Role)` with the `store` fallback retained.
- **MIRROR**: `ALLOWLIST_GATE` (the before/after shown there).
- **IMPORTS**: `github.com/nexto/hr-ats/internal/rbac` in each handler.
- **GOTCHA**: `scope.New(...)` has ~10 call sites (`fit`, `search`, `interview`, `candidates`, `applications` ×many). Either thread the authorizer to each `scopeFrom(c)` or make the authorizer a package-level singleton set once at startup (simpler, acceptable for a process-wide read-only resolver — set in `main.go` before serving). Prefer the singleton to avoid touching 10 call sites: `rbac.SetAuthorizer(authz)` + `Scope.Kind()` uses it. Keep `default → store` so a missing authorizer (tests) still fails closed. Update existing handler tests that construct handlers to pass a stub authorizer (or use a default-allow-by-seed test authorizer).
- **VALIDATE**: `go build ./... && go vet ./... && go test ./...` — existing handler/scope tests must still pass (behavior unchanged because the seed matrix == old allowlists). Add a parity test (Task 8).

### Task 4: rbac admin API + `/me` permissions
- **ACTION**: Create `internal/rbac/handler.go` (+ `RegisterRoutes`); update `internal/users/handler.go`.
- **IMPLEMENT**:
  - Routes (all gated by `authz.Can(role, "rbac.admin")` → super_admin): `GET /api/v1/admin/rbac/permissions`, `GET /api/v1/admin/rbac/roles`, `POST /api/v1/admin/rbac/roles`, `PATCH /api/v1/admin/rbac/roles/:key`, `DELETE /api/v1/admin/rbac/roles/:key`. On any write → `authz.Reload(ctx)` + write an `activity` record.
  - `/me`: wrap the `DevUser` in `{id,email,role,store_id,subregion, permissions: authz.Permissions(u.Role), scope: authz.ScopeKind(u.Role)}`.
  - `hrauth` role validation (`model.go` `allowedRoles`): replace with `repo/authorizer.RoleExists(key)` so custom roles are assignable; keep built-ins seeded.
- **MIRROR**: `ADMIN_API_HANDLER` (hrauth/handler.go super_admin CRUD), `ME_PAYLOAD`.
- **IMPORTS**: fiber, httpx, rbac, middleware.
- **GOTCHA**: Guards — cannot DELETE `is_builtin` role; cannot DELETE a role any user still holds (return 409 with count, mirror self-lockout intent); cannot remove `rbac.admin`/narrow scope on super_admin (UI locks it, API double-checks). Validate `scope_kind ∈ {all,subregion,store}` and permission keys ∈ catalog (reject unknown). New role `key` slug: lowercase, `[a-z0-9_]+`, unique (23505 → 409).
- **VALIDATE**: `go test ./internal/rbac/...`; manual `curl` the endpoints behind a super_admin session.

### Task 5: Frontend — Me payload, `can()`, queries, nav
- **ACTION**: Update `lib/types.ts`, `lib/roles.ts`, `lib/queries.ts`, `components/shell/nav-config.tsx`.
- **IMPLEMENT**:
  - `Me` += `permissions: string[]`, `scope?: string`. Add `RbacRole`, `RbacPermission`.
  - `lib/roles.ts`: add `can(me, perm)`; reimplement each `canX` as `can(me, "perm")` — **change signatures to take `me` (or `permissions`)** rather than `role`. Keep a `PERMS` constant of keys mirroring the backend catalog.
  - `lib/queries.ts`: `useRbacRoles`, `useRbacPermissions`, `useCreateRole`, `useUpdateRole`, `useDeleteRole` — invalidate `["rbac","roles"]` and `["me"]` on success.
  - `nav-config.tsx`: `navForRole(me)` → push items by `can(me, perm)`.
- **MIRROR**: `FRONTEND_CAPABILITY`, `FRONTEND_QUERY`.
- **IMPORTS**: existing query/client conventions.
- **GOTCHA**: every call site of a changed `canX(me?.role)` must pass `me` instead — grep `canBulkUpload|canViewExecutive|canAccessApprovals|canViewReports|isLineManager|isMemberAdmin|isSuperAdmin|canReassignPlacement|canManageOffer|canManageLetters|canManageOnboarding|canRecordTaScorecard|canRecordLmScorecard|canRecordInterviewFeedback|canDecideApprovalLevel|canSubmitApproval` and update each (resume panels, pages, nav, MemberActions). Keep `isSuperAdmin` as `can(me,"rbac.admin")` or `me.role==="super_admin"` (keep both meanings consistent). tsc will catch missed signature changes — lean on it.
- **VALIDATE**: `pnpm exec tsc --noEmit` (zero errors = all call sites updated); `pnpm exec next build`.

### Task 6: Frontend — Admin Roles & Permissions UI
- **ACTION**: Create `components/admin/RolesPermissions.tsx`; update `UserManagement.tsx` + `admin/page.tsx` + messages.
- **IMPLEMENT**: A table of roles (label, scope, permission count, edit/delete) + an edit drawer/dialog with a grouped permission checkbox matrix and a scope `<Select>` + a "New role" form (key, labels, scope, perms). `UserManagement` `RoleSelect` fetches `useRbacRoles()` instead of `HR_ROLES`; role label from the API. super_admin row: all checked + locked, no delete. Add i18n keys to `admin` namespace (en+th), run parity.
- **MIRROR**: existing `UserManagement` dialog/table + `ui-styling`/admin namespace conventions; reuse `Dialog`, `Select`, `Switch`, `Table`, `Checkbox` primitives.
- **IMPORTS**: the new query hooks, ui primitives, `useTranslations("admin")`.
- **GOTCHA**: optimistic-free is fine; show server error toasts (mirror existing). Disable delete on built-in/in-use roles with a tooltip. Keep `HR_ROLES`/`roleLabel` removable once UserManagement is data-driven (or keep as offline fallback labels).
- **VALIDATE**: `node scripts/check-i18n-parity.mjs`; `tsc`; `next build`; browser: edit a role's permission, confirm it persists + the matrix reflects it.

### Task 7: Remove dead allowlists + frontend role-list constants
- **ACTION**: Delete the now-unused `var xxxRoles` maps (backend) and the `*_ROLES` arrays (frontend `lib/roles.ts`) once all references are gone.
- **IMPLEMENT**: grep to confirm zero references, then delete.
- **MIRROR**: n/a (cleanup).
- **GOTCHA**: Keep `approvalChain` (level→role ordering for *display/notification*, not a gate) unless fully replaced. Keep `hrNotifyRoles`/`lineManagerRoles` (recipient routing).
- **VALIDATE**: `go vet ./...` (no unused); `tsc`; `next build`.

### Task 8: Tests — parity is the safety net
- **ACTION**: Create backend table-driven tests; add a frontend smoke if practical.
- **IMPLEMENT**:
  - **Parity test** (most important): for each old allowlist (captured as a literal in the test), assert `authz.Can(role, perm) == oldAllowlist[role]` for all 7 roles — proves the migration seed didn't change behavior.
  - super_admin bypass; unknown role → no perms + store scope; scope `Kind()` for all 7 roles unchanged.
  - role CRUD: create custom role, set perms, `Can` reflects it after `Reload`; delete guards (built-in, in-use); scope validation.
  - `/me` returns expected permission set per role.
- **MIRROR**: `golang-testing` table-driven style; `go test -race -cover ./...`.
- **VALIDATE**: `cd backend && go test -race ./...` all green; coverage on `internal/rbac` ≥ 80%.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| Seed-matrix parity | each (role, perm) vs old allowlist literal | identical for all 7 roles × 21 perms | core safety |
| super_admin bypass | `Can("super_admin", any)` | true | yes |
| unknown role | `Can("foo", "reports.view")`, `ScopeKind("foo")` | false, `store` | yes (fail-closed) |
| scope unchanged | `Kind()` for all 7 roles | all/subregion/store as before | regression |
| create+grant | new role + SetRolePermissions + Reload | `Can(newrole, perm)` true | yes |
| delete built-in | `DeleteRole("super_admin")` | error (409) | yes |
| delete in-use | role held by a user | error w/ count | yes |
| /me payload | super_admin / hr_staff | correct permissions[] + scope | yes |
| cache TTL | grant, wait < TTL on 2nd replica sim | reflects after reload | concurrency |

### Edge Cases Checklist
- [ ] Empty/null role (legacy user, Entra without claim) → no perms, store scope, nothing breaks
- [ ] Custom role with zero permissions → can authenticate, sees only ungated surfaces
- [ ] Removing a permission from a role mid-session → enforced on next request (≤ TTL)
- [ ] Concurrent role edit + request (RWMutex; no torn reads)
- [ ] super_admin cannot be locked out via any edit
- [ ] Unknown permission key in API write → rejected
- [ ] Migration re-run (idempotent) → no dupes (ON CONFLICT)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib
```
EXPECT: zero errors.

### Unit Tests
```bash
cd backend && go test -race ./internal/rbac/... ./internal/applications/... ./internal/members/...
```
EXPECT: all pass; rbac coverage ≥ 80%.

### Full Test Suite
```bash
cd backend && go test -race ./...
node scripts/check-i18n-parity.mjs
```
EXPECT: no regressions; i18n th/en in parity.

### Database Validation
```bash
# scratch/staging DB
~/go/bin/migrate -path backend/migrations -database "$DBURL" up
psql "$DBURL" -c "SELECT r.key, r.scope_kind, count(rp.permission) FROM rbac_roles r LEFT JOIN rbac_role_permissions rp ON rp.role_key=r.key GROUP BY 1,2 ORDER BY 1;"
```
EXPECT: schema_migrations version 28; per-role permission counts match the matrix.

### Browser Validation
```bash
cd frontend && pnpm exec next build && pnpm dev   # or staging
```
EXPECT: /admin shows Roles & Permissions (super_admin); toggling a permission changes a non-super user's access; non-super_admin still 403 on admin API.

### Manual Validation
- [ ] Log in as super_admin → /admin → edit "hr_manager" remove `offer.write` → an hr_manager loses the offer panel (≤ TTL)
- [ ] Create custom role "auditor_store" scope=store, perms=[reports.view] → assign to a user → they see only Reports, store-scoped
- [ ] Confirm every previously-working role still has identical access (spot-check each of the 7)
- [ ] super_admin row is locked (all perms, scope all, no delete)

---

## Acceptance Criteria
- [ ] All tasks completed
- [ ] All validation commands pass
- [ ] Seed-matrix parity test proves zero behavior change at cutover
- [ ] No type errors / no lint errors / `go vet` clean
- [ ] Admin can CRUD roles + toggle permissions + set scope; changes enforced server-side
- [ ] Frontend gates on `me.permissions`, not hardcoded role lists
- [ ] super_admin cannot be locked out

## Completion Checklist
- [ ] Code follows discovered patterns (pgxpool repo, Fiber handler, allowlist→Can swap)
- [ ] Error handling matches `fmt.Errorf("rbac: …: %w", err)` + 23505/ErrNoRows sentinels
- [ ] Migration matches 000018 additive style; down mirrors up
- [ ] Tests follow table-driven style; ≥80% on internal/rbac
- [ ] No hardcoded role lists remain (backend allowlists + frontend `*_ROLES` deleted)
- [ ] i18n keys added (en+th parity) for the admin UI
- [ ] Self-contained — no further codebase search needed to implement

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Seed matrix drift from old allowlists → silent access change | Med | High | Parity test (Task 8) is mandatory and blocks merge |
| super_admin lockout via bad edit | Low | Critical | Hard code bypass + built-in non-editable/non-deletable + API guards |
| Multi-replica cache staleness on prod api | Med | Low | TTL refresh (60s) on every replica; backend remains the gate |
| Missed `canX` call-site signature change (frontend) | Med | Med | tsc fails the build until all are updated — lean on the type checker |
| Missed handler swap leaves an old allowlist live | Low | Med | grep audit in Task 7 + parity covers behavior; vet flags unused vars |
| Migration applied but app not redeployed (or vice-versa) | Med | Med | Phases 1–2 are no-op until Phase 3; deploy order = migrate → api → dashboard (runbook) |
| Entra-claim roles not present as `rbac_roles` rows | Low | Med | Validate role in service (fail-closed), NOT via users.role FK; unknown role → no perms |

## Notes
- **Deploy shape** (per `docs/module-3-deploy-runbook.md` + session memory): migration 000028 is **manual** (`~/go/bin/migrate … up`) BEFORE rolling images; this touches **api** (enforcement + admin API) and **dashboard** (admin UI + capability gates) — worker/scheduler/portal unaffected. Dashboard build must pass the 4 `NEXT_PUBLIC_AZURE_AD_*` build-args incl. **`AUTHORITY=https://login.microsoftonline.com/organizations`** (not a gh repo var — pass manually or SSO regresses to single-tenant; see memory `uat-feedback-fixes`).
- **CI is billing-blocked** → squash + `--admin` merge per PR; consider one PR per phase (1+2 infra, 3 cutover, 4 API, 5 fe-core, 6 fe-ui, 7 cleanup) so each is reviewable and independently deployable.
- **Why permissions are code, roles are data**: each gate is a *call site*; a truly new capability can't appear without code. The valuable dynamism — recomposing existing capabilities into roles and scoping them — is fully delivered.
- **Backend is the real gate**; the frontend `can()` only decides what to render. This invariant is preserved (the agent confirmed it's already how the codebase works).
