# Plan: ATS Scorecard (TA/LM split) + Top-5 → Line Manager Shortlist

## Summary
First slice of the ATS lifecycle (Module 3). Splits the single interview scorecard into two **perspectives** — TA (recruiter) and Line Manager (LM = `sgm`) — each rating its own competency set, with a combined **aggregate** view. Adds a **Top-5 shortlist** surface: a ranked, store-scoped list of `shortlisted` candidates (composite of AI score + TA rating) that the Line Manager sees on a dedicated `/shortlist` page (only their 5, not the full inbox), with approve/decline actions that reuse the existing status state machine. Notifies the store's LM by email when candidates are shortlisted.

## User Story
As a **Line Manager (Store GM / `sgm`)**, I want **to see only my store's top shortlisted candidates with the TA's scorecard and AI summary, record my own LM scorecard, and approve or decline them**, so that **I review a focused shortlist instead of the whole pipeline and the hiring decision captures both the recruiter's and the manager's assessment**.
As a **TA / recruiter (`hr_staff` / `hr_manager`)**, I want **to record a TA-perspective scorecard (technical, communication, attitude)** so that **my assessment feeds the shortlist ranking and the LM's review**.

## Problem → Solution
**Current:** one shared `interview_feedback` form (no perspective), recordable by super_admin/hr_manager/sgm; the inbox is ranked by `ai_score` but returns full pages — no Top-N, no LM-scoped view, no TA-vs-LM split. → **Desired:** perspective-aware scorecards (TA vs LM), an aggregate summary, a Top-5 composite-ranked shortlist scoped to the LM's store, and LM approve/decline reusing existing transitions.

## Metadata
- **Complexity**: XL (cross-cutting: migration + backend package extensions + new endpoints + new frontend page + i18n). Implementable in two ordered phases — **Phase A: scorecard split** (Tasks 1–8, 14–17), **Phase B: shortlist + LM view** (Tasks 9–13, 18–21) — pausable between A and B.
- **Source PRD**: Module 3 spec items **3.4** (Post-interview Scorecard TA/LM) + **3.1** (Top 5 → Line Manager flow)
- **PRD Phase**: ATS slice 1 of 5
- **Estimated Files**: ~20 (migration + ~8 backend + ~9 frontend + 2 i18n)

---

## UX Design

### Before
```
applications/[id] aside:  AiSummary · Interview · InterviewFeedback(single form) · FitAnalysis
nav:  Overview · Inbox · Candidates · Search · Analytics · [Executive] · …
```

