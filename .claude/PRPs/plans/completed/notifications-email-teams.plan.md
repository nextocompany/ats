# Plan: Notifications — Email (candidate + HR) + MS Teams (HR)

## Summary
Extend the existing outbound-notification seam so the system delivers **email** (to
candidates on status changes, and to HR/hiring managers on key events) and **MS
Teams** messages (to an HR channel). LINE-to-candidate stays exactly as-is. The
`notify.Notifier` seam and a real ACS email sender (`pkg/email`) already exist — this
plan wires them together, adds a Teams channel, and adds the HR-recipient/event
triggers.

## User Story
As an **HR/hiring manager**, I want to be notified by email and Microsoft Teams when
a relevant candidate event happens (new scored applicant for my store, interview
feedback recorded), and as a **candidate** I want status updates by email (not only
LINE), so that no one misses a recruitment event regardless of channel.

## Problem → Solution
Today only LINE-to-candidate notifications are live; the email channel is a
hard-coded error stub (`internal/notify/rest.go:40`) and there is no HR-facing
notification at all. → Email becomes a real channel (reusing the ACS sender already
used for OTP), candidates also receive email on the notifiable status set, and HR
gets email + Teams pings on defined events.

## Metadata
- **Complexity**: Medium
- **Source PRD**: `.claude/PRPs/plans/delivery-scope-roadmap.md` (PRP-1)
- **PRD Phase**: PRP-1 (P0)
- **Estimated Files**: ~12 (backend only; no frontend)

---

## UX Design

### Before
```
status change → LINE push to candidate (only, and only if LINE handle exists)
HR             → no notification ever
```

### After
```
status change (shortlisted/interview/hired)
   → LINE push to candidate    (if LINE handle)   [unchanged]
   → Email to candidate        (if email present) [new]
new application scored+assigned to store
   → Email to store HR users   [new]
   → MS Teams message to HR channel [new]
interview feedback recorded
   → Email to store HR users + Teams channel [new]
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Candidate status update | LINE only | LINE + Email | Both best-effort; rejection still NOT pushed (policy) |
| HR awareness | none | Email + Teams on scored+assigned / feedback | Store-scoped recipients; Teams = single channel webhook |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/notify/notify.go` | all | Notifier seam, Channel consts, Message struct, factory — the contract to extend |
| P0 | `backend/internal/notify/rest.go` | all | restNotifier; email stub at L40 to replace; LINE pattern to mirror for Teams HTTP |
| P0 | `backend/internal/notify/message.go` | all | StatusMessage + statusBody — mirror for email/HR message builders |
| P0 | `backend/pkg/email/email.go` | all | `email.Sender` seam (ACS real / mock) to reuse for the Email channel |
| P0 | `backend/internal/applications/notify.go` | all | statusNotifyDeps.notifyStatusChange — the candidate dispatch to extend |
| P1 | `backend/pkg/email/acs_sender.go` | all | Real ACS send (shared-key HMAC); confirms Message→provider mapping |
| P1 | `backend/pkg/config/config.go` | 91-95, 113-125, 230-253, 299-355, 404-418 | Notify/email/teams env vars + validate() + UsesRealX helpers — add Teams here |
| P1 | `backend/internal/applications/feedback_handler.go` | all | Where the "feedback recorded" HR trigger hooks in |
| P1 | `backend/internal/pipeline/process.go` | 87-272 | Pipeline end (scored+assigned) — where the "new scored applicant" HR trigger hooks in |
| P2 | `backend/internal/candidates/*.go` | — | Candidate.Email + Candidate.FullName fields used as the email recipient |
| P2 | `backend/migrations/000001_init_schema.up.sql` | 33-45 | users table (email, role, store_id) — HR recipient lookup source |
| P2 | `backend/cmd/api/main.go` | 197-294 | Where notifier is built + SetNotifier wired; mirror for new deps |
| P2 | `backend/cmd/worker/main.go` | all | Worker wiring — needs a notifier for the pipeline HR trigger |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Teams Incoming Webhook | Microsoft Learn: "Create an Incoming Webhook" | POST JSON `{"text": "..."}` (or MessageCard/Adaptive Card) to the webhook URL; no auth header, the URL is the secret. Simplest reliable HR channel — no Graph perms. |
| Teams MessageCard | Legacy actionable message card | For richer formatting use `{"@type":"MessageCard","@context":"...","summary":"...","sections":[...]}`; plain `text` is fine for v1. |
| ACS Email REST | Microsoft Learn: ACS Email send | Already implemented in `pkg/email/acs_sender.go` (shared-key HMAC). Reuse — do not re-implement. |

