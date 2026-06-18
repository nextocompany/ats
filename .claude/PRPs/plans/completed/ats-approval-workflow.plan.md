# Plan: ATS Slice 2 — Multi-Level Hiring Approval Workflow (Module-3 item 3.5)

## Summary
After a candidate is interviewed, HR submits a **hiring approval request** that must be signed off by a fixed 4-level chain — Staff → HR Manager → SGM → Regional Director — before an offer can be made. Each level approves or **rejects-with-mandatory-reason**; a worker job **escalates** any step left pending past its SLA. Backend follows the existing `applications` package patterns (narrow store interfaces, scope gating, per-role allowlists, transition guards, asynq scheduler+worker); frontend adds an `/approvals` queue page plus an approval panel on the application detail page, with i18n parity.

## User Story
As an **HR team (Staff, HR Manager, SGM, Regional Director)**, I want a structured, auditable sign-off chain on every hire decision with reminders when a step stalls, so that no candidate reaches an offer without the required approvals and bottlenecks surface automatically.

## Problem → Solution
**Current:** A candidate at `interviewed` is moved straight to `offer` by a single "Hire" button (one PATCH, no sign-off, no audit trail). → **Desired:** `interviewed` → **submit for approval** → `pending_approval` → 4 sequential role-gated approvals → `offer`; any rejection (with reason) → `rejected`; overdue steps trigger an escalation email. Full per-step audit trail (who/when/comment).

## Metadata
- **Complexity**: Large (≈22 files, new migration with 2 tables, new worker/scheduler entry, new dashboard page)
- **Source PRD**: N/A (free-form; Module-3 slice 2, follows the 3.4+3.1 scorecard/shortlist slice)
- **PRD Phase**: N/A
- **Estimated Files**: ~22 (backend ~13, frontend ~9)

---

## Design Decisions (LOCKED — answered by user this session)

