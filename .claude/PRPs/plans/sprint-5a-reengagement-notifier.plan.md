# Plan: Sprint 5a — Re-engagement + Notifier seam

## Summary
Introduce a **Notifier seam** (mock-default, real LINE/email behind config) and a **candidate re-engagement** flow: when a vacancy opens, find matching talent-pool / prior candidates and notify them about the new opening — feeding them back into the existing apply→pipeline. Adds the notification capability the codebase has been deferring to "Sprint 5" (see `pipeline/process.go:1-4` Step 7 note and `peoplesoft/webhook.go:79-80`). The Notifier seam is foundational and reused by **5b** (report delivery).

## User Story
As an **HR operations team**, I want **prior/talent-pool candidates automatically re-contacted when a matching vacancy opens**, so that **we refill roles faster from people who already applied**, without manual outreach.

## Problem → Solution
**Current state:** Talent-pool candidates (`applications.talent_pool = true`) and prior applicants sit idle. The PeopleSoft vacancy-opened webhook (`internal/peoplesoft/webhook.go`) makes a vacancy visible on the portal but notifies no one (`// NOTE: HR LINE notification is Sprint 5`). There is **no notification mechanism at all** in the codebase.
**Desired state:** A `notify.Notifier` seam (mock by default — logs; real LINE-push/email when configured). When a vacancy opens, an asynq job finds matching candidates, sends each a re-engagement message (suppressing repeats), and records an activity entry. HR can also trigger re-engagement for a position on demand.

## Metadata
- **Complexity**: Large (new seam + new domain pkg + worker task + webhook hook + migration; ~14 files)
- **Source PRD**: Nexto PRP v1.0 — Sprint 5 (re-engagement); roadmap §20
- **Decisions locked**: Notifier is **mock-default-behind-config** (mirrors peoplesoft/ai/line seams); re-engagement runs as an **asynq task**; suppression via a `reengagement_contacts` table; event trigger = vacancy-opened webhook + manual admin endpoint
- **Estimated Files**: ~14 (backend) + 1 migration pair
- **Dependents**: 5b (report scheduler) reuses `internal/notify`

---

## UX Design
Internal/backend change — no HR-dashboard UX in 5a beyond one admin action.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Vacancy opens (PS webhook) | portal-only | also enqueues re-engagement of matching candidates | event trigger |
| Re-engage a position | none | `POST /api/v1/positions/:id/reengage` (RBAC-gated) enqueues the job | manual trigger |
| Candidate notified | none | mock logs `[mock notify]`; real sends LINE push / email | behind `NOTIFY_PROVIDER` |
| Audit | none | `activity_logs` row, action `reengage` | reuse activity.Writer |

---

## Mandatory Reading (the contract + patterns to reuse)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/peoplesoft/client.go` | 9-21 | **THE seam pattern**: interface + `NewClient(cfg)` mock-default factory — mirror exactly for `notify.NewNotifier` |
| P0 | `backend/internal/peoplesoft/mock.go` | 9-20 | mock impl shape + structured `log.Info()...Msg("[mock peoplesoft] ...")` to mirror |
| P0 | `backend/pkg/queue/tasks.go` | 11-51 | task-type const + payload + `New*Task` builder + `Parse*Payload` — clone for `TypeReengageVacancy` |
| P0 | `backend/cmd/worker/main.go` | 115-126 | `mux.HandleFunc(queue.Type..., handler)` registration |
| P0 | `backend/internal/applications/service.go` | 15-18, 108-124 | `Enqueuer` interface + enqueue call pattern |
| P0 | `backend/internal/peoplesoft/webhook.go` | 79-80 | the exact spot to enqueue re-engagement on vacancy open |
| P1 | `backend/internal/activity/activity.go` | 15-21, 32-40 | `Writer.Record(...)` + action constants — add `ActionReengage` |
| P1 | `backend/pkg/config/config.go` | 33-53, 75-126 | seam config fields, `getenv`, predicate methods (`UsesRealPeopleSoft`), provider consts |
| P1 | `backend/internal/branch/assigner.go` | 35, 72 | `Assign` / `LocationScore` — province↔subregion matching to reuse for candidate matching |
| P1 | `backend/internal/candidates/repository.go` | 100-128 | SQL query + row-scan + `nullable` helper pattern |
| P1 | `backend/migrations/000005_ps_public.up.sql` | all | migration style (additive, indexes) |
| P2 | `backend/internal/peoplesoft/service.go` | 47-92 | "external failure never fails the core flow" pattern (apply to notify failures) |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| LINE Messaging push | developers.line.biz/en/reference/messaging-api/#send-push-message | real push = `POST https://api.line.me/v2/bot/message/push` with channel access token; mock by default — real is a deploy-time swap behind `notify.NewNotifier`. |
| asynq tasks | github.com/hibiken/asynq | task = type string + JSON payload; register handler on `ServeMux`; reuse existing `MaxRetry/Timeout/Retention` options from `tasks.go`. |

