# Plan: Candidate Status State Machine + Interview Scheduling

## Summary
Replace the current free-form application-status changes with a **guarded state machine** enforced server-side: each status only permits specific next actions. Add **human-interview scheduling** (date/time + onsite/online) that, for online interviews, creates a Microsoft Graph calendar event with a Teams meeting and emails the candidate an invite. Add a **mandatory rejection reason** (stored, not sent to the candidate). "Hire" enters an `offer` stage (the Offer Package itself is a future feature).

## User Story
As an **HR recruiter**, I want the candidate pipeline to enforce a fixed progression (screened → AI interview → shortlist/interview → hire/reject) with the right action available at each step, so that nobody can skip stages or move a candidate inconsistently, and interview logistics (calendar + Teams link) happen automatically.

## Problem → Solution
**Current**: any allowed status (`scored`, `shortlisted`, `interview`, `hired`, `rejected`) can be set from any other status — the backend allowlist (`internal/applications/handler.go:143-147`) checks only the *target*, never the *current* state. The 4 action buttons (`AiSummaryPanel.tsx:11-16`) are always shown. No interview date/time, no calendar/Teams, no reject reason, no offer stage.
**Desired**: a transition map gates every change (server + UI); the "Interview" action collects a schedule and fires a Graph/Teams calendar invite; "Reject" requires a reason; "Hire" → `offer`.

## Metadata
- **Complexity**: Large (≈18–22 files, new `internal/calendar` package + Graph integration + migration + frontend dialogs)
- **Source PRD**: N/A (free-form, 8-step spec)
- **PRD Phase**: N/A
- **Estimated Files**: ~20

---

## Status State Machine (the spec, formalized)

Canonical `applications.status` values (string column `VARCHAR(50)`):

| Status | Meaning | Set by | Allowed manual next actions |
|---|---|---|---|
| `scored` | **Screened** — passed AI screening (entry to HR funnel) | pipeline (existing) | **Send AI Interview** only |
| `ai_interview` | AI pre-interview invited / in progress | "Send AI Interview" action | _(none — waits for completion)_ |
| `ai_interviewed` | AI pre-interview completed | **system** (interview session → completed) | **Shortlist** \| **Interview** \| **Reject** |
| `shortlisted` | HR shortlisted | Shortlist action | **Interview** \| **Reject** |
| `interview` | Human interview **scheduled** (carries date/time + mode) | Interview action (requires schedule) | **Mark interview done** \| **Reject** |
| `interviewed` | Human interview completed | "Mark interview done" action | **Hire** \| **Reject** |
| `offer` | Entered Offer Package process | **Hire** action | **Reject** _(Offer Package = future)_ |
| `rejected` | Terminal — carries `rejection_reason` | Reject action (any HR state) | _(terminal)_ |