> GOTCHA: O365 connector webhooks are being retired in favor of "Workflows" (Power
> Automate) webhooks, but existing Incoming Webhook URLs still post the same JSON
> shape. Keep the channel behind `TEAMS_WEBHOOK_URL` so the URL can be swapped
> without code changes. If the org blocks connectors, fall back to a Power Automate
> "When a webhook request is received" flow — same POST contract.

---

## Patterns to Mirror

### CHANNEL_SEAM
```go
// SOURCE: backend/internal/notify/notify.go
const (
	ChannelLINE  = "line"
	ChannelEmail = "email"
)
type Message struct {
	Channel   string
	Recipient string
	Subject   string
	Body      string
}
type Notifier interface { Send(ctx context.Context, m Message) error }
func NewNotifier(cfg *config.Config) Notifier {
	if cfg.UsesRealNotify() { return newRESTNotifier(cfg) }
	return mockNotifier{}
}
```

### HTTP_POST_PATTERN (mirror for Teams webhook)
```go
// SOURCE: backend/internal/notify/rest.go  (sendLINE)
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
resp, err := n.http.Do(req)
defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
if resp.StatusCode >= 300 { return fmt.Errorf("notify: ... status %d", resp.StatusCode) }
```

### EMAIL_SENDER_SEAM (reuse — do NOT re-implement ACS)
```go
// SOURCE: backend/pkg/email/email.go
type Message struct { To, Subject, PlainText, HTML string }
type Sender interface { Send(ctx context.Context, m Message) error }
func NewSender(cfg *config.Config) Sender {
	if cfg.UsesRealEmail() { return newACSSender(cfg.ACSEmailEndpoint, cfg.ACSEmailAccessKey, cfg.ACSEmailSender) }
	return mockSender{logBody: cfg.IsDevelopment()}
}
```

### MESSAGE_BUILDER (mirror for email + HR variants)
```go
// SOURCE: backend/internal/notify/message.go
func StatusMessage(lineUserID, fullName, status, portalBaseURL string) Message {
	if lineUserID == "" { return Message{} }      // empty Recipient → caller skips
	body, ok := statusBody(fullName, status, portalBaseURL)
	if !ok { return Message{} }
	return Message{Channel: ChannelLINE, Recipient: lineUserID, Subject: "อัปเดตสถานะใบสมัคร", Body: body}
}
// statusBody covers shortlisted/interview/hired only; default → ("", false)
```

### BEST_EFFORT_DISPATCH (mirror for the extended candidate + HR dispatch)
```go
// SOURCE: backend/internal/applications/notify.go
func (d statusNotifyDeps) notifyStatusChange(ctx context.Context, apps Repository, appID uuid.UUID, status string) {
	if d.notifier == nil || d.cands == nil { return }
	// ...load app + candidate...
	msg := notify.StatusMessage(cand.LineUserID, cand.FullName, status, d.portalBaseURL)
	if msg.Recipient == "" { return }
	if err := d.notifier.Send(ctx, msg); err != nil {
		log.Warn().Err(err).Str("application", appID.String()).Msg("status notify: send failed (non-fatal)")
	}
}
```