### Research Notes
```
KEY_INSIGHT: No notifier exists anywhere; this sprint creates the seam.
APPLIES_TO: internal/notify (new).
GOTCHA: mirror peoplesoft.NewClient EXACTLY — interface in client.go, mock in mock.go, real in rest/line, factory switches on cfg.UsesRealNotify(). Mock must require zero creds and only log.

KEY_INSIGHT: re-engagement must not spam.
APPLIES_TO: internal/reengage + migration.
GOTCHA: a candidate must not be re-contacted twice for the SAME position. Enforce with a unique(candidate_id, position_id) row in reengagement_contacts; INSERT ... ON CONFLICT DO NOTHING and only notify when the insert created a row.

KEY_INSIGHT: notify failure must never fail the job/hire.
APPLIES_TO: reengage service.
GOTCHA: follow peoplesoft.Service.SyncHired — log a warning on notify error, continue; the job succeeds so asynq doesn't retry-storm. Record the contact row only on successful send (so a failed send can be retried by a later run).
```

---

## Patterns to Mirror

### NOTIFIER_SEAM (mirror peoplesoft/client.go:14-21)
```go
// internal/notify/notify.go
type Notifier interface {
	Send(ctx context.Context, m Message) error
}
type Message struct {
	Channel   string // "line" | "email"
	Recipient string // LINE user id or email
	Subject   string
	Body      string
}
func NewNotifier(cfg *config.Config) Notifier {
	if cfg.UsesRealNotify() {
		return newRESTNotifier(cfg) // LINE push / email
	}
	return mockNotifier{}
}
```

### MOCK_IMPL (mirror peoplesoft/mock.go:9-20)
```go
type mockNotifier struct{}
func (mockNotifier) Send(_ context.Context, m Message) error {
	log.Info().Str("channel", m.Channel).Str("to", m.Recipient).Str("subject", m.Subject).Msg("[mock notify] send")
	return nil
}
```

### QUEUE_TASK (mirror pkg/queue/tasks.go:11-51)
```go
const TypeReengageVacancy = "vacancy:reengage"
type ReengageVacancyPayload struct {
	PositionID string `json:"position_id"`
	Subregion  string `json:"subregion"`
}
func NewReengageVacancyTask(p ReengageVacancyPayload) (*asynq.Task, error) { /* json.Marshal + asynq.NewTask with MaxRetry/Timeout/Retention */ }
func ParseReengageVacancyPayload(body []byte) (ReengageVacancyPayload, error) { /* json.Unmarshal */ }
```

### ENQUEUER (mirror applications/service.go:15-18)
```go
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
```

### REPOSITORY_SQL (mirror candidates/repository.go:100-128)
```go
const q = `SELECT c.id, c.full_name, COALESCE(c.phone,''), COALESCE(c.email,'')
  FROM candidates c
  JOIN applications a ON a.candidate_id = c.id
  WHERE a.position_id = $1 AND (a.talent_pool IS TRUE OR a.status = 'rejected')
  AND NOT EXISTS (SELECT 1 FROM reengagement_contacts rc WHERE rc.candidate_id = c.id AND rc.position_id = $1)`
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/notify/notify.go` | CREATE | `Notifier` interface + `Message` + `NewNotifier(cfg)` factory |
| `backend/internal/notify/mock.go` | CREATE | mock notifier (logs only, default) |
| `backend/internal/notify/rest.go` | CREATE | real LINE-push/email impl (REST over net/http; constructed only when configured) |
| `backend/internal/notify/notify_test.go` | CREATE | mock send + message routing unit tests |
| `backend/internal/reengage/service.go` | CREATE | find matches → send → record contact + activity (notify-fail-safe) |
| `backend/internal/reengage/repository.go` | CREATE | matching-candidate query + `RecordContact` (ON CONFLICT DO NOTHING) |
| `backend/internal/reengage/handler.go` | CREATE | `POST /api/v1/positions/:id/reengage` (RBAC) → enqueue |
| `backend/internal/reengage/service_test.go` | CREATE | suppression + notify-failure-does-not-fail unit tests |
| `backend/pkg/queue/tasks.go` | UPDATE | add `TypeReengageVacancy` + payload + builder + parser |
| `backend/cmd/worker/main.go` | UPDATE | build notifier + reengage svc; `mux.HandleFunc(queue.TypeReengageVacancy, ...)` |
| `backend/cmd/api/main.go` | UPDATE | build notifier + reengage enqueuer; wire webhook trigger + register reengage routes |
| `backend/internal/peoplesoft/webhook.go` | UPDATE | on vacancy open, enqueue `ReengageVacancy` (replace the Sprint-5 NOTE) |
| `backend/pkg/config/config.go` | UPDATE | `NOTIFY_PROVIDER` + LINE/email creds + `UsesRealNotify()` |
| `backend/migrations/0000NN_reengagement.{up,down}.sql` | CREATE | `reengagement_contacts` table (via `make migrate-create name=reengagement`) |

