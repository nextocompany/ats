# Plan: AI Pre-Interview (HR-Human Conversational Screening)

## Summary
Add an AI-conducted conversational screening interview as the next step after resume AI-scoring. HR invites a shortlisted candidate from the dashboard; the candidate completes an adaptive ~5–8 turn text chat (Thai/English) with an AI that behaves like an HR interviewer, grounded in the position JD. The AI produces a transcript + structured evaluation (interview score, recommendation, strengths, concerns) that HR reviews in the dashboard — HR still makes the actual hiring decision (no auto-advance).

## User Story
As an **HR recruiter**, I want to send shortlisted candidates an AI pre-interview and read an AI-generated evaluation of their answers, so that I can screen more candidates faster while keeping the final decision in human hands.

(Secondary) As a **candidate**, I want to answer a few conversational questions from an AI recruiter on my phone, so that I can advance in the process without scheduling a live call.

## Problem → Solution
**Current state:** After the resume pipeline scores an application, HR can only set status (shortlist/interview/hire/reject) from the AI *resume* summary. There is no structured signal beyond the CV. → **Desired state:** HR triggers an AI pre-interview per candidate; the candidate chats with an AI HR interviewer; HR gets a second, conversation-based evaluation alongside the resume score before deciding.

## Metadata
- **Complexity**: Large
- **Source PRD**: N/A (free-form feature request)
- **PRD Phase**: N/A — standalone slice (suggest "Slice 2.5 — AI Pre-Interview")
- **Estimated Files**: ~28 (≈14 backend create, ≈4 backend edit, ≈6 portal, ≈6 dashboard, 1 migration)

---

## UX Design

### Before
```
HR Dashboard — candidate detail
┌───────────────────────────────────────────┐
│ Resume (iframe)     │ AI summary           │
│                     │  Score 78  Strong fit│
│                     │  จุดแข็ง / ข้อสังเกต   │
│                     │ [Shortlist][Interview]│
│                     │ [Hire]    [Reject]   │
└───────────────────────────────────────────┘
Candidate: applied → got status token → /status page. No interaction.
```

### After
```
HR Dashboard — candidate detail
┌───────────────────────────────────────────┐
│ Resume (iframe)  │ AI summary              │
│                  │  Score 78  Strong fit   │
│                  │  [Shortlist][Interview] │
│                  │  [Hire][Reject]         │
│                  │  ▶ Send AI Interview     │  ← new
│                  │  ─────────────────────  │
│                  │  AI Interview  ✓ done    │  ← new InterviewPanel
│                  │   Score 81 · Recommend ✓ │
│                  │   จุดแข็ง / ข้อสังเกต      │
│                  │   ▸ transcript (8 turns) │
└───────────────────────────────────────────┘

Candidate (career portal) — /interview?token=…
┌─────────────────────────────┐
│  AI HR สัมภาษณ์เบื้องต้น        │
│  🤖 เล่าประสบการณ์ขายหน้าร้าน… │
│             คุณ: …(typed)    │
│  🤖 แล้วเคยรับมือลูกค้าโกรธ…?  │
│  [ พิมพ์คำตอบ…        ] [ส่ง] │
│  …→ ✅ ขอบคุณค่ะ เสร็จสิ้นแล้ว │
└─────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Dashboard detail | status buttons only | + "Send AI Interview" button + InterviewPanel | Invite is idempotent (re-invite reuses session) |
| Candidate notify | LINE status message on shortlist/interview/hired | + interview invitation message with `/interview?token=` link | Reuses `notify.Notifier` seam; no-op in mock |
| Career portal | apply + status pages | + `/interview` chat page (token-gated, no login) | Token is the only credential, like the status token |
| Application status | scored→shortlisted→interview→hired | unchanged enum; interview tracked on its own session | No new `applications.status` values (keeps `allowedStatuses` untouched) |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/ai/factory.go` | 1-19 | Provider-seam factory pattern to mirror for `interview.New` |
| P0 | `backend/internal/scoring/azure.go` | 1-116 | EXACT Azure OpenAI chat-completions call (chatMessage array, json_object, headers, URL) to mirror for the interviewer + evaluator |
| P0 | `backend/internal/notify/message.go` | 1-47 | Message-builder pattern (primitives in, `Message` out, empty-Recipient = skip) to add `InterviewInviteMessage` |
| P0 | `backend/internal/applications/notify.go` | 1-47 | Best-effort notify wiring (`statusNotifyDeps`, never returns error) to mirror for interview invite |
| P0 | `backend/internal/public/routes.go` | 1-12 | Public route group `/api/v1/public/*` shape |
| P0 | `backend/internal/applications/handler.go` | 143-187 | Status handler + `httpx.OK` envelope + uuid parse + fiber.NewError patterns |
| P0 | `backend/internal/applications/repository.go` | 14-34, 199-220 | Repository interface shape + `FindByPublicToken`/`SetPublicToken` token pattern to mirror |
| P1 | `backend/pkg/config/config.go` | 124-172, 246-306 | `getenv*` helpers, provider predicates (`UsesAzureAI`), `PortalBaseURL`, `AzureOpenAIDeployment` defaults |
| P1 | `backend/internal/applications/model.go` | 1-82 | Status consts, model + pre-serialized `Score` write pattern, JSONB readback |
| P1 | `backend/internal/scoring/scoring_test.go` | 1-100 | Table-driven test + mock LLM + fixture-builder pattern |
| P1 | `backend/cmd/api/main.go` | ~100-150 | Where repos/services/handlers are constructed, auth middleware, route registration, queue client |
| P0 | `career-portal/lib/api.ts` | 1-47 | Envelope client; needs a new JSON `post` method (only `get`+`postForm` today) |
| P0 | `career-portal/lib/queries.ts` | 1-58 | TanStack query/mutation hook conventions to mirror |
| P1 | `career-portal/components/ApplyStepper.tsx` | all | Client component + mutation + success-screen structure to mirror for chat |
| P1 | `career-portal/app/globals.css` | 1-100 | CP Axtra tokens (blue `#0B47B8`, yellow `#FFC02E`, fonts) for chat styling |
| P0 | `frontend/lib/queries.ts` | 22-60, 129-147 | `useApplication`, `useSetStatus`, `useBulk` patterns to mirror for `useInterview`/`useInviteInterview` |
| P0 | `frontend/components/resume/AiSummaryPanel.tsx` | 1-124 | Where to add the "Send AI Interview" button; score-hero/tone pattern to reuse in InterviewPanel |
| P1 | `frontend/components/resume/ScoreBreakdown.tsx` | 1-95 | Strength/red-flag bullet rendering to mirror for strengths/concerns |
| P1 | `frontend/app/(app)/applications/[id]/page.tsx` | 1-51 | Detail page layout where InterviewPanel mounts |
| P1 | `frontend/lib/api.ts` | 1-52 | `api.get/post/patch` + Entra bearer header |
| P1 | `frontend/lib/types.ts` | 19-71 | Where to add `InterviewSession`/`InterviewTurn` types |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Azure OpenAI chat completions | Already encoded in `scoring/azure.go` | API version `2024-08-01-preview`; header `api-key`; URL `{endpoint}/openai/deployments/{deployment}/chat/completions?api-version=…`; `response_format:{type:"json_object"}` for the evaluation call only (the conversational turns return free text). **No external research needed — established internal pattern.** |
| Multi-turn chat | `scoring/azure.go` `chatMessage[]` | Pass full conversation history as `messages` each turn (system + alternating user/assistant). Stateless server, history persisted in DB. |