Pre-HR system statuses unchanged and **not** part of the manual machine: `pending`, `parsed`, `failed` (and the pipeline's own gate-fail `rejected`). `hired` is retained as a constant but **no longer the funnel terminal** — superseded by `offer` (see NOT Building / Risks for the PeopleSoft-sync implication).

**Transition rules** (the single source of truth, enforced in the backend):
```
scored        --Send AI Interview-->  ai_interview          (interview pkg, guard: from scored only)
ai_interview  --(session completed)-> ai_interviewed        (system, interview pkg)
ai_interviewed--Shortlist-->          shortlisted
ai_interviewed--Interview(schedule)-> interview
ai_interviewed--Reject(reason)-->     rejected
shortlisted   --Interview(schedule)-> interview
shortlisted   --Reject(reason)-->     rejected
interview     --Mark done-->          interviewed
interview     --Reject(reason)-->     rejected
interviewed   --Hire-->               offer
interviewed   --Reject(reason)-->     rejected
offer         --Reject(reason)-->     rejected
```
- The `interview` target is **only** reachable via the dedicated schedule endpoint (it requires a payload), never via plain status PATCH.
- `Reject` is reachable from every HR state and **requires a non-empty reason**.

---

## UX Design

### Before
```
AiSummaryPanel (always shows, regardless of status):
  [ Shortlist ] [ Interview ]
  [ Hire ]      [ Reject ]
  [ ▶ Send AI interview ]
```

### After
```
AiSummaryPanel renders ONLY the actions allowed by current status:

status=scored:        [ ▶ Send AI interview ]
status=ai_interview:  (no actions — "AI interview in progress" note)
status=ai_interviewed:[ Shortlist ] [ Interview… ] [ Reject… ]
status=shortlisted:   [ Interview… ] [ Reject… ]
status=interview:     [ Mark interview done ] [ Reject… ]   + schedule summary card
status=interviewed:   [ Hire ] [ Reject… ]
status=offer:         "In offer process"  [ Reject… ]
status=rejected:      "Not selected — <reason>" (read-only)

[ Interview… ]  → opens Schedule dialog (date/time, duration, onsite|online, location/notes)
[ Reject… ]     → opens Reject dialog (required reason textarea)
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Action buttons | 4 fixed + AI interview, always shown | Subset gated by `app.status` | Mirrors backend transition map |
| Interview | one-click status=interview | Dialog: datetime + onsite/online (+ location/notes) | Online → Graph/Teams invite |
| Reject | one-click status=rejected | Dialog: required reason; reason stored, **not** sent to candidate | New `rejection_reason` column |
| Hire | status=hired (+ PeopleSoft sync) | status=`offer` (enter Offer Package) | PS sync deferred — see Risks |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/handler.go` | 143-194 | The status allowlist + `UpdateStatus` handler to replace with the machine |
| P0 | `backend/internal/applications/model.go` | 11-19 | Status constants — add new ones here |
| P0 | `backend/internal/applications/repository.go` | 14-39, 117-201 | Repository interface + `SetStatus`/`SetHired` patterns |
| P0 | `backend/internal/applications/dashboard_handler.go` | 32-76, 127-179 | DI pattern (`SetNotifier`/`SetIndexer`), Bulk handler to re-gate |
| P0 | `backend/internal/interview/service.go` | 69-95, 143-207, 244-265 | AI invite (set `ai_interview`) + completion (auto `ai_interviewed`) hooks |
| P0 | `backend/internal/applications/notify.go` | 26-47 | Status-change notify seam to reuse for interview-scheduled message |
| P1 | `backend/internal/fit/azure.go` | 1-60, 132-166 | **Mirror this** for the new Graph HTTP client (Azure-style external client) |
| P1 | `backend/internal/peoplesoft/rest.go` | 29-72 | **Mirror this** for OAuth2 client-credentials + retry (Graph token) |
| P1 | `backend/internal/notify/notify.go` | 28-39 | Provider interface + mock/real factory seam to mirror for calendar |
| P1 | `backend/pkg/config/config.go` | 26-28, 90-93, 184, 206-212, 268-288, 381-383 | Provider config + validation pattern to add `GRAPH_*` |
| P1 | `backend/cmd/api/main.go` | 150-200, 254-300 | Wiring: where notifier/handlers are built + routes registered |
| P0 | `frontend/components/resume/AiSummaryPanel.tsx` | 11-16, 22-33, 101-130 | The action buttons + handlers to gate by status |
| P0 | `frontend/lib/queries.ts` | 188-226 | `useSetStatus`, `useInviteInterview` — add `useScheduleInterview`, reason |
| P1 | `frontend/components/people/PeopleBits.tsx` | 135-174 | `STATUS_LABELS` + `toneForStatus` — add new statuses |
| P1 | `frontend/components/ui/dialog.tsx` | 1-160 | Dialog primitive for the schedule + reject modals |
| P1 | `frontend/components/ui/select.tsx` | 1-201 | Select for onsite/online mode |
| P2 | `backend/migrations/000018_hr_password_auth.up.sql` | all | Most recent migration — mirror style for `000019` |
| P2 | `backend/internal/interview/repository.go` | 142-167 | Optimistic-lock + idempotent update style for the appointment repo |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Create calendar event | learn.microsoft.com `user-post-events` | `POST /users/{mailbox}/events`, app perm **Calendars.ReadWrite**. Body: `subject`, `body{contentType,content}`, `start/end{dateTime,timeZone}`, `location{displayName}`, `attendees[{emailAddress{address,name},type:"required"}]`. **"When an event is sent, the server sends invitations to all the attendees"** — the candidate gets the email invite automatically. Use header `Prefer: outlook.timezone="SE Asia Standard Time"`. |
| App-only Teams meeting | MS Q&A + cloud-communication-online-meeting-application-access-policy | **GOTCHA**: creating an event with `isOnlineMeeting=true` via an **app-only** token is unreliable (silently stops minting Teams meetings). **Robust path**: `POST /users/{mailbox}/onlineMeetings` (app perm **OnlineMeetings.ReadWrite**) to mint the meeting → take `joinWebUrl` → create the event and embed the join link in `body`/`location`. |
| Application Access Policy | learn.microsoft.com cloud-communication doc | App-only access to `/onlineMeetings` (and recommended scoping for `Calendars.ReadWrite`) requires a **Teams Application Access Policy** (`New-CsApplicationAccessPolicy` + `Grant-CsApplicationAccessPolicy -PolicyName … -Identity <service-mailbox>`). Without it: `"No application access policy found for this app"`. Changes take **up to 30 min** to apply. |
| Token | OAuth2 client credentials | `POST https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token`, `grant_type=client_credentials`, `scope=https://graph.microsoft.com/.default`. Use `golang.org/x/oauth2/clientcredentials` exactly like `peoplesoft/rest.go:29-37`. |
| Time zone | dateTimeTimeZone resource | Graph wants a **Windows** time-zone name by default; Thailand = `"SE Asia Standard Time"`. |

---

## Patterns to Mirror

### NAMING_CONVENTION (status constants)
```go
// SOURCE: backend/internal/applications/model.go:11-19
const (
	StatusPending  = "pending"
	StatusParsed   = "parsed"
	StatusFailed   = "failed"
	StatusScored   = "scored"   // == "screened" in the funnel
	StatusRejected = "rejected"
	StatusHired    = "hired"
)
```

### STATUS_CHANGE_HANDLER (to replace with the machine)
```go
// SOURCE: backend/internal/applications/handler.go:143-194
var allowedStatuses = map[string]bool{
	StatusScored: true, StatusRejected: true, StatusHired: true,
	"shortlisted": true, "interview": true,
}
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	...
	if !allowedStatuses[req.Status] { return fiber.NewError(400, "unsupported status transition") }
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); ... // per-record authz
	if req.Status == StatusHired { h.apps.SetHired(...); h.hired.SyncHired(...); ... }
	if err := h.apps.SetStatus(c.UserContext(), id, req.Status); err != nil { return err }
	h.notifyDeps.notifyStatusChange(c.UserContext(), h.apps, id, req.Status)
	...
}
```

### REPOSITORY_PATTERN
```go
// SOURCE: backend/internal/applications/repository.go:117-123
func (r *pgRepository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE applications SET status = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, status); err != nil {
		return fmt.Errorf("applications: set status: %w", err)
	}
	return nil
}
```

### EXTERNAL_HTTP_CLIENT (mirror for Graph — Azure style)
```go
// SOURCE: backend/internal/fit/azure.go:132-166
func (a azureSummarizer) call(ctx context.Context, cr chatRequest) (string, error) {
	body, _ := json.Marshal(cr)
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.endpoint, a.deployment, openAIAPIVersion)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("api-key", a.key); req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req); if err != nil { return "", fmt.Errorf("fit: call: %w", err) }
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("fit: status %d: %s", resp.StatusCode, string(raw)) }
	...
}
```

### OAUTH2_CLIENT_CREDENTIALS (mirror for Graph token)
```go
// SOURCE: backend/internal/peoplesoft/rest.go:29-37
cc := &clientcredentials.Config{
	ClientID:     cfg.PSIBClientID,
	ClientSecret: cfg.PSIBClientSecret,
	TokenURL:     cfg.PSIBTokenURL,
}
httpClient := cc.Client(context.Background())
httpClient.Timeout = restTimeout
// For Graph: TokenURL = https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token,
// add  Scopes: []string{"https://graph.microsoft.com/.default"}
```

### PROVIDER_FACTORY_SEAM (mirror for calendar mock/real)
```go
// SOURCE: backend/internal/notify/notify.go:28-39
type Notifier interface { Send(ctx context.Context, m Message) error }
func NewNotifier(cfg *config.Config) Notifier {
	if cfg.UsesRealNotify() { return newRESTNotifier(cfg) }
	return mockNotifier{}
}
```

### CONFIG_PROVIDER_PATTERN
```go
// SOURCE: backend/pkg/config/config.go:220-222, 268-288, 381-383
NotifyProvider:  getenv("NOTIFY_PROVIDER", "mock"),
// validation: {"NOTIFY_PROVIDER", c.NotifyProvider, []string{"mock", ProviderReal}},
func (c *Config) UsesRealNotify() bool { return c.NotifyProvider == ProviderReal }
```

### DASHBOARD_DI (optional dependency injection)
```go
// SOURCE: backend/internal/applications/dashboard_handler.go:56-68
func (h *DashboardHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notifyDeps = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}
```

### FRONTEND_ACTION_BUTTONS (to gate by status)
```tsx
// SOURCE: frontend/components/resume/AiSummaryPanel.tsx:11-16, 101-114
const NEXT_ACTIONS = [
  { label: "Shortlist", value: "shortlisted", variant: "secondary" },
  { label: "Interview", value: "interview", variant: "secondary" },
  { label: "Hire", value: "hired" },
  { label: "Reject", value: "rejected", variant: "destructive" },
];
// rendered as a fixed 2-col grid + a "Send AI interview" button below
```

### FRONTEND_MUTATION
```tsx
// SOURCE: frontend/lib/queries.ts:188-197
export function useSetStatus(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (status: string) => api.patch(`/api/v1/applications/${id}/status`, { status }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["application", id] }); qc.invalidateQueries({ queryKey: ["applications"] }); },
  });
}
```

### TEST_STRUCTURE (Go table-driven, the project standard)
```go
// SOURCE: convention across backend/internal/**/*_test.go
func TestX(t *testing.T) {
	cases := []struct{ name string; ...; want ... }{ ... }
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T){ /* arrange-act-assert */ }) }
}
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000019_status_machine.up.sql` / `.down.sql` | CREATE | `applications.rejection_reason TEXT`; `interview_appointments` table |
| `backend/internal/applications/model.go` | UPDATE | Add `StatusAIInterview`, `StatusAIInterviewed`, `StatusShortlisted`, `StatusInterview`, `StatusInterviewed`, `StatusOffer` constants |
| `backend/internal/applications/transitions.go` | CREATE | Central transition map + `CanTransition(from,to)` + `RejectionRequiresReason` |
| `backend/internal/applications/transitions_test.go` | CREATE | Table-driven coverage of every allowed/denied transition |
| `backend/internal/applications/handler.go` | UPDATE | `UpdateStatus`: load current status, enforce `CanTransition`, require reason on reject; route `offer` (replaces hired branch) |
| `backend/internal/applications/repository.go` | UPDATE | Add `SetStatusWithReason`, `SetRejection`; `FindByID` already returns status; add `Appointment` CRUD or new repo |
| `backend/internal/applications/dashboard_handler.go` | UPDATE | Re-gate `Bulk` to only `shortlisted` + `reject(reason)`; reject reason on bulk |
| `backend/internal/applications/schedule_handler.go` | CREATE | `POST /applications/:id/interview-schedule` — guard, persist appointment, fire calendar, notify |
| `backend/internal/applications/routes.go` | UPDATE | Register the schedule route |
| `backend/internal/interview/service.go` | UPDATE | `Invite`: guard from `scored`, set app status `ai_interview`. On completion (`Respond`), set app status `ai_interviewed` |
| `backend/internal/calendar/calendar.go` | CREATE | `Provider` interface + `Appointment`/`Result` types + mock/real factory |
| `backend/internal/calendar/graph.go` | CREATE | Graph client: client-creds token, `POST /onlineMeetings` (online) → joinUrl, `POST /events` with attendee |
| `backend/internal/calendar/mock.go` | CREATE | Log-only, returns deterministic fake joinUrl |
| `backend/internal/calendar/graph_test.go` | CREATE | httptest server asserting payloads + joinUrl extraction |
| `backend/pkg/config/config.go` | UPDATE | `GRAPH_PROVIDER` + `GRAPH_TENANT_ID/CLIENT_ID/CLIENT_SECRET/ORGANIZER_MAILBOX/TIMEZONE`, `UsesRealGraph()`, validation |
| `backend/cmd/api/main.go` | UPDATE | Build `calendar.NewProvider(cfg)`, inject into the schedule handler; register route |
| `frontend/lib/types.ts` | UPDATE | Add `InterviewAppointment` type; extend `Application` with `rejection_reason?`, `appointment?` |
| `frontend/lib/queries.ts` | UPDATE | `useSetStatus` accepts `{status,reason?}`; add `useScheduleInterview`; guard invalidations |
| `frontend/lib/statusMachine.ts` | CREATE | UI mirror of the transition map → `allowedActions(status, interviewState)` |
| `frontend/components/resume/AiSummaryPanel.tsx` | UPDATE | Render only allowed actions; wire dialogs |
| `frontend/components/resume/ScheduleInterviewDialog.tsx` | CREATE | datetime + duration + onsite/online + location/notes |
| `frontend/components/resume/RejectDialog.tsx` | CREATE | Required reason textarea |
| `frontend/components/people/PeopleBits.tsx` | UPDATE | `STATUS_LABELS` + `toneForStatus` for new statuses |
| `frontend/components/bulk/BulkActionBar.tsx` | UPDATE | Restrict to Shortlist + Reject(reason); drop Interview from bulk |

## NOT Building
- **Offer Package** itself (offer letter, compensation, send-offer, candidate accept/decline). "Hire" only transitions to `offer`. Separate future PRD.
- **PeopleSoft sync on hire** is **deferred**: the existing `SyncHired` path (`handler.go:176-184`, `peoplesoft/service.go`) is retained but no longer auto-fired by the funnel (it belongs to the future "offer accepted" step). See Risks.
- **Reschedule / cancel** of an interview appointment (single schedule per app for v1; the table supports history but no reschedule UI).
- **Candidate-facing calendar for onsite** interviews (onsite stores location text only; no calendar event — only `online` fires Graph per the spec).
- **Reject notification to candidate** — explicitly NOT sent (spec step 8). Reason is internal only.
- A bespoke date/time picker component — use native `<input type="datetime-local">`.
- Bulk Interview/Hire/Send-AI (these need per-candidate payloads/guards).

---

## Step-by-Step Tasks

### Task 1: Migration 000019
- **ACTION**: Create `backend/migrations/000019_status_machine.up.sql` + `.down.sql`.
- **IMPLEMENT**:
  ```sql
  -- up
  ALTER TABLE applications ADD COLUMN IF NOT EXISTS rejection_reason TEXT;
  CREATE TABLE IF NOT EXISTS interview_appointments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    scheduled_at    TIMESTAMPTZ NOT NULL,
    duration_min    INT NOT NULL DEFAULT 60,
    mode            TEXT NOT NULL,            -- 'onsite' | 'online'
    location_text   TEXT,                     -- onsite address / room
    online_join_url TEXT,                     -- Teams join link (online)
    calendar_event_id TEXT,                   -- Graph event id (for future cancel)
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX IF NOT EXISTS idx_interview_appointments_app ON interview_appointments (application_id);
  ```
  ```sql
  -- down
  DROP TABLE IF EXISTS interview_appointments;
  ALTER TABLE applications DROP COLUMN IF EXISTS rejection_reason;
  ```
- **MIRROR**: `migrations/000018_hr_password_auth.up.sql` (TEXT, `IF NOT EXISTS`, explicit index).
- **GOTCHA**: `mode` is free TEXT — validate in Go, not a DB enum (matches project convention of `VARCHAR` status + app-level validation).
- **VALIDATE**: `migrate up` locally → schema advances; `\d applications` shows `rejection_reason`.

### Task 2: Status constants + transition map
- **ACTION**: Add constants to `model.go`; create `transitions.go`.
- **IMPLEMENT**:
  ```go
  // model.go — add
  const (
  	StatusAIInterview   = "ai_interview"
  	StatusAIInterviewed = "ai_interviewed"
  	StatusShortlisted   = "shortlisted"
  	StatusInterview     = "interview"
  	StatusInterviewed   = "interviewed"
  	StatusOffer         = "offer"
  )
  // transitions.go
  // allowedTransitions[current] = set of manual target statuses (HR actions).
  var allowedTransitions = map[string]map[string]bool{
  	StatusAIInterviewed: {StatusShortlisted: true, StatusInterview: true, StatusRejected: true},
  	StatusShortlisted:   {StatusInterview: true, StatusRejected: true},
  	StatusInterview:     {StatusInterviewed: true, StatusRejected: true},
  	StatusInterviewed:   {StatusOffer: true, StatusRejected: true},
  	StatusOffer:         {StatusRejected: true},
  }
  func CanTransition(from, to string) bool { return allowedTransitions[from][to] }
  // RequiresSchedule reports a target only reachable via the schedule endpoint.
  func RequiresSchedule(to string) bool { return to == StatusInterview }
  ```
- **MIRROR**: `handler.go:143-147` allowlist style, upgraded to a 2-level map.
- **GOTCHA**: `StatusInterview` is in the map (reachable from `ai_interviewed`/`shortlisted`) but plain status PATCH must REJECT it (`RequiresSchedule`) and force the schedule endpoint.
- **VALIDATE**: `transitions_test.go` table covers each `(from,to)` pair → matches the table above.

### Task 3: Repository — reason + appointment
- **ACTION**: Extend `applications.Repository`.
- **IMPLEMENT**:
  ```go
  SetRejection(ctx, id uuid.UUID, reason string) error // sets status=rejected, rejection_reason=$2
  CreateAppointment(ctx, a Appointment) (Appointment, error)
  FindAppointment(ctx, applicationID uuid.UUID) (*Appointment, error)
  ```
  `SetRejection`: `UPDATE applications SET status='rejected', rejection_reason=$2, updated_at=NOW() WHERE id=$1`.
- **MIRROR**: `repository.go:117-123` SetStatus; `SetHired:187-193` for the multi-column update.
- **IMPORTS**: existing (`uuid`, `pgxpool`, `fmt`).
- **GOTCHA**: `FindByID` must also select `rejection_reason` into the `Application` struct — add the column to the scan + struct (`model.go` Application).
- **VALIDATE**: `go build ./...`; repo method signatures satisfy the interface.

### Task 4: UpdateStatus handler → enforce machine
- **ACTION**: Rewrite `Handler.UpdateStatus` (`handler.go:156-194`).
- **IMPLEMENT**:
  - Load current app (`h.apps.FindByID`) after the scope check.
  - If `RequiresSchedule(req.Status)` → `400 "use the schedule endpoint for interviews"`.
  - If `!CanTransition(app.Status, req.Status)` → `400 "transition not allowed from <current>"`.
  - If `req.Status == StatusRejected`: require `req.Reason != ""` → else `400 "rejection reason is required"`; call `SetRejection`. Do **not** notify (spec).
  - If `req.Status == StatusOffer`: `SetStatus` to `offer`; **no** PS sync (deferred), notify candidate optional (no message defined → skip).
  - Else `SetStatus`; `notifyStatusChange` (only `shortlisted`/`interview`/`hired` produce a message today — `shortlisted` still notifies).
  - Record activity (`activity.ActionStatusChange`) with old/new value.
- **MIRROR**: existing `UpdateStatus` structure + `ExistsInScope` authz.
- **GOTCHA**: `updateStatusReq` gains `Reason string json:"reason"`. Keep the `hired` constant working for any legacy callers but the funnel uses `offer`.
- **VALIDATE**: handler test: scored→shortlisted denied (must AI-interview first); ai_interviewed→shortlisted ok; interviewed→offer ok; any→rejected without reason = 400.

### Task 5: Interview package — drive app status
- **ACTION**: Update `interview/service.go`.
- **IMPLEMENT**:
  - In `Invite` (`:69-95`): before creating the session, load the application; if `app.Status != applications.StatusScored` → return a typed error → handler maps to `409 "AI interview only from screened"`. After successful create, set `app.status = ai_interview` (inject an `appStatusSetter` dependency to avoid an import cycle — interview already imports applications types? It imports `apps Repository` already per `:244-265`).
  - In `Respond` completion branch (`:189-206`, after `StatusCompleted`): set `app.status = ai_interviewed` (best-effort, log on failure).
- **MIRROR**: `notifyInvite` dependency style (`s.apps`, `s.cands`).
- **GOTCHA**: completion auto-advance must be idempotent (re-completion is a no-op; `SetEvaluation` is already idempotent). Only advance if current status is `ai_interview` (don't clobber a later manual state).
- **VALIDATE**: service test — invite from `scored` sets `ai_interview`; invite from `shortlisted` errors; completing the session sets `ai_interviewed`.

### Task 6: Calendar package (mock + Graph)
- **ACTION**: Create `internal/calendar/{calendar.go,graph.go,mock.go,graph_test.go}`.
- **IMPLEMENT**:
  ```go
  type Appointment struct {
  	Subject, BodyHTML string
  	Start, End        time.Time
  	Mode              string // "onsite" | "online"
  	LocationText      string
  	AttendeeEmail     string
  	AttendeeName      string
  }
  type Result struct { EventID, JoinURL string }
  type Provider interface { CreateInterview(ctx context.Context, a Appointment) (Result, error) }
  func NewProvider(cfg *config.Config) Provider {
  	if cfg.UsesRealGraph() { return newGraphProvider(cfg) }
  	return mockProvider{}
  }
  ```
  Graph provider (`graph.go`):
  1. Token via `clientcredentials.Config{ClientID, ClientSecret, TokenURL: "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token", Scopes: ["https://graph.microsoft.com/.default"]}.Client(ctx)`.
  2. If `Mode=="online"`: `POST https://graph.microsoft.com/v1.0/users/{organizer}/onlineMeetings` `{startDateTime,endDateTime,subject}` → parse `joinWebUrl`. (Mint FIRST — app-only `isOnlineMeeting` is unreliable.)
  3. `POST https://graph.microsoft.com/v1.0/users/{organizer}/events` with header `Prefer: outlook.timezone="<cfg.GraphTimeZone>"`, body: `subject`, `body{contentType:"HTML", content: BodyHTML + joinUrl}`, `start/end{dateTime: RFC3339-local, timeZone: cfg.GraphTimeZone}`, `attendees:[{emailAddress:{address,name}, type:"required"}]`, and for online `location{displayName:"Microsoft Teams Meeting"}` else `location{displayName: LocationText}`. Capture event `id`.
  4. Return `{EventID, JoinURL}`.
- **MIRROR**: `fit/azure.go:132-166` (request/err/status/decode) + `peoplesoft/rest.go:29-37` (clientcredentials).
- **IMPORTS**: `net/http`, `bytes`, `encoding/json`, `io`, `fmt`, `time`, `golang.org/x/oauth2/clientcredentials`.
- **GOTCHA**: Graph wants the event `start.dateTime` WITHOUT a zone suffix (local wall time) paired with `timeZone`. Format `t.Format("2006-01-02T15:04:05")`. Online-meeting mint REQUIRES the Application Access Policy (operational, see Risks) — on `403 "No application access policy"`, return a wrapped error so the schedule still saves but `JoinURL` is empty + logged.
- **VALIDATE**: `graph_test.go` with `httptest.Server` asserts the two POST bodies + that `joinWebUrl` flows into `Result.JoinURL`; mock returns `https://teams.microsoft.com/l/meetup-join/mock`.

### Task 7: Config — GRAPH_*
- **ACTION**: Add fields to `config.go`.
- **IMPLEMENT**:
  ```go
  GraphProvider        string // "mock" (default) | "real"
  GraphTenantID        string
  GraphClientID        string
  GraphClientSecret    string
  GraphOrganizerMailbox string // service mailbox, e.g. interviews@ert...
  GraphTimeZone        string // default "SE Asia Standard Time"
  // load: getenv("GRAPH_PROVIDER","mock"); os.Getenv(...); getenv("GRAPH_TIMEZONE","SE Asia Standard Time")
  // validate: {"GRAPH_PROVIDER", c.GraphProvider, []string{"mock", ProviderReal}}
  // UsesRealGraph(): c.GraphProvider == ProviderReal
  // if UsesRealGraph(): require TenantID/ClientID/ClientSecret/OrganizerMailbox
  ```
- **MIRROR**: `config.go:90-93,206-212,268-288,302-309,381-383` (Notify/PS provider).
- **VALIDATE**: `go test ./pkg/config/...`; `GRAPH_PROVIDER=real` without creds → startup error.

### Task 8: Schedule endpoint
- **ACTION**: Create `schedule_handler.go`; register `POST /api/v1/applications/:id/interview-schedule`.
- **IMPLEMENT**:
  - DTO `{scheduled_at (RFC3339), duration_min int, mode "onsite"|"online", location_text string}`.
  - Scope check (`ExistsInScope`); load app; guard `CanTransition(app.Status, StatusInterview)` (from `ai_interviewed`/`shortlisted`).
  - Validate `mode`; `scheduled_at` in the future; duration 15–480.
  - If `mode=="online"`: build `calendar.Appointment` (subject `"สัมภาษณ์งาน: <position>"`, attendee = candidate email + name from `candidates.FindByID`), call `cal.CreateInterview`; capture `EventID`/`JoinURL`. Best-effort: a Graph failure logs + proceeds with empty `JoinURL` (don't fail the schedule).
  - Persist appointment (`CreateAppointment`); `SetStatus(id, StatusInterview)`.
  - Notify candidate (LINE) reusing the notify seam with a new `interview` scheduled message including date + (online) join link.
  - Return the appointment + status.
- **MIRROR**: `dashboard_handler.go` handler structure + DI; `notify.go` seam.
- **GOTCHA**: candidate may have no email (onsite is fine; online needs email — if `mode==online && email==""` → `400 "candidate has no email for an online invite"`).
- **VALIDATE**: handler test — schedule from `shortlisted` ok (status→interview, appointment row); from `scored` denied; online without email → 400.

### Task 9: Bulk re-gate
- **ACTION**: Update `dashboard_handler.go` `Bulk` (`:127-179`).
- **IMPLEMENT**: allow only `action:"status",value:"shortlisted"` and `action:"reject"` (require `req.Reason`); for each id, load + `CanTransition` (skip+count `failed` if not allowed); reject → `SetRejection`. Drop `interview`/`hired` from bulk.
- **MIRROR**: existing Bulk loop + activity log.
- **GOTCHA**: per-id transition check means a mixed-status selection partially succeeds — return `{updated,failed}` (already the contract).
- **VALIDATE**: bulk shortlist on `ai_interviewed` ids → updated; on `scored` ids → failed.

### Task 10: Wire calendar in main.go
- **ACTION**: Build provider + inject.
- **IMPLEMENT**: `cal := calendar.NewProvider(cfg)`; pass to the schedule handler constructor; `applications.RegisterScheduleRoutes(app, scheduleHandler)` (or extend existing `RegisterDashboardRoutes`).
- **MIRROR**: `main.go:254-300` handler construction + route registration ordering (static routes before `:id`).
- **VALIDATE**: `go build ./...`; route present.

### Task 11: Frontend status machine mirror + types
- **ACTION**: Create `lib/statusMachine.ts`; extend `lib/types.ts`.
- **IMPLEMENT**:
  ```ts
  export type Action = "send_ai_interview" | "shortlist" | "interview" | "mark_interviewed" | "hire" | "reject";
  export function allowedActions(status: string, aiState?: string): Action[] {
    switch (status) {
      case "scored": return ["send_ai_interview"];
      case "ai_interview": return [];
      case "ai_interviewed": return ["shortlist", "interview", "reject"];
      case "shortlisted": return ["interview", "reject"];
      case "interview": return ["mark_interviewed", "reject"];
      case "interviewed": return ["hire", "reject"];
      case "offer": return ["reject"];
      default: return [];
    }
  }
  ```
  `types.ts`: `Application.rejection_reason?: string | null`; `InterviewAppointment { scheduled_at; duration_min; mode; location_text?; online_join_url?|null }`.
- **MIRROR**: backend `transitions.go` exactly (keep in sync; comment cross-reference).
- **VALIDATE**: `tsc --noEmit`.

### Task 12: Frontend dialogs + panel gating
- **ACTION**: Create `ScheduleInterviewDialog.tsx`, `RejectDialog.tsx`; rewrite `AiSummaryPanel.tsx` action area.
- **IMPLEMENT**:
  - `AiSummaryPanel`: compute `allowedActions(app.status)`; render only those buttons. `send_ai_interview` → existing `useInviteInterview`. `shortlist`→`useSetStatus("shortlisted")`. `interview`→open ScheduleInterviewDialog. `mark_interviewed`→`useSetStatus("interviewed")`. `hire`→`useSetStatus("offer")` (toast "Entered offer process"). `reject`→open RejectDialog.
  - `ScheduleInterviewDialog`: `<input type="datetime-local">`, duration `<Select>` (30/45/60/90), mode `<Select>` onsite/online, `location_text` `<Input>` (label adapts: "สถานที่" onsite / "หมายเหตุ" online). Submit → `useScheduleInterview`.
  - `RejectDialog`: required `<textarea>` reason → `useSetStatus({status:"rejected", reason})`; disable submit until non-empty.
- **MIRROR**: `components/admin/UserManagement.tsx` dialog pattern (this repo's Dialog usage) + `ConsentStep` checkbox-gating style.
- **IMPORTS**: `@/components/ui/dialog`, `select`, `input`, `button`; `useScheduleInterview`, `useSetStatus`.
- **GOTCHA**: `datetime-local` yields local time without zone → convert to RFC3339 with the browser zone before sending (`new Date(value).toISOString()`).
- **VALIDATE**: `tsc`+`eslint`+`next build`; manual: each status shows only its allowed buttons.

### Task 13: Status labels/tones + bulk bar + queries
- **ACTION**: Update `PeopleBits.tsx`, `BulkActionBar.tsx`, `queries.ts`.
- **IMPLEMENT**:
  - `STATUS_LABELS`: add `ai_interview:"AI interview"`, `ai_interviewed:"AI interview done"`, `interviewed:"Interviewed"`, `offer:"Offer"`. `toneForStatus`: `offer`+`interviewed`→pass; `ai_interview`/`ai_interviewed`→pending.
  - `BulkActionBar`: actions = `[{Shortlist,status,shortlisted},{Reject,reject,rejected}]`; Reject opens a shared reason prompt.
  - `queries.ts`: `useSetStatus` mutationFn `(v:{status:string;reason?:string})`; add `useScheduleInterview(id)` → `api.post('/api/v1/applications/${id}/interview-schedule', payload)`, invalidate `["application",id]`.
- **MIRROR**: existing `queries.ts` mutation + invalidation style.
- **VALIDATE**: `tsc`; inbox status pills render new labels.

### Task 14: Tests + validation pass
- **ACTION**: Add/verify tests; run full suites.
- **IMPLEMENT**: `transitions_test.go`, calendar `graph_test.go`, applications handler tests for the machine, interview service auto-advance test.
- **VALIDATE**: see Validation Commands.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| transition allowed | `(ai_interviewed, shortlisted)` | true | no |
| transition denied | `(scored, shortlisted)` | false | yes (must AI-interview first) |
| schedule-only target | `CanTransition(shortlisted, interview)`=true but PATCH rejects | 400 via `RequiresSchedule` | yes |
| reject needs reason | PATCH `{status:rejected}` no reason | 400 | yes |
| reject stores reason | PATCH `{status:rejected, reason:"x"}` | row.rejection_reason="x", no notify | no |
| AI invite guard | Invite when status=`shortlisted` | 409 | yes |
| AI completion advance | session→completed when app=`ai_interview` | app=`ai_interviewed` | no |
| AI completion no-clobber | session→completed when app=`shortlisted` | app unchanged | yes |
| schedule online no email | mode=online, candidate email="" | 400 | yes |
| graph online flow | mock/httptest | event POST + onlineMeetings POST, JoinURL set | no |
| hire → offer | PATCH `{status:offer}` from `interviewed` | status=offer, **no** PS sync | yes |

### Edge Cases Checklist
- [ ] Transition from a terminal `rejected` → all denied
- [ ] `scored` → only `send_ai_interview` exposed
- [ ] Graph 403 (no app-access-policy) → schedule saved, JoinURL empty, logged (not a 500)
- [ ] Candidate with no LINE handle → schedule notify is a no-op (not an error)
- [ ] datetime-local zone conversion correct (Asia/Bangkok)
- [ ] Bulk mixed-status selection → partial `updated`/`failed`
- [ ] Concurrent status change (optimistic) — last-write-wins acceptable for v1

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l internal/ && go vet ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint <changed files>
```
EXPECT: clean.

### Unit Tests
```bash
cd backend && go test -race ./internal/applications/... ./internal/interview/... ./internal/calendar/... ./pkg/config/...
```
EXPECT: all pass.

### Full Suite
```bash
cd backend && go build ./... && go test ./...
cd frontend && pnpm exec next build
```
EXPECT: no regressions; 14 routes build.

### Database Validation
```bash
# local (Makefile): make migrate-up DB_URL=...
migrate -path backend/migrations -database "$DB_URL" up
```
EXPECT: schema → 000019; `applications.rejection_reason` + `interview_appointments` exist.

### Manual Validation
- [ ] scored → only "Send AI interview" visible
- [ ] After AI interview completes → Shortlist/Interview/Reject visible
- [ ] Interview → dialog requires date+mode; online → candidate receives Teams invite email; appointment shows join link
- [ ] interviewed → Hire/Reject; Hire → status `offer`
- [ ] Reject → reason required; reason stored; candidate NOT notified

---

## Acceptance Criteria
- [ ] Every transition in the spec table enforced server-side (PATCH + schedule + bulk)
- [ ] UI shows only allowed actions per status
- [ ] Online interview creates Graph event + Teams link + emails candidate
- [ ] Reject requires + stores a reason, sends nothing to the candidate
- [ ] Hire → `offer`; Offer Package untouched
- [ ] All validation commands pass

## Completion Checklist
- [ ] Backend transition map = frontend `statusMachine.ts` (cross-referenced)
- [ ] Calendar follows the mock/real provider seam; mock default (CI needs no creds)
- [ ] Graph client mirrors `fit/azure.go` + `peoplesoft/rest.go`
- [ ] Error handling + logging match codebase (best-effort side-effects never fail the action)
- [ ] No hardcoded mailbox/tenant — all via `GRAPH_*` config
- [ ] Tests table-driven; ≥80% on new logic

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| App-only `isOnlineMeeting` doesn't mint Teams | High | High | Mint via `POST /onlineMeetings` first, embed joinUrl (per research) |
| Missing Teams Application Access Policy | High (first deploy) | Med | Runbook: `New-/Grant-CsApplicationAccessPolicy` for the service mailbox; tolerate 403 (save schedule, empty link, log); ~30 min propagation |
| PeopleSoft sync deferral changes live hire behavior | Med | Med | Document: PS sync moves to future "offer accepted"; entering `offer` does not sync. Demo data was wiped, so no `hired` rows depend on it now |
| Graph admin consent friction (like Entra consent earlier) | Med | Med | App-only consent is a one-time admin grant in ERT's tenant (Calendars.ReadWrite + OnlineMeetings.ReadWrite) |
| Candidate has no email → no online invite | Med | Low | Validate + 400 with a clear message; HR shares link manually |
| Status vocabulary drift FE/BE | Med | Med | Single transition table mirrored; cross-reference comments; covered by tests |

## Notes
- "Screened" in the spec == existing `scored` (the UI already labels `scored` as "Screened" in `PeopleBits.tsx`). No data migration of existing rows.
- The AI pre-interview (`interview_sessions`) stays a separate lifecycle; this plan only adds two app-status side-effects (invite→`ai_interview`, complete→`ai_interviewed`).
- Time zone: store `scheduled_at` as `TIMESTAMPTZ` (UTC); render in Asia/Bangkok; Graph event uses Windows tz `"SE Asia Standard Time"`.
- Keep `StatusHired` constant for backward compatibility; the funnel terminal action is now `offer`.