### CONFIG_VAR + VALIDATE (mirror for TEAMS_WEBHOOK_URL)
```go
// SOURCE: backend/pkg/config/config.go
NotifyProvider:  getenv("NOTIFY_PROVIDER", "mock"),
NotifyLINEToken: os.Getenv("NOTIFY_LINE_TOKEN"),
// validate(): {"NOTIFY_PROVIDER", c.NotifyProvider, []string{"mock", ProviderReal}},
func (c *Config) UsesRealNotify() bool { return c.NotifyProvider == ProviderReal }
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/notify/rest.go` | UPDATE | Replace email stub with `email.Sender`; add `sendTeams` + Teams channel |
| `backend/internal/notify/notify.go` | UPDATE | Add `ChannelTeams`; `NewNotifier` constructs the email sender + teams webhook |
| `backend/internal/notify/message.go` | UPDATE | Add `StatusEmailMessage`; add HR message builders (email + teams) |
| `backend/internal/notify/teams.go` | CREATE | Teams webhook sender (mirrors LINE HTTP pattern) |
| `backend/internal/notify/hr_message.go` | CREATE | HR-facing message builders (new-scored / feedback-recorded) |
| `backend/internal/notify/*_test.go` | UPDATE/CREATE | Cover email mapping, teams payload, builder gating |
| `backend/pkg/config/config.go` | UPDATE | Add `TeamsWebhookURL` env + (optional) validate |
| `backend/internal/applications/notify.go` | UPDATE | Extend candidate dispatch to also send email; add HR dispatch helper |
| `backend/internal/applications/feedback_handler.go` | UPDATE | Fire HR notify after a feedback row is created (best-effort) |
| `backend/internal/applications/hr_directory.go` | CREATE | `EmailsForStore` lookup (active sgm/hr_manager/hr_staff in a store) |
| `backend/internal/pipeline/process.go` | UPDATE | Fire HR notify after scored+assigned (best-effort) |
| `backend/cmd/api/main.go` + `cmd/worker/main.go` | UPDATE | Build email sender + teams into notifier; wire HR directory + notifier into pipeline |

## NOT Building
- No new frontend/UI (notifications are server-side only).
- No per-user Teams direct messages via Graph (HR Teams = single **channel webhook**; Graph chat is a future option).
- No candidate SMS, no push web notifications.
- No notification-preferences/opt-out management UI.
- No change to LINE behavior or the candidate-notifiable status set (still shortlisted/interview/hired; rejection never pushed).
- No retry/queue for notifications beyond best-effort (failures logged, never block the action) — matches existing seam.

---

## Step-by-Step Tasks

### Task 1: Add Teams config
- **ACTION**: Add `TeamsWebhookURL string` to `Config`; load `getenv`/`os.Getenv("TEAMS_WEBHOOK_URL", "")`.
- **IMPLEMENT**: Field near the notify block (config.go:91-95); load near line 230-232. Empty = Teams disabled (no validation error — optional channel).
- **MIRROR**: CONFIG_VAR pattern.
- **IMPORTS**: none new.
- **GOTCHA**: Do NOT make it required under `UsesRealNotify` — Teams is independent of LINE; gate sends on `url != ""`.
- **VALIDATE**: `go build ./...`; add the var to `.env.example` if present.

### Task 2: Teams sender
- **ACTION**: Create `internal/notify/teams.go` with a function posting `{"text": body}` to the webhook URL.
- **IMPLEMENT**: `func sendTeams(ctx, httpClient, webhookURL, body string) error` mirroring `sendLINE` (POST JSON, drain+close, status>=300 → error). No auth header.
- **MIRROR**: HTTP_POST_PATTERN.
- **IMPORTS**: `bytes`, `context`, `encoding/json`, `fmt`, `io`, `net/http`.
- **GOTCHA**: Teams returns 200 with body `1` on success; treat any 2xx as success.
- **VALIDATE**: unit test with `httptest.Server` asserting the posted JSON has `text`.