> No new external libraries. Feature uses established internal patterns (Fiber, pgx, asynq not needed — interview is synchronous, TanStack Query, Azure OpenAI REST).

---

## Patterns to Mirror

### PROVIDER_SEAM_FACTORY
```go
// SOURCE: backend/internal/ai/factory.go:8-18
func New(cfg *config.Config) (OCR, Parser) {
	if cfg.UsesGeminiAI() { ... }
	if cfg.UsesAzureAI() {
		return NewAzureOCR(...), NewAzureParser(cfg.AzureOpenAIEndpoint, cfg.AzureOpenAIKey, cfg.AzureOpenAIDeployment)
	}
	return NewMockOCR(), NewMockParser()   // mock default — local/CI need no creds
}
```
→ `interview.New(cfg) Interviewer`: `if cfg.UsesAzureAI() { return newAzureInterviewer(cfg) }; return mockInterviewer{}`.

### AZURE_CHAT_CALL
```go
// SOURCE: backend/internal/scoring/azure.go:43-115
type chatMessage struct { Role, Content string `json:"..."` }
type chatRequest struct {
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format"`
}
url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.endpoint, a.deployment, openAIAPIVersion)
req.Header.Set("api-key", a.key)
req.Header.Set("Content-Type", "application/json")
// non-200 → fmt.Errorf("...: status %d: %s", resp.StatusCode, raw); 0 choices → error
```
→ Two methods: `NextTurn` (free-text reply, NO `response_format`) and `Evaluate` (`response_format:{type:"json_object"}`, parse into struct like `llmJSON`). 60s `http.Client` timeout.

### MESSAGE_BUILDER
```go
// SOURCE: backend/internal/notify/message.go:15-29
func StatusMessage(lineUserID, fullName, status, portalBaseURL string) Message {
	if lineUserID == "" { return Message{} }       // empty Recipient = caller skips
	body, ok := statusBody(fullName, status, portalBaseURL)
	if !ok { return Message{} }
	return Message{Channel: ChannelLINE, Recipient: lineUserID, Subject: "...", Body: body}
}
```
→ `InterviewInviteMessage(lineUserID, fullName, portalBaseURL, token string) Message` — body: greeting + `… เชิญทำสัมภาษณ์ AI เบื้องต้น %s/interview?token=%s`.

### BEST_EFFORT_NOTIFY
```go
// SOURCE: backend/internal/applications/notify.go:26-47
func (d statusNotifyDeps) notifyStatusChange(ctx, apps, appID, status) {
	if d.notifier == nil || d.cands == nil { return }   // no-op when unset
	app, err := apps.FindByID(...) ; cand, err := d.cands.FindByID(...)
	msg := notify.StatusMessage(cand.LineUserID, cand.FullName, status, d.portalBaseURL)
	if msg.Recipient == "" { return }
	if err := d.notifier.Send(ctx, msg); err != nil { log.Warn()... }  // never returns err
}
```
→ Interview service sends invite the same way: best-effort, log-and-continue, never fail the HR action.

### TOKEN_LOOKUP
```go
// SOURCE: backend/internal/applications/repository.go:199-215
const q = `UPDATE applications SET public_token = $2, updated_at = NOW() WHERE id = $1`
func (r *pgRepository) FindByPublicToken(ctx, token) (*Application, error) {
	... `SELECT ... FROM applications WHERE public_token = $1`
}
// SOURCE: backend/internal/public/handler.go:165-166
func (h *Handler) Status(c *fiber.Ctx) error {
	app, err := h.apps.FindByPublicToken(c.UserContext(), c.Params("token"))
```
→ `interview_sessions.access_token` (crypto-random, unique); `FindByToken(ctx, token)`. Generate token like the existing public_token generator (find with `grep -rn "public_token" backend/internal/applications/service.go`).

### HANDLER_ENVELOPE
```go
// SOURCE: backend/internal/applications/handler.go:156-187
id, err := uuid.Parse(c.Params("id")); if err != nil { return fiber.NewError(fiber.StatusBadRequest, "...") }
var req updateStatusReq; if err := c.BodyParser(&req); err != nil { return fiber.NewError(400, "invalid payload") }
return httpx.OK(c, fiber.Map{"id": id, "status": req.Status})
```
→ All interview handlers: parse → service call → `httpx.OK(c, payload)`; errors via `fiber.NewError`.

### DASHBOARD_QUERY_HOOKS
```ts
// SOURCE: frontend/lib/queries.ts:44-50, 129-138
export function useApplication(id: string) {
  return useQuery({ queryKey: ["application", id],
    queryFn: () => api.get<Application>(`/api/v1/applications/${id}`).then(r => r.data), enabled: !!id });
}
export function useSetStatus(id: string) {
  const qc = useQueryClient();
  return useMutation({ mutationFn: (status: string) => api.patch(`/api/v1/applications/${id}/status`, { status }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["application", id] }); qc.invalidateQueries({ queryKey: ["applications"] }); } });
}
```
→ `useInterview(id)` (GET `/applications/:id/interview`, enabled:!!id) + `useInviteInterview(id)` (POST `/applications/:id/interview`, invalidate `["interview", id]`).

### PORTAL_QUERY_HOOKS
```ts
// SOURCE: career-portal/lib/queries.ts:39-58
export function useApplyMutation() {
  return useMutation<ApplyResult, Error, ApplyInput>({
    mutationFn: (input) => api.postForm<ApplyResult>("/api/v1/public/apply", buildApplyForm(input), { "X-LINE-IdToken": input.lineIdToken }).then(r => r.data) });
}
export function useStatus(token: string) {
  return useQuery({ queryKey: ["public-status", token],
    queryFn: () => api.get<ApplicationStatus>(`/api/v1/public/status/${encodeURIComponent(token)}`).then(r => r.data), enabled: !!token, retry: false });
}
```
→ `useInterviewSession(token)` (GET start), `useInterviewRespond(token)` (POST message). Add JSON `post` to `career-portal/lib/api.ts` (mirror `frontend/lib/api.ts` post).

### SCORE_HERO_UI
```tsx
// SOURCE: frontend/components/resume/AiSummaryPanel.tsx:28-66
const tone = score === null ? "var(--muted-foreground)"
  : score >= 75 ? "var(--score-high)" : score >= 50 ? "var(--score-mid)" : "var(--score-low)";