1. **Trigger & levels**: Chain runs **after shortlist/interview, before offer** — initiated from application status `interviewed`. **4 levels** in fixed order: L1 `hr_staff`, L2 `hr_manager`, L3 `sgm`, L4 `regional_director`. (`super_admin` may act on any level — mirrors the scorecard allowlists where `super_admin` is in every set.)
2. **L1 = the submitter.** Creating the request IS the Staff-level sign-off: the creator (an `hr_staff` or `super_admin`) is recorded as L1's approver at creation, L1 is stored `approved`, and L2 becomes the first *active* (pending) step. The chain therefore shows all 4 levels with L1 already done.
3. **SLA escalation = worker job.** A scheduler cron entry (gated by `APPROVAL_SLA_ENABLED`, default false) enqueues a sweep task; the worker finds active steps past `due_at` not yet escalated, emails the responsible approvers (resolved via `HRDirectory`), and marks them `escalated=true` (so it never re-spams). `due_at` is set only on the *currently active* step, computed as `now + APPROVAL_SLA_HOURS` (default 48h) when that step activates.
4. **Role mapping uses existing roles, mapped 1:1.** No new role, no new migration for roles. Gate is per-level (`map[int]map[string]bool`), mirroring the TA/LM `map[string]bool` write-gate pattern.
5. **Rejection is terminal.** A reject at any level sets the request `rejected` and the application `rejected` (with the reason persisted via the same `applications.rejection_reason` column), inside one DB transaction with the step write. No "send back a level" in v1.
6. **Approval bypasses the generic status PATCH.** Submitting and deciding go through dedicated endpoints that mutate `applications.status` *inside the approval transaction* — not the `PATCH /applications/:id/status` route. The generic state machine therefore **removes** the direct `interviewed → offer` edge and leaves `pending_approval` with no manual generic transitions (so no one can bypass the chain). `interviewed → rejected` and `offer → rejected` stay.
7. **Atomicity**: create and decide are each a single `pgx` transaction (`pool.Begin` → `tx.Exec` ×N → `tx.Commit`), so the approval-table writes and the `applications.status` write never diverge (this is the lesson from the prior slice's SQL/Go composite divergence — keep the single source of truth transactional).

### Accepted limitations (note in PR, do not build)
- Reject is terminal (no re-route / no "request changes").
- No withdraw/cancel of an in-flight request in v1.
- SLA escalation emails the approver role; it does **not** auto-advance or auto-approve.
- Approver identity is by **role**, not a named per-step assignee (matches answer 3 on Role Mapping: "ใช้ role ที่มีอยู่ map ตรงๆ").

---

## UX Design

### Before
```
Application detail (status: interviewed)
┌───────────────────────────────────────┐
│ AI summary / Next step:                │
│   [ Hire ]  [ Reject… ]                │   ← one click → offer, no sign-off
└───────────────────────────────────────┘
```

### After
```
Application detail (status: interviewed)            New: /approvals (queue page)
┌───────────────────────────────────────┐          ┌──────────────────────────────────┐
│ AI summary / Next step:                │          │ Approvals awaiting YOUR decision   │
│   [ Reject… ]                          │          │ ┌────────────────────────────────┐ │
│ Approval                               │          │ │ Somchai · Store 0123 · L2 HRMgr │ │
│   [ ส่งขออนุมัติจ้าง ]  ← submit         │          │ │ AI 78 · waiting 1d · [View]      │ │
└───────────────────────────────────────┘          │ └────────────────────────────────┘ │
                                                     └──────────────────────────────────┘
Application detail (status: pending_approval)
┌───────────────────────────────────────┐
│ Approval chain                         │
│  ① Staff        ✓ approved (you, 6/18) │
│  ② HR Manager   ● awaiting · due 6/20  │  ← if my role = L2: [Approve] [Reject…]
│  ③ SGM            pending              │
│  ④ Regional       pending             │
└───────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Detail page, status `interviewed` | "Hire" button → PATCH `offer` | "Reject" only in AiSummaryPanel; **ApprovalPanel** shows "Submit for approval" | Hire path removed |
| Detail page, status `pending_approval` | n/a | ApprovalPanel shows 4-step chain; Approve/Reject buttons appear only for the actor's active level | Reject = mandatory-reason dialog |
| New nav item "Approvals" | n/a | Visible to `hr_staff/hr_manager/sgm/regional_director/super_admin` | Queue of requests where active level = my role |
| Final approval (L4) | n/a | Application auto-advances to `offer` | Same terminal state as old Hire |

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/feedback_handler.go` | 20-177 | The canonical write-handler + per-role `map[string]bool` allowlist + `canRecordPerspective` gate + narrow `feedbackStore` interface + `RegisterFeedbackRoutes`. Mirror exactly. |
| P0 | `backend/internal/applications/transitions.go` | 1-49 | State machine map + `CanTransition`/`RequiresReason`. Add `StatusPendingApproval` + `CanRequestApproval`. |
| P0 | `backend/internal/applications/model.go` | 18-73 | Status constants + `Application` struct (nullable = pointer types). Add `StatusPendingApproval`. |
| P0 | `backend/internal/applications/repository.go` | 16-53, 186-246, 251-310, 421-435 | `Repository` interface; `CreateFeedback` (INSERT…RETURNING + `applications: %w` wrap); `ListFeedback` (Query loop); `Shortlist` (scope-clause splice); `ExistsInScope`. |
| P0 | `backend/migrations/000020_interview_feedback.up.sql` + `000021_*.{up,down}.sql` | all | Migration conventions: header comment, `IF NOT EXISTS`, UUID PK `gen_random_uuid()`, `TIMESTAMPTZ DEFAULT NOW()`, `ON DELETE CASCADE`, paired indexes; down reverses in inverse order. Template for `000022`. |
| P0 | `backend/internal/applications/feedback_test.go` | 67-153 | `fakeFeedbackStore` + `feedbackTestApp` harness + role-gate tests (403/201). Template for `approval_test.go`. |
| P1 | `backend/internal/notify/hr_message.go` | 1-58 | `ShortlistReadyLM` + `hrMessages` fan-out. Add approval builders here. |
| P1 | `backend/internal/applications/dashboard_handler.go` | 83-116 | `SetLineManagerNotifier` setter + `notifyShortlistLM` + `scopeFrom`. Mirror notifier injection + best-effort dispatch. |
| P1 | `backend/internal/applications/hr_directory.go` | 12-68 | `HRDirectory` interface, `lineManagerRoles`, `emailsForStoreRoles`. Add `EmailsForRoleStore`. |
| P1 | `backend/internal/applications/notify.go` | 56-65 | `dispatchHR` best-effort loop (never returns error). |
| P1 | `backend/pkg/queue/tasks.go` | 12-50, 129-156 | Task triplet (Type const + Payload + `NewXxxTask` with `asynq.Unique`) + `ParseXxxPayload`. Template for `TypeApprovalSLASweep`. |
| P1 | `backend/cmd/scheduler/main.go` | 30-76 | Gated cron registration (`if cfg.RetentionSweepEnabled { scheduler.Register(... ) }`). Mirror for SLA. |
| P1 | `backend/cmd/worker/main.go` | 152-158 | `mux.HandleFunc(queue.Type…, svc.Handle…)`. Add the SLA handler. |
| P1 | `backend/pkg/config/config.go` | 247-289, 310-329, 432-485 | Provider/knob env loading (`getenv`, `getenvBool`, `getenvDuration`), enum validation loop, `ReportRecipientList` comma-split, accessor helpers. Add `APPROVAL_SLA_*`. |
| P1 | `backend/internal/pdpa/worker.go` (retention sweep) | all | Closest existing "scheduled sweep service with a `Handle…` method" reference implementation. |
| P0 | `frontend/lib/queries.ts` | 231-300, 333-364 | `useSetStatus` mutation (invalidate keys), `useShortlist` query, `enabled`-gated + 404-tolerant query patterns. |
| P0 | `frontend/lib/roles.ts` | 31-74 | Allowlist-array + predicate pattern (`EXECUTIVE_ROLES`/`canViewExecutive`, `LINE_MANAGER_ROLES`/`isLineManager`). Add `APPROVAL_ROLES`/`canAccessApprovals` + per-level helper. |
| P0 | `frontend/components/shell/nav-config.tsx` | 17-63 | `NavItem`, standalone gated nav const, `navForRole` append, `ALL_NAV`. Add `APPROVALS_NAV`. |
| P0 | `frontend/app/(app)/shortlist/page.tsx` | 1-85 | Role-gated client page: `useMe()` + `if (me && !allowed)` card, `<Skeleton>`, `<PageHeader>`, `enabled` fetch gating. Template for `/approvals`. |
| P0 | `frontend/components/resume/RejectDialog.tsx` | all | Mandatory-reason dialog, inline `role="alert"` error + toast, disabled+spinner while pending. Template for the reject decision. |
| P0 | `frontend/components/resume/Scorecards.tsx` | 160-249 | Role-gated form component: `canRecord` boolean from wrapper, `mutateAsync` with inline onSuccess/onError toasts, local `Label`/`Textarea`. Template for `ApprovalPanel`. |
| P0 | `frontend/components/resume/AiSummaryPanel.tsx` | 22, 40-62 | `allowedActions(app.status)` switch + `renderAction`. Remove `hire`; the submit lives in ApprovalPanel. |
| P0 | `frontend/lib/statusMachine.ts` | 1-33 | UI mirror of the backend machine. Remove `hire` from `interviewed`; add `submit_approval`. |
| P0 | `frontend/app/(app)/applications/[id]/page.tsx` | 39-60 | Detail page `<aside>` where panels render. Add `<ApprovalPanel>`. |
| P1 | `frontend/lib/types.ts` | 27-147 | `Application`, `ShortlistItem`, scorecard union/`XInput`/`| null` conventions. Add approval types. |
| P1 | `frontend/messages/en.json` + `messages/th.json` | shortlist/nav blocks | Catalog shape (`nav.*`, `shortlist.{eyebrow,title,meta,empty…}`). Add `nav.approvals` + `approvals.*` to BOTH. |
| P1 | `scripts/check-i18n-parity.mjs` | 10-43 | Parity gate — both locales must have identical key sets across both apps. Run after editing catalogs. |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| asynq delayed/periodic tasks | github.com/hibiken/asynq (already a dep) | `asynq.Unique(ttl)` on the task dedups across scheduler restarts (used by retention/auth sweeps); `scheduler.Register(cron, task)` enqueues a fixed copy each tick. No new dependency. |

No other external research needed — feature uses established internal patterns.

---

## Patterns to Mirror

### NAMING_CONVENTION (status constants + per-role allowlist)
```go
// SOURCE: backend/internal/applications/model.go:18-33
const (
	StatusInterviewed = "interviewed"
	StatusOffer       = "offer"
)
// SOURCE: backend/internal/applications/feedback_handler.go:20-31
var (
	taRecordRoles = map[string]bool{"super_admin": true, "hr_manager": true, "hr_staff": true}
	lmRecordRoles = map[string]bool{"super_admin": true, "sgm": true}
)
func canRecordPerspective(role, perspective string) bool {
	if perspective == PerspectiveLineManager { return lmRecordRoles[role] }
	return taRecordRoles[role]
}
```

### ERROR_HANDLING (handler: 4xx via fiber.NewError, 5xx via raw err; envelope is central)
```go
// SOURCE: backend/internal/applications/feedback_handler.go:107-145
id, err := uuid.Parse(c.Params("id"))
if err != nil { return fiber.NewError(fiber.StatusBadRequest, "invalid application id") }
if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
	return serr
} else if !ok { return fiber.NewError(fiber.StatusNotFound, "application not found") }
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
// ...status guard, body parse, role gate...
return httpx.Created(c, saved)   // or httpx.OK(c, data)
```

### REPOSITORY_PATTERN (INSERT…RETURNING + applications: %w wrap; tx for multi-write)
```go
// SOURCE: backend/internal/applications/repository.go:186-208
const q = `INSERT INTO interview_feedback (...) VALUES ($1,...,NULLIF($10,'')) RETURNING id, created_at`
err = r.pool.QueryRow(ctx, q, ...).Scan(&f.ID, &f.CreatedAt)
if err != nil { return InterviewFeedback{}, fmt.Errorf("applications: create feedback: %w", err) }
// For the multi-write approval ops use a transaction:
//   tx, err := r.pool.Begin(ctx); defer tx.Rollback(ctx)
//   tx.Exec / tx.QueryRow ... ; return tx.Commit(ctx)
```

### REPOSITORY_PATTERN (scope-clause splice for RBAC lists)
```go
// SOURCE: backend/internal/applications/repository.go:251-310
var args []any
add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
where := "a.status = 'pending_approval'"
if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
	where += " AND " + clause
	args = append(args, cargs...)
}
```

### SERVICE_PATTERN (scheduled sweep service with a Handle method + best-effort notify)
```go
// SOURCE: backend/cmd/worker/main.go:152-158 (registration)
mux.HandleFunc(queue.TypeRetentionSweep, retentionSvc.HandleRetentionSweep)
// SOURCE: backend/internal/applications/notify.go:56-65 (best-effort dispatch — never breaks the trigger)
func dispatchHR(ctx context.Context, n notify.Notifier, msgs []notify.Message) {
	for _, m := range msgs {
		if m.Channel == notify.ChannelEmail && m.Recipient == "" { continue }
		if err := n.Send(ctx, m); err != nil { log.Warn().Err(err)... }
	}
}
```

### TEST_STRUCTURE (fake store + fiber harness with injected DevUser + role-gate assertions)
```go
// SOURCE: backend/internal/applications/feedback_test.go:97-153
func feedbackTestApp(store feedbackStore, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error { c.Locals(middleware.UserContextKey, user); return c.Next() })
	RegisterFeedbackRoutes(app, NewFeedbackHandler(store))
	return app
}
// TestCreateFeedback_RoleGate: hr_staff + LM perspective → 403
// TestCreateFeedback_LMPerspectiveBySgm: sgm → 201
```

### FRONTEND: role gate + query + mutation
```tsx
// SOURCE: frontend/lib/roles.ts:57-63
export const EXECUTIVE_ROLES = ["super_admin", "regional_director", "auditor"];
export function canViewExecutive(role?: string): boolean { return !!role && EXECUTIVE_ROLES.includes(role); }
// SOURCE: frontend/lib/queries.ts:231-241 (mutation invalidates affected keys)
export function useSetStatus(id: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (vars: { status: string; reason?: string }) => api.patch(`/api/v1/applications/${id}/status`, vars),
		onSuccess: () => { qc.invalidateQueries({ queryKey: ["application", id] }); qc.invalidateQueries({ queryKey: ["applications"] }); },
	});
}
```

### FRONTEND: mandatory-reason dialog (reject)
```tsx
// SOURCE: frontend/components/resume/RejectDialog.tsx:41-94
const trimmed = reason.trim(); if (!trimmed) return;
await setStatus.mutateAsync({ status: "rejected", reason: trimmed }, {
	onSuccess: () => { toast.success("Candidate rejected"); close(); },
	onError: (err) => toast.error(err instanceof Error ? err.message : "Could not reject"),
});
// inline error: {setStatus.isError && <p role="alert" className="text-xs font-medium text-destructive">…</p>}
// button: <Button type="submit" variant="destructive" disabled={!reason.trim() || setStatus.isPending}>{setStatus.isPending && <Loader2 className="size-4 animate-spin" />}Reject</Button>
```

---

## Data Model (migration 000022)

Two tables. `approval_requests` = one row per hiring decision; `approval_steps` = exactly 4 rows per request.

```sql
-- 000022_approval_workflow.up.sql
-- Multi-level hiring approval chain (Module-3 3.5): Staff→HR Manager→SGM→Regional.
-- One request per application hire decision; four ordered step rows per request.
CREATE TABLE IF NOT EXISTS approval_requests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | rejected
    current_level  INT  NOT NULL DEFAULT 2,          -- next pending step (L1 done at creation)
    created_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at     TIMESTAMPTZ,
    decision_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_approval_requests_application ON approval_requests (application_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests (status);

CREATE TABLE IF NOT EXISTS approval_steps (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id   UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    level        INT  NOT NULL,                      -- 1..4
    role         TEXT NOT NULL,                      -- hr_staff|hr_manager|sgm|regional_director
    status       TEXT NOT NULL DEFAULT 'pending',    -- pending | approved | rejected
    approver_id  UUID REFERENCES users(id),
    comment      TEXT,
    due_at       TIMESTAMPTZ,                         -- set only while this step is the active pending one
    escalated    BOOLEAN NOT NULL DEFAULT FALSE,
    decided_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (request_id, level)
);
CREATE INDEX IF NOT EXISTS idx_approval_steps_request ON approval_steps (request_id);
-- SLA sweep query: active pending steps past due, not yet escalated.
CREATE INDEX IF NOT EXISTS idx_approval_steps_sla ON approval_steps (status, due_at) WHERE status = 'pending';
```
```sql
-- 000022_approval_workflow.down.sql  (inverse order)
DROP TABLE IF EXISTS approval_steps;
DROP TABLE IF EXISTS approval_requests;
```

---

## Files to Change

### Backend
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000022_approval_workflow.up.sql` | CREATE | 2 tables above |
| `backend/migrations/000022_approval_workflow.down.sql` | CREATE | inverse drop |
| `backend/internal/applications/approval.go` | CREATE | Domain types (`ApprovalRequest`, `ApprovalStep`, `ApprovalQueueItem`, `ApprovalDecisionInput`), constants (statuses, level→role map), `approvalLevelRoles` allowlist + `canDecideLevel`, validators, pure helpers (`buildSteps`, `nextLevel`). |
| `backend/internal/applications/approval_handler.go` | CREATE | `ApprovalHandler`, narrow `approvalStore` interface, `NewApprovalHandler`, optional `SetNotifier`, `RegisterApprovalRoutes`, the 4 handlers (Create / Decide / GetForApplication / ListQueue). |
| `backend/internal/applications/approval_sla.go` | CREATE | `ApprovalSLAService` with `HandleApprovalSLASweep(ctx, *asynq.Task) error` — load overdue steps, resolve approver emails, dispatch escalation, mark escalated. |
| `backend/internal/applications/transitions.go` | UPDATE | Add `StatusPendingApproval` handling: remove `StatusInterviewed→StatusOffer`; `pending_approval` has no generic transitions; add `CanRequestApproval(status)`. |
| `backend/internal/applications/model.go` | UPDATE | Add `StatusPendingApproval = "pending_approval"`. |
| `backend/internal/applications/repository.go` | UPDATE | Add 7 methods to `Repository` interface + `pgRepository` impls (with tx for create/decide). |
| `backend/internal/applications/hr_directory.go` | UPDATE | Add `EmailsForRoleStore(ctx, role string, storeID *int)` to interface + impl (store-scoped roles filter by store; all-scope roles ignore store). |
| `backend/internal/notify/hr_message.go` | UPDATE | Add `ApprovalPendingHR`, `ApprovalDecidedHR`, `ApprovalEscalationHR` builders (reuse `hrMessages`). |
| `backend/pkg/queue/tasks.go` | UPDATE | Add `TypeApprovalSLASweep` + `ApprovalSLASweepPayload` + `NewApprovalSLASweepTask` (with `asynq.Unique`) + `ParseApprovalSLASweepPayload`. |
| `backend/pkg/config/config.go` | UPDATE | Add `ApprovalSLAEnabled bool`, `ApprovalSLACron string`, `ApprovalSLAHours int` (struct + load + accessor; cron has no enum validation). |
| `backend/cmd/api/main.go` | UPDATE | Construct `NewApprovalHandler(appRepo)`, `SetNotifier(...)`, `RegisterApprovalRoutes(app, h)` after `app.Use(authMW)`. |
| `backend/cmd/worker/main.go` | UPDATE | Build `ApprovalSLAService`, `mux.HandleFunc(queue.TypeApprovalSLASweep, svc.HandleApprovalSLASweep)`. |
| `backend/cmd/scheduler/main.go` | UPDATE | Gated `if cfg.ApprovalSLAEnabled { scheduler.Register(cfg.ApprovalSLACron, NewApprovalSLASweepTask(...)) }`. |

### Backend tests
| File | Action | Justification |
|---|---|---|
| `backend/internal/applications/approval_test.go` | CREATE | `fakeApprovalStore` + fiber harness; role-gate (wrong-level → 403, right-level → 200), wrong-status submit → 400, reject-without-reason → 400, full happy chain advances levels and flips terminal status. |
| `backend/internal/applications/approval_sla_test.go` | CREATE | Sweep selects only overdue+non-escalated, dispatches, marks escalated (fake store + fake notifier). |
| `backend/internal/applications/transitions_test.go` | UPDATE (or CREATE if absent) | Assert `interviewed→offer` is now false, `CanRequestApproval("interviewed")` true, `pending_approval` has no generic transitions. |

### Frontend
| File | Action | Justification |
|---|---|---|
| `frontend/lib/types.ts` | UPDATE | `ApprovalStepStatus`, `ApprovalDecision` unions; `ApprovalStep`, `ApprovalRequest`, `ApprovalQueueItem`, `ApprovalDecisionInput` interfaces (mirror Go JSON). |
| `frontend/lib/roles.ts` | UPDATE | `APPROVAL_ROLES` + `canAccessApprovals(role?)`; `APPROVAL_LEVEL_ROLES` map + `roleLevel(role)` helper (which level a role decides). |
| `frontend/lib/queries.ts` | UPDATE | `useApprovalForApplication(appId)` (404-tolerant → null), `useApprovalQueue(enabled)`, `useSubmitApproval(appId)`, `useDecideApproval(requestId)` (invalidate `["approval", appId]`, `["approval-queue"]`, `["application", appId]`, `["applications"]`). |
| `frontend/lib/statusMachine.ts` | UPDATE | Remove `hire` from `interviewed` (add `submit_approval`); `pending_approval` → `[]` (panel-driven). |
| `frontend/components/shell/nav-config.tsx` | UPDATE | `APPROVALS_NAV` const; push in `navForRole` behind `canAccessApprovals`; add to `ALL_NAV`. |
| `frontend/app/(app)/approvals/page.tsx` | CREATE | `"use client"` queue page; `useTranslations("approvals")`; `useMe()` gate; `enabled` fetch; `<Skeleton>`/empty card; list rows → link to `/applications/:id`. |
| `frontend/components/resume/ApprovalPanel.tsx` | CREATE | Detail-page panel: at `interviewed` → "Submit for approval" button (gated to L1 roles); at `pending_approval`/decided → 4-step chain display + Approve/Reject (mandatory-reason dialog) when actor role = active level. |
| `frontend/components/resume/AiSummaryPanel.tsx` | UPDATE | Drop the `hire` case (and its renderAction). |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATE | Render `<ApprovalPanel applicationId={app.id} app={app} />` in the `<aside>` (after AiSummaryPanel). |
| `frontend/messages/en.json` + `frontend/messages/th.json` | UPDATE | `nav.approvals` + `approvals.*` block in BOTH (parity). |

## NOT Building
- Re-route/"send back a level" on reject (reject is terminal).
- Withdraw/cancel an in-flight approval request.
- Named per-step assignees / delegation (gate is by role).
- Auto-approve or auto-advance on SLA breach (escalation only emails).
- Teams notification wiring (org has no Teams license — parked; the `teamsEnabled` flag plumbs through but stays false on prod).
- Offer management / letter generation (that is slice 3.6 / 3.3).
- Career-portal changes (this is HR-dashboard only).

---

## Step-by-Step Tasks

### Task 1: Migration 000022
- **ACTION**: Create `backend/migrations/000022_approval_workflow.{up,down}.sql` with the SQL in **Data Model** above.
- **MIRROR**: `000020_interview_feedback.up.sql` (UUID PK, TIMESTAMPTZ DEFAULT NOW, ON DELETE CASCADE, `IF NOT EXISTS`, paired indexes); `000021` header-comment style.
- **GOTCHA**: There is **no Go schema-version constant** — "schema v22" is just the migration sequence number. Do not search for / bump any `SchemaVersion`. Down drops `approval_steps` before `approval_requests` (FK order).
- **VALIDATE**: `migrate -path backend/migrations -database "$DB_URL" up` locally (or visual review against 000020); `make migrate-down` then `up` round-trips clean.

### Task 2: Status constant + state machine
- **ACTION**: In `model.go` add `StatusPendingApproval = "pending_approval"` next to the funnel statuses (`model.go:24-32`).
- **IMPLEMENT** (`transitions.go`):
  - Change `StatusInterviewed: {StatusOffer: true, StatusRejected: true}` → `StatusInterviewed: {StatusRejected: true}` (hire now routes through the approval request, not a generic PATCH).
  - Add `StatusPendingApproval: {StatusRejected: false}` — i.e. do **not** add a generic entry (omit it so `CanTransition` returns false for all generic moves from `pending_approval`). Update the doc comment block to describe the new `interviewed → pending_approval → offer` path and that the decide endpoint owns `pending_approval`'s exits.
  - Add `func CanRequestApproval(status string) bool { return status == StatusInterviewed }`.
- **MIRROR**: `transitions.go:26-49`.
- **GOTCHA**: Existing rows already in `offer` are unaffected (`offer → rejected` stays). The schedule handler's `CanTransition(app.Status, StatusInterview)` is unchanged.
- **VALIDATE**: `go build ./...`; unit test in Task 14.

### Task 3: Domain types + level/role logic (`approval.go`)
- **ACTION**: Create `backend/internal/applications/approval.go`.
- **IMPLEMENT**:
  - Status consts: `ApprovalPending="pending"`, `ApprovalApproved="approved"`, `ApprovalRejected="rejected"`; step consts reuse the same strings.
  - Level→role: `var approvalChain = []struct{ Level int; Role string }{{1,"hr_staff"},{2,"hr_manager"},{3,"sgm"},{4,"regional_director"}}` and `const approvalMaxLevel = 4`.
  - Per-level gate: `var approvalLevelRoles = map[int]map[string]bool{1:{"hr_staff":true,"super_admin":true}, 2:{"hr_manager":true,"super_admin":true}, 3:{"sgm":true,"super_admin":true}, 4:{"regional_director":true,"super_admin":true}}` + `func canDecideLevel(role string, level int) bool { return approvalLevelRoles[level][role] }`.
  - Decision union: `const DecisionApprove="approve"; DecisionReject="reject"` + `func validDecision(d string) bool`.
  - Structs `ApprovalRequest{ ID, ApplicationID uuid.UUID; Status string; CurrentLevel int; CreatedBy *uuid.UUID; CreatedAt time.Time; DecidedAt *time.Time; DecisionReason string \`json:"decision_reason,omitempty"\`; Steps []ApprovalStep \`json:"steps"\` }`, `ApprovalStep{ ID uuid.UUID; Level int; Role,Status string; ApproverID *uuid.UUID \`json:"-"\`; ApproverName string \`json:"approver_name,omitempty"\`; Comment string; DueAt,DecidedAt *time.Time; Escalated bool }`, `ApprovalQueueItem{ RequestID, ApplicationID uuid.UUID; CandidateName, PositionTitle string; StoreID *int; ActiveLevel int; ActiveRole string; AIScore *float64; DueAt *time.Time; WaitingSince time.Time }`, `ApprovalDecisionInput{ Decision, Comment, Reason string }`.
  - Pure helper `func defaultApprovalSteps() []ApprovalStep` building the 4 rows (L1 marked approved is done in the repo with the creator id; here just role/level seeds).
- **MIRROR**: `feedback.go:14-34` (const+validator), `feedback_handler.go:20-31` (allowlist map), `model.go:36-73` (pointer types for nullable, `json:"-"` for server-only ids).
- **GOTCHA**: `ApproverID`/`CreatedBy` are `*uuid.UUID` with `json:"-"` — never client-supplied; the joined `ApproverName` (from `users`) is the exposed field, exactly like `InterviewFeedback.InterviewerName`.
- **VALIDATE**: `go build ./internal/applications/`.

### Task 4: Repository methods (`repository.go`)
- **ACTION**: Add to the `Repository` interface (after `ListFeedback`, ~`repository.go:30`) and implement on `pgRepository`:
  - `CreateApprovalRequest(ctx, applicationID, createdBy uuid.UUID, slaHours int) (ApprovalRequest, error)` — **tx**: INSERT request (`current_level=2`); INSERT 4 steps (L1 `approved`, `approver_id=createdBy`, `decided_at=NOW()`; L2 `pending` with `due_at=NOW()+slaHours*interval`; L3/L4 `pending`, `due_at NULL`); `UPDATE applications SET status='pending_approval' WHERE id=$app AND status='interviewed'` (guard in SQL too); commit; then re-read via `GetApprovalRequest`.
  - `GetApprovalRequest(ctx, applicationID uuid.UUID) (*ApprovalRequest, error)` — request by application + its steps ordered by level (LEFT JOIN users for approver_name). `(nil,nil)` if none (mirror `FindAppointment`).
  - `GetApprovalRequestByID(ctx, id uuid.UUID) (*ApprovalRequest, error)` — same, by request id (used by Decide to load current state + application_id).
  - `DecideApproval(ctx, in approvalDecideArgs) (ApprovalRequest, error)` where `approvalDecideArgs{ RequestID uuid.UUID; Level int; Approve bool; ApproverID uuid.UUID; Comment, Reason string; SLAHours int }` — **tx**: UPDATE the step at `(request_id, level)` set status/approver_id/comment/decided_at; then branch: **approve & level<4** → set request.current_level=level+1, set next step `due_at=NOW()+sla`; **approve & level==4** → request status='approved', decided_at=NOW(), `UPDATE applications SET status='offer'`; **reject** → request status='rejected', decision_reason=reason, decided_at=NOW(), `UPDATE applications SET status='rejected', rejection_reason=$reason`. Commit; re-read.
  - `ListPendingApprovals(ctx, scope rbac.Scope) ([]ApprovalQueueItem, error)` — JOIN requests→active step (status='pending', level=current_level)→applications→candidates→positions→stores; `WHERE r.status='pending'` + scope clause on `applications`; ORDER BY active step `due_at NULLS LAST, created_at`.
  - `ListOverdueApprovalSteps(ctx) ([]OverdueApprovalStep, error)` — `SELECT … FROM approval_steps s JOIN approval_requests r … JOIN applications a … WHERE s.status='pending' AND s.escalated=false AND s.due_at IS NOT NULL AND s.due_at < NOW() AND r.status='pending'` returning step id, role, application store id, candidate name, position title. (`OverdueApprovalStep` struct lives in `approval.go`.)
  - `MarkApprovalStepEscalated(ctx, stepID uuid.UUID) error`.
- **MIRROR**: `CreateFeedback` (INSERT…RETURNING + `applications: %w` wrap, `repository.go:186-208`), `ListFeedback` (Query loop + LEFT JOIN users, `:210-246`), `Shortlist` (scope splice, `:251-310`), `FindAppointment` (`pgx.ErrNoRows`→`(nil,nil)`, `:163-184`).
- **IMPORTS**: already in file — `context`, `errors`, `fmt`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5`, `…/internal/rbac`.
- **GOTCHA**: For the tx, `tx, err := r.pool.Begin(ctx)`; `defer tx.Rollback(ctx)` (no-op after commit); every query uses `tx`, not `r.pool`. Wrap every error `fmt.Errorf("applications: <op>: %w", err)`. Interval: pass slaHours as an int arg and use `make_interval(hours => $n)` or `NOW() + ($n || ' hours')::interval`; prefer `NOW() + make_interval(hours => $n)` (parameter-safe, no string concat).
- **VALIDATE**: `go build ./... && go vet ./...` (vet catches a missing interface method on `pgRepository`).

### Task 5: HRDirectory approver-email resolver
- **ACTION**: Add `EmailsForRoleStore(ctx, role string, storeID *int) ([]string, error)` to the `HRDirectory` interface (`hr_directory.go:12-20`) and `pgHRDirectory`.
- **IMPLEMENT**: If `role` is store-scoped (`hr_staff`/`hr_manager`/`sgm`) filter `users` by `store_id=$store` (nil store → no recipients, mirror `:44-46`); if all-scope (`regional_director`/`super_admin`) return all active users with that role regardless of store. Reuse the `emailsForStoreRoles` query shape; add a `roleIsStoreScoped(role)` helper using `rbac.New(role,nil,"").Kind()==rbac.KindStore`.
- **MIRROR**: `hr_directory.go:39-68`.
- **GOTCHA**: Update **both** the production `HRDirectory` interface AND any test stub that implements it (build will fail otherwise — same lesson as the prior slice's `LineManagerEmailsForStore`/`fakeHRDir`).
- **VALIDATE**: `go build ./...`.

### Task 6: Notify builders
- **ACTION**: In `notify/hr_message.go` add: `ApprovalPendingHR(emails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []notify.Message`, `ApprovalDecidedHR(emails []string, teamsEnabled bool, candName string, approved bool, dashURL string) []notify.Message`, `ApprovalEscalationHR(emails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []notify.Message`.
- **MIRROR**: `ShortlistReadyLM` + `hrMessages` (`hr_message.go:35-58`) — build Subject/Body from primitives, fan out via `hrMessages`.
- **GOTCHA**: Builders take primitives, not domain structs (keeps `notify` decoupled from `applications`). Thai-friendly copy is fine (existing HR messages are short English; match whatever `ShortlistReadyLM` uses).
- **VALIDATE**: `go build ./internal/notify/`.

### Task 7: Approval handler + routes (`approval_handler.go`)
- **ACTION**: Create `backend/internal/applications/approval_handler.go`.
- **IMPLEMENT**:
  - `approvalStore interface` (narrow): `ExistsInScope`, `FindByID`, the 7 approval repo methods, plus what notify needs.
  - `type ApprovalHandler struct { apps approvalStore; slaHours int; notify approvalNotify }`; `NewApprovalHandler(apps, slaHours)`; `SetNotifier(n, hr, dashURL, teamsEnabled)` (mirror `dashboard_handler.go:83-85`).
  - `RegisterApprovalRoutes(app, h)`:
    - `app.Post("/api/v1/applications/:id/approval-request", h.Create)`
    - `app.Get("/api/v1/applications/:id/approval-request", h.GetForApplication)`
    - `app.Post("/api/v1/approval-requests/:id/decide", h.Decide)`
    - `app.Get("/api/v1/approvals", h.ListQueue)`
  - **Create**: parse `:id`; `ExistsInScope` (404); read `u`; `FindByID`; `if !CanRequestApproval(app.Status)` → 400 "can only request approval from the interviewed stage"; `if !canDecideLevel(u.Role, 1)` → 403; parse `u.ID`→uuid; `CreateApprovalRequest(ctx, id, uid, h.slaHours)`; fire `notifyApprovalPending` to L2 approvers (best-effort); `httpx.Created(c, req)`.
  - **Decide**: parse request `:id`; `GetApprovalRequestByID` (404 if nil); `ExistsInScope(app.ApplicationID)` (scope on the application); `if req.Status != ApprovalPending` → 409 "approval already decided"; read `u`; `level := req.CurrentLevel`; `if !canDecideLevel(u.Role, level)` → 403 "not your approval level"; parse body `ApprovalDecisionInput`; `if !validDecision(req.Decision)` → 400; if reject and `strings.TrimSpace(reason)==""` → 400 "a rejection reason is required"; build `approvalDecideArgs`; `DecideApproval(...)`; fire notify (pending-next on advance, decided on terminal); `httpx.OK(c, updated)`.
  - **GetForApplication**: parse `:id`; `ExistsInScope` (404); `GetApprovalRequest`; if nil → `httpx.OK(c, nil)` (frontend treats null as "not submitted"); else `httpx.OK(c, req)`.
  - **ListQueue**: read `u`; `ListPendingApprovals(ctx, scopeFrom(c))`; **filter to the actor's level** in Go: keep items where `canDecideLevel(u.Role, item.ActiveLevel)` (so HR Manager sees only L2-active items, etc.; super_admin sees all); `httpx.OK(c, items)`.
- **MIRROR**: `feedback_handler.go:43-177` (handler shape, ordering, narrow store iface, `SetNotifier`), `shortlist_handler.go:30-52` (read handler + static route).
- **IMPORTS**: `strings`, `github.com/gofiber/fiber/v2`, `github.com/google/uuid`, `…/internal/middleware`, `…/internal/rbac`, `…/pkg/httpx`, `…/internal/notify`.
- **GOTCHA — route ordering**: `/api/v1/approvals` and `/api/v1/approval-requests/:id/decide` are new prefixes (no clash with `/applications/:id`). The two `/applications/:id/approval-request` routes sit under the existing `:id` param and are safe. Register approval routes in `main.go` like the others — no static-vs-param trap here, but keep `/approvals` (static collection) registered before any future `/approvals/:id`.
- **VALIDATE**: `go build ./... && go vet ./...`.

### Task 8: SLA sweep task + service
- **ACTION**: `pkg/queue/tasks.go` — add `TypeApprovalSLASweep = "approval:sla_sweep"`, `ApprovalSLASweepPayload struct{}`, `NewApprovalSLASweepTask` (with `asynq.MaxRetry`, `asynq.Timeout`, **`asynq.Unique(time.Hour)`** to dedup across scheduler restarts), `ParseApprovalSLASweepPayload`. Create `backend/internal/applications/approval_sla.go` with `ApprovalSLAService{ store slaStore; notifier notify.Notifier; hr HRDirectory; dashURL string; teamsEnabled bool }`, `NewApprovalSLAService(...)`, and `HandleApprovalSLASweep(ctx, t *asynq.Task) error`: `ListOverdueApprovalSteps` → for each, `EmailsForRoleStore(role, storeID)` → `ApprovalEscalationHR(...)` → `dispatchHR` → `MarkApprovalStepEscalated`. Log a count summary.
- **MIRROR**: `tasks.go:129-156` (retention task triplet), `internal/pdpa/worker.go` (sweep service), `notify.go:56-65` (dispatch).
- **GOTCHA**: Sweep must be resilient — one failed step's email must not abort the loop (continue + log, like `dispatchHR`). Returning nil even on partial failure is acceptable for a best-effort reminder; only return an error if the initial `ListOverdueApprovalSteps` query fails (so asynq retries the whole sweep).
- **VALIDATE**: `go build ./...`.

### Task 9: Config knobs
- **ACTION**: `pkg/config/config.go` — add struct fields `ApprovalSLAEnabled bool`, `ApprovalSLACron string`, `ApprovalSLAHours int`; load `ApprovalSLAEnabled: getenvBool("APPROVAL_SLA_ENABLED", false)`, `ApprovalSLACron: getenv("APPROVAL_SLA_CRON", "0 * * * *")` (hourly), `ApprovalSLAHours: getenvInt("APPROVAL_SLA_HOURS", 48)`.
- **MIRROR**: `RetentionSweepEnabled`/`RetentionSweepCron` (`config.go:282-289`).
- **GOTCHA**: Cron strings get **no** enum validation (only `*_PROVIDER` vars go in the `isOneOf` loop). Default `ApprovalSLAEnabled=false` so prod stays quiet until explicitly enabled (mirror retention's safe-default reasoning).
- **VALIDATE**: `go build ./...`.

### Task 10: Wire api / worker / scheduler mains
- **ACTION**:
  - `cmd/api/main.go` (after `app.Use(authMW)`, near the other applications registrations ~line 303): `approvalHandler := applications.NewApprovalHandler(appRepo, cfg.ApprovalSLAHours)`; `approvalHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")`; `applications.RegisterApprovalRoutes(app, approvalHandler)`.
  - `cmd/worker/main.go` (~line 158): build `approvalSLASvc := applications.NewApprovalSLAService(appRepo, notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")`; `mux.HandleFunc(queue.TypeApprovalSLASweep, approvalSLASvc.HandleApprovalSLASweep)`.
  - `cmd/scheduler/main.go` (after the auth-cleanup block ~line 76): `if cfg.ApprovalSLAEnabled { task,_ := queue.NewApprovalSLASweepTask(queue.ApprovalSLASweepPayload{}); id,_ := scheduler.Register(cfg.ApprovalSLACron, task); log.Info()... } else { log.Info().Msg("scheduler: approval SLA sweep disabled") }`.
- **MIRROR**: `scheduler/main.go:48-76`, `worker/main.go:152-158`, the applications wiring in `api/main.go:199-303`.
- **GOTCHA**: The worker needs whatever deps `appRepo`/`notifier` it already builds — reuse the existing ones in each main; do not construct a second pool. Check each main already has `notifier` and a pool in scope (api+worker do; scheduler does NOT need them — it only enqueues).
- **VALIDATE**: `go build ./cmd/...`.

### Task 11: Frontend types
- **ACTION**: `lib/types.ts` — add unions `ApprovalDecision = "approve" | "reject"`, `ApprovalEntityStatus = "pending" | "approved" | "rejected"`; interfaces `ApprovalStep` (`{ id; level; role; status; approver_name?; comment?; due_at: string|null; escalated: boolean; decided_at: string|null }`), `ApprovalRequest` (`{ id; application_id; status; current_level; created_at; decided_at: string|null; decision_reason?; steps: ApprovalStep[] }`), `ApprovalQueueItem` (`{ request_id; application_id; candidate_name; position_title; store_id: number|null; active_level; active_role; ai_score: number|null; due_at: string|null; waiting_since: string }`), `ApprovalDecisionInput` (`{ decision: ApprovalDecision; comment?: string; reason?: string }`).
- **MIRROR**: `lib/types.ts:90-147` (string-literal unions, `XInput`, `| null`, comment naming the Go struct).
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 12: Frontend roles + nav + statusMachine
- **ACTION**:
  - `lib/roles.ts`: `export const APPROVAL_ROLES = ["super_admin","hr_staff","hr_manager","sgm","regional_director"];` + `canAccessApprovals(role?)`; `export const APPROVAL_LEVEL_ROLES: Record<number,string> = {1:"hr_staff",2:"hr_manager",3:"sgm",4:"regional_director"};` + `roleLevel(role?)` (returns the level a role decides, or 0) + `canSubmitApproval(role?)` (role is hr_staff or super_admin).
  - `nav-config.tsx`: `export const APPROVALS_NAV: NavItem = { href:"/approvals", label:"Approvals", key:"approvals", icon: <pick a lucide icon e.g. CheckSquare> };` push in `navForRole` behind `canAccessApprovals(role)`; add to `ALL_NAV`.
  - `statusMachine.ts`: add `"submit_approval"` to the `Action` union; `case "interviewed": return ["submit_approval","reject"];`; add `case "pending_approval": return [];`.
- **MIRROR**: `roles.ts:57-63`, `nav-config.tsx:42-63`, `statusMachine.ts:5-33`.
- **GOTCHA**: Import the chosen icon from `lucide-react` in `nav-config.tsx`. Keep `Action` union and the backend in sync (comment already says so).
- **VALIDATE**: `pnpm exec tsc --noEmit && pnpm exec eslint app components lib`.

### Task 13: Frontend queries
- **ACTION**: `lib/queries.ts` — add:
  - `useApprovalForApplication(appId: string)` → `api.get<ApprovalRequest | null>(\`/api/v1/applications/${appId}/approval-request\`)`; 404/null-tolerant (catch `ApiError`/return null, `retry:false`) like `useFitAnalysis` (`:333-347`); `queryKey:["approval", appId]`.
  - `useApprovalQueue(enabled: boolean)` → `api.get<ApprovalQueueItem[]>("/api/v1/approvals")`; `queryKey:["approval-queue"]`; `enabled`.
  - `useSubmitApproval(appId: string)` → `useMutation` POST `/api/v1/applications/${appId}/approval-request`; onSuccess invalidate `["approval", appId]`, `["application", appId]`, `["applications"]`.
  - `useDecideApproval(requestId: string, appId: string)` → `useMutation` POST `/api/v1/approval-requests/${requestId}/decide` body `ApprovalDecisionInput`; onSuccess invalidate `["approval", appId]`, `["approval-queue"]`, `["application", appId]`, `["applications"]`.
- **MIRROR**: `queries.ts:231-241` (mutation+invalidate), `:294-300` (query), `:333-347` (404-tolerant).
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 14: Backend tests
- **ACTION**: Create `approval_test.go` + `approval_sla_test.go`; update/create `transitions_test.go`.
- **IMPLEMENT** (table-driven where natural):
  - `fakeApprovalStore` implementing `approvalStore` (in the test file), + `approvalTestApp(store, user)` harness mirroring `feedbackTestApp`.
  - Create: `interviewed` + `hr_staff` → 201 & app→pending_approval recorded in fake; non-`interviewed` → 400; `hr_manager` submitting → 403.
  - Decide: actor role ≠ active level → 403; reject with empty reason → 400; already-decided request → 409; happy approve at L2 by `hr_manager` → 200 & current_level advances; L4 approve → request approved; reject → request rejected.
  - ListQueue: fake returns mixed-level items; assert `hr_manager` sees only L2-active; `super_admin` sees all.
  - SLA: fake store returns 2 overdue (1 already escalated) + fake notifier; assert only the non-escalated one dispatched + marked.
  - transitions: `CanTransition("interviewed","offer")==false`, `CanRequestApproval("interviewed")==true`, `CanTransition("pending_approval", X)==false` for all X, `CanTransition("offer","rejected")==true`.
- **MIRROR**: `feedback_test.go:67-153`.
- **GOTCHA**: Use `middleware.DevUser{ID: <a valid uuid string>, Role: "hr_manager", ...}` — `uuid.Parse(u.ID)` must succeed for the approver to be stamped; use `uuid.NewString()`.
- **VALIDATE**: `go test ./internal/applications/... -run Approval -v`; then `go test ./...`.

### Task 15: ApprovalPanel component
- **ACTION**: Create `frontend/components/resume/ApprovalPanel.tsx` (`"use client"`).
- **IMPLEMENT**:
  - Props `{ applicationId: string; app: Application }`. `const { data: me } = useMe();` `const { data: req } = useApprovalForApplication(applicationId);`.
  - If `req == null`: when `app.status === "interviewed"` and `canSubmitApproval(me?.role)` → render "Submit for approval" button (`variant="default"`) calling `useSubmitApproval` with toast on success/error (mirror `AiSummaryPanel.move`). Otherwise render nothing (`return null`).
  - If `req != null`: render the **4-step chain** (ordered list): each step shows level label (from i18n `approvals.level1..4`), status icon (✓ approved / ● awaiting / ✕ rejected / dim pending), approver_name + decided_at, and `due_at` on the active step. If `req.status==="pending"` and `roleLevel(me?.role) === req.current_level` (or super_admin) → show **Approve** (`variant="default"`) and **Reject…** (`variant="destructive"` → inline mandatory-reason form/dialog) wired to `useDecideApproval(req.id, applicationId)`. On reject reuse the RejectDialog pattern (mandatory reason, `role="alert"` error, disabled+`<Loader2 className="animate-spin"/>`).
  - i18n via `useTranslations("approvals")`.
- **MIRROR**: `Scorecards.tsx:160-249` (role-gated panel + mutateAsync toasts), `RejectDialog.tsx` (mandatory-reason), `AiSummaryPanel.tsx` (button styling/tokens, status badges).
- **GOTCHA**: **Use `useTranslations`, never `getTranslations`** — this is a client component rendered by a client page; `getTranslations` 500s at runtime (the PR #78 lesson). Use semantic tokens (`bg-card`, `ring-hairline`, `text-muted-foreground`, `variant="destructive"`), never raw hex.
- **VALIDATE**: `pnpm exec tsc --noEmit && pnpm exec eslint app components lib`.

### Task 16: Wire panel into detail page + drop Hire
- **ACTION**: In `app/(app)/applications/[id]/page.tsx` import `ApprovalPanel` and render `<ApprovalPanel applicationId={app.id} app={app} />` in the `<aside>` right after `<AiSummaryPanel app={app} />` (`page.tsx:52`). In `AiSummaryPanel.tsx` remove the `case "hire":` from `renderAction` (the action no longer appears since statusMachine dropped it, but remove the dead case too).
- **MIRROR**: existing panel composition (`page.tsx:52-57`).
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 17: i18n catalogs
- **ACTION**: Add to BOTH `frontend/messages/en.json` and `th.json`:
  - `nav.approvals`: EN "Approvals" / TH "อนุมัติ".
  - A top-level `approvals` block: `eyebrow`, `title`, `meta`, `emptyTitle`, `emptyHint`, `notAvailable`, `notAvailableHint`, `submit`, `approve`, `reject`, `rejectReasonLabel`, `rejectReasonRequired`, `chainTitle`, `awaiting`, `approvedBy`, `rejectedBy`, `dueOn`, `escalated`, `level1`("Staff"/"พนักงาน HR"), `level2`("HR Manager"/"ผู้จัดการ HR"), `level3`("SGM"/"ผู้จัดการสาขา"), `level4`("Regional Director"/"ผู้อำนวยการภูมิภาค"), plus toast strings `submitted`, `decided`. Mirror the `shortlist` block's key shape.
- **MIRROR**: `messages/en.json` shortlist/nav blocks.
- **GOTCHA**: Identical key set in both files (and the script also compares against `career-portal` — do NOT add `approvals` keys to career-portal; the script asserts parity *between locales within each app*, so as long as both `frontend/en.json` and `frontend/th.json` get the same keys it passes).
- **VALIDATE**: `node scripts/check-i18n-parity.mjs` (exit 0).

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Create_HappyPath | interviewed + hr_staff | 201, app→pending_approval, L1 approved/L2 active | |
| Create_WrongStatus | shortlisted + hr_staff | 400 | ✓ |
| Create_WrongRole | interviewed + hr_manager | 403 | ✓ |
| Decide_WrongLevel | L2-active, actor sgm | 403 | ✓ |
| Decide_RejectNoReason | reject, reason="" | 400 | ✓ |
| Decide_AlreadyDecided | request status=approved | 409 | ✓ |
| Decide_ApproveAdvances | L2 approve by hr_manager | 200, current_level=3 | |
| Decide_FinalApprove | L4 approve by regional_director | 200, request approved, app→offer | |
| Decide_Reject | L3 reject by sgm | 200, request rejected, app→rejected+reason | |
| Queue_LevelFilter | mixed items, actor hr_manager | only L2-active returned | ✓ |
| Queue_SuperAdmin | mixed items, super_admin | all returned | ✓ |
| SLA_Sweep | 2 overdue (1 escalated) | 1 dispatched + marked | ✓ |
| Transitions | interviewed→offer | false (routed via approval) | ✓ |

### Edge Cases Checklist
- [ ] Submit from non-interviewed status → 400
- [ ] Decide by wrong-level / wrong-role → 403
- [ ] Reject with blank reason → 400
- [ ] Decide an already-terminal request → 409
- [ ] Out-of-scope application (different store) → 404 on get/decide
- [ ] SLA step already escalated → skipped (no re-spam)
- [ ] SLA step with NULL due_at (not yet active) → never escalated
- [ ] Permission denied (queue) → only your level's items

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib
```
EXPECT: zero errors (ignore the known pre-existing dirty files: `cmd/seedresumes/main.go`, reengage/search integration-test gofmt; `components/shell/AppHeader.tsx` eslint setState-in-effect — NOT introduced here).

### Unit Tests
```bash
cd backend && go test ./internal/applications/... -run 'Approval|Transition' -v
```
EXPECT: all pass.

### Full Test Suite
```bash
cd backend && go test ./...
```
EXPECT: exit 0, no regressions (watch the two legacy feedback tests — they are already fixed for perspective gating).

### Database Validation
```bash
# local DB
migrate -path backend/migrations -database "$DB_URL" up      # applies 000022
migrate -path backend/migrations -database "$DB_URL" down 1  # round-trip
migrate -path backend/migrations -database "$DB_URL" up
```
EXPECT: schema reaches migration 000022; down cleanly drops both tables.

### Build Verification
```bash
cd frontend && pnpm exec next build        # plain, NO --webpack
node scripts/check-i18n-parity.mjs
```
EXPECT: build OK; parity exit 0.

### Manual Validation (dev)
- [ ] As `hr_staff` (dev mock = super_admin; set role to test gates), open an `interviewed` application → "Submit for approval" appears → click → status flips to `pending_approval`, chain shows L1 ✓ / L2 ● awaiting.
- [ ] As `hr_manager`, `/approvals` lists that request; open it → Approve → advances to L3.
- [ ] As `sgm` → Reject with reason → application becomes `rejected`, reason shown.
- [ ] As `regional_director`, final Approve → application becomes `offer`.
- [ ] Toggle `APPROVAL_SLA_ENABLED=true`, set `APPROVAL_SLA_HOURS=0`, run worker+scheduler locally → overdue step escalation logged once.

---

## Acceptance Criteria
- [ ] Migration 000022 applies and round-trips
- [ ] Submit / decide / queue / get endpoints enforce status + per-level role gates (403/400/409 as specified)
- [ ] Final approval → `offer`; any reject (with mandatory reason) → `rejected`; both atomic with step write
- [ ] SLA sweep escalates only overdue, non-escalated active steps, once each
- [ ] `/approvals` page + ApprovalPanel render, role-gated; reject uses mandatory-reason dialog
- [ ] All validation commands pass; i18n parity green

## Completion Checklist
- [ ] Code mirrors discovered patterns (narrow store iface, `applications: %w` wrap, allowlist maps, scope splice, best-effort dispatch)
- [ ] Error handling: 4xx `fiber.NewError`, 5xx raw err; envelope via `httpx`
- [ ] No new schema-version constant invented; no Teams wiring
- [ ] Tests follow `feedback_test.go` harness; valid uuid in `DevUser.ID`
- [ ] Both i18n files updated; `useTranslations` (not `getTranslations`) in the client panel/page
- [ ] No hardcoded hex (semantic tokens); no scope creep beyond NOT-Building list
- [ ] Self-contained — no further codebase searching required

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Multi-write atomicity bug (step vs app status diverge) | Med | High | Single `pgx` tx per create/decide; test terminal-state assertions |
| Removing `interviewed→offer` strands a half-built UI path | Low | Med | Drop `hire` from both statusMachine + AiSummaryPanel in same task; manual click-through |
| `regional_director` is all-scope → store filter wrong for escalation emails | Med | Med | `roleIsStoreScoped` via `rbac.Kind()`; unit-test `EmailsForRoleStore` both branches |
| i18n parity CI fail on missing key | Med | Low | Add keys to both files; run parity script before PR |
| SLA cron double-enqueue across deploys | Low | Low | `asynq.Unique(1h)` on the task (matches retention/auth pattern); scheduler is single-replica |
| Migration must run BEFORE new image (old code unaffected = additive) | — | — | Deploy recipe: migrate first, then roll api/worker/scheduler (session-proven order) |

## Notes
- Deploy order (from session recipe): **migrate 000022 first** (additive, old code ignores new tables), then roll `api`, `worker`, `scheduler` images. New scheduler entry is inert until `APPROVAL_SLA_ENABLED=true`.
- Prod is CI-billing-blocked → squash + `--admin` merge; operator runs `az` deploy + the migration recipe (temp PG firewall rule, `migrate up`, delete rule on exit).
- This slice deliberately keeps `interviewed → pending_approval → offer` as the only hire path; the old one-click Hire is gone. Mention this behavior change in the PR body.
- Next slices after this: 3.6 Offer Management → 3.3 Interview Letter → 3.8 Onboarding/Docs → 3.9 ATS Reports.