### After
```
applications/[id] aside:  AiSummary · Interview ·
                          ScorecardSummary (aggregate: TA avg + LM avg + tally) ·
                          TaScorecard (form+list, TA roles) ·
                          LineManagerScorecard (form+list, sgm) · FitAnalysis

nav (sgm only): + [Shortlist]   →  /shortlist  (Top-5 for my store)
┌──────────── Shortlist · {storeName} ────────────────────────┐
│  Top candidates awaiting your review            5 shown      │
│  1. สมชาย   composite 88  (AI 82 · TA 4.5/5)  [Review →]     │
│     TA: เก่งเทคนิค · สื่อสารดี                                │
│  2. …                                                        │
└──────────────────────────────────────────────────────────────┘
Review → applications/[id]  (LM fills LM scorecard, then Approve→interview / Decline→reject)
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Scorecard panel | one form (4 fixed competencies) | TA form (technical/communication/attitude) + LM form (culture_fit/growth/leadership) + aggregate card | perspective-aware |
| Who records | super_admin/hr_manager/sgm (one allowlist) | TA perspective: super_admin/hr_manager/hr_staff · LM perspective: super_admin/sgm | per-perspective gate |
| LM experience | full inbox (store-scoped) | dedicated `/shortlist` Top-5 page | focused review |
| Shortlist notify | none | email to store LM when a candidate becomes `shortlisted` | best-effort |
| LM decision | generic status PATCH | Approve (→interview) / Decline (→reject w/ reason) on shortlist/detail | reuses existing transitions |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/feedback.go` | 14–83 | Scorecard model/validation/competencies/status-gate to extend |
| P0 | `backend/internal/applications/feedback_handler.go` | 19–23, 54–177 | Role allowlist, routes, Create flow (interviewer stamp, notify), SetNotifier |
| P0 | `backend/internal/applications/repository.go` | 183–240 | CreateFeedback / ListFeedback SQL to extend with `perspective` |
| P0 | `backend/migrations/000020_interview_feedback.up.sql` | all | Table shape; next migration = `000021` |
| P0 | `backend/internal/applications/list.go` | 15–30, 85–129 | ListFilter + ranked query — model for the shortlist composite query |
| P0 | `backend/internal/rbac/scope.go` | 9–69 | KindStore scoping (sgm = store) for LM-scoped shortlist |
| P0 | `backend/internal/applications/transitions.go` | 26–49 | `shortlisted→interview/rejected`, RequiresSchedule/Reason — LM actions reuse these |
| P1 | `backend/internal/applications/model.go` | 18–33 | Status constants |
| P1 | `backend/internal/notify/hr_message.go` | 12–47 | HR message builder + `hrMessages` fan-out to copy for LM-notify |
| P1 | `backend/internal/applications/notify.go` | 26–65 | `dispatchHR` best-effort + SetNotifier dispatch |
| P1 | `backend/internal/applications/hr_directory.go` | 12–16 | `EmailsForStore` — add an LM (sgm) resolver alongside |
| P1 | `backend/internal/activity/activity.go` | 16–64 | Activity log Record (add scorecard + LM-decision audit) |
| P1 | `backend/internal/applications/feedback_test.go` | 67–195 | Test harness: fake store + DevUser role injection |
| P0 | `backend/cmd/api/main.go` | 285–303 | Wiring site for dashboard/feedback handlers |
| P0 | `frontend/components/resume/InterviewFeedbackPanel.tsx` | 29–299 | Panel to split into TA/LM + aggregate |
| P0 | `frontend/lib/types.ts` | 77–110 | InterviewFeedback / Competencies / Input — add perspective + new dims |
| P0 | `frontend/lib/queries.ts` | 255–275, 91–107 | Feedback hooks + useApplications filter — model for shortlist hook |
| P0 | `frontend/app/(app)/applications/[id]/page.tsx` | all | Detail aside — swap in new panels |
| P0 | `frontend/app/(app)/applications/page.tsx` | 32–64, 188–373 | Inbox list — model for `/shortlist` rows |
| P0 | `frontend/app/(app)/executive/page.tsx` | all | EXACT template for a new role-gated client page (`/shortlist`) |
| P0 | `frontend/lib/roles.ts` | 16–24, 46–58 | `INTERVIEW_FEEDBACK_ROLES` + gate pattern — add TA/LM gates |
| P0 | `frontend/components/shell/nav-config.tsx` | 16–58 | Add `SHORTLIST_NAV` gated to sgm |
| P1 | `frontend/components/inbox/ScoreBadge.tsx` | 1–90 | ScoreBadge/FitLabel/ScoreRail for shortlist rows |
| P1 | `frontend/components/people/PeopleBits.tsx` | 33–189 | InitialChip/Pill/StatusPill/statusLabel |
| P1 | `frontend/messages/{en,th}.json` | `nav.*` | i18n keys (both locales) |
| P1 | `scripts/check-i18n-parity.mjs` | all | Parity guard — run after i18n edits |

## External Documentation
No external research needed — extends established internal patterns (Fiber handler+repo, JSONB competencies, RBAC store scope, React Query, role-gated client page). No new libraries.

---

## Patterns to Mirror

### SCORECARD_MODEL (extend, don't replace)
```go
// SOURCE: backend/internal/applications/feedback.go:32-55,81-83
type InterviewCompetencies struct {
    Communication int `json:"communication"`
    Technical     int `json:"technical"`
    Experience    int `json:"experience"`
    CultureFit    int `json:"culture_fit"`
}
func CanRecordFeedback(status string) bool { return status == StatusInterview || status == StatusInterviewed }
```
→ EXTEND the struct with `Attitude`, `GrowthPotential`, `Leadership` (all `int`, 0=not rated). Add `Perspective string json:"perspective"` to `InterviewFeedback` + `InterviewFeedbackInput`.

### ROLE_ALLOWLIST (per-perspective)
```go
// SOURCE: backend/internal/applications/feedback_handler.go:19-23 + Create gate :108-111
var feedbackRecordRoles = map[string]bool{"super_admin": true, "hr_manager": true, "sgm": true}
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
if !feedbackRecordRoles[u.Role] { return fiber.NewError(fiber.StatusForbidden, "...") }
```
→ Replace single allowlist with `taRecordRoles = {super_admin, hr_manager, hr_staff}` and `lmRecordRoles = {super_admin, sgm}`; pick by request `perspective`.

### REPO_INSERT (NULLIF + JSONB)
```go
// SOURCE: backend/internal/applications/repository.go:183-202
INSERT INTO interview_feedback (application_id, appointment_id, interviewer_id, overall_rating,
  recommendation, competencies, strengths, concerns, notes)
VALUES ($1,$2,$3,$4,$5,$6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''))
RETURNING id, created_at
```
→ Add `perspective` column to the INSERT + the `ListFeedback` SELECT.