## NOT Building (later / out of scope)
- Real LINE channel-access-token provisioning / email SMTP infra — seam only; mock is default.
- Candidate notification *preferences* / opt-out UI (PDPA opt-out beyond suppression table).
- Bulk campaign console in the dashboard (only the single per-position trigger endpoint).
- Re-ranking / ML match scoring — matching is position + talent-pool/prior-applicant + province↔subregion.
- Scheduled/periodic re-engagement sweep (event + manual trigger only; periodic dispatch arrives with 5b's scheduler).

---

## Step-by-Step Tasks

### Task 1: Config — notify provider toggle
- **ACTION**: Add `NotifyProvider string` (+ `LINEPushToken`, `NotifyEmailFrom` or similar) to `Config`; read `NOTIFY_PROVIDER` (default `"mock"`) via `getenv`; add `func (c *Config) UsesRealNotify() bool { return c.NotifyProvider == ProviderReal }`. Validate creds present when real.
- **MIRROR**: `config.go:33-53` (PS fields/consts) + `:118` predicate.
- **VALIDATE**: `go build ./...`; default config → `UsesRealNotify()==false`.

### Task 2: Notifier seam (notify pkg)
- **ACTION**: Create `notify.go` (interface + Message + factory), `mock.go` (log-only), `rest.go` (real LINE push via `POST https://api.line.me/v2/bot/message/push`; email optional stub). Factory picks mock unless `cfg.UsesRealNotify()`.
- **MIRROR**: NOTIFIER_SEAM + MOCK_IMPL above; `peoplesoft/rest.go:24-38` for the REST client shape.
- **GOTCHA**: real impl constructed only inside the `if cfg.UsesRealNotify()` branch so missing creds never break mock/CI.
- **VALIDATE**: `go test ./internal/notify/...` — mock Send returns nil and logs.

### Task 3: Migration — reengagement_contacts
- **ACTION**: `make migrate-create name=reengagement`. Up: `CREATE TABLE reengagement_contacts (id uuid PK default gen_random_uuid(), candidate_id uuid REFERENCES candidates(id), position_id uuid, channel varchar(20), created_at timestamptz default now(), UNIQUE(candidate_id, position_id));`. Down: drop table.
- **MIRROR**: `migrations/000005_ps_public.up.sql`.
- **GOTCHA**: use `make migrate-create` so the sequence number is assigned correctly (do NOT hardcode `000006` — it may collide with 5b/5c if those merge first).
- **VALIDATE**: `make migrate-up && make migrate-down && make migrate-up`.

### Task 4: reengage repository
- **ACTION**: `repository.go` with `MatchingCandidates(ctx, positionID, subregion) ([]Target, error)` (query in REPOSITORY_SQL) and `RecordContact(ctx, candidateID, positionID, channel) (bool, error)` returning whether a row was inserted (`ON CONFLICT (candidate_id, position_id) DO NOTHING ... RETURNING` → row count).
- **MIRROR**: `candidates/repository.go:100-128`.
- **VALIDATE**: integration test (tag `integration`) — seed talent-pool app, assert one target; second `RecordContact` returns false.

### Task 5: reengage service
- **ACTION**: `service.go` — `Reengage(ctx, positionID, subregion)`: load matches; for each, `RecordContact` first (suppression); only if inserted, build `notify.Message` (link to `/jobs/<id>` on the portal) and `Send`; on send error log warn + continue (do NOT return error for a single failure); `activity.Record(ctx, ActionReengage, "position", positionID, {count})`.
- **MIRROR**: `peoplesoft/service.go:47-92` fail-safe; `activity.go:32-40`.
- **GOTCHA**: record contact BEFORE send to prevent double-send under retry; if send fails, leave the row (accept "at most once" — note in code). Add `ActionReengage = "reengage"` to `activity` consts.
- **VALIDATE**: unit test with a fake notifier + fake repo: suppressed candidate not sent; notify error doesn't fail `Reengage`.

### Task 6: queue task + worker handler
- **ACTION**: Add `TypeReengageVacancy` + payload/builder/parser to `tasks.go`. In `cmd/worker/main.go`, build `notify.NewNotifier(cfg)` + reengage service (needs a pgx pool + repos + activity), register `mux.HandleFunc(queue.TypeReengageVacancy, handler)` where handler parses payload → `svc.Reengage(...)`.
- **MIRROR**: `tasks.go:11-51`; `worker/main.go:115-126`; pool/repo construction already in worker main.
- **VALIDATE**: `go build ./cmd/worker`; integration test enqueues + handles a task → contact rows + activity row.

### Task 7: webhook trigger + admin endpoint + wiring
- **ACTION**: In `cmd/api/main.go`, build the queue client (exists), a small `reengage` enqueuer, register `reengage.RegisterRoutes` (`POST /api/v1/positions/:id/reengage` → enqueue, RBAC: super_admin/regional_director/operation_director). In `peoplesoft/webhook.go`, replace the Sprint-5 NOTE: after the vacancy is opened, enqueue `ReengageVacancy{PositionID, Subregion}`.
- **MIRROR**: route registration (`applications/routes.go:1-12`), RBAC role-gate (`rbac/scope.go:29-38`), `applications/service.go:108-124` enqueue.
- **GOTCHA**: webhook enqueue must be best-effort — a queue error must not fail the webhook (log warn, return 200).
- **VALIDATE**: `curl -XPOST /api/v1/positions/<id>/reengage` → 202/200; worker logs `[mock notify]` for each matched candidate; `reengagement_contacts` rows appear.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| mock notifier send | any Message | nil + log | — |
| reengage suppression | candidate already contacted | not sent again | yes |
| reengage notify failure | notifier returns err | `Reengage` still returns nil | yes |
| matching query | talent_pool + rejected apps for position | only un-contacted matches returned | yes |

### Edge Cases Checklist
- [ ] Position with no matching candidates → no sends, no error
- [ ] Candidate with no phone/email → skip channel gracefully (log), no crash
- [ ] Duplicate vacancy-open webhook → suppression prevents re-send
- [ ] Queue down on webhook → webhook still 200 (best-effort)
- [ ] Notify provider misconfigured (real, missing creds) → startup config error (fail fast)

---

## Validation Commands
### Static + unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Migration round-trip
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
make migrate-up && make migrate-down && make migrate-up
```
### Integration (stack up)
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./internal/reengage/... ./internal/pipeline/...
```
### Manual
- [ ] `POST /api/v1/positions/<id>/reengage` → worker logs `[mock notify] send` per match
- [ ] re-run → no new sends (suppressed); `SELECT count(*) FROM reengagement_contacts` stable
- [ ] PS vacancy-opened webhook → re-engagement enqueued (worker log)

## Acceptance Criteria
- [ ] `notify.Notifier` seam: mock default (zero creds), real behind `NOTIFY_PROVIDER=real`.
- [ ] Vacancy-open webhook **and** `POST /positions/:id/reengage` enqueue re-engagement.
- [ ] Matching talent-pool/prior candidates are notified once per position (suppression enforced).
- [ ] Notify failures never fail the job; an `activity_logs` `reengage` row is recorded.
- [ ] vet/lint/`go test -race` pass; migration round-trips.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Spamming candidates | Med | High | unique(candidate_id, position_id) suppression; record-before-send |
| Notify outage fails jobs | Med | Med | fail-safe (log + continue), mirror peoplesoft.Service |
| Real LINE push diverges from mock | Med | Med | isolate behind `notify` seam; mock mirrors the call; real is deploy-time |
| Double webhook delivery | Med | Low | suppression makes re-engagement idempotent |

## Notes
- The `notify` seam is the dependency for **5b** (report delivery). Land 5a first, or 5b can stub a minimal notifier and swap later.
- Matching is intentionally simple (position + talent-pool/rejected + province↔subregion via `branch.LocationScore` semantics). Smarter matching is a later enhancement.