### Task 3: Wire Email + Teams into restNotifier
- **ACTION**: Give `restNotifier` an `email email.Sender` and `teamsWebhook string`; route `ChannelEmail` → `email.Send` (map fields), `ChannelTeams` → `sendTeams`.
- **IMPLEMENT**: In `newRESTNotifier(cfg)` set `email: email.NewSender(cfg)`, `teamsWebhook: cfg.TeamsWebhookURL`. In `Send`, replace the email error stub with a real map: `email.Message{To: m.Recipient, Subject: m.Subject, PlainText: m.Body}`. Add `case ChannelTeams: if n.teamsWebhook == "" { return fmt.Errorf("notify: teams webhook not configured") }; return sendTeams(ctx, n.http, n.teamsWebhook, m.Body)`.
- **MIRROR**: CHANNEL_SEAM, EMAIL_SENDER_SEAM.
- **IMPORTS**: `github.com/nexto/hr-ats/pkg/email`.
- **GOTCHA**: `email.NewSender` returns the **mock** sender when `EMAIL_PROVIDER!=real`, so the channel is safe in CI/local (logs only) — no stub error anymore. Add `ChannelTeams = "teams"` to notify.go consts.
- **VALIDATE**: `go test ./internal/notify/...`.

### Task 4: Candidate email message builder
- **ACTION**: Add `StatusEmailMessage(emailAddr, fullName, status, portalBaseURL) Message` returning a `ChannelEmail` message reusing `statusBody`.
- **IMPLEMENT**: Same gating as `StatusMessage` (empty addr or non-notifiable status → zero Message). Subject `"อัปเดตสถานะใบสมัคร"`. Body = `statusBody(...)` text (PlainText).
- **MIRROR**: MESSAGE_BUILDER.
- **IMPORTS**: none new.
- **GOTCHA**: Reuse `statusBody` so LINE and email never drift; do NOT duplicate the copy.
- **VALIDATE**: table test: notifiable status + addr → non-empty; rejected/empty → zero.

### Task 5: Extend candidate dispatch to email
- **ACTION**: In `statusNotifyDeps.notifyStatusChange`, after the LINE send, also build+send the email when `cand.Email != ""`.
- **IMPLEMENT**: `if em := notify.StatusEmailMessage(cand.Email, cand.FullName, status, d.portalBaseURL); em.Recipient != "" { if err := d.notifier.Send(ctx, em); err != nil { log.Warn()... } }`. Keep both sends best-effort and independent.
- **MIRROR**: BEST_EFFORT_DISPATCH.
- **IMPORTS**: none new.
- **GOTCHA**: Don't early-return after LINE; both channels are independent. A candidate with no LINE handle but an email must still get the email.
- **VALIDATE**: extend notify_test for the dispatch (fake notifier records channels sent).

### Task 6: HR directory lookup
- **ACTION**: Create `internal/applications/hr_directory.go` with `EmailsForStore(ctx, storeID *int) ([]string, error)` returning active `sgm/hr_manager/hr_staff` emails for that store (and store-less all-scope roles optional — keep store-scoped for v1).
- **IMPLEMENT**: pgx query `SELECT email FROM users WHERE is_active AND store_id = $1 AND role = ANY($2) AND email <> ''`. Expose via a narrow interface `HRDirectory` so callers accept the interface.
- **MIRROR**: repository.go query style (`internal/applications/repository.go`).
- **IMPORTS**: `pgxpool`, `context`.
- **GOTCHA**: `store_id` nullable — when the application has no `assigned_store_id` (talent pool), skip HR notify (return empty). Don't email all stores.
- **VALIDATE**: covered indirectly by the trigger tests (mock directory) + a manual SQL check.