### RANKED_LIST_QUERY (model for shortlist)
```go
// SOURCE: backend/internal/applications/list.go:128-129 + scope.ApplicationsClause
ORDER BY applications.ai_score DESC NULLS LAST, applications.created_at DESC LIMIT $x OFFSET $y
// SOURCE: backend/internal/rbac/scope.go:48-52  (KindStore)
return fmt.Sprintf("assigned_store_id = $%d", argStart), []any{*s.StoreID}
```
→ Shortlist query: `WHERE status='shortlisted' AND <scope clause>` , LEFT JOIN aggregated TA ratings, `ORDER BY composite DESC LIMIT 5`.

### STATE_MACHINE (LM actions reuse)
```go
// SOURCE: backend/internal/applications/transitions.go:26-49
StatusShortlisted: {StatusInterview: true, StatusRejected: true},
func RequiresReason(to string) bool { return to == StatusRejected }
func RequiresSchedule(to string) bool { return to == StatusInterview }
```
→ LM Approve = transition `shortlisted→interview` (existing schedule flow). LM Decline = `shortlisted→rejected` (+reason). No new transitions.

### HR_NOTIFY_BUILDER (copy for LM-notify)
```go
// SOURCE: backend/internal/notify/hr_message.go:23-47
func FeedbackRecordedHR(toEmails []string, teamsEnabled bool, candName, positionTitle, interviewer, recommendation, dashURL string) []Message {
    return hrMessages(toEmails, teamsEnabled, subject, body)
}
// dispatch: backend/internal/applications/notify.go:56-65 dispatchHR (best-effort, never errors)
```
→ Add `ShortlistReadyLM(lmEmails, teamsEnabled, candName, positionTitle, dashURL)`; resolve LM emails via a new directory method.

### HANDLER_WIRING
```go
// SOURCE: backend/cmd/api/main.go:296-303
feedbackHandler := applications.NewFeedbackHandler(appRepo)
feedbackHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")
applications.RegisterFeedbackRoutes(app, feedbackHandler)
```

### FRONTEND_FEEDBACK_HOOK
```ts
// SOURCE: frontend/lib/queries.ts:255-275
export function useInterviewFeedback(id: string) { return useQuery({ queryKey:["interview-feedback",id], queryFn:()=>api.get<InterviewFeedback[]>(`/api/v1/applications/${id}/interview-feedback`).then(r=>r.data), enabled:!!id }); }
```

### ROLE_GATED_PAGE (template for /shortlist)
```tsx
// SOURCE: frontend/app/(app)/executive/page.tsx:17-35
const { data: me } = useMe();
const allowed = canViewExecutive(me?.role);
if (me && !allowed) return <NotAvailablePanel/>;
```
→ Mirror with `isLineManager(me?.role)`.

### NAV_GATE
```tsx
// SOURCE: frontend/components/shell/nav-config.tsx:43-58
if (canViewExecutive(role)) items.push(EXECUTIVE_NAV);
export const ALL_NAV: NavItem[] = [...NAV, EXECUTIVE_NAV, BULK_NAV, MEMBERS_NAV, ADMIN_NAV];
```

