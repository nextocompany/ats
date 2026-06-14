# Plan: Career-Portal Member Management (HR Admin, end-to-end)

## Summary
A comprehensive HR-dashboard section to administer **career-portal members** (the `candidate_accounts` created by the membership feature). Covers a searchable/filterable **directory** + member **detail**, **lifecycle** actions (suspend/reactivate, force-logout, edit, PDPA anonymize/erasure), and **CRM** extras (notes, tags, bulk actions, CSV export, stats). New backend package `internal/members` (HR-facing, distinct from the public `internal/candidateauth`), a new `/members` dashboard area gated to **super_admin + hr_manager**, with destructive PDPA actions gated to **super_admin only**.

## User Story
As an **HR manager / super admin**, I want to browse, search, inspect, and manage the people who signed up through the career portal — including their linked providers, applications, consent, and the ability to suspend or erase an account on request — so that I can run the candidate community day-to-day and honour PDPA requests without touching the database.

## Problem → Solution
**Current:** `candidate_accounts` exists and the public portal lets candidates sign up / log in (LINE/Google/email-OTP) and apply, but HR has **no UI at all** to see or manage members. The only lifecycle automation is the scheduled retention sweep (anonymizes *per-application `candidates`* after 365d) and the auth-cleanup sweep (deletes expired sessions/OTPs). There's no manual suspend, no on-demand PDPA erasure for an account, no member directory, no notes/tags/export.
**Desired:** A first-class "Members" console: list + search + filter + stats, a rich member detail, and governed lifecycle + CRM operations with a full audit trail.

## Metadata
- **Complexity**: XL — **split into 3 PRs** (Phase A directory, Phase B lifecycle, Phase C CRM)
- **Source PRD**: N/A (free-form request, scoped via clarifying Q: "CRM เต็ม", access "super_admin + hr_manager")
- **PRD Phase**: N/A
- **Estimated Files**: ~30 across the 3 phases (backend `internal/members/*`, 2 migrations, frontend members area + nav/types/queries, tests)

---

## UX Design

### Before
```
HR dashboard nav: Overview · Inbox · Candidates · Search · Analytics · (Admin*)
                                                                       *super_admin only
  → no way to see/manage career-portal members at all
```