### Task 7: HR message builders
- **ACTION**: Create `internal/notify/hr_message.go` with `NewScoredHRMessages(...)` and `FeedbackRecordedHRMessages(...)` returning the email Message(s) + a Teams Message.
- **IMPLEMENT**: e.g. `func NewScoredHR(toEmails []string, teamsEnabled bool, candName, positionTitle string, score int, storeName, dashURL string) []Message` — one `ChannelEmail` Message per address + one `ChannelTeams` Message (Recipient empty for teams; the webhook is the target). Thai copy, link to the dashboard application.
- **MIRROR**: MESSAGE_BUILDER.
- **IMPORTS**: `fmt`.
- **GOTCHA**: Teams Message needs no Recipient (channel webhook is the destination); the restNotifier ignores Recipient for ChannelTeams.
- **VALIDATE**: builder unit tests (recipient count, channel set, body contains name+score).

### Task 8: Fire HR notify on scored+assigned (pipeline)
- **ACTION**: After `persistScore` + assignment in `pipeline/process.go`, best-effort notify HR.
- **IMPLEMENT**: Inject an optional `notifier notify.Notifier`, `hrDir HRDirectory`, `dashBaseURL`, `teamsEnabled` into the pipeline processor (nil = no-op, like the existing seams). Resolve emails for `assigned_store_id`; build + send each Message; log failures, never fail the job.
- **MIRROR**: BEST_EFFORT_DISPATCH; the optional-deps style of `statusNotifyDeps`.
- **IMPORTS**: notify, the HRDirectory interface.
- **GOTCHA**: This runs in the **worker**, not the api — wire the notifier in `cmd/worker/main.go` (currently the worker may not build a notifier). Only notify when status ended as `scored` (passed gate); skip rejected/failed/talent-pool.
- **VALIDATE**: pipeline test with a fake notifier asserts N HR messages on a scored+assigned fixture, 0 on rejected.

### Task 9: Fire HR notify on feedback recorded
- **ACTION**: In `FeedbackHandler.Create`, after `CreateFeedback` succeeds, best-effort notify HR (email + Teams).
- **IMPLEMENT**: Add optional notify deps to `FeedbackHandler` (SetNotifier mirror). Resolve store emails from the application's `assigned_store_id`; build `FeedbackRecordedHR(...)`; send best-effort.
- **MIRROR**: `Handler.SetNotifier` (`internal/applications/handler.go:44`).
- **IMPORTS**: notify, HRDirectory.
- **GOTCHA**: Must not change the 201 response or fail the write if notify errors. Skip when no assigned store.
- **VALIDATE**: handler test: happy-path create still 201 with a failing notifier.

### Task 10: Wire everything in main.go + worker main.go
- **ACTION**: Build `notify.NewNotifier(cfg)` (now email+teams capable), construct `HRDirectory` from the pool, pass into the pipeline (worker) and feedback handler (api).
- **IMPLEMENT**: api: `feedbackHandler.SetNotifier(notifier, hrDir, dashBaseURL, cfg.TeamsWebhookURL != "")`. worker: build notifier + hrDir, inject into the pipeline processor constructor.
- **MIRROR**: existing `SetNotifier` wiring at `cmd/api/main.go:199, 286, 292`.
- **IMPORTS**: as needed.
- **GOTCHA**: dashboard base URL — check if a `DASHBOARD_BASE_URL`/equivalent exists in config; if not, add it (HR links point to the dashboard, not the portal). Portal base URL is candidate-facing.
- **VALIDATE**: `go build ./...` (api + worker), `go vet ./...`.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| Teams sender posts JSON | httptest server | body has `text`, 2xx → nil err | yes (status>=300 → err) |
| Email channel maps fields | notify.Message email | email.Sender receives To/Subject/PlainText | — |
| StatusEmailMessage gating | status=rejected | zero Message | yes |
| StatusEmailMessage gating | status=interview, addr set | ChannelEmail message | — |
| Candidate dispatch both channels | cand with email + line | LINE + Email both sent | yes (email-only / line-only) |
| HR builder | toEmails len 2 + teams | 2 email + 1 teams Message | yes (0 emails → only teams) |
| Pipeline HR trigger | scored+assigned fixture | N HR messages | yes (rejected/talent-pool → 0) |
| Feedback HR trigger | create feedback | still 201 with failing notifier | yes |