<div className="grid size-16 ... text-white" style={{ backgroundColor: tone }}>{Math.round(score)}</div>
```
→ Reuse identical tone logic + score hero in `InterviewPanel.tsx`.

### TEST_STRUCTURE
```go
// SOURCE: backend/internal/scoring/scoring_test.go:11-100
func qualifiedProfile() ai.Profile { ... }          // fixture builder
func Test_Score_HappyPath(t *testing.T) {
	cases := []struct{ name string; ...; want ... }{ ... }
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { ... }) }
}
// mock LLM: backend/internal/scoring/mock.go — deterministic, no network
```
→ `interview/service_test.go` with an in-memory repo + `mockInterviewer`; table-driven turn/finish cases.

---

## Files to Change

### Backend — CREATE
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000012_interview_sessions.up.sql` | CREATE | `interview_sessions` table |
| `backend/migrations/000012_interview_sessions.down.sql` | CREATE | `DROP TABLE interview_sessions` |
| `backend/internal/interview/model.go` | CREATE | Session, Turn, Evaluation structs; status consts; token gen |
| `backend/internal/interview/interviewer.go` | CREATE | `Interviewer` interface (NextTurn, Evaluate) + JD/profile context struct |
| `backend/internal/interview/azure.go` | CREATE | Real Azure OpenAI multi-turn interviewer + evaluator |
| `backend/internal/interview/mock.go` | CREATE | Deterministic mock interviewer (dev/CI) |
| `backend/internal/interview/factory.go` | CREATE | `New(cfg) Interviewer` provider seam |
| `backend/internal/interview/repository.go` | CREATE | pgx repo: Create, FindByToken, FindByApplicationID, SaveTurn, SetEvaluation, SetStatus |
| `backend/internal/interview/service.go` | CREATE | Invite / Start / Respond orchestration + max-turn enforcement |
| `backend/internal/interview/handler.go` | CREATE | Public (Start, Respond) + Dashboard (Invite, Get) handlers |
| `backend/internal/interview/routes.go` | CREATE | RegisterPublicRoutes + RegisterDashboardRoutes |
| `backend/internal/interview/service_test.go` | CREATE | Table-driven service tests w/ mock interviewer + in-mem repo |
| `backend/internal/interview/azure_test.go` | CREATE | Prompt-shape + JSON-parse unit tests |
| `backend/internal/notify/interview_message.go` | CREATE | `InterviewInviteMessage(...)` builder |

