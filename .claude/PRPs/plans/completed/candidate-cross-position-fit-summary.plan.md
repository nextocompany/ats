# Plan: AI Cross-Position Fit Summary (Candidate Application Page)

## Summary
Add an AI-generated "fit analysis" to the candidate **application detail page** that, after a candidate has passed CV Screening and the AI Pre-Interview, summarizes their overall **pros/cons** and recommends **which Master JD position(s)** in the organization they best fit — with reasons — or states plainly that they fit **no** position. The analysis is HR-triggered (a button, like "Send AI interview"), persisted, and rendered as a new panel alongside the existing screening + interview panels.

## User Story
As an **HR reviewer**, I want an AI verdict that combines a candidate's screening result and AI-interview result and tells me which of our Master JD positions they fit (and why) — or that none fit — so that I can place a strong applicant in the right role instead of rejecting/forcing them into only the single role they applied for.

## Problem → Solution
**Current:** The application detail page shows a per-position screening score (`AiSummaryPanel` → 5-dim breakdown + จุดแข็ง/ข้อสังเกต) and a per-position interview evaluation (`InterviewPanel`). Both are scoped to the **one** position the candidate applied to. There is no cross-position recommendation; `ai_suggested_positions` exists but is a flat list of free-text strings produced during single-JD scoring, not grounded in the full Master JD and not combined with the interview.
**Desired:** A new "ความเหมาะสมกับตำแหน่ง / Fit analysis" panel. HR clicks **วิเคราะห์ความเหมาะสม**; the backend feeds the candidate's screening summary + interview evaluation + the **entire Master JD catalogue** to the LLM and gets back: an overall verdict, strengths, concerns, and a ranked list of recommended positions (each with a fit score + Thai reasons) — or a clear "ไม่เหมาะสมกับตำแหน่งใดเลย" with the reason. Result is persisted (cached + auditable) and re-generatable.

## Metadata
- **Complexity**: Large (new backend package + DB migration + new positions repo method + frontend panel/hooks/types)
- **Source PRD**: N/A (free-form request)
- **PRD Phase**: N/A
- **Estimated Files**: ~16 (10 backend, 4 frontend, 1 migration pair, tests)

---

## UX Design

### Before
```
Application detail  (/applications/[id])
┌──────────────────────────┬─────────────────────────────┐
│  Resume (ResumeViewer)   │  AiSummaryPanel              │
│                          │   • AI score 0-100 + 5 dims  │
│                          │   • จุดแข็ง / ข้อสังเกต        │
│                          │   • status actions           │
│                          │   • Send AI interview ▶      │
│                          │  ─────────────────────────   │
│                          │  InterviewPanel              │
│                          │   • interview score + rec    │
│                          │   • จุดแข็ง / ข้อสังเกต        │
│                          │   • transcript               │
└──────────────────────────┴─────────────────────────────┘
```

### After
```
Application detail  (/applications/[id])
┌──────────────────────────┬─────────────────────────────┐
│  Resume (ResumeViewer)   │  AiSummaryPanel  (unchanged) │
│                          │  InterviewPanel  (unchanged) │
│                          │  ─────────────────────────   │
│                          │  FitAnalysisPanel  ★ NEW     │
│                          │   [empty] → "ยังไม่ได้วิเคราะห์"│
│                          │            [วิเคราะห์ความเหมาะสม]│
│                          │   [done]  → overall verdict   │
│                          │            • จุดเด่น / จุดที่ต้อง│
│                          │              พิจารณา           │
│                          │            • แนะนำตำแหน่ง:      │
│                          │               1. <title> 86%  │
│                          │                  reasons…     │
│                          │               2. <title> 72%  │
│                          │            OR "ไม่เหมาะสมกับ    │
│                          │             ตำแหน่งใดเลย" + เหตุผล│
│                          │            [วิเคราะห์ใหม่]      │
└──────────────────────────┴─────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Application aside | screening + interview only | + Fit analysis panel | Renders below `InterviewPanel` in the same `<aside>` |
| Generate action | n/a | `POST /api/v1/applications/:id/fit-analysis` button | Mirrors "Send AI interview" mutation+toast pattern |
| Read | n/a | `GET /api/v1/applications/:id/fit-analysis` (404 → render empty/CTA) | Same 404→null convention as `InterviewPanel` |
| Pre-conditions | n/a | Button enabled only when screened **and** interview completed | Otherwise show "ต้องผ่าน Screening และ AI Interview ก่อน" |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/interview/azure.go` | 1-201 | **Primary mirror** for the new AI client: `chatRequest`/`chatResponse`, `call()` helper, JSON-mode, `json.Number` int-as-string tolerance, system prompt const |
| P0 | `backend/internal/interview/factory.go` | 1-14 | Provider selection (`mock`/`azure`) to mirror for `fit.New(cfg)` |
| P0 | `backend/internal/interview/handler.go` | 1-172 | Mirror HR handler: `ScopeChecker`, `authorizeApplication` (404 not 403), `httpx.OK`, `mapError`, route handlers |
| P0 | `backend/internal/interview/repository.go` | 95-167 | `FindByApplicationID` + `SetEvaluation` patterns; JSONB marshal; pgx `pool.Exec`/`QueryRow` |
| P0 | `backend/internal/interview/service.go` | (whole) | Service wiring: deps (appRepo, positionRepo, candidateRepo), `buildContext`, error sentinels |
| P0 | `backend/internal/positions/model.go` | 15-136 | `Position` struct, `Repository` interface, pgx query style — must add `ListAll` |
| P0 | `backend/internal/applications/model.go` | 22-70 | `Application` fields (`AIScore`, `AIScoreBreakdown`, `AISummary`, `AIRedFlags`) consumed as screening inputs |
| P0 | `backend/internal/applications/repository.go` | 63-99, 160-177 | `FindByID` (reads explainability), `SetScore` (UPDATE+JSONB template), and `ExistsInScope` (satisfies `ScopeChecker`) |
| P0 | `frontend/components/resume/InterviewPanel.tsx` | 1-166 | **Primary mirror** for `FitAnalysisPanel.tsx` — loading/error/empty states, tone colors, จุดแข็ง/ข้อสังเกต list markup, Badge |
| P0 | `frontend/lib/queries.ts` | 91-189 | `useApplication`, `useInterview`, `useInviteInterview` (query + mutation+invalidate) to mirror |
| P1 | `frontend/app/(app)/applications/[id]/page.tsx` | 33-50 | Exact slot: add `<FitAnalysisPanel applicationId={app.id} app={app} />` after `<InterviewPanel/>` |
| P1 | `frontend/components/resume/AiSummaryPanel.tsx` | (whole) | Mutation+toast pattern (`useInviteInterview`, `toast.success/error`, `isPending` button) |
| P1 | `frontend/lib/types.ts` | 19-107 | `Application`, `ScoreBreakdown`, `InterviewSession` shapes to mirror new types after |
| P1 | `backend/cmd/api/main.go` | 247-280 | Wiring site — register fit service/handler next to interview (line ~257/275) |
| P1 | `backend/migrations/000012_interview_sessions.up.sql` | (whole) | Migration template (table + indexes + down) for `000015` |
| P2 | `backend/internal/scoring/azure.go` | 19-115 | Secondary AI mirror (scoring prompt + `suggested_positions` JSON), for prompt phrasing |
| P2 | `backend/internal/interview/mock.go` | (whole) | Mirror for `fit` mock summarizer (deterministic Thai output for local/test) |
| P2 | `backend/internal/applications/findbyid_integration_test.go` | 16-55 | Integration test template (`//go:build integration`, TRUNCATE+seed) |