### After
```
HR dashboard nav: Overview · Inbox · Candidates · Search · Analytics · Members* · (Admin*)
                                                                       *super_admin + hr_manager

/members (list)                              /members/[id] (detail)
┌───────────────────────────────────┐       ┌──────────────────────────────────┐
│ Members  · stats strip (total/new/ │       │ ← Back   [Suspend][Erase*][Email] │
│   suspended/with-apps)             │       │ Identity: name·email·phone·prov   │
│ filters: search · provider · status│       │ Providers: LINE✓ Google✓ Email✓   │
│   · has-resume · date    [Export▾] │       │ Resume: view (signed URL)         │
│ ┌───────────────────────────────┐ │       │ PDPA: consent vX · date           │
│ │☑ name  email  prov  prov-icons│ │       │ Applications (N): list→/apps/[id]  │
│ │  apps  status  joined    →    │ │       │ Sessions: 2 active  [Force logout]│
│ └───────────────────────────────┘ │       │ Tags: [retail][north] +           │
│ [bulk: tag · suspend · export]    │       │ Notes (HR-only): timeline + add   │
│ pagination                         │       │ Activity: suspend/erase audit     │
└───────────────────────────────────┘       └──────────────────────────────────┘
*Erase = super_admin only
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Sidebar nav | super_admin sees Admin | super_admin + hr_manager also see **Members** | `navForRole` change |
| Member directory | none | `/members` list (search/filter/paginate/stats/bulk/export) | mirrors `/applications` inbox |
| Member detail | none | `/members/[id]` (profile, providers, apps, sessions, consent, tags, notes, activity) | mirrors `/candidates/[id]` layout |
| Suspend/reactivate | none | toggle that blocks login + revokes sessions | super_admin + hr_manager |
| PDPA erasure | scheduled only (candidates) | on-demand account anonymize (confirm dialog) | **super_admin only** |
| Export | analytics snapshot only | members CSV (respects filters) | sync download |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/list.go` | 14-152 | **Primary mirror** for the members List: `ListFilter`, `normalize()`, dynamic WHERE with `add()` placeholders, COUNT + paginated data query, rbac clause integration |
| P0 | `backend/internal/applications/dashboard_handler.go` | 51-124, 78-81 | Dashboard handler + `RegisterDashboardRoutes`, `scopeFrom`, paginated `httpx.Envelope` with `Meta` |
| P0 | `backend/internal/settings/handler.go` | 10-64 | **Role-gate pattern**: `adminRolesAllowed` map + 403; mirror for super_admin+hr_manager and the super_admin-only destructive gate |
| P0 | `backend/internal/candidateauth/repository.go` | 20-47, 65-94, 193-259 | Account schema columns, `accountColumns`/`scanAccount`, GetByID, UpdateProfile, RevokeSession — the data the members repo reads/writes |
| P0 | `backend/internal/candidateauth/model.go` | 14-59 | `Account` struct + JSON tags (note LineUserID/GoogleSub/ResumeBlobURL are `json:"-"`); sentinel errors |
| P0 | `backend/internal/pdpa/retention.go` | 33-198 | **Anonymize pattern to mirror** for on-demand account erasure: tx, redact UPDATEs, `…AND anonymized_at IS NULL` guard, gather blobs, delete-after-commit, audit |
| P0 | `backend/internal/activity/activity.go` | 15-64 | Audit: `Record(ctx, action, entityType, entityID, newValue)` + action constants — add member_* actions |
| P0 | `frontend/app/(app)/applications/page.tsx` | 65-378 | **Primary mirror** for the members list page: PageHeader, filter chips, Select filters, SummaryStrip, table+mobile cards, `<Pagination>` (exported here), `<BulkActionBar>` mount, selection state |
| P0 | `frontend/components/shell/nav-config.tsx` | 17-35 | `NAV`/`ADMIN_NAV`/`navForRole`/`ALL_NAV` — add `MEMBERS_NAV` + gate to super_admin+hr_manager |
| P0 | `frontend/lib/queries.ts` | 26-28, 49-64, 152-160, 227-234 | `useMe`, list hook with `buildQuery`, mutation+invalidate, bulk mutation — mirror for member hooks |
| P0 | `frontend/lib/api.ts` | 21-61 | envelope unwrap (`{data, meta}`), `buildQuery`, `ApiError` |
| P1 | `frontend/app/(app)/candidates/[id]/page.tsx` | 1-100 | Detail page layout (grid `[1fr_320px]`, sections, loading/error) to mirror for `/members/[id]` |
| P1 | `frontend/app/(app)/admin/page.tsx` | 10-44 | role-aware page gating (`useMe` → if not allowed, show restricted panel) — mirror for `/members` (super_admin+hr_manager) |
| P1 | `frontend/components/bulk/BulkActionBar.tsx` | 19-50 | bulk selection + mutation + toast pattern |
| P1 | `frontend/components/ui/dialog.tsx` | 10-161 | confirm dialog for destructive (suspend/erase) — **no AlertDialog primitive; use Dialog** |
| P1 | `backend/internal/reports/export_service.go` | 49-77 | CSV encoding style (`encoding/csv`, header row) — but members export is a SYNC download, not blob+notify |
| P1 | `backend/cmd/api/main.go` | 261-292 | dashboard handler wiring block — register the members handler here |
| P1 | `backend/internal/candidateauth/service.go` | (whole) | login/session finalize paths — must reject suspended/anonymized accounts (Phase B cross-cutting) |
| P2 | `backend/internal/applications/list_integration_test.go` | 23-97 | integration test template (TRUNCATE+seed+assert filter/rank/paginate/scope) |
| P2 | `backend/internal/fit/handler_test.go` | 31-61 | handler unit test (role/403, routes) template |
| P2 | `frontend/components/analytics/Charts.tsx` | 42-87 | KPI/stats card pattern for the members stats strip |
| P2 | `backend/pkg/blob/blob.go` | 171-190 | `DeleteStored`/`Delete` for erasing a member's resume blob |
| P2 | `backend/internal/candidateauth/cleanup.go` | 24-96 | batched-delete pattern (for any member-row sweeps if needed) |

## External Documentation
No external research needed — every capability maps to an established internal pattern (dashboard list/RBAC/activity, pdpa anonymize, reports CSV, candidateauth repo). PDPA semantics already implemented in `internal/pdpa`.

---

## Patterns to Mirror

### NAMING_CONVENTION (Go package = noun; HR-facing dashboard handler + RegisterDashboardRoutes)
```go
// SOURCE: backend/internal/applications/dashboard_handler.go:71-75
func RegisterDashboardRoutes(app *fiber.App, h *DashboardHandler) {
	v1 := app.Group("/api/v1")
	v1.Get("/applications", h.List)
	v1.Post("/applications/bulk", h.Bulk)
}
```

### ROLE_GATE (allowlist map → 403; two tiers)
```go
// SOURCE: backend/internal/settings/handler.go:12, 47-49
var adminRolesAllowed = map[string]bool{"super_admin": true}
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
if !adminRolesAllowed[u.Role] {
	return fiber.NewError(fiber.StatusForbidden, "insufficient role")
}
// members: var memberAdminRoles = {"super_admin":true,"hr_manager":true}
//          var memberEraseRoles = {"super_admin":true}
```

### LIST_FILTER (dynamic WHERE + COUNT + paginate)
```go
// SOURCE: backend/internal/applications/list.go:75-131
var args []any
add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
var conds []string
if f.Status != "" { conds = append(conds, "candidate_accounts.status = "+add(f.Status)) }
// COUNT(*) … where; then SELECT … LIMIT add(limit) OFFSET add((page-1)*limit)
```

### PAGINATED_ENVELOPE
```go
// SOURCE: backend/internal/applications/dashboard_handler.go:120-124
return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Member]{
	Success: true, Data: items, Meta: &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
})
```

### REPOSITORY (pgx, accountColumns/scan helper, COALESCE)
```go
// SOURCE: backend/internal/candidateauth/repository.go:65-82, 193-201
const accountColumns = `id, full_name, COALESCE(email,''), email_verified, ...`
// UPDATE candidate_accounts SET full_name = COALESCE(NULLIF($2,''), full_name) ... WHERE id=$1
```