### Backend — UPDATE
| File | Action | Justification |
|---|---|---|
| `backend/pkg/config/config.go` | UPDATE | Add `InterviewMaxTurns` (`getenvInt("INTERVIEW_MAX_TURNS",6)`); reuse `UsesAzureAI()` for provider |
| `backend/cmd/api/main.go` | UPDATE | Construct interview repo/interviewer/service/handler; register routes; inject notifier + PortalBaseURL + positions/candidates/apps repos |
| `backend/internal/notify/notify.go` | UPDATE (if needed) | Ensure `ChannelLINE`/`Message` reused; no change expected |
| `backend/.env.example` (if present) | UPDATE | Document `INTERVIEW_MAX_TURNS` |

### Frontend — career-portal — CREATE/UPDATE
| File | Action | Justification |
|---|---|---|
| `career-portal/lib/api.ts` | UPDATE | Add JSON `post<T>(path, body)` method (mirror dashboard) |
| `career-portal/lib/types.ts` | UPDATE | `InterviewTurn`, `InterviewSessionState` types |
| `career-portal/lib/queries.ts` | UPDATE | `useInterviewSession(token)`, `useInterviewRespond(token)` |
| `career-portal/app/interview/page.tsx` | CREATE | Token-gated route hosting the chat |
| `career-portal/components/InterviewChat.tsx` | CREATE | Chat bubbles + input + send + completion screen |
| `career-portal/e2e/interview.spec.ts` | CREATE | Playwright flow against mock interviewer |

### Frontend — dashboard — CREATE/UPDATE
| File | Action | Justification |
|---|---|---|
| `frontend/lib/types.ts` | UPDATE | `InterviewSession`, `InterviewTurn`, `InterviewEvaluation` types |
| `frontend/lib/queries.ts` | UPDATE | `useInterview(id)`, `useInviteInterview(id)` |
| `frontend/components/resume/AiSummaryPanel.tsx` | UPDATE | "Send AI Interview" button + invite mutation + status hint |
| `frontend/components/resume/InterviewPanel.tsx` | CREATE | Transcript + evaluation (score hero, recommendation, strengths/concerns) |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATE | Mount InterviewPanel when a session exists |
| `frontend/e2e/interview.spec.ts` | CREATE | Invite + transcript render |

## NOT Building
- **Voice / speech / avatar** — text chat only this slice (data model leaves room but no audio pipeline).
- **Auto-advance / auto-reject** — AI never changes `applications.status`; HR decides.
- **Merging interview score into `ai_score`** — interview evaluation is shown separately, not blended into the resume score/ranking.
- **Async worker for interviews** — turns are synchronous public API calls; the asynq pipeline is untouched.
- **New `applications.status` enum values** — interview lifecycle lives on `interview_sessions.status`; `allowedStatuses` is not modified.
- **Candidate-side login (LINE/Entra) for the interview** — access is by opaque token only, like the existing public status token.
- **Re-scoring / re-ranking the inbox** by interview result.
- **Editing/curating AI questions by HR** — adaptive AI only (fixed/hybrid question banks are a future slice).

---

## Step-by-Step Tasks

### Task 1: Migration — interview_sessions table
- **ACTION**: Create `000012_interview_sessions.up.sql` and `.down.sql`.
- **IMPLEMENT**:
```sql
-- up
CREATE TABLE IF NOT EXISTS interview_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
  access_token    TEXT NOT NULL UNIQUE,
  status          TEXT NOT NULL DEFAULT 'invited',  -- invited|in_progress|completed|expired
  conversation    JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{role,content,ts}]
  turn_count      INT  NOT NULL DEFAULT 0,
  interview_score NUMERIC(5,2),
  recommendation  TEXT,                              -- strong_recommend|recommend|neutral|caution
  strengths       JSONB,
  concerns        JSONB,
  summary         TEXT,
  invited_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at      TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_interview_sessions_app ON interview_sessions(application_id);
-- down
DROP TABLE IF EXISTS interview_sessions;
```
- **MIRROR**: existing migration files in `backend/migrations/` (numbering, `IF NOT EXISTS`, `gen_random_uuid()`, `updated_at` convention).
- **GOTCHA**: Prod does NOT auto-run migrations (per project history — slice 2.3 apply-500). Note in plan: apply `000012` to prod manually before deploying api code that reads the table.
- **VALIDATE**: `docker compose run --rm migrate` then `\d interview_sessions` shows the table.

### Task 2: interview model + token
- **ACTION**: Create `internal/interview/model.go`.
- **IMPLEMENT**: `Session` struct (mirror DB columns; `Conversation []Turn`, `Strengths/Concerns []string`, pointers/`sql.Null*` for nullable eval fields), `Turn{Role, Content string; TS time.Time}`, `Evaluation{Score float64; Recommendation string; Strengths, Concerns []string; Summary string}`, status consts `StatusInvited/StatusInProgress/StatusCompleted/StatusExpired`, role consts `RoleAssistant="assistant"`, `RoleUser="user"`. Add `newAccessToken() string` using `crypto/rand` (reuse the helper that generates `public_token` if one exists — `grep -rn "rand" backend/internal/applications`).
- **MIRROR**: `applications/model.go:1-82` (status consts block, JSONB readback).
- **IMPORTS**: `time`, `crypto/rand`, `encoding/hex` (or existing token helper).
- **GOTCHA**: Keep this package free of `applications`/`candidates` imports to avoid cycles — pass primitives/small context structs (mirror `notify` decoupling note at `notify/message.go:6-8`).
- **VALIDATE**: `go build ./internal/interview/...`.