### Edge Cases Checklist
- [ ] Candidate has email but no LINE handle (email still sent)
- [ ] Candidate has LINE but no email (LINE only, no email error surfaced)
- [ ] Application in talent pool / no assigned store (HR notify skipped)
- [ ] `TEAMS_WEBHOOK_URL` empty (Teams skipped, no error bubbles to action)
- [ ] `EMAIL_PROVIDER=mock` (email logs only, channel still "succeeds")
- [ ] Notifier Send returns error (action/job still succeeds; warn logged)
- [ ] Concurrent: pipeline + status change firing notifications simultaneously (no shared mutable state — each builds its own Message)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./...
```
EXPECT: no errors

### Unit Tests
```bash
cd backend && go test ./internal/notify/... ./internal/applications/... ./internal/pipeline/...
```
EXPECT: all pass

### Full build (api + worker)
```bash
cd backend && go build ./...
```
EXPECT: builds clean

### Manual Validation (prod/staging, after deploy)
- [ ] Set `EMAIL_PROVIDER=real` (ACS already provisioned for OTP — reuse `ACS_EMAIL_*`) and `NOTIFY_PROVIDER=real`; set `TEAMS_WEBHOOK_URL` to a test channel webhook.
- [ ] Move a test application → interview: candidate receives BOTH LINE and email.
- [ ] Upload+score a CV assigned to a store with HR users: those HR emails + the Teams channel get the "new scored applicant" message.
- [ ] Record interview feedback: store HR email + Teams channel notified.
- [ ] Empty `TEAMS_WEBHOOK_URL`: everything else still works, no errors.

---

## Acceptance Criteria
- [ ] Email channel sends via ACS (real) / logs (mock) — no more stub error
- [ ] Candidates receive email on shortlisted/interview/hired (rejection excluded)
- [ ] HR receive email + Teams on new-scored-assigned and feedback-recorded
- [ ] Teams channel gated on `TEAMS_WEBHOOK_URL`; all channels best-effort (never break the action/job)
- [ ] `go vet` + tests + build green

## Completion Checklist
- [ ] Reuses `pkg/email.Sender` (no re-implemented ACS)
- [ ] `statusBody` shared between LINE + email (no copy drift)
- [ ] Optional-deps pattern (nil → no-op) for tests/CI
- [ ] Best-effort dispatch with `log.Warn` on failure (matches seam)
- [ ] No new frontend; no LINE behavior change
- [ ] Store-scoped HR recipients (no tenant-wide email blast)

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Teams connector webhooks retiring | Med | Med | Behind `TEAMS_WEBHOOK_URL`; same POST contract works with Power Automate flow |
| Email blast to wrong/too many HR | Low | Med | Store-scoped query, `is_active`, exclude empty email, talent-pool skip |
| Worker has no notifier today | Med | Low | Wire notifier+hrDir in `cmd/worker/main.go`; nil-safe optional deps |
| Candidate email spam / PDPA | Low | Med | Same notifiable set as LINE; rejection excluded; only consented candidates have email on file |
| ACS sender rate/quota | Low | Low | Best-effort; failures logged, retried next event |

## Notes
- The candidate-notifiable status set is intentionally unchanged (shortlisted/
  interview/hired). If stakeholders want an "application received" email, add it as a
  follow-up (new statusBody case + dispatch on intake).
- HR Teams uses a single channel **Incoming Webhook** (URL is the secret) — simplest
  and most reliable. Per-user Graph chat messages are deliberately out of scope.
- A `DASHBOARD_BASE_URL` config may need adding (HR links target the dashboard;
  `PortalBaseURL` is candidate-facing) — verify in config.go during Task 10.
```