### ANONYMIZE_TX (on-demand account erasure)
```go
// SOURCE: backend/internal/pdpa/retention.go:163-198
tx, _ := s.pool.Begin(ctx); defer tx.Rollback(ctx)
// UPDATE candidate_accounts SET full_name='[ลบข้อมูลแล้ว]', email=NULL, phone=NULL,
//   line_user_id=NULL, google_sub=NULL, resume_blob_url=NULL, status='anonymized',
//   anonymized_at=NOW() WHERE id=$1 AND anonymized_at IS NULL
// DELETE FROM candidate_sessions WHERE account_id=$1   (cascade already, but force-logout)
tx.Commit(ctx)
// after commit: blob.DeleteStored(resumeURL); activity.Record(member_anonymize)
```

### ACTIVITY_AUDIT
```go
// SOURCE: backend/internal/activity/activity.go:50-64 + usage dashboard_handler.go:169
_ = h.activity.Record(c.UserContext(), activity.ActionMemberSuspend, "member", id, fiber.Map{"by": u.Email})
```

### FRONTEND_LIST_HOOK
```ts
// SOURCE: frontend/lib/queries.ts:49-64
export function useMembers(filter: MemberFilter) {
  return useQuery({
    queryKey: ["members", filter],
    queryFn: () => api.get<Member[]>("/api/v1/admin/members" + buildQuery({ ...filter })),
  });
}
```

### FRONTEND_NAV_GATE
```tsx
// SOURCE: frontend/components/shell/nav-config.tsx:31-32
export function navForRole(role?: string): NavItem[] {
  const base = role === "super_admin" || role === "hr_manager" ? [...NAV, MEMBERS_NAV] : NAV;
  return role === "super_admin" ? [...base, ADMIN_NAV] : base;
}
```

### CONFIRM_DIALOG (no AlertDialog primitive → Dialog)
```tsx
// SOURCE: frontend/components/ui/dialog.tsx:10-161
<Dialog><DialogTrigger asChild><Button variant="destructive">ลบข้อมูล</Button></DialogTrigger>
  <DialogContent><DialogHeader><DialogTitle>ยืนยันการลบข้อมูลสมาชิก</DialogTitle>
  <DialogDescription>ดำเนินการแล้วย้อนกลับไม่ได้</DialogDescription></DialogHeader>
  <DialogFooter><DialogClose asChild><Button variant="outline">ยกเลิก</Button></DialogClose>
  <Button variant="destructive" onClick={erase}>ลบถาวร</Button></DialogFooter></DialogContent></Dialog>
```

### TEST_STRUCTURE
```go
// SOURCE: backend/internal/applications/list_integration_test.go:23-97 (TRUNCATE+seed+assert)
// SOURCE: backend/internal/fit/handler_test.go:31-61 (role/403 handler unit test)
```

---

## Files to Change