### Task 3: Interviewer interface + context
- **ACTION**: Create `internal/interview/interviewer.go`.
- **IMPLEMENT**:
```go
// Context the LLM needs, passed by the service (no domain imports).
type InterviewContext struct {
	PositionTitle    string
	Responsibilities string
	Qualifications   string
	CandidateName    string
	ProfileSummary   string // short text from parsed profile
	MaxTurns         int
}
type Interviewer interface {
	// NextTurn returns the AI's next message and whether the interview is complete.
	NextTurn(ctx context.Context, ic InterviewContext, history []Turn) (reply string, done bool, err error)
	// Evaluate scores the completed conversation.
	Evaluate(ctx context.Context, ic InterviewContext, history []Turn) (Evaluation, error)
}
```
- **MIRROR**: small-interface rule (golang patterns); `scoring.Scorer` interface shape (`scorer.go:25-26`).
- **GOTCHA**: `done` is decided by the interviewer (LLM may end early) OR when `len(userTurns) >= ic.MaxTurns` — enforce the cap in the service, not only the LLM.
- **VALIDATE**: compiles; referenced by mock + azure.

### Task 4: Mock interviewer
- **ACTION**: Create `internal/interview/mock.go`.
- **IMPLEMENT**: `mockInterviewer{}`; `NextTurn` returns canned Thai questions indexed by `len(history)`/2, sets `done=true` once `MaxTurns` reached; `Evaluate` returns deterministic `Evaluation{Score:75, Recommendation:"recommend", Strengths:["…","…"], Concerns:["…"], Summary:"…"}`.
- **MIRROR**: `scoring/mock.go:10-28` (deterministic, no network, Thai dummy text).
- **GOTCHA**: Must be 100% deterministic for CI (no `time.Now()`-dependent branching in outputs).
- **VALIDATE**: used by `service_test.go`; `go test ./internal/interview/...`.

### Task 5: Azure interviewer
- **ACTION**: Create `internal/interview/azure.go`.
- **IMPLEMENT**: `azureInterviewer{endpoint,key,deployment string; http *http.Client}` (`newAzureInterviewer(cfg)` trims endpoint, 60s client). Reuse copies of `chatMessage`/`chatRequest`/`chatResponse` shapes.
  - `NextTurn`: build `messages` = `{system: interviewerSystemPrompt(ic)}` + history mapped to chat roles; **no** `response_format`; `Temperature: 0.4`, `MaxTokens: 400`. The system prompt instructs: act as a warm Thai HR interviewer, ask ONE question at a time grounded in the responsibilities/qualifications, ≤`MaxTurns` questions, and when finished reply with a closing line containing the sentinel token `[[END]]`. Parse: if reply contains `[[END]]` → strip it, `done=true`.
  - `Evaluate`: separate call with `response_format:{type:"json_object"}`, `evaluatorSystemPrompt`, returns `{"interview_score":0-100,"recommendation":"strong_recommend|recommend|neutral|caution","strengths":["Thai…"],"concerns":["Thai…"],"summary":"Thai 2-3 sentences"}`; tolerate int-as-string for score (see line-int-parse crash lesson — use a tolerant unmarshal or `json.Number`).