## External Documentation

No external research needed — the feature reuses established internal patterns (Azure OpenAI chat-completions JSON mode is already implemented in `interview/azure.go` and `scoring/azure.go`). API version pinned at `2024-08-01-preview`.

---

## Patterns to Mirror

### NAMING_CONVENTION (Go packages: noun, lowercase; constructors `New*`)
```go
// SOURCE: backend/internal/interview/factory.go:1-14
func New(cfg *config.Config) Interviewer {
	if cfg.UsesAzureAI() {
		return newAzureInterviewer(cfg)
	}
	return mockInterviewer{}
}
```

### AI_CLIENT (Azure chat-completions, JSON mode, shared HTTP shape)
```go
// SOURCE: backend/internal/interview/azure.go:104-150
content, err := a.call(ctx, chatRequest{
	Messages: []chatMessage{
		{Role: "system", Content: evaluatorSystemPrompt},
		{Role: "user", Content: user},
	},
	Temperature:    0,
	MaxTokens:      500,
	ResponseFormat: map[string]string{"type": "json_object"},
})
// call(): POST {endpoint}/openai/deployments/{deployment}/chat/completions?api-version=2024-08-01-preview
//         header api-key; decode chatResponse; return Choices[0].Message.Content
```

### LLM_JSON_PARSE (tolerate number-or-string; clamp)
```go
// SOURCE: backend/internal/interview/azure.go:152-186
type evalJSON struct {
	Score json.Number `json:"interview_score"` // accepts 75 AND "75"
	...
}
raw := strings.TrimSpace(parsed.Score.String())
score, perr := strconv.ParseFloat(raw, 64)
if score < 0 { score = 0 }
if score > 100 { score = 100 }
```

### ERROR_HANDLING (wrap with `pkg: op: %w`; sentinel errors → HTTP via mapError)
```go
// SOURCE: backend/internal/interview/handler.go:157-171
switch {
case errors.Is(err, ErrNotFound):
	return fiber.NewError(fiber.StatusNotFound, "interview not found")
default:
	return err
}
```

### REPOSITORY_PATTERN (pgx v5, $1 placeholders, JSONB marshal)
```go
// SOURCE: backend/internal/interview/repository.go:142-167
func (r *pgRepository) SetEvaluation(ctx context.Context, id uuid.UUID, ev Evaluation) error {
	strengths, _ := json.Marshal(nonNil(ev.Strengths))
	const q = `UPDATE interview_sessions SET ... strengths=$5 ... WHERE id=$1`
	if _, err := r.pool.Exec(ctx, q, id, ...); err != nil {
		return fmt.Errorf("interview: set evaluation: %w", err)
	}
	return nil
}
```

### HR_AUTHORIZATION (per-record scope check, 404 to avoid existence leak)
```go
// SOURCE: backend/internal/interview/handler.go:44-56
func (h *Handler) authorizeApplication(c *fiber.Ctx, id uuid.UUID) error {
	if h.scoper == nil { return nil }
	ok, err := h.scoper.ExistsInScope(c.UserContext(), id, scopeFrom(c))
	if err != nil { return err }
	if !ok { return fiber.NewError(fiber.StatusNotFound, "application not found") }
	return nil
}
```