### TEST_HARNESS
```go
// SOURCE: backend/internal/applications/feedback_test.go:97-105
app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
app.Use(func(c *fiber.Ctx) error { c.Locals(middleware.UserContextKey, user); return c.Next() })
RegisterFeedbackRoutes(app, NewFeedbackHandler(store))
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000021_scorecard_perspective.up.sql` (+`.down.sql`) | CREATE | Add `perspective TEXT NOT NULL DEFAULT 'ta'` + index to `interview_feedback` |
| `backend/internal/applications/feedback.go` | UPDATE | Extend competencies (attitude/growth/leadership), add `Perspective`, perspective constants, validation |
| `backend/internal/applications/feedback_handler.go` | UPDATE | Per-perspective role gate; accept `perspective` in DTO; pass to repo |
| `backend/internal/applications/repository.go` | UPDATE | `perspective` in CreateFeedback INSERT + ListFeedback SELECT; add `ScorecardSummary` + `Shortlist` queries |
| `backend/internal/applications/scorecard.go` | CREATE | Aggregate types + summary computation (TA avg, LM avg, recommendation tally, composite) |
| `backend/internal/applications/shortlist_handler.go` | CREATE | `GET /api/v1/shortlist` (LM store-scoped Top-N) + `GET /api/v1/applications/:id/scorecard-summary` |
| `backend/internal/applications/hr_directory.go` | UPDATE | Add `LineManagerEmailsForStore` (users WHERE role='sgm' AND store_id=…) |
| `backend/internal/notify/hr_message.go` | UPDATE | Add `ShortlistReadyLM` builder |
| `backend/internal/applications/notify.go` (or dashboard_handler.go) | UPDATE | Fire LM-notify on transition to `shortlisted` (best-effort) |
| `backend/cmd/api/main.go` | UPDATE | Wire shortlist handler |
| `backend/internal/applications/scorecard_test.go` | CREATE | Aggregate + composite unit tests |
| `backend/internal/applications/shortlist_handler_test.go` | CREATE | Route + LM scope + role-gate tests |
| `backend/internal/applications/feedback_test.go` | UPDATE | Perspective role-gate cases |
| `frontend/lib/types.ts` | UPDATE | Perspective + new competencies + ScorecardSummary + ShortlistItem |
| `frontend/lib/queries.ts` | UPDATE | `useShortlist`, `useScorecardSummary`, perspective in feedback hooks |
| `frontend/lib/roles.ts` | UPDATE | `TA_SCORECARD_ROLES`/`canRecordTaScorecard`, `LINE_MANAGER_ROLES`/`isLineManager` |
| `frontend/components/resume/Scorecards.tsx` | CREATE | `TaScorecard`, `LineManagerScorecard`, `ScorecardSummary` (split from old panel) |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATE | Swap InterviewFeedbackPanel → new scorecard panels |
| `frontend/app/(app)/shortlist/page.tsx` | CREATE | LM Top-5 view (gated to sgm) |
| `frontend/components/shell/nav-config.tsx` | UPDATE | `SHORTLIST_NAV` gated by `isLineManager` |
| `frontend/messages/{en,th}.json` | UPDATE | `nav.shortlist` + `scorecard.*` + `shortlist.*` keys |

## NOT Building
- **Multi-level approval chain (3.5)** — separate PRP. LM here records ONE decision (approve/decline); no Staff→Manager→SGM→Regional escalation/SLA.
- **Offer management (3.6)**, **interview letter generation (3.3)**, **document/onboarding (3.8)** — later PRPs.
- **Real Teams/Graph meeting** — LM "approve→interview" reuses the existing (mock-on-prod) schedule flow; no Graph changes.
- **Per-position configurable scorecard criteria** — fixed TA/LM dimension sets this slice.
- **LINE push to LM** — LM are internal staff (no `line_user_id`); LM-notify is **email only** (+ Teams when `TEAMS_WEBHOOK_URL` set).
- **Changing the existing `shortlisted` semantics** — Top-5 = `status='shortlisted'` for the store, composite-ranked, `LIMIT 5`. No new status.
- **Backfilling legacy feedback perspective** — existing rows default to `'ta'` (documented).

---

## Step-by-Step Tasks

### Phase A — Scorecard split

### Task 1: Migration 000021 — perspective column
- **ACTION**: Create `backend/migrations/000021_scorecard_perspective.up.sql` + `.down.sql`.
- **IMPLEMENT**: up: `ALTER TABLE interview_feedback ADD COLUMN IF NOT EXISTS perspective TEXT NOT NULL DEFAULT 'ta';` + `CREATE INDEX IF NOT EXISTS idx_interview_feedback_perspective ON interview_feedback (application_id, perspective);` down: drop index + column.
- **MIRROR**: `migrations/000020_interview_feedback.up.sql` style (IF NOT EXISTS, snake_case).
- **GOTCHA**: 000020 is the latest — number this `000021`. Legacy rows become `'ta'`.
- **VALIDATE**: file parses; `migrate` recipe applies cleanly on a scratch DB (or review only — prod apply is operator-run).

### Task 2: Extend scorecard model + perspective constants + validation
- **ACTION**: Update `backend/internal/applications/feedback.go`.
- **IMPLEMENT**:
  - Add to `InterviewCompetencies`: `Attitude int json:"attitude"`, `GrowthPotential int json:"growth_potential"`, `Leadership int json:"leadership"`.
  - Add `const ( PerspectiveTA = "ta"; PerspectiveLineManager = "line_manager" )` + `validPerspectives` map.
  - Add `Perspective string json:"perspective"` to `InterviewFeedback`; add `Perspective string json:"perspective"` to the POST DTO struct in feedback_handler (Task 3).
  - Extend `ValidateFeedback` to validate all 7 competencies 0..`compMax` and (new) `ValidatePerspective(p string) bool`.