### Phase A — Directory (PR 1)
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000016_member_admin.up.sql` / `.down.sql` | CREATE | `candidate_accounts` add `status VARCHAR(16) DEFAULT 'active'`, `suspended_at`, `suspended_by UUID`, `anonymized_at`; indexes for search (email/full_name/phone) |
| `backend/internal/members/model.go` | CREATE | `Member` (admin view: account fields + linked-provider bools + applications_count + active_sessions + status), `ListFilter`, `Stats` |
| `backend/internal/members/repository.go` | CREATE | List (filter+paginate+search), GetByID (with joins: app count, sessions, consent), Stats |
| `backend/internal/members/handler.go` | CREATE | `List`, `Detail`, `Stats`; role-gate super_admin+hr_manager |
| `backend/internal/members/routes.go` | CREATE | `RegisterDashboardRoutes` under `/api/v1/admin/members` |
| `backend/cmd/api/main.go` | UPDATE | wire members handler (pool + activity + blob signer) |
| `backend/internal/members/*_test.go` | CREATE | repo integration (filter/paginate/search/stats) + handler unit (role 403) |
| `frontend/lib/types.ts` | UPDATE | `Member`, `MemberFilter`, `MemberStats` |
| `frontend/lib/queries.ts` | UPDATE | `useMembers`, `useMember`, `useMemberStats` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | `MEMBERS_NAV` + gate super_admin+hr_manager |
| `frontend/app/(app)/members/page.tsx` | CREATE | list (mirror inbox) + role gate |
| `frontend/app/(app)/members/[id]/page.tsx` | CREATE | detail (mirror candidate detail) |

### Phase B — Lifecycle (PR 2)
| File | Action | Justification |
|---|---|---|
| `backend/internal/members/lifecycle.go` (or extend service/repo) | CREATE/UPDATE | Suspend/Reactivate (+ revoke sessions), UpdateProfile, ForceLogout, Anonymize (tx, super_admin) |
| `backend/internal/members/handler.go` | UPDATE | PATCH status, PATCH profile, POST force-logout, POST anonymize (super_admin gate) |
| `backend/internal/candidateauth/repository.go` + `service.go` | UPDATE | **Reject suspended/anonymized accounts** on session resolve + login finalize (suspension must actually block login) |
| `backend/internal/activity/activity.go` | UPDATE | add `ActionMemberSuspend/Reactivate/Anonymize/ForceLogout/ProfileEdit` |
| `backend/internal/members/*_test.go` | UPDATE | suspend-blocks-login, anonymize idempotent + erases blob, role gates |
| `frontend/lib/queries.ts` | UPDATE | `useSetMemberStatus`, `useForceLogout`, `useAnonymizeMember`, `useUpdateMember` |
| `frontend/app/(app)/members/[id]/page.tsx` | UPDATE | action buttons + confirm Dialogs (erase = super_admin only) |

### Phase C — CRM (PR 3)
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000017_member_crm.up.sql` / `.down.sql` | CREATE | `member_notes` (id, account_id FK, author_email, body, created_at), `member_tags` (account_id FK, tag, PRIMARY KEY(account_id,tag)) |
| `backend/internal/members/notes.go`, `tags.go` (or extend) | CREATE | notes CRUD, tag add/remove + filter-by-tag in List |
| `backend/internal/members/export.go` | CREATE | sync CSV (`encoding/csv`) returning text/csv + Content-Disposition |
| `backend/internal/members/handler.go` | UPDATE | notes/tags endpoints, bulk endpoint, `GET …/export.csv` |
| `frontend/components/members/*` + `[id]/page.tsx` | CREATE/UPDATE | Notes timeline, Tag chips, BulkActionBar (members variant), Export button, stats segments |
| tests | CREATE | notes/tags repo + handler; export CSV content |

## NOT Building
- **No self-service "delete my account"** on the public portal (this is HR-side only).
- **No merge-accounts** (deduping two member accounts) — out of scope.
- **No real-time presence / login analytics** beyond active-session count + last-session timestamp.
- **No email campaigns/marketing** — the "Email member" action is a single transactional send via `pkg/email` only (Phase C optional; can defer).
- **No change to the scheduled retention sweep** (`internal/pdpa`) — on-demand erasure is separate.
- **No new RBAC roles** — reuse existing `super_admin`/`hr_manager` strings.
- **No hard DELETE of `candidate_accounts`** — erasure = anonymize-in-place (FK from `candidates.account_id` is RESTRICT; hard delete would fail or orphan applications). Anonymize keeps referential integrity.

---

## Step-by-Step Tasks

### PHASE A — DIRECTORY

#### Task A1: Migration 000016 (status + search indexes)
- **ACTION**: Create `backend/migrations/000016_member_admin.{up,down}.sql`.
- **IMPLEMENT** (up):
  ```sql
  ALTER TABLE candidate_accounts
    ADD COLUMN IF NOT EXISTS status       VARCHAR(16) NOT NULL DEFAULT 'active', -- active | suspended | anonymized
    ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS suspended_by UUID,
    ADD COLUMN IF NOT EXISTS anonymized_at TIMESTAMPTZ;
  CREATE INDEX IF NOT EXISTS idx_candidate_accounts_status  ON candidate_accounts (status);
  CREATE INDEX IF NOT EXISTS idx_candidate_accounts_created ON candidate_accounts (created_at DESC);
  ```
  (down): drop the indexes + columns.
- **MIRROR**: `migrations/000013_candidate_accounts.up.sql` (IF NOT EXISTS idempotency).
- **GOTCHA**: `suspended_by` no FK to `users` (mock/dev users may be absent — same decision as fit `generated_by`). status default 'active' so existing rows stay active.
- **VALIDATE**: `migrate up` (dev v15→16) then `down 1` then `up`; clean.

#### Task A2: `members` model
- **ACTION**: Create `backend/internal/members/model.go`.
- **IMPLEMENT**:
  ```go
  type Member struct {
    ID uuid.UUID `json:"id"`; FullName, Email, Phone, Province string
    EmailVerified bool `json:"email_verified"`
    LineLinked, GoogleLinked, EmailLinked bool `json:"line_linked"` // etc.
    HasResume bool `json:"has_resume"`; ResumeFileType string `json:"resume_file_type"`
    Status string `json:"status"`
    PDPAConsent bool `json:"pdpa_consent"`; PDPAVersion string `json:"pdpa_version"`
    ApplicationsCount int `json:"applications_count"`
    ActiveSessions int `json:"active_sessions"`
    LastSeenAt *time.Time `json:"last_seen_at"` // max(candidate_sessions.created_at)
    CreatedAt time.Time `json:"created_at"`
  }
  type ListFilter struct { Search, Provider, Status string; HasResume *bool; From,To *time.Time; Page,Limit int }
  type Stats struct { Total, Active, Suspended, WithApplications, NewThisWeek int; ByProvider map[string]int }
  ```
  Add `(f *ListFilter) normalize()` (default Limit 20, max 100, Page≥1) — mirror applications.
- **MIRROR**: `applications/list.go:14-30` + `candidateauth/model.go`.
- **GOTCHA**: never expose raw `line_user_id`/`google_sub` — only booleans (PII minimization, matches `json:"-"` in candidateauth).
- **VALIDATE**: `go build ./internal/members/...`.

#### Task A3: `members` repository (List + GetByID + Stats)
- **ACTION**: Create `backend/internal/members/repository.go`.
- **IMPLEMENT**:
  - `List(ctx, f ListFilter) ([]Member, int, error)`: dynamic WHERE on `candidate_accounts` —
    - search: `(full_name ILIKE '%'||$n||'%' OR email ILIKE … OR phone ILIKE …)`
    - provider: `line_user_id IS NOT NULL` / `google_sub IS NOT NULL` / `email IS NOT NULL AND email_verified` per value
    - status filter; has_resume → `resume_blob_url IS NOT NULL`; date range on created_at.
    - applications_count via correlated subquery `(SELECT COUNT(*) FROM applications ap JOIN candidates c ON c.id=ap.candidate_id WHERE c.account_id = candidate_accounts.id)`; active_sessions via `(SELECT COUNT(*) FROM candidate_sessions s WHERE s.account_id=candidate_accounts.id AND s.revoked_at IS NULL AND s.expires_at>NOW())`; last_seen via `(SELECT MAX(created_at) FROM candidate_sessions …)`.
    - COUNT(*) then paginated SELECT `ORDER BY created_at DESC LIMIT/OFFSET`.
  - `GetByID(ctx, id) (*Member, error)` → ErrNotFound on no rows.
  - `Stats(ctx) (Stats, error)` → a few aggregate queries (total/active/suspended, with-apps, new-this-week, counts by provider).
  - NO rbac.Scope (members are global, not store-scoped — access is role-gated at the handler, like settings).
- **MIRROR**: `applications/list.go:72-152` (dynamic WHERE/COUNT/paginate), `candidateauth/repository.go:65-94` (scan helper).
- **IMPORTS**: `context, fmt, strings, time, uuid, pgx, pgxpool`.
- **GOTCHA**: define `var ErrNotFound = errors.New("members: not found")`; correlated subqueries are fine for ≤ a few thousand members (add the created_at index from A1). Use a single `scanMember` shared by List/GetByID.
- **VALIDATE**: integration test A7.

#### Task A4: `members` handler + routes + wiring
- **ACTION**: Create `handler.go`, `routes.go`; edit `cmd/api/main.go`.
- **IMPLEMENT**:
  - `var memberAdminRoles = map[string]bool{"super_admin": true, "hr_manager": true}`; `authorized(c)` → 403 if not in map (mirror settings).
  - `List` (parse ListFilter from query via `c.Query`), `Detail` (`:id`), `Stats`. Return paginated envelope for List; `httpx.OK` for Detail/Stats.
  - Detail also returns a signed resume URL (inject a `ResumeSigner` like dashboard_handler) when `has_resume`.
  - `routes.go`: `g := app.Group("/api/v1/admin/members"); g.Get("/", h.List); g.Get("/stats", h.Stats); g.Get("/:id", h.Detail)` — **register `/stats` BEFORE `/:id`** (static path precedence, same lesson as candidates/search).
  - `main.go`: `members.RegisterDashboardRoutes(app, members.NewHandler(members.NewRepository(pool), activityLog, blobClient))` near the dashboard block (~line 290).
- **MIRROR**: `settings/handler.go:10-64` (role gate), `applications/dashboard_handler.go:51-124` (handler+routes+envelope), `cmd/api/main.go:271-291` (wiring).
- **GOTCHA**: endpoints are under `/api/v1/...` (authed, not in `isUnauthedPath`). The role gate is the access control (no store scope).
- **VALIDATE**: `go build ./...`; handler unit test A7; curl with mock super_admin.

#### Task A5: Frontend types
- **ACTION**: Edit `frontend/lib/types.ts`.
- **IMPLEMENT**: `Member`, `MemberFilter` (search?, provider?, status?, has_resume?, from?, to?, page?, limit?), `MemberStats` mirroring the Go JSON.
- **MIRROR**: `types.ts:27-58, 193-200`.
- **VALIDATE**: `pnpm tsc --noEmit`.

#### Task A6: Frontend hooks + nav
- **ACTION**: Edit `frontend/lib/queries.ts` + `frontend/components/shell/nav-config.tsx`.
- **IMPLEMENT**:
  - `useMembers(filter)` (returns `{data, meta}` for pagination — like inbox), `useMember(id)`, `useMemberStats()`.
  - nav: add `MEMBERS_NAV = { href:"/members", label:"Members", icon: UserCog }` (import from lucide); update `navForRole` per FRONTEND_NAV_GATE; add to `ALL_NAV`.
- **MIRROR**: `queries.ts:49-64`, `nav-config.tsx:17-35`.
- **GOTCHA**: keep `useMembers` returning the full `{data, meta}` (don't `.then(r=>r.data)`) so the page can read `meta.total` for pagination — match how the inbox reads meta.
- **VALIDATE**: `pnpm tsc --noEmit` + `pnpm eslint`.

#### Task A7: Backend tests (Phase A)
- **ACTION**: Create `members/repository_integration_test.go` (`//go:build integration`) + `members/handler_test.go`.
- **IMPLEMENT**: integration — TRUNCATE candidate_accounts/candidate_sessions/candidates/applications; seed members (varied providers, statuses, with/without apps); assert List filter+search+paginate, applications_count join, Stats. handler — role 403 for non-admin role, 200 for hr_manager, 400 bad id.
- **MIRROR**: `applications/list_integration_test.go:23-97`, `fit/handler_test.go:31-61`.
- **VALIDATE**: `go test ./internal/members/...` + `go test -tags=integration ./internal/members/...`.

#### Task A8: Members list + detail pages
- **ACTION**: Create `frontend/app/(app)/members/page.tsx` + `[id]/page.tsx`.
- **IMPLEMENT**:
  - list: role gate (`useMe` → if not super_admin/hr_manager show restricted panel, mirror admin/page.tsx); `PageHeader`; SummaryStrip from `useMemberStats`; Select filters (provider/status/has-resume) + search input (debounced → `setParam`); table (name/email/province/provider-icons/apps/status badge/joined) + mobile cards; `<Pagination>` (import from applications/page or extract); rows link to `/members/[id]`.
  - detail: mirror candidate detail grid; sections: Identity, Providers (LINE/Google/Email badges), Resume (signed URL link), PDPA consent, Applications list (→ `/applications/[id]`), Sessions (active count), placeholder areas for Tags/Notes/Actions (filled in B/C).
- **MIRROR**: `applications/page.tsx:65-378`, `candidates/[id]/page.tsx:1-100`, `admin/page.tsx:10-44` (gate).
- **GOTCHA**: Thai UI strings; CP-Axtra tokens; `tabular-nums`; no `console.log`. URL-as-state for filters (searchParams) like the inbox.
- **VALIDATE**: `pnpm build`; load `/members` + `/members/<id>` in dev.

### PHASE B — LIFECYCLE

#### Task B1: Suspend / reactivate / force-logout (repo + handler)
- **ACTION**: Extend `members` repo + handler.
- **IMPLEMENT**:
  - repo `SetStatus(ctx, id, status, byUserID)` → `UPDATE candidate_accounts SET status=$2, suspended_at=CASE WHEN $2='suspended' THEN NOW() END, suspended_by=$3, updated_at=NOW() WHERE id=$1 AND status<>'anonymized'`; on suspend also `DELETE FROM candidate_sessions WHERE account_id=$1` (force logout).
  - repo `ForceLogout(ctx, id)` → delete the member's sessions.
  - handler `PATCH /api/v1/admin/members/:id/status` (body `{status}`) + `POST /:id/force-logout`; gate super_admin+hr_manager; `activity.Record`.
- **MIRROR**: `candidateauth/repository.go:253-259` (RevokeSession), settings role gate.
- **GOTCHA**: never let status move OUT of 'anonymized'.
- **VALIDATE**: integration test (suspend sets status + clears sessions).

#### Task B2: Enforce suspension on login (cross-cutting in candidateauth)
- **ACTION**: Edit `backend/internal/candidateauth/repository.go` (`FindAccountBySessionHash`) + `service.go` (login finalize for email/LINE/Google).
- **IMPLEMENT**: when resolving a session or finalizing a login, if the account `status <> 'active'` → treat as unauthenticated (return ErrNotFound / refuse to issue a session). Simplest: add `AND ca.status = 'active'` to the session-resolve JOIN and a status check after FindOrCreate in the OAuth/email finalize.
- **MIRROR**: `candidateauth/repository.go:232-251` (session resolve query).
- **GOTCHA**: this is the whole point of suspend — without it, suspend is cosmetic. Add a test: suspended account's existing cookie stops working AND a fresh login is refused.
- **VALIDATE**: integration test; manual: suspend → `GET /api/v1/public/auth/me` with that session → 401.

#### Task B3: On-demand PDPA anonymize (super_admin only)
- **ACTION**: Add `members.Anonymize(ctx, id, byUserID)` (tx) + handler `POST /:id/anonymize`.
- **IMPLEMENT**: mirror `pdpa/retention.go` anonymize tx but for the ACCOUNT: redact `candidate_accounts` (full_name='[ลบข้อมูลแล้ว]', email/phone/line_user_id/google_sub/line_display_id/province/resume_blob_url=NULL, status='anonymized', anonymized_at=NOW()) with `AND anonymized_at IS NULL` guard; delete sessions; gather resume blob URL first; after commit `blob.DeleteStored`; `activity.Record(ActionMemberAnonymize, "member", id, {by})`. Gate **super_admin only** (`memberEraseRoles`). Also offer to anonymize the member's per-application `candidates` rows? — **out of scope here** (the scheduled retention sweep handles candidates; account-level erasure removes the login identity + saved resume). Document this boundary.
- **MIRROR**: `pdpa/retention.go:163-198`, `blob/blob.go:171-190`, `activity`.
- **GOTCHA**: anonymize is irreversible → super_admin only + confirm dialog. Idempotent (guard). Don't hard-DELETE (FK RESTRICT from candidates.account_id).
- **VALIDATE**: integration: anonymize redacts + deletes blob + idempotent; role test 403 for hr_manager.

#### Task B4: Edit member profile (admin)
- **ACTION**: repo `AdminUpdateProfile(ctx, id, fields)` + handler `PATCH /:id`.
- **IMPLEMENT**: sparse update (full_name/phone/province/email) mirroring `candidateauth UpdateProfile` COALESCE NULLIF pattern; activity log.
- **MIRROR**: `candidateauth/repository.go:193-206`.
- **VALIDATE**: integration test.

#### Task B5: Frontend lifecycle actions
- **ACTION**: hooks + detail page actions.
- **IMPLEMENT**: `useSetMemberStatus`, `useForceLogout`, `useAnonymizeMember`, `useUpdateMember` (mutations invalidate `["member",id]` + `["members"]`). Detail page: Suspend/Reactivate button, Force-logout, Edit (inline form/dialog), **Erase** button visible only when `me.role==='super_admin'`, each destructive action behind a confirm `<Dialog>` with toast.
- **MIRROR**: `queries.ts:152-160` (mutation), `AiSummaryPanel.tsx`/`BulkActionBar.tsx` (toast), `dialog.tsx` (confirm).
- **VALIDATE**: `pnpm build`; manual suspend/erase flows.

### PHASE C — CRM

#### Task C1: Migration 000017 (notes + tags)
- **ACTION**: Create `backend/migrations/000017_member_crm.{up,down}.sql`.
- **IMPLEMENT**:
  ```sql
  CREATE TABLE IF NOT EXISTS member_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
    author_email VARCHAR(255) NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
  CREATE INDEX IF NOT EXISTS idx_member_notes_account ON member_notes(account_id, created_at DESC);
  CREATE TABLE IF NOT EXISTS member_tags (
    account_id UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
    tag VARCHAR(50) NOT NULL,
    PRIMARY KEY (account_id, tag));
  ```
- **MIRROR**: migration template + ON DELETE CASCADE (notes/tags follow the account).
- **VALIDATE**: up/down/up.

#### Task C2: Notes + tags (repo + handler)
- **ACTION**: `members/notes.go`, `tags.go` + handler endpoints.
- **IMPLEMENT**: notes — `AddNote(id, author, body)`, `ListNotes(id)`; tags — `AddTag(id, tag)`, `RemoveTag(id, tag)`, `ListTags(id)`, and tag filter in `List` (`EXISTS (SELECT 1 FROM member_tags t WHERE t.account_id=candidate_accounts.id AND t.tag=$n)`). Endpoints: `GET/POST /:id/notes`, `POST/DELETE /:id/tags`. Activity log on writes. Notes are HR-only (never exposed to the portal `/me`).
- **MIRROR**: repository + handler patterns; activity.
- **VALIDATE**: integration tests.

#### Task C3: Bulk actions
- **ACTION**: handler `POST /api/v1/admin/members/bulk` + frontend BulkActionBar (members variant).
- **IMPLEMENT**: actions = add-tag, suspend, reactivate, export-selected; iterate ids in a tx-per-id (mirror applications Bulk); destructive (suspend) allowed for admin roles; activity log per action. Frontend: selection checkboxes + `<BulkActionBar>` adapted with member actions.
- **MIRROR**: `applications/dashboard_handler.go` Bulk + `BulkActionBar.tsx`.
- **VALIDATE**: integration + manual.

#### Task C4: CSV export (sync download)
- **ACTION**: `members/export.go` + handler `GET /api/v1/admin/members/export.csv`.
- **IMPLEMENT**: re-run the List query (no pagination, respect filters) → write CSV with `encoding/csv` (header: name,email,phone,province,providers,status,applications,joined) → `c.Set("Content-Type","text/csv"); c.Set("Content-Disposition","attachment; filename=members.csv"); return c.Send(buf.Bytes())`. Gate super_admin+hr_manager. Activity log `member_export`.
- **MIRROR**: `reports/export.go:49-77` (csv encoding) — but **sync download**, not blob+notify (no existing direct-download helper; build it).
- **GOTCHA**: cap export rows (e.g. 50k) and `log` if capped (no silent truncation). Frontend: an `<a href>`/Button that hits the URL (browser downloads).
- **VALIDATE**: curl `-o members.csv` returns CSV with header + rows.

#### Task C5: Frontend CRM UI + stats segments
- **ACTION**: `frontend/components/members/{NotesPanel,TagEditor}.tsx` + detail/list updates.
- **IMPLEMENT**: Notes timeline + add box; Tag chips (add/remove); Export button on list; stats strip segments (by provider). Hooks: `useMemberNotes/useAddNote`, `useAddTag/useRemoveTag`.
- **MIRROR**: `InterviewPanel.tsx`/`FitAnalysisPanel.tsx` (panel + mutation+toast), `SummaryStrip`/`KpiCards`.
- **VALIDATE**: `pnpm build`; manual.

---

## Testing Strategy

### Unit / Integration Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| List filter+search | members w/ varied name/email/provider | correct subset + total | Yes |
| List paginate | 25 members, limit 10 page 2 | 10 rows, total 25 | Yes |
| applications_count join | member w/ 3 apps via candidates.account_id | count=3 | No |
| Stats | seeded mix | total/active/suspended/with-apps/new-week correct | Yes |
| Role gate | hr_staff token | 403 on /members | Yes |
| Suspend blocks login | suspend → resolve session | 401 / ErrNotFound | Yes (critical) |
| Anonymize | account w/ resume | redacted + blob deleted + status=anonymized | Yes |
| Anonymize idempotent | run twice | 2nd no-op | Yes |
| Erase role | hr_manager → anonymize | 403 | Yes |
| Tag filter | members tagged 'retail' | only tagged returned | Yes |
| CSV export | filtered set | CSV header+rows, attachment | No |

### Edge Cases Checklist
- [ ] Member with 0 applications / 0 sessions
- [ ] Member with only one provider vs all three
- [ ] Suspended member cannot log in (existing cookie + fresh login)
- [ ] Anonymized member excluded from re-anonymize + can't be reactivated
- [ ] Search with empty / special chars (ILIKE escaping)
- [ ] hr_manager allowed for read/suspend but blocked from erase
- [ ] Export with no results → header-only CSV

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l . && go vet ./...
cd ../frontend && pnpm tsc --noEmit && pnpm eslint .
```
EXPECT: clean

### Unit + Integration
```bash
cd backend && go test -race ./internal/members/...
cd backend && go test -tags=integration ./internal/members/...
```
EXPECT: pass

### Build / Full suite
```bash
cd backend && go build ./... && go test -race ./...
cd ../frontend && pnpm build
```
EXPECT: green (no regressions — note: adding repo interface methods may require updating test fakes elsewhere; grep for fakes if the build breaks)

### Database (dev v15 → 16 → 17)
```bash
~/go/bin/migrate -path backend/migrations -database "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable" up
# reversibility per migration: down 1 then up
```
EXPECT: version 16 (Phase A) / 17 (Phase C), clean down/up

### Browser/API (local, AUTH_PROVIDER=mock → super_admin)
```bash
# list
curl -s "localhost:8097/api/v1/admin/members?limit=5" | jq '.meta, .data[0]'
# stats
curl -s "localhost:8097/api/v1/admin/members/stats" | jq
# suspend (Phase B)
curl -s -X PATCH "localhost:8097/api/v1/admin/members/<ID>/status" -H 'Content-Type: application/json' -d '{"status":"suspended"}' -w '\n%{http_code}\n'
# export (Phase C)
curl -s "localhost:8097/api/v1/admin/members/export.csv" -o members.csv && head -3 members.csv
```
EXPECT: paginated list, stats object, 200 on suspend, CSV file

### Manual
- [ ] `/members` shows directory for super_admin AND hr_manager; restricted panel for hr_staff
- [ ] search/filter/paginate works; stats strip correct
- [ ] detail shows providers/apps/sessions/consent
- [ ] suspend → that member can't log in on the portal; reactivate restores
- [ ] erase (super_admin) redacts + removes resume; button hidden for hr_manager
- [ ] notes/tags/bulk/export work

---

## Acceptance Criteria
- [ ] All tasks for the targeted phase completed
- [ ] `go build/vet/test -race` + `tsc/eslint/build` green
- [ ] Migrations apply + reverse cleanly
- [ ] Role gates enforced (super_admin+hr_manager read/manage; super_admin-only erase)
- [ ] **Suspension actually blocks portal login** (not cosmetic)
- [ ] Anonymize is irreversible, idempotent, deletes resume blob, audited
- [ ] All member-management actions write `activity_logs`

## Completion Checklist
- [ ] Mirrors dashboard (list/RBAC/envelope), candidateauth (repo/scan), pdpa (anonymize tx), reports (csv)
- [ ] PII minimized (no raw line/google sub in API; notes HR-only)
- [ ] Error wrapping `pkg: op: %w`; sentinels → HTTP
- [ ] Thai UI strings; CP-Axtra tokens; no console.log
- [ ] Tests follow table-driven + `//go:build integration`
- [ ] Each phase is an independent, shippable PR

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Suspend left cosmetic (login not blocked) | Med | High | Task B2 cross-cutting candidateauth change + explicit test |
| Adding repo interface methods breaks other test fakes | Med | Low | grep for fakes implementing the interface; members is a NEW pkg so low blast radius |
| Anonymize hard-delete attempt fails on FK | Low | Med | Anonymize-in-place (no DELETE); documented |
| Correlated subqueries slow at scale | Low | Low | created_at/status indexes (A1); members count is small (hundreds–thousands) |
| Export memory blow-up | Low | Med | row cap + log; sync send |
| RBAC: hr_manager doing destructive ops | Low | High | erase gated super_admin-only (separate allowlist) + confirm dialog |
| CI billing-blocked | High | Low | validate locally; admin-merge; operator `az` deploy (schema 16/17 → prod, roll api+dashboard) |

## Notes
- **Why a new `internal/members` pkg (not extend candidateauth):** candidateauth is the public self-service identity layer; members is the HR-facing admin layer with different auth (role-gated, not session-cookie) and different concerns (audit, lifecycle, CRM). Keeping them separate avoids leaking admin queries into the public surface. members READS candidate_accounts directly + writes status/notes/tags.
- **Access model:** role-gated (super_admin + hr_manager), NOT store-scoped — members aren't tied to a store. Destructive PDPA erasure is super_admin-only (second allowlist).
- **Deploy (per phase):** migration 16 (A) / 17 (C) → prod, roll **api + dashboard** (operator `az`, CI billing-blocked). worker/scheduler unaffected. Same recipe as [[fit]]/[[entra]] deploys.
- **Recommended order:** ship Phase A first (immediate value: HR can finally see members), then B (governance), then C (CRM). Each is its own `/prp-implement` run + PR.
- After each phase: update memory + session file.