### RESPONSE_ENVELOPE
```go
// SOURCE: backend/pkg/httpx/response.go:16-26
return httpx.OK(c, fiber.Map{"analysis": result})
```

### POSITIONS_QUERY (add ListAll mirroring FindByID scan)
```go
// SOURCE: backend/internal/positions/model.go:60-83 (FindByID) — ListAll mirrors the SELECT + Scan over rows
const q = `SELECT id, title_th, COALESCE(title_en,''), COALESCE(level,''),
	COALESCE(must_have_criteria,'{}'::jsonb), COALESCE(keywords,'{}'),
	COALESCE(format_types,'{}'), COALESCE(responsibilities,''), COALESCE(qualifications,'')
	FROM positions WHERE is_active = TRUE ORDER BY title_th`
```

### FRONTEND_PANEL (loading/error/empty + tone + Thai bullet lists)
```tsx
// SOURCE: frontend/components/resume/InterviewPanel.tsx:27-47, 102-116
if (isLoading) return null;
if (isError) return <p className="...">Could not load…</p>;
if (!data) return null; // 404 → render nothing
const tone = score >= 75 ? "var(--score-high)" : score >= 50 ? "var(--score-mid)" : "var(--score-low)";
// list:
<ul className="space-y-1.5 text-sm text-foreground">
  {items.map((t, i) => <li key={i} className="flex gap-2"><span className="text-[var(--score-high)]">•</span><span>{t}</span></li>)}
</ul>
```

### FRONTEND_MUTATION (button + isPending + toast + invalidate)
```ts
// SOURCE: frontend/lib/queries.ts:183-189
export function useInviteInterview(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<InterviewInviteResult>(`/api/v1/applications/${id}/interview`).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["interview-session", id] }),
  });
}
```

### TEST_STRUCTURE (table-driven unit + integration build tag)
```go
// SOURCE: backend/internal/applications/findbyid_integration_test.go:16-55
//go:build integration
// TRUNCATE … RESTART IDENTITY CASCADE; seed; call repo; assert structs
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000015_application_fit_analyses.up.sql` | CREATE | New table to persist the analysis (1 current row per application) |
| `backend/migrations/000015_application_fit_analyses.down.sql` | CREATE | Reversibility (drop table) |
| `backend/internal/fit/model.go` | CREATE | `Analysis`, `RecommendedPosition`, `PositionCard`, `Inputs` structs + JSON tags |
| `backend/internal/fit/summarizer.go` | CREATE | `Summarizer` interface (`Summarize(ctx, Inputs) (Analysis, error)`) |
| `backend/internal/fit/factory.go` | CREATE | `New(cfg) Summarizer` → mock/azure selection |
| `backend/internal/fit/azure.go` | CREATE | Azure OpenAI impl: prompt build + JSON-mode call + parse (mirror interview/azure.go) |
| `backend/internal/fit/mock.go` | CREATE | Deterministic Thai mock for local/tests |
| `backend/internal/fit/repository.go` | CREATE | `Upsert`/`FindByApplicationID` over `application_fit_analyses` |
| `backend/internal/fit/service.go` | CREATE | Orchestrates: load app + interview + all positions + candidate → Summarize → persist |
| `backend/internal/fit/handler.go` | CREATE | `POST`/`GET /api/v1/applications/:id/fit-analysis`, scope-checked |
| `backend/internal/fit/routes.go` | CREATE | `RegisterDashboardRoutes` |
| `backend/internal/positions/model.go` | UPDATE | Add `ListAll(ctx) ([]Position, error)` to interface + pgRepository |
| `backend/cmd/api/main.go` | UPDATE | Wire `fit.NewService(...)` + handler; register routes (near interview, ~line 257/275) |
| `backend/internal/fit/*_test.go` | CREATE | Unit (parse, mock, prompt non-empty) + integration (repo upsert/read) |
| `frontend/lib/types.ts` | UPDATE | Add `FitAnalysis`, `RecommendedPosition` interfaces |
| `frontend/lib/queries.ts` | UPDATE | Add `useFitAnalysis`, `useGenerateFitAnalysis` |
| `frontend/components/resume/FitAnalysisPanel.tsx` | CREATE | The new panel |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATE | Render `<FitAnalysisPanel/>` in the aside |