- **MIRROR**: `scoring/azure.go:43-115` verbatim for HTTP/URL/headers/error handling.
- **IMPORTS**: `bytes, context, encoding/json, fmt, io, net/http, strings, time`.
- **GOTCHA**: Azure may wrap JSON in prose if `response_format` is omitted — only the `Evaluate` call uses json_object; `NextTurn` is free text. Score may come back as a string → tolerant parse (prior PR #26 fixed exactly this class of bug).
- **VALIDATE**: `azure_test.go` asserts prompt contains JD text + URL shape; build green.

### Task 6: Repository
- **ACTION**: Create `internal/interview/repository.go`.
- **IMPLEMENT**: `Repository` interface + `pgRepository` (pgx pool). Methods: `Create(ctx, appID, token) (*Session,error)`; `FindByToken(ctx, token) (*Session,error)`; `FindByApplicationID(ctx, appID) (*Session,error)` (returns `nil,nil` or a not-found sentinel when none); `SaveConversation(ctx, id, conv []Turn, turnCount int, status string)`; `SetStatus(ctx,id,status,startedAt)`; `SetEvaluation(ctx, id, ev Evaluation)` (writes score/recommendation/strengths/concerns/summary, status=completed, completed_at=NOW()). JSONB via `json.Marshal`.
- **MIRROR**: `applications/repository.go:14-34, 155-220` (interface shape, single-UPDATE writes, `updated_at=NOW()`, `FindByPublicToken`).
- **GOTCHA**: `conversation` column is `NOT NULL DEFAULT '[]'` — always marshal a non-nil slice (`[]Turn{}` not nil) to avoid `null`.
- **VALIDATE**: integration test against the docker postgres OR rely on service unit tests with an in-memory repo; `go build`.

### Task 7: Service
- **ACTION**: Create `internal/interview/service.go`.
- **IMPLEMENT**: `Service` with injected `repo Repository`, `interviewer Interviewer`, `apps ApplicationReader`, `positions PositionReader`, `cands CandidateReader`, `notifier notify.Notifier`, `portalBaseURL string`, `maxTurns int`. Define small reader interfaces locally (accept interfaces).
  - `Invite(ctx, appID) (*Session, error)`: if a session exists → return it (idempotent); else generate token, `repo.Create`, build `InterviewContext` later (lazy), best-effort `notifier.Send(InterviewInviteMessage(...))` (load candidate for `line_user_id`/name). Never fail invite on notify error.
  - `Start(ctx, token) (*Session, error)`: `FindByToken`; reject if expired/completed; if `invited` → set `in_progress` + `started_at`; if conversation empty → `NextTurn` to get first question, append assistant turn, persist; return session (turns + done flag).
  - `Respond(ctx, token, answer) (*Session, error)`: load; guard status==in_progress + not expired + answer non-empty; append user turn; if `userTurns >= maxTurns` force done; else `NextTurn`; append assistant reply; if `done` → `Evaluate` + `SetEvaluation` (status completed); else `SaveConversation`. Return updated session.
- **MIRROR**: `applications/service.go` constructor-injection; `applications/notify.go:26-47` best-effort notify.
- **IMPORTS**: `context`, `time`, `github.com/google/uuid`, `internal/notify`, `rs/zerolog/log`.
- **GOTCHA**: build `InterviewContext` from the position JD (`positions` has `responsibilities`/`qualifications` from migration 000010) + a short profile summary; if position fetch fails, degrade to title-only context (don't hard-fail the interview).
- **VALIDATE**: `service_test.go` covers invite-idempotency, start-first-question, respond-until-done→evaluated, max-turn cap, expired/completed guards.

### Task 8: Notify invite message
- **ACTION**: Create `internal/notify/interview_message.go`.
- **IMPLEMENT**:
```go
func InterviewInviteMessage(lineUserID, fullName, portalBaseURL, token string) Message {
	if lineUserID == "" || token == "" { return Message{} }
	greeting := "สวัสดีค่ะ"; if fullName != "" { greeting = "สวัสดีคุณ" + fullName }
	body := greeting + " คุณได้รับเชิญทำสัมภาษณ์ AI เบื้องต้น กรุณาทำผ่านลิงก์นี้ " +
		fmt.Sprintf("%s/interview?token=%s", portalBaseURL, token)
	return Message{Channel: ChannelLINE, Recipient: lineUserID, Subject: "เชิญสัมภาษณ์ AI เบื้องต้น", Body: body}
}
```
- **MIRROR**: `notify/message.go:15-29` exactly.
- **GOTCHA**: empty `lineUserID` → zero Message so caller skips (mock/demo candidates have none).
- **VALIDATE**: `go test ./internal/notify/...`.

### Task 9: HTTP handler + routes
- **ACTION**: Create `internal/interview/handler.go` + `routes.go`.
- **IMPLEMENT**:
  - Public: `GET /api/v1/public/interview/:token` → `Start` (returns `{status, turns:[{role,content}], done}`); `POST /api/v1/public/interview/:token/message` body `{content}` → `Respond`.
  - Dashboard: `POST /api/v1/applications/:id/interview` → `Invite` (returns `{id, status, access_token, interview_url}`); `GET /api/v1/applications/:id/interview` → `Get` (returns full session incl. evaluation; 404/empty if none).
  - `RegisterPublicRoutes(app, h)` mounts under `/api/v1/public`; `RegisterDashboardRoutes(router, h)` mounts on the authed group used by `applications.RegisterDashboardRoutes`.
- **MIRROR**: `public/routes.go:6-12`, `applications/handler.go:156-187` (`uuid.Parse`, `BodyParser`, `httpx.OK`, `fiber.NewError`).
- **GOTCHA**: public routes are already IP rate-limited at the group level (`public` group) — register interview public routes the same way so they inherit it. Validate `content` length (reject empty / cap e.g. 4000 chars) to bound LLM cost.
- **VALIDATE**: `handler_test.go` (httptest/fiber app + mock service) for happy path + invalid token + empty content.

### Task 10: Config
- **ACTION**: Update `pkg/config/config.go`.
- **IMPLEMENT**: add field `InterviewMaxTurns int` set to `getenvInt("INTERVIEW_MAX_TURNS", 6)`. Reuse `UsesAzureAI()` for the interviewer provider (same Azure OpenAI deployment as scoring) — no new required secret. Document in `.env.example`.
- **MIRROR**: `config.go:124-172` (`getenvInt`), `258/285` predicates.
- **GOTCHA**: do NOT add a new required Azure var — mock-default must keep CI credential-free.
- **VALIDATE**: `go test ./pkg/config/...`.

### Task 11: Wire into cmd/api/main.go
- **ACTION**: Update `backend/cmd/api/main.go`.
- **IMPLEMENT**: after existing repos/services: `interviewRepo := interview.NewRepository(pool)`; `interviewer := interview.New(cfg)`; `interviewSvc := interview.NewService(interviewRepo, interviewer, appsRepo, positionsRepo, candsRepo, notifier, cfg.PortalBaseURL, cfg.InterviewMaxTurns)`; `interviewH := interview.NewHandler(interviewSvc)`; `interview.RegisterPublicRoutes(app, interviewH)`; `interview.RegisterDashboardRoutes(<authed group>, interviewH)`.
- **MIRROR**: existing construction/registration block (read `cmd/api/main.go:100-150`); reuse the same `notifier` built for applications.
- **GOTCHA**: interview is synchronous in the **api** process — do NOT add anything to `cmd/worker`. Ensure positions + candidates repos are already constructed (reuse them).
- **VALIDATE**: `go build ./...`; `docker compose up -d --no-deps api` boots; `GET /api/v1/public/interview/<bad>` returns a clean 404 envelope.

### Task 12: Backend tests + gates
- **ACTION**: Create `service_test.go`, `azure_test.go`, `handler_test.go`; ensure `notify` test covers invite.
- **IMPLEMENT**: table-driven; in-memory repo struct implementing `Repository`; `mockInterviewer`.
- **MIRROR**: `scoring/scoring_test.go:1-100`.
- **VALIDATE**: `gofmt -w ./... && go vet ./... && golangci-lint run && gosec -quiet ./... && go test -race ./...` all green.

### Task 13: career-portal — api.post + types + queries
- **ACTION**: Update `career-portal/lib/api.ts`, `lib/types.ts`, `lib/queries.ts`.
- **IMPLEMENT**: add `post: async <T>(path, body) => unwrap<T>(await fetch(`${BASE}${path}`, {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(body)}))`. Types: `InterviewTurn{role:"assistant"|"user"; content:string}`, `InterviewSessionState{status:string; turns:InterviewTurn[]; done:boolean}`. Hooks: `useInterviewSession(token)` (GET start, `enabled:!!token, retry:false`); `useInterviewRespond(token)` (mutation POST `/api/v1/public/interview/${token}/message`).
- **MIRROR**: `frontend/lib/api.ts` post; `career-portal/lib/queries.ts:39-58`.
- **GOTCHA**: portal `api` currently has NO json post — must add it; keep envelope `unwrap`.
- **VALIDATE**: `npx tsc --noEmit` in career-portal.

### Task 14: career-portal — chat UI
- **ACTION**: Create `app/interview/page.tsx` + `components/InterviewChat.tsx`.
- **IMPLEMENT**: page reads `?token` (client component, `useSearchParams`), renders `<InterviewChat token=…/>`. Chat: on mount call `useInterviewSession(token)` to load/seed first question; message list (assistant left / user right bubbles, Thai font); textarea + send (disabled while `respond.isPending`); on send → `useInterviewRespond` → append; when `done` show completion card ("ขอบคุณค่ะ การสัมภาษณ์เสร็จสิ้น"). Handle invalid/expired token + error states.
- **MIRROR**: `career-portal/components/ApplyStepper.tsx` (client mutation + success screen) + CP Axtra tokens in `globals.css`.
- **GOTCHA**: optimistic-append the user's message, then reconcile with server response (roll back on error per web patterns). Keep compositor-friendly transitions only.
- **VALIDATE**: `npm run build`; manual chat at `http://localhost:3001/interview?token=<seeded>`.

### Task 15: dashboard — types + queries + invite button
- **ACTION**: Update `frontend/lib/types.ts`, `lib/queries.ts`, `components/resume/AiSummaryPanel.tsx`.
- **IMPLEMENT**: types `InterviewTurn`, `InterviewEvaluation{interview_score:number|null; recommendation:string|null; strengths:string[]|null; concerns:string[]|null; summary:string|null}`, `InterviewSession{id; application_id; status; conversation:InterviewTurn[]; …evaluation fields; access_token; interview_url; …timestamps}`. Hooks `useInterview(id)` + `useInviteInterview(id)` (invalidate `["interview", id]`). In `AiSummaryPanel`, add a `Send AI Interview` button → `useInviteInterview(app.id).mutate()` with `toast.success("Interview invite sent")`; show small status text from `useInterview`.
- **MIRROR**: `frontend/lib/queries.ts:44-50,129-138`; `AiSummaryPanel.tsx:11-26,94-108`.
- **GOTCHA**: invite is idempotent server-side; disable button while pending; if `line_user_id` empty the backend still creates the session — surface the `interview_url` so HR can copy/share manually.
- **VALIDATE**: `npx tsc --noEmit` in frontend.

### Task 16: dashboard — InterviewPanel + detail page
- **ACTION**: Create `frontend/components/resume/InterviewPanel.tsx`; update `app/(app)/applications/[id]/page.tsx`.
- **IMPLEMENT**: `useInterview(id)`; if no session → render nothing (or a subtle "no interview yet"). Else: score hero (reuse tone logic), recommendation badge, จุดแข็ง/ข้อสังเกต bullet lists, collapsible transcript (assistant/candidate turns). Mount under the AI summary in the right pane.
- **MIRROR**: `AiSummaryPanel.tsx:28-66` (score hero), `ScoreBreakdown.tsx:1-95` (bullets).
- **GOTCHA**: nullable eval fields until `status==completed` — show "in progress / awaiting completion" state.
- **VALIDATE**: `npm run build`; Playwright screenshot at 1440px shows panel.

### Task 17: Frontend e2e
- **ACTION**: Create `career-portal/e2e/interview.spec.ts` + `frontend/e2e/interview.spec.ts`.
- **IMPLEMENT**: portal — seed a session (via API/mock) → open `/interview?token=`, answer until completion. dashboard — set session cookie, open a candidate, click Send AI Interview, assert toast + panel.
- **MIRROR**: `career-portal/e2e/apply-form.spec.ts`, `frontend/e2e/dashboard.spec.ts`.
- **GOTCHA**: avoid timeout-based waits — wait on visible assistant message / completion text (web testing rules). Use mock interviewer (deterministic).
- **VALIDATE**: `npx playwright test interview` (both apps) green.

---

## Testing Strategy

### Unit Tests (backend)
| Test | Input | Expected | Edge? |
|---|---|---|---|
| `Invite_Idempotent` | invite same appID twice | one session, same token | yes |
| `Start_SeedsFirstQuestion` | invited session | status in_progress, 1 assistant turn | no |
| `Respond_AppendsAndContinues` | answer, turns<max | new assistant turn, done=false | no |
| `Respond_HitsMaxTurns_Evaluates` | answer at max | done=true, status completed, eval set | yes |
| `Respond_Expired_Rejected` | expired session | error, no LLM call | yes |
| `Respond_EmptyContent_Rejected` | "" | 400-class error | yes |
| `InterviewInviteMessage_NoLine` | empty lineUserID | zero Message | yes |
| `Azure_Evaluate_ScoreAsString` | `"interview_score":"81"` | parsed 81.0, no crash | yes |

### Frontend
- Portal: chat renders turns, send disabled while pending, completion screen on `done`.
- Dashboard: invite button fires mutation + toast; InterviewPanel renders eval when completed, "in progress" otherwise.

### Edge Cases Checklist
- [ ] Empty answer / whitespace-only
- [ ] Max-length answer (cap ~4000 chars)
- [ ] Invalid / expired / already-completed token
- [ ] Candidate with no `line_user_id` (invite still creates session; HR copies link)
- [ ] LLM call failure mid-interview (surface retryable error, don't corrupt conversation)
- [ ] Re-invite an already-interviewed candidate (idempotent)
- [ ] Position with no responsibilities/qualifications (degrade to title-only context)

---

## Validation Commands

### Static Analysis (backend)
```bash
cd backend && gofmt -l . && go vet ./... && golangci-lint run && ~/go/bin/gosec -quiet ./...
```
EXPECT: gofmt no output; vet/lint/gosec zero findings.

### Unit Tests (backend)
```bash
cd backend && go test -race ./internal/interview/... ./internal/notify/... ./pkg/config/...
```
EXPECT: all pass.

### Full Backend Suite
```bash
cd backend && go test -race ./...
```
EXPECT: no regressions.

### Frontend type/lint/build
```bash
cd career-portal && npx tsc --noEmit && npx eslint . && npm run build
cd ../frontend     && npx tsc --noEmit && npx eslint . && npm run build
```
EXPECT: zero type/lint errors; both builds succeed.

### Database Validation
```bash
docker compose run --rm migrate
docker compose exec -T postgres psql -U <user> -d hr_db -c "\d interview_sessions"
docker compose run --rm migrate -path /migrations -database "$DB_URL" down 1 && docker compose run --rm migrate  # up/down round-trip
```
EXPECT: table present; down then up clean.

### Browser Validation
```bash
docker compose up -d                       # api/postgres/redis/azurite + apps
# Dashboard: open a scored candidate → click "Send AI Interview" → copy interview_url
# Portal:   open /interview?token=… → answer ~6 turns → completion screen
# Dashboard: reopen candidate → InterviewPanel shows score + recommendation + transcript
```
EXPECT: full loop works in mock mode (no Azure creds).

### Manual Validation
- [ ] Invite is idempotent (button twice → one session)
- [ ] Mock interview completes and produces an evaluation
- [ ] With `AI_PROVIDER=azure`, a real adaptive Thai conversation occurs and evaluation parses
- [ ] No console errors in either frontend

---

## Acceptance Criteria
- [ ] HR can send an AI interview from the candidate detail view
- [ ] Candidate completes a token-gated text chat; conversation persists
- [ ] On completion the AI produces score + recommendation + strengths + concerns + summary
- [ ] HR sees the transcript + evaluation in the dashboard; status decision stays manual
- [ ] Works in mock (CI/local) AND real Azure mode behind config
- [ ] All validation commands pass; tests written; no type/lint errors

## Completion Checklist
- [ ] Follows provider-seam, repository, handler-envelope, message-builder patterns
- [ ] Error handling: best-effort notify never fails HR action; LLM errors surfaced, not swallowed
- [ ] Logging via `rs/zerolog/log` like `applications/notify.go`
- [ ] Go table-driven tests + Playwright e2e
- [ ] No hardcoded secrets; Azure creds via config; mock default credential-free
- [ ] `INTERVIEW_MAX_TURNS` documented in `.env.example`
- [ ] Self-contained — no further codebase searching needed (one `grep` noted for the token helper)

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Prod migration not applied before api deploy → 500s | Medium | High | Apply `000012` to prod FIRST (documented; matches slice 2.3 lesson) |
| LLM returns score as string / wraps JSON | Medium | Medium | Tolerant unmarshal / `json.Number`; `response_format:json_object` on Evaluate only (PR #26 precedent) |
| LLM ignores `[[END]]` sentinel / never ends | Low | Medium | Hard `MaxTurns` cap enforced in service regardless of LLM |
| Cost per interview (multi-turn LLM) | Medium | Medium | Cap turns (default 6) + cap answer length; gpt-4o-mini; synchronous so no runaway retries |
| Candidate has no LINE handle → can't be notified | Medium | Low | Session still created; HR copies `interview_url` from dashboard |
| Public endpoint abuse | Low | Medium | Inherit public-group IP rate limit; opaque token; 7-day expiry; content length cap |
| Package import cycle (interview↔applications) | Low | Medium | interview depends on small reader interfaces + primitives, not the reverse |

## Notes
- **Synchronous by design**: interview turns are live API calls in `cmd/api`, NOT asynq worker tasks (the worker pipeline stays scoring-only). This is the right call for an interactive chat.
- **No new application status**: lifecycle lives on `interview_sessions.status`, leaving `allowedStatuses` and the existing funnel untouched. HR may still manually move the application to `interview`/`hired` as today.
- **Provider**: reuses the existing Azure OpenAI deployment (`AZURE_OPENAI_DEPLOYMENT`, default `hr-screening-gpt4o`) and `UsesAzureAI()` — no new credential. Mock default keeps CI green.
- **Suggested slice name**: "Slice 2.5 — AI Pre-Interview"; ships as a stacked PR (backend first, then portal, then dashboard) per project convention.
- **One lookup left for the implementer**: `grep -rn "public_token" backend/internal/applications/service.go` to reuse the existing secure-token generator instead of writing a new one.