- **MIRROR**: existing validation loop in `ValidateFeedback` (feedback.go:59-77).
- **GOTCHA**: keep 0 = not-rated; don't hard-require specific keys per perspective (frontend sends the relevant subset) — only bound 0..5.
- **VALIDATE**: `go build ./internal/applications/...`

### Task 3: Per-perspective role gate in feedback handler
- **ACTION**: Update `backend/internal/applications/feedback_handler.go`.
- **IMPLEMENT**:
  - Replace `feedbackRecordRoles` with `taRecordRoles = {super_admin, hr_manager, hr_staff}` and `lmRecordRoles = {super_admin, sgm}`.
  - Add `Perspective string json:"perspective"` to the create DTO (default `"ta"` if empty).
  - In `Create`: after parse, validate perspective; pick allowlist by perspective; 403 if role not allowed for that perspective; set `fb.Perspective`.
- **MIRROR**: existing role-gate block (feedback_handler.go:108-111).
- **GOTCHA**: keep interviewer stamping + appointment link + notify untouched. Reads (`List`) stay open within scope.
- **VALIDATE**: `go test ./internal/applications/ -run Feedback`

### Task 4: Repo — perspective in insert/select
- **ACTION**: Update `backend/internal/applications/repository.go`.
- **IMPLEMENT**: add `perspective` to `CreateFeedback` INSERT columns + `$N` value; add `f.perspective` to `ListFeedback` SELECT + scan target.
- **MIRROR**: repository.go:183-240.
- **GOTCHA**: keep column order consistent between INSERT and the `RETURNING`/scan; add the scan var to the struct read.
- **VALIDATE**: `go build ./...`

### Task 5: Aggregate computation (scorecard.go)
- **ACTION**: Create `backend/internal/applications/scorecard.go`.
- **IMPLEMENT**:
  - Types: `ScorecardSummary{ TA *PerspectiveAgg; LineManager *PerspectiveAgg; RecommendationTally map[string]int; CompositeScore *float64 }`, `PerspectiveAgg{ Count int; AvgOverall float64; AvgCompetencies map[string]float64; Recommendations map[string]int }`.
  - Pure func `SummarizeFeedback(list []InterviewFeedback, aiScore *float64) ScorecardSummary` that groups by perspective, averages overall + competencies, tallies recommendations, and computes `composite = aiScore*0.6 + (taAvgOverall*20)*0.4` (TA weight collapses to AI-only when no TA scorecard — composite = aiScore). Round to 1dp.
- **MIRROR**: pure-compute style + 1dp rounding from `internal/executive` (`pct` helper).
- **GOTCHA**: no `time.Now`/rand — pure function for testability. Guard divide-by-zero (count==0).
- **VALIDATE**: `go test ./internal/applications/ -run Scorecard`

### Task 6: Shortlist + summary repo queries
- **ACTION**: Add to `backend/internal/applications/repository.go` (or `list.go`): `ScorecardSummaryData(ctx, id)` (reuse ListFeedback + FindByID ai_score) and `Shortlist(ctx, scope rbac.Scope, limit int) ([]ShortlistItem, error)`.
- **IMPLEMENT**: Shortlist SQL —
  ```sql
  SELECT a.id, c.full_name, a.position_id, COALESCE(p.title_en,p.title_th), a.assigned_store_id,
         a.ai_score, ta.avg_overall,
         (COALESCE(a.ai_score,0)*0.6 + COALESCE(ta.avg_overall,0)*20*0.4) AS composite
  FROM applications a
  JOIN candidates c ON c.id=a.candidate_id
  JOIN positions p ON p.id=a.position_id
  LEFT JOIN (SELECT application_id, AVG(overall_rating) avg_overall
             FROM interview_feedback WHERE perspective='ta' GROUP BY application_id) ta ON ta.application_id=a.id
  WHERE a.status='shortlisted' AND <scope.ApplicationsClause>
  ORDER BY composite DESC, a.ai_score DESC NULLS LAST LIMIT $N
  ```
- **MIRROR**: `list.go` arg-builder + `scope.ApplicationsClause(len(args)+1)`.
- **GOTCHA**: apply the RBAC scope clause so an `sgm` only ever gets their store; default `limit=5`.
- **VALIDATE**: `go build ./...`

### Task 7: Shortlist + summary handler + routes
- **ACTION**: Create `backend/internal/applications/shortlist_handler.go`.
- **IMPLEMENT**:
  - `GET /api/v1/shortlist?limit=` → `scopeFrom(c)` → `Shortlist(...)` → `httpx.OK`. Role gate: `lmRecordRoles` ∪ broad-view roles (super_admin/regional_director/auditor/operation_director) may view; store-scoped sgm gets their store via scope.
  - `GET /api/v1/applications/:id/scorecard-summary` → scope-exists check (reuse `ExistsInScope`) → `SummarizeFeedback` → `httpx.OK`.
  - `RegisterShortlistRoutes(app, h)`.