## NOT Building
- **No pipeline auto-generation.** Fit analysis is HR-triggered only (sending ~65 JDs to the LLM is costly and only meaningful post-interview). The scoring pipeline (`internal/pipeline`) is untouched.
- **No changes to existing `ai_suggested_positions`** (the flat single-JD list stays as-is).
- **No worker/scheduler/queue work** — the call is synchronous in the API request (same as interview turns), with a 60s HTTP timeout on the LLM client.
- **No multi-application aggregation** — analysis is per **application** (it already has the position the candidate applied to + that application's interview).
- **No editing/override UI** for the AI output, **no PDF/export**, **no notifications** to the candidate.
- **No new RBAC roles** — reuse existing scope (`ExistsInScope`); any HR role that can see the application can generate/read it.
- **No Gemini implementation** (mock + azure only; matches `interview` which also skips Gemini).

---

## Step-by-Step Tasks

### Task 1: Migration — `application_fit_analyses` table
- **ACTION**: Create `backend/migrations/000015_application_fit_analyses.{up,down}.sql`.
- **IMPLEMENT** (up):
  ```sql
  CREATE TABLE IF NOT EXISTS application_fit_analyses (
      application_id UUID PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
      overall_fit    VARCHAR(16) NOT NULL,            -- strong | moderate | weak | none
      summary        TEXT NOT NULL DEFAULT '',         -- 2-3 Thai sentences
      strengths      JSONB NOT NULL DEFAULT '[]',      -- []string (Thai)
      concerns       JSONB NOT NULL DEFAULT '[]',      -- []string (Thai)
      recommended    JSONB NOT NULL DEFAULT '[]',      -- [{position_id,title,fit_score,reasons[]}]
      no_match_reason TEXT NOT NULL DEFAULT '',         -- set when overall_fit='none'
      model          VARCHAR(64) NOT NULL DEFAULT '',  -- provider/deployment for audit
      generated_by   UUID REFERENCES users(id),
      created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  ```
  (down): `DROP TABLE IF EXISTS application_fit_analyses;`
- **MIRROR**: `000012_interview_sessions.up.sql` (table + `IF NOT EXISTS` idempotency).
- **GOTCHA**: PK = `application_id` makes regenerate a natural upsert (one current row). `ON DELETE CASCADE` so removing demo apps cleans up.
- **VALIDATE**: `migrate up` then `migrate down` then `up` on dev DB (currently v14 → v15) with no error.

### Task 2: `positions.ListAll`
- **ACTION**: Add `ListAll(ctx context.Context) ([]Position, error)` to the `Repository` interface and `pgRepository` in `backend/internal/positions/model.go`.
- **IMPLEMENT**: SELECT all columns (same as `FindByID`) `WHERE is_active = TRUE ORDER BY title_th`; loop `rows.Next()`, scan into `Position`, unmarshal `must_have_criteria` per row (mirror FindByID's `mustHaveRaw` handling).
- **MIRROR**: `positions/model.go:60-83` (FindByID scan) + `ListPublic:110-136` (rows loop + `rows.Err()`).
- **IMPORTS**: already present (`encoding/json`, `fmt`, `context`, `uuid`).
- **GOTCHA**: ~65 rows, each with full `responsibilities`+`qualifications` TEXT — fine for one query. Titles are unique in the current Master JD, but reference positions by **UUID `id`** in output (stable within an env); include `title_th` for display.
- **VALIDATE**: `go test ./internal/positions/...` (add a small integration test or rely on Task 9); `go build ./...`.

### Task 3: `fit` domain model
- **ACTION**: Create `backend/internal/fit/model.go`.
- **IMPLEMENT**:
  ```go
  type RecommendedPosition struct {
      PositionID uuid.UUID `json:"position_id"`
      Title      string    `json:"title"`
      FitScore   int       `json:"fit_score"`   // 0-100
      Reasons    []string  `json:"reasons"`     // Thai bullets
  }
  type Analysis struct {
      ApplicationID uuid.UUID             `json:"application_id"`
      OverallFit    string                `json:"overall_fit"` // strong|moderate|weak|none
      Summary       string                `json:"summary"`
      Strengths     []string              `json:"strengths"`
      Concerns      []string              `json:"concerns"`
      Recommended   []RecommendedPosition `json:"recommended"`
      NoMatchReason string                `json:"no_match_reason,omitempty"`
      Model         string                `json:"-"`
      GeneratedAt   time.Time             `json:"generated_at"`
  }
  // Inputs is what the service gathers and the Summarizer consumes.
  type Inputs struct {
      CandidateName  string
      ScreeningScore *float64
      ScreeningSummary string   // app.AISummary (จุดแข็ง joined)
      ScreeningRedFlags string  // app.AIRedFlags
      InterviewScore *float64
      InterviewSummary string
      InterviewStrengths []string
      InterviewConcerns  []string
      Transcript     []Turn      // role+content, from interview session
      Positions      []PositionCard
  }
  type PositionCard struct {
      ID uuid.UUID; Title, Responsibilities, Qualifications string
  }
  type Turn struct{ Role, Content string }
  ```
- **MIRROR**: `interview/model.go:44-58` (Evaluation) for tags/shape.
- **GOTCHA**: Define a local `Turn` (don't import `interview` to avoid coupling); the service maps `interview.Turn → fit.Turn`.
- **VALIDATE**: `go build ./internal/fit/...`.

### Task 4: `Summarizer` interface + factory + mock
- **ACTION**: Create `summarizer.go`, `factory.go`, `mock.go`.
- **IMPLEMENT**:
  - `summarizer.go`: `type Summarizer interface { Summarize(ctx context.Context, in Inputs) (Analysis, error) }`
  - `factory.go`: `func New(cfg *config.Config) Summarizer { if cfg.UsesAzureAI() { return newAzureSummarizer(cfg) }; return mockSummarizer{} }`
  - `mock.go`: deterministic — if `len(in.Positions)==0` return `OverallFit:"none"`; else recommend the first 1-2 positions with Thai canned reasons and `OverallFit:"moderate"`.
- **MIRROR**: `interview/factory.go:1-14`, `interview/mock.go`.
- **GOTCHA**: Mock must be safe with empty inputs (tests + local default `AI_PROVIDER=mock`).
- **VALIDATE**: `go test ./internal/fit/...` (mock test in Task 9).

### Task 5: Azure summarizer (prompt + call + parse)
- **ACTION**: Create `backend/internal/fit/azure.go`.
- **IMPLEMENT**:
  - Copy the `chatMessage`/`chatRequest`/`chatResponse`/`call()` shape and `openAIAPIVersion` from `interview/azure.go` (these are package-private there, so re-declare in `fit`).
  - System prompt const (English instructions, Thai output) requesting **strict JSON**:
    ```
    {"overall_fit":"strong|moderate|weak|none",
     "summary":"2-3 Thai sentences",
     "strengths":["Thai bullets"],"concerns":["Thai bullets"],
     "recommended":[{"position_id":"<uuid from the catalogue>","title":"<th>","fit_score":<0-100 int>,"reasons":["Thai bullets"]}],
     "no_match_reason":"Thai sentence — REQUIRED when overall_fit=none, else empty"}
    ```
    Instruct: ground every judgement in screening + interview + the position's responsibilities/qualifications; only use `position_id`s present in the provided catalogue; if no position fits, set `overall_fit:"none"`, `recommended:[]`, and explain in `no_match_reason`.
  - User message: candidate name, screening (score+summary+red flags), interview (score+summary+strengths+concerns+compact transcript), then the **position catalogue** as a numbered list: `position_id | title_th\nResponsibilities: …\nQualifications: …`.
  - Call with `Temperature: 0`, `MaxTokens: 1200`, `ResponseFormat: {"type":"json_object"}`.
  - Parse with a `fitJSON` struct using `json.Number` for `fit_score` (number-or-string tolerance); coerce/validate `position_id` to `uuid.UUID` (skip a recommended entry whose id isn't in the catalogue set); clamp `fit_score` to [0,100]; lowercase/validate `overall_fit` (default `"weak"` if unknown). Set `Model = "azure:"+deployment`.
- **MIRROR**: `interview/azure.go:92-186` (Evaluate + call + parseEvaluation + evalJSON).
- **IMPORTS**: `bytes, context, encoding/json, fmt, io, net/http, strconv, strings, time`, `pkg/config`, `github.com/google/uuid`.
- **GOTCHA**: gpt-4o-mini sometimes returns numbers as strings → `json.Number` is mandatory (see the comment at `interview/azure.go:152-154`). Also it may hallucinate a `position_id` — **filter to the catalogue's id set** before returning. Keep prompt within budget: title + responsibilities + qualifications for 65 roles is well under gpt-4o-mini's 128k context.
- **VALIDATE**: `go build ./...`; `go vet ./internal/fit/...`; unit test for `parseFit` with a sample JSON blob (number-as-string + bogus id) in Task 9.

### Task 6: `fit` repository
- **ACTION**: Create `backend/internal/fit/repository.go`.
- **IMPLEMENT**:
  - `type Repository interface { Upsert(ctx, a Analysis, generatedBy *uuid.UUID) error; FindByApplicationID(ctx, appID uuid.UUID) (*Analysis, error) }`
  - `pgRepository{pool *pgxpool.Pool}`, `NewRepository(pool)`.
  - `Upsert`: `INSERT … ON CONFLICT (application_id) DO UPDATE SET …, updated_at=NOW()`; JSON-marshal `strengths/concerns/recommended` (use a `nonNil` helper so empty slices serialize as `[]`).
  - `FindByApplicationID`: SELECT + unmarshal JSONB into slices; return `(nil, ErrNotFound)` on `pgx.ErrNoRows`.
  - Sentinel: `var ErrNotFound = errors.New("fit: analysis not found")`.
- **MIRROR**: `interview/repository.go:95-167` (FindByApplicationID + SetEvaluation + JSONB marshal), `applications/repository.go:63-99` (JSONB unmarshal guard).
- **GOTCHA**: `errors.Is(err, pgx.ErrNoRows)` → return `ErrNotFound` (don't wrap so the handler's `errors.Is` works).
- **VALIDATE**: integration test (Task 9) upsert→read round-trip incl. regenerate (second upsert overwrites).

### Task 7: `fit` service
- **ACTION**: Create `backend/internal/fit/service.go`.
- **IMPLEMENT**:
  - Deps (constructor injection, accept interfaces): `repo Repository`, `summarizer Summarizer`, plus minimal interfaces it needs:
    - `appReader`: `FindByID(ctx, id) (*applications.Application, error)`
    - `interviewReader`: `FindByApplicationID(ctx, appID) (*interview.Session, error)`
    - `positionLister`: `ListAll(ctx) ([]positions.Position, error)`
    - `candidateReader`: `FindByID(ctx, id) (*candidates.Candidate, error)`
  - `Generate(ctx, appID, generatedBy *uuid.UUID) (*Analysis, error)`:
    1. `app := appReader.FindByID(appID)`; if `app.AIScore == nil` → `ErrNotScored`.
    2. `sess := interviewReader.FindByApplicationID(appID)`; if `sess == nil || sess.Status != "completed"` → `ErrInterviewIncomplete`.
    3. `positions := positionLister.ListAll()`; `cand := candidateReader.FindByID(app.CandidateID)`.
    4. Build `Inputs` (map screening fields from `app`, interview from `sess`, positions → `PositionCard`, transcript → `[]fit.Turn`).
    5. `a := summarizer.Summarize(ctx, in)`; set `a.ApplicationID=appID`; `repo.Upsert(a, generatedBy)`; return `&a`.
  - `Get(ctx, appID) (*Analysis, error)` → `repo.FindByApplicationID`.
  - Sentinels: `ErrNotScored`, `ErrInterviewIncomplete` (+ re-export `ErrNotFound`).
- **MIRROR**: `interview/service.go` (deps + buildContext + sentinels).
- **GOTCHA**: Define the reader interfaces **in `fit`** (accept-interfaces) so `main.go` passes the existing concrete repos; avoids import cycles. `interview.Session.Status == interview.StatusCompleted`.
- **VALIDATE**: unit test with stub readers + mock summarizer asserting the two pre-condition errors and the happy path (Task 9).

### Task 8: `fit` handler + routes + wiring
- **ACTION**: Create `handler.go`, `routes.go`; edit `cmd/api/main.go`.
- **IMPLEMENT**:
  - Handler mirrors `interview/handler.go`: `ScopeChecker` (satisfied by `appRepo.ExistsInScope`), `authorizeApplication` (404), `scopeFrom`, `mapError` (`ErrNotScored`→409 "ต้องผ่าน Screening ก่อน", `ErrInterviewIncomplete`→409 "ต้องทำ AI Interview ให้เสร็จก่อน", `ErrNotFound`→404).
  - `Generate` handler: parse `:id`, authorize, read user id from `c.Locals(middleware.UserContextKey)` for `generatedBy`, call `svc.Generate`, `httpx.OK(c, fiber.Map{"analysis": a})`.
  - `Get` handler: parse, authorize, `svc.Get`; if nil → 404 "no fit analysis"; else `httpx.OK(c, fiber.Map{"analysis": a})`.
  - `routes.go`: `func RegisterDashboardRoutes(app *fiber.App, h *Handler){ v1:=app.Group("/api/v1"); v1.Post("/applications/:id/fit-analysis", h.Generate); v1.Get("/applications/:id/fit-analysis", h.Get) }`.
  - `main.go` (~after line 257, near interview wiring):
    ```go
    fitSvc := fit.NewService(fit.NewRepository(pool), fit.New(cfg), appRepo, interview.NewRepository(pool), positionRepo, candidateRepo)
    fit.RegisterDashboardRoutes(app, fit.NewHandler(fitSvc, appRepo, cfg))
    ```
- **MIRROR**: `interview/handler.go:105-171`, `interview` route registration in `main.go:256-275`.
- **GOTCHA**: routes live under `/api/v1/applications/:id/...` which is authed (not in `isUnauthedPath`). Register **after** dashboard routes block; reuse the existing `positionRepo`/`candidateRepo`/`appRepo` already constructed earlier in `main.go`.
- **VALIDATE**: `go build ./...`; `go vet ./...`; manual curl after `go run` (Task: Validation Commands).

### Task 9: Backend tests
- **ACTION**: Create `fit/azure_test.go` (parse), `fit/mock_test.go`, `fit/service_test.go` (stubs), `fit/repository_integration_test.go` (`//go:build integration`).
- **IMPLEMENT**:
  - `parseFit`: feed JSON with `"fit_score":"86"` (string), one valid + one bogus `position_id` → assert clamp, string-coerce, bogus filtered, `overall_fit` normalized; an `overall_fit:"none"` blob → `NoMatchReason` preserved, `recommended` empty.
  - `mockSummarizer`: empty positions → `none`; non-empty → ≥1 recommended.
  - service: stub readers; assert `ErrNotScored` (nil AIScore), `ErrInterviewIncomplete` (no/!completed session), happy path persists + returns.
  - repo integration: TRUNCATE, seed position+candidate+application, upsert analysis, read back, upsert again (regenerate) → single row reflects latest.
- **MIRROR**: `applications/findbyid_integration_test.go:16-55`, `interview/service_test.go:128-136`.
- **VALIDATE**: `go test ./internal/fit/...` (unit) and `go test -tags=integration ./internal/fit/...` against local stack.

### Task 10: Frontend types
- **ACTION**: Edit `frontend/lib/types.ts`.
- **IMPLEMENT**:
  ```ts
  export interface RecommendedPosition {
    position_id: string;
    title: string;
    fit_score: number;
    reasons: string[];
  }
  export interface FitAnalysis {
    application_id: string;
    overall_fit: "strong" | "moderate" | "weak" | "none";
    summary: string;
    strengths: string[];
    concerns: string[];
    recommended: RecommendedPosition[];
    no_match_reason?: string;
    generated_at: string;
  }
  ```
- **MIRROR**: `types.ts:90-107` (InterviewSession).
- **VALIDATE**: `pnpm tsc --noEmit`.

### Task 11: Frontend hooks
- **ACTION**: Edit `frontend/lib/queries.ts`.
- **IMPLEMENT**:
  ```ts
  export function useFitAnalysis(id: string) {
    return useQuery({
      queryKey: ["fit-analysis", id],
      queryFn: () => api.get<{ analysis: FitAnalysis }>(`/api/v1/applications/${id}/fit-analysis`).then((r) => r.data.analysis),
      enabled: !!id,
      retry: false, // 404 = not generated yet; don't hammer
    });
  }
  export function useGenerateFitAnalysis(id: string) {
    const qc = useQueryClient();
    return useMutation({
      mutationFn: () => api.post<{ analysis: FitAnalysis }>(`/api/v1/applications/${id}/fit-analysis`).then((r) => r.data.analysis),
      onSuccess: () => qc.invalidateQueries({ queryKey: ["fit-analysis", id] }),
    });
  }
  ```
- **MIRROR**: `queries.ts:91-98` (useInterview shape — confirm whether the API client unwraps the envelope; `useInterview` returns `data.session`, so the envelope `data` is already unwrapped by `api.get` → access `.analysis`). Match the existing access pattern exactly.
- **GOTCHA**: Confirm how `useInterview` reads its payload (it uses `{ session, interview_url }`) to know whether `.then((r) => r.data...)` unwraps the `httpx` envelope; mirror that precisely so the new hook doesn't double-unwrap.
- **VALIDATE**: `pnpm tsc --noEmit`.

### Task 12: `FitAnalysisPanel` component
- **ACTION**: Create `frontend/components/resume/FitAnalysisPanel.tsx`.
- **IMPLEMENT**: Props `{ applicationId: string; app: Application }`.
  - `const { data, isLoading, isError } = useFitAnalysis(applicationId); const gen = useGenerateFitAnalysis(applicationId);`
  - Pre-condition: `const ready = app.ai_score != null && /* interview completed */;` — interview completion isn't on `Application`; rely on the backend 409 + show CTA always, but disable with hint when `app.ai_score == null`. (Keep it simple: enable the button; on 409 show `toast.error(message)`.)
  - States: loading → small skeleton/null; empty (`isError`/no data) → CTA card "ยังไม่ได้วิเคราะห์ความเหมาะสม" + button **วิเคราะห์ความเหมาะสม** (`gen.isPending` → "กำลังวิเคราะห์…").
  - Loaded: header "ความเหมาะสมกับตำแหน่ง" + `Badge` for `overall_fit` (map strong→แนะนำอย่างยิ่ง/var(--score-high), moderate→แนะนำ/mid, weak→ควรพิจารณา/mid, none→ไม่เหมาะสม/low). Summary paragraph. จุดเด่น / จุดที่ต้องพิจารณา bullet lists (mirror InterviewPanel จุดแข็ง/ข้อสังเกต). Then recommended list: each row title + `fit_score` chip (tone by score) + reasons bullets. If `overall_fit==="none"`: render `no_match_reason` in a `--score-low`-toned callout instead of the list. Footer: small "วิเคราะห์ใหม่" button (regenerate).
  - Wrap consistent with the aside: this panel renders **inside** the existing `<aside>`, so use the `mt-6 border-t border-hairline pt-6` separator like InterviewPanel (not its own card).
- **MIRROR**: `InterviewPanel.tsx` end-to-end (tones, `REC_LABEL`, list markup, separators); button+toast from `AiSummaryPanel.tsx`.
- **IMPORTS**: `Badge`, `Button`, `Skeleton`, `useFitAnalysis`, `useGenerateFitAnalysis`, `toast` (from `sonner`, as used in AiSummaryPanel).
- **GOTCHA**: No `console.log`. Thai UI strings. Use `tabular-nums` for scores. `key` on lists.
- **VALIDATE**: `pnpm tsc --noEmit`; `pnpm eslint` on the file; visual check in dev.

### Task 13: Mount the panel
- **ACTION**: Edit `frontend/app/(app)/applications/[id]/page.tsx`.
- **IMPLEMENT**: import `FitAnalysisPanel`; inside the `<aside>` add after `<InterviewPanel applicationId={app.id} />`:
  ```tsx
  <FitAnalysisPanel applicationId={app.id} app={app} />
  ```
- **MIRROR**: existing `<AiSummaryPanel app={app} />` / `<InterviewPanel applicationId={app.id} />` mount at lines 46-47.
- **VALIDATE**: `pnpm build`; load `/applications/<id>` in dev.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `parseFit` number-as-string | `{"recommended":[{"fit_score":"86",...}]}` | `FitScore==86` | Yes |
| `parseFit` bogus position_id | recommended id not in catalogue set | entry filtered out | Yes |
| `parseFit` none verdict | `{"overall_fit":"none","no_match_reason":"…","recommended":[]}` | NoMatchReason kept, empty list | Yes |
| `parseFit` unknown overall_fit | `{"overall_fit":"banana"}` | normalized to `"weak"` | Yes |
| `mockSummarizer` empty positions | `Inputs{Positions:nil}` | `OverallFit=="none"` | Yes |
| `mockSummarizer` with positions | 2 positions | ≥1 recommended | No |
| `service.Generate` unscored | `app.AIScore==nil` | `ErrNotScored` | Yes |
| `service.Generate` no interview | session nil / not completed | `ErrInterviewIncomplete` | Yes |
| `service.Generate` happy path | scored + completed | Analysis persisted + returned | No |
| `positions.ListAll` | seeded active+inactive | only active, ordered | Yes |

### Edge Cases Checklist
- [ ] No positions in DB → `overall_fit:"none"`
- [ ] Candidate scored but interview not completed → 409 (not 500)
- [ ] Regenerate overwrites the single row (no duplicates)
- [ ] LLM returns score as string / out-of-range → coerced + clamped
- [ ] LLM hallucinates a position_id → filtered against catalogue
- [ ] RBAC: store-scoped HR cannot generate/read another store's application (404)
- [ ] `AI_PROVIDER=mock` (local/CI) path works without Azure creds

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l . && go vet ./...
cd ../frontend && pnpm tsc --noEmit && pnpm eslint .
```
EXPECT: no output / zero errors

### Unit Tests
```bash
cd backend && go test -race ./internal/fit/... ./internal/positions/...
```
EXPECT: all pass

### Integration Tests (local stack up + migrated to v15)
```bash
cd backend && go test -tags=integration ./internal/fit/...
```
EXPECT: pass (repo upsert/read round-trip)

### Full Suites
```bash
cd backend && go build ./... && go test -race ./...
cd ../frontend && pnpm build
```
EXPECT: green, no regressions

### Database Validation
```bash
# dev DB currently v14 → v15
~/go/bin/migrate -path backend/migrations -database "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable" up
# reversibility
~/go/bin/migrate -path backend/migrations -database "$DBURL" down 1 && ~/go/bin/migrate -path backend/migrations -database "$DBURL" up
```
EXPECT: `version 15`, clean down/up

### Browser/API Validation (local, AUTH_PROVIDER=mock, HTTP_PORT=8097, AI_PROVIDER=mock)
```bash
# generate (mock summarizer) for a scored+interviewed application
curl -s -X POST localhost:8097/api/v1/applications/<APP_ID>/fit-analysis | jq .
# read back
curl -s localhost:8097/api/v1/applications/<APP_ID>/fit-analysis | jq .
# pre-condition: unscored application → expect 409
```
EXPECT: 200 with `{success:true,data:{analysis:{…}}}`; 409 on unscored/no-interview.

### Manual Validation
- [ ] Open `/applications/<id>` for a candidate that is scored + interview-completed → panel shows CTA
- [ ] Click **วิเคราะห์ความเหมาะสม** → spinner → recommendations render with Thai reasons
- [ ] A weak/irrelevant candidate → "ไม่เหมาะสมกับตำแหน่งใดเลย" + reason
- [ ] Click **วิเคราะห์ใหม่** → result refreshes (single persisted row)
- [ ] Open an application not yet interviewed → button returns a friendly 409 toast

---

## Acceptance Criteria
- [ ] All tasks completed
- [ ] `go build ./...`, `go vet`, `go test -race ./...` green
- [ ] `pnpm tsc --noEmit`, `pnpm eslint`, `pnpm build` green
- [ ] Migration v15 applies + reverses cleanly
- [ ] Panel renders recommendations OR a clear "no fit" verdict with reasons
- [ ] Pre-conditions enforced server-side (409, not 500/crash)
- [ ] RBAC scope honored (404 cross-store)

## Completion Checklist
- [ ] Mirrors `interview` package structure (factory/mock/azure/repo/service/handler)
- [ ] Error wrapping `pkg: op: %w`; sentinels mapped to HTTP
- [ ] JSON parse tolerates number-as-string (`json.Number`) + filters hallucinated ids
- [ ] Tests follow table-driven + `//go:build integration` patterns
- [ ] No hardcoded secrets (Azure creds from config/env)
- [ ] No `console.log`; Thai UI strings; CP Axtra tones (`--score-high/mid/low`)
- [ ] Scope is on-demand only — pipeline untouched
- [ ] Self-contained — no further codebase search needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LLM hallucinates a `position_id` not in catalogue | Med | Med | Filter recommended entries against the catalogue id set in `parseFit` |
| gpt-4o-mini returns `fit_score` as string | Med | Low | `json.Number` + `strconv.ParseFloat` (proven in interview/scoring) |
| Prompt too large with 65 full JDs | Low | Med | gpt-4o-mini 128k context; send title+responsibilities+qualifications only; `MaxTokens:1200` output |
| Synchronous LLM call slows the request | Med | Low | 60s HTTP client timeout (mirror interview); on-demand button, not page-load |
| Frontend envelope double-unwrap | Low | Low | Mirror `useInterview` access exactly (Task 11 GOTCHA) |
| CI billing-blocked (can't run Actions) | High | Low | Validate locally; deploy via operator `az` per project runbook |

## Notes
- **Design decision (on-demand, not auto):** the analysis sends the whole Master JD to the LLM and is only meaningful after Screening + AI Interview, so it's an HR-triggered action (mirrors "Send AI interview"), persisted in `application_fit_analyses` (one current row per application, upsert on regenerate). This keeps the scoring pipeline untouched and cost controlled. If product later wants it automatic, add a call after interview completion in `interview/service.go` — out of scope here.
- **Why a new table, not columns on `applications`:** the result is a rich nested structure (recommended[] with reasons[]); a dedicated JSONB-backed table mirrors `interview_sessions`, keeps `applications` lean, and records `generated_by`/`model` for audit.
- **Provider parity:** mock + azure only (Gemini intentionally skipped, matching `internal/interview`).
- **Master JD facts:** ~65 active positions, unique `title_en`, stable `ps_position_code`; reference by UUID `id` in the analysis output, display `title_th`.
- After implementation, update memory (`score-explainability-live` / a new `fit-analysis-*` note) and the session file.