- **MIRROR**: `feedback_handler.go` (scopeFrom, ExistsInScope, httpx.OK) + `dashboard_handler.go:79-82`.
- **GOTCHA**: `/api/v1/shortlist` is a static path — register before any `/api/v1/applications/:id`-style catch-alls (there's no conflict, but keep it with dashboard routes).
- **VALIDATE**: `go test ./internal/applications/ -run Shortlist`

### Task 8: LM directory resolver + notify builder + trigger
- **ACTION**: Update `hr_directory.go`, `notify/hr_message.go`, and the status-change path.
- **IMPLEMENT**:
  - `hr_directory.go`: `LineManagerEmailsForStore(ctx, storeID int) ([]string, error)` → `SELECT email FROM users WHERE role='sgm' AND store_id=$1 AND is_active`.
  - `notify/hr_message.go`: `ShortlistReadyLM(lmEmails []string, teamsEnabled bool, candName, positionTitle, dashURL string) []Message` via `hrMessages`.
  - Trigger: in the status-update handler (`dashboard_handler.go` status PATCH) when target == `StatusShortlisted`, best-effort resolve LM emails for `assigned_store_id` + `dispatchHR`. Reuse the SetNotifier deps already on the dashboard handler.
- **MIRROR**: `notifyFeedbackRecorded` (feedback_handler.go:162-177) + `dispatchHR` (notify.go:56-65).
- **GOTCHA**: best-effort — never block the status write (log on failure). Teams only when enabled. No LINE (staff).
- **VALIDATE**: `go test ./internal/...` ; `go vet ./...`

### Phase B — Frontend

### Task 9: Frontend types
- **ACTION**: Update `frontend/lib/types.ts`.
- **IMPLEMENT**: add to `InterviewCompetencies`: `attitude/growth_potential/leadership: number`; add `perspective: "ta" | "line_manager"` to `InterviewFeedback` + `InterviewFeedbackInput`; add `ScorecardSummary` + `ShortlistItem` interfaces (snake_case to match Go tags).
- **MIRROR**: existing snake_case interfaces (types.ts:77-110).
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 10: Frontend hooks
- **ACTION**: Update `frontend/lib/queries.ts`.
- **IMPLEMENT**: `useShortlist(limit=5)` → `/api/v1/shortlist`; `useScorecardSummary(id)` → `/api/v1/applications/:id/scorecard-summary`; ensure `useAddInterviewFeedback` passes `perspective` (it already posts the full input — just include perspective in the input type); invalidate `["scorecard-summary",id]` + `["shortlist"]` on add.
- **MIRROR**: queries.ts:255-275 + executive `useExecutiveOverview`.
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 11: Frontend role gates
- **ACTION**: Update `frontend/lib/roles.ts`.
- **IMPLEMENT**: `TA_SCORECARD_ROLES=["super_admin","hr_manager","hr_staff"]`+`canRecordTaScorecard`; `LINE_MANAGER_ROLES=["sgm"]`+`isLineManager`; (keep `canRecordInterviewFeedback` or repoint LM scorecard to it). Mirror backend allowlists in comments.
- **MIRROR**: roles.ts:28-41 gate pattern.
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 12: Scorecards component (split + aggregate)
- **ACTION**: Create `frontend/components/resume/Scorecards.tsx` (export `TaScorecard`, `LineManagerScorecard`, `ScorecardSummary`).
- **IMPLEMENT**:
  - Factor the existing form/list from `InterviewFeedbackPanel.tsx` into a shared internal `<ScorecardForm perspective competencies={[...]} roleGate=… />` + `<FeedbackList items filterPerspective />`.
  - TA competencies: technical/communication/attitude; LM competencies: culture_fit/growth_potential/leadership; both keep overall_rating + recommendation + strengths/concerns/notes.
  - `ScorecardSummary` renders the aggregate (TA avg, LM avg, recommendation tally, composite) from `useScorecardSummary`.
  - Each panel self-gates: TA form shows if `canRecordTaScorecard(me.role)` && stageOpen; LM form if `isLineManager(me.role)` && stageOpen. Submit posts with the right `perspective`.
- **MIRROR**: `InterviewFeedbackPanel.tsx` (form state, Select grid, FeedbackCard, recTone, stageOpen, render-null rules).
- **GOTCHA**: client component (`"use client"`); keep hard-coded Thai labels like the existing panel OR add i18n keys to both catalogs (Task 17) — pick one and be consistent. Animate nothing layout-bound.
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 13: Wire panels into detail page
- **ACTION**: Update `frontend/app/(app)/applications/[id]/page.tsx`.
- **IMPLEMENT**: replace `<InterviewFeedbackPanel … />` with `<ScorecardSummary applicationId={app.id} />`, `<TaScorecard applicationId={app.id} status={app.status} />`, `<LineManagerScorecard applicationId={app.id} status={app.status} />` in the aside (summary first).
- **MIRROR**: existing aside composition (page.tsx:35-53).
- **GOTCHA**: keep the old `InterviewFeedbackPanel.tsx` only if still referenced; otherwise delete to avoid dead code (refactor-clean). Verify no other importers first.
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 14: Shortlist page (LM Top-5)
- **ACTION**: Create `frontend/app/(app)/shortlist/page.tsx`.
- **IMPLEMENT**: client page; `useMe()` + `isLineManager` gate (render not-available panel otherwise, mirroring executive); `useShortlist()`; render rows (InitialChip + name link to `/applications/[id]` + ScoreBadge(ai_score) + composite + TA avg + a one-line TA strength). Use `PageHeader` + the row layout from the inbox (drop bulk-select + filters). Empty state when none.
- **MIRROR**: `executive/page.tsx` (gate) + `applications/page.tsx` (rows, ScoreBadge/FitLabel, EmptyState).
- **GOTCHA**: plain `next build` (no `--webpack`); client-page chrome uses `useTranslations`. Approve/Decline reuse the existing status mutation on the detail page (link there) — keep the shortlist page read+navigate to stay bounded.
- **VALIDATE**: `pnpm exec next build`

### Task 15: Nav entry
- **ACTION**: Update `frontend/components/shell/nav-config.tsx`.
- **IMPLEMENT**: import an icon (e.g. `ClipboardCheck`) + `isLineManager`; `export const SHORTLIST_NAV = { href:"/shortlist", label:"Shortlist", key:"shortlist", icon: ClipboardCheck }`; in `navForRole` `if (isLineManager(role)) items.push(SHORTLIST_NAV)`; add to `ALL_NAV`.
- **MIRROR**: `EXECUTIVE_NAV` pattern (nav-config.tsx:38,43-58).
- **VALIDATE**: `pnpm exec tsc --noEmit`

### Task 16: i18n keys
- **ACTION**: Update `frontend/messages/en.json` + `th.json`.
- **IMPLEMENT**: `nav.shortlist` (en "Shortlist" / th "รายชื่อคัดสรร"); if Task 12/14 use i18n, add a `scorecard.*` + `shortlist.*` block (title/eyebrow/meta/taTitle/lmTitle/notAvailable/notAvailableHint…) to BOTH catalogs.
- **MIRROR**: existing `nav` + `executive` blocks.
- **GOTCHA**: same keys in both files or parity guard fails.
- **VALIDATE**: `node scripts/check-i18n-parity.mjs`

### Backend tests (interleave with Phase A)

### Task 17: Scorecard aggregate tests
- **ACTION**: Create `scorecard_test.go`.
- **IMPLEMENT**: table-driven `SummarizeFeedback`: TA-only (composite=ai*0.6+ta*20*0.4), LM-only, both, none (composite=ai), competency averaging, recommendation tally, count==0 guard.
- **MIRROR**: `feedback_test.go` table style.
- **VALIDATE**: `go test ./internal/applications/ -run Scorecard`

### Task 18: Shortlist + perspective handler tests
- **ACTION**: Create `shortlist_handler_test.go`; extend `feedback_test.go`.
- **IMPLEMENT**: shortlist — fake store returns N items; assert sgm gets store-scoped results, limit honored, 200 envelope; feedback — TA perspective allowed for hr_staff (was previously 403), LM perspective 403 for hr_staff, LM allowed for sgm, perspective validation 400 on bad value.
- **MIRROR**: `feedback_test.go:97-195` harness + role injection.
- **VALIDATE**: `go test ./internal/applications/...`

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Summarize TA-only | 1 TA fb rating 4, ai 80 | composite = 80*.6+80*.4=80; TA avg 4 | ✓ |
| Summarize both | TA 4 + LM 5, ai 90 | TA/LM aggs distinct; composite from TA | ✓ |
| Summarize none | [], ai 70 | composite=70; aggs nil | empty ✓ |
| Perspective gate TA | hr_staff + perspective=ta | 201 | role newly allowed |
| Perspective gate LM | hr_staff + perspective=line_manager | 403 | ✓ |
| Perspective invalid | perspective="foo" | 400 | ✓ |
| Shortlist scope | sgm store=5 | only store-5 items, ≤5 | RBAC ✓ |
| Competency bound | leadership=6 | 400 | ✓ |

### Edge Cases Checklist
- [ ] Legacy feedback rows (perspective defaulted 'ta') still list/aggregate
- [ ] Application with no TA scorecard → shortlist composite = ai_score
- [ ] sgm with null store_id → shortlist empty (scope `1=0`)
- [ ] Non-leadership/non-LM role hitting `/shortlist` → empty (scope) or 403
- [ ] tsc/eslint clean; i18n parity

---

## Validation Commands
### Static Analysis
```bash
cd backend && go build ./... && go vet ./... && gofmt -l internal/applications/ migrations/
cd ../frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib
```
EXPECT: zero errors (gofmt -l empty).

### Unit Tests
```bash
cd backend && go test ./internal/applications/... && go test ./...
```
EXPECT: all pass, no regressions (exit 0).

### Build
```bash
cd frontend && pnpm exec next build   # plain, NO --webpack
```
EXPECT: success; `/shortlist` route listed.

### i18n
```bash
node scripts/check-i18n-parity.mjs
```
EXPECT: frontend th/en parity OK.

### DB (review / staging only)
```bash
# 000021 applies cleanly; prod apply is operator-run via the migrate recipe
```

### Manual (local dev)
- [ ] As sgm: nav shows "Shortlist"; `/shortlist` lists store's shortlisted (≤5) ranked by composite.
- [ ] As hr_staff: TA scorecard form visible on detail; LM form hidden; submitting TA works.
- [ ] As sgm: LM scorecard form visible; aggregate card shows TA + LM averages + composite.
- [ ] Shortlisting a candidate (status→shortlisted) sends LM email (mock/log if NOTIFY mock locally).
- [ ] TH/EN switch translates nav + page chrome.

---

## Acceptance Criteria
- [ ] All tasks done; `go build/vet/test` + `tsc`/`eslint`/`next build` + i18n parity green
- [ ] TA vs LM scorecards split with correct per-perspective role gates (server-enforced)
- [ ] Aggregate summary (TA avg, LM avg, tally, composite) renders + has unit tests
- [ ] `/shortlist` shows store-scoped Top-5 composite ranking; nav gated to sgm
- [ ] LM-notify on shortlist (best-effort, email; Teams when enabled)
- [ ] No new status/transition; LM approve/decline reuse existing
- [ ] Migration 000021 additive; legacy rows default 'ta'

## Completion Checklist
- [ ] Mirrors feedback/repo/scope/notify patterns; envelope via httpx
- [ ] No `time.Now`/rand in aggregate compute; best-effort notify never blocks writes
- [ ] snake_case FE types match Go tags; client-page chrome uses useTranslations
- [ ] Old InterviewFeedbackPanel removed if unreferenced (no dead code)
- [ ] i18n keys in both catalogs; gofmt clean
- [ ] Self-contained — no further codebase search needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LM-email resolver: sgm users lack store_id/email on prod | Med | Med | best-effort + log; notify is non-blocking; document seeding need |
| Composite formula disputed by client | Med | Low | formula isolated in `SummarizeFeedback`; weights are constants, easy to tune |
| Extending competencies breaks legacy reads | Low | Med | additive JSONB keys (0=not rated); perspective defaults 'ta'; covered by tests |
| `shortlisted`-trigger notify too noisy | Med | Low | per-event best-effort now; note debounce/digest as follow-up |
| RequiresSchedule couples LM "approve" to schedule flow | Med | Low | LM approve links to existing detail schedule dialog; not re-implemented here |

## Notes
- **Deploy** (when shipping): backend `az acr build … SVC=api` + `containerapp update` + **migration 000021** via the operator migrate recipe (temp PG firewall rule); dashboard build needs the 4 `NEXT_PUBLIC_AZURE_AD_*` build-args + authority `/organizations`; verify active revision; smoke many routes (PRP-4 lesson). No worker change.
- **LM = sgm** (no `line_manager` role exists; documented in feedback_handler.go + rbac). If a dedicated `line_manager` role is later added, extend the allowlists + `scope.go` switch (no DB enum).
- **Next ATS PRPs** (sequence): 3.5 Approval Workflow → 3.6 Offer Management → 3.3 Interview Letter → 3.8 Document/Onboarding → 3.9 ATS Reports. Track in a Module-3 roadmap.
