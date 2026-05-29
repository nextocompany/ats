# Plan: Sprint 1 — Intake Pipeline + CV Parser + OCR

## Summary
Build the asynchronous intake pipeline on top of the Sprint 0 foundation: an HTTP intake endpoint that stores a resume in Blob, persists candidate + application records, and enqueues an asynq job; and a worker that runs **OCR (Step 1)** then **CV Parse (Step 2)** to produce a structured JSON profile. AI providers sit behind interfaces with a deterministic **mock default** for local/CI and real Azure clients for the POC.

## User Story
As an **intake source (HR staff, Career Portal, Power Automate, manual upload)**,
I want **to submit a resume and have it OCR'd and parsed into a structured candidate profile automatically**,
So that **HR sees normalized, machine-readable candidate data instead of raw PDFs (the first half of the ≤60s pipeline SLA)**.

## Problem → Solution
**Current state (post-Sprint 0):** The stack runs and the schema exists, but there is no way to submit a resume and nothing consumes the queue.
**Desired state:** `POST /api/v1/applications` (multipart) → file in Blob + `candidate`/`application` rows (status `pending`) + asynq job enqueued → worker OCRs the file, parses it with GPT-4o into the F02 JSON schema, writes the structured fields back, uploads `parsed_profile.json` to Blob, and marks the application `parsed`. Job status is pollable.

## Metadata
- **Complexity**: Large (first real domain layer + queue + AI provider abstraction; ~28 files)
- **Source PRD**: PRP "AI HR Recruitment System v1.0" — Sprint 1 (W3–4, "Intake pipeline + CV Parser + OCR", 22 MD)
- **Covers**: F01 (Centralized Candidate DB — intake side), F02 (AI CV Parser), Pipeline §8 Steps 1–2
- **Estimated Files**: ~28
- **Decisions locked**: Queue = **hibiken/asynq**; Azure AI = **interface + mock default** (env-toggled to real Azure)

---

## UX Design
**N/A — backend/internal.** No HR Dashboard or Career Portal UI in Sprint 1 (those are S3/S4). The "interface" is the HTTP API + queue.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Submit resume | No endpoint | `POST /api/v1/applications` (multipart) → 202 + `application_id` + `job_id` | async |
| Poll status | None | `GET /api/v1/ai/jobs/:job_id` → queued/active/completed/failed | asynq task state |
| Read application | None | `GET /api/v1/applications/:id` → record + parsed fields | role-scoped later |

---

## Mandatory Reading (Sprint 0 patterns — read before coding)

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/pkg/httpx/response.go` | 16–36 | Response envelope — every handler returns `OK`/`Created`/`Fail` |
| P0 | `backend/pkg/httpx/errors.go` | 13–40 | Central error handler — handlers return `*fiber.Error` for 4xx, never leak 5xx |
| P0 | `backend/cmd/api/main.go` | 41–91 | Dependency wiring + `bootstrap.Retry` pattern; where to register routes + asynq client |
| P0 | `backend/cmd/worker/main.go` | 81–115 | Where the asynq **server** replaces the heartbeat loop; health checker reuse |
| P0 | `backend/internal/health/health.go` | 19–95 | `Checker` interface + concurrent `Evaluate` — add a `queue` checker the same way |
| P1 | `backend/pkg/blob/blob.go` | 21–50 | Blob client — extend with upload/download methods, mirror error wrapping |
| P1 | `backend/pkg/config/config.go` | 13–58 | Config loader — add AI provider + queue settings, fail-fast for required |
| P1 | `backend/internal/middleware/mock_jwt.go` | 6–30 | `UserContextKey` + `DevUser` — read auth context in handlers |
| P2 | `backend/migrations/000001_init_schema.up.sql` | all | candidates/applications columns the pipeline writes to |
| P2 | `.claude/PRPs/plans/completed/sprint-0-foundation.plan.md` | "Conventions" | repository/migration/logging conventions established |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| asynq | github.com/hibiken/asynq | `asynq.NewClient(RedisClientOpt)` to enqueue; `asynq.NewServer` + `ServeMux` to consume; `asynq.NewTask(type, payload, opts...)`; retries/timeout via `asynq.MaxRetry`, `asynq.Timeout`. |
| asynq + go-redis | asynq docs | asynq takes its own `RedisClientOpt{Addr,...}` — derive from the same `REDIS_URL` we already parse. |
| Azure Document Intelligence | learn.microsoft.com — prebuilt-layout, API 2024-11-30 | Analyze by blob URL; poll the operation; output markdown; per-span confidence — flag `< 0.7`. |
| Azure OpenAI Go | github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai | Chat completions with `Temperature=0`, JSON response format; deployment name `hr-screening-gpt4o`. |
| Fiber file upload | gofiber.io/docs | `c.FormFile("resume")` → `*multipart.FileHeader`; validate size (≤10MB) and content-type before reading. |

### Research Notes
```
KEY_INSIGHT: asynq result/state polling.
APPLIES_TO: GET /api/v1/ai/jobs/:job_id.
GOTCHA: Use asynq.Inspector (NewInspector(RedisClientOpt)) to look up a task by queue+id and report State. Persist our own job_id↔application_id mapping (applications.queue_task_id) so status maps back to a record.

KEY_INSIGHT: OCR confidence gate.
APPLIES_TO: pipeline Step 1.
GOTCHA: When overall confidence < 0.7, do NOT abort — set applications.needs_manual_review=true, still attempt parse, and surface the flag (PRP §8 Step 1 fallback).

KEY_INSIGHT: GPT-4o determinism.
APPLIES_TO: Step 2 parse.
GOTCHA: Temperature=0, max_tokens≈2000, and request a strict JSON object. Validate the returned JSON against the F02 schema before persisting; on parse failure, retry once then flag needs_manual_review.
```

---

## Patterns to Mirror

### RESPONSE_ENVELOPE
```go
// SOURCE: backend/pkg/httpx/response.go:24-36
func OK[T any](c *fiber.Ctx, data T) error { return c.Status(fiber.StatusOK).JSON(Envelope[T]{Success: true, Data: data}) }
func Fail(c *fiber.Ctx, status int, msg string) error { return c.Status(status).JSON(Envelope[any]{Success: false, Error: msg}) }
```

### ERROR_HANDLING
```go
// SOURCE: backend/pkg/httpx/errors.go:13-40 — handlers return *fiber.Error for client errors; 5xx are logged + masked centrally.
return fiber.NewError(fiber.StatusBadRequest, "resume file is required")
```

### REPOSITORY_PATTERN (first real implementation — established conceptually in S0)
```go
// internal/candidates/repository.go — pool injected, never global (mirrors S0 convention).
type Repository interface {
    Create(ctx context.Context, c Candidate) (Candidate, error)
    FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error)
}
type pgRepository struct{ pool *pgxpool.Pool }
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }
```

### CONFIG_EXTENSION
```go
// SOURCE: backend/pkg/config/config.go:26-50 — add fields, fail-fast on required (real-Azure mode only).
AIProvider string // "mock" | "azure"   (default "mock")
```

### HEALTH_CHECKER (add a queue checker)
```go
// SOURCE: backend/internal/health/health.go:33 — same NewChecker shape.
health.NewChecker("queue", func(ctx context.Context) error { return asynqInspector.Ping() /* or redis ping */ })
```

### LOGGING / STRUCTURED ERRORS
```go
// Mirror S0: log.Error().Err(err).Str("step","ocr").Str("application_id",id).Msg("..."); wrap errors with fmt.Errorf("%w").
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/go.mod` | UPDATE | add `github.com/hibiken/asynq`, `github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai`, doc-intelligence SDK |
| `backend/migrations/000003_pipeline_columns.up.sql` / `.down.sql` | CREATE | add pipeline cols: `applications.raw_file_blob_url`, `raw_file_type`, `ocr_text_blob_url`, `parsed_profile_blob_url`, `ocr_confidence`, `needs_manual_review`, `queue_task_id`, `parsed_at` |
| `backend/pkg/config/config.go` | UPDATE | AI provider toggle + Azure endpoints/keys + queue settings |
| `backend/pkg/blob/blob.go` | UPDATE | add `Upload(ctx, name, data, contentType)` + `Download(ctx, name)` + `SignedURL` (stub) |
| `backend/pkg/queue/queue.go` | CREATE | asynq client + inspector wrappers built from `REDIS_URL`; task type constants |
| `backend/pkg/queue/tasks.go` | CREATE | `ProcessApplicationPayload` + `NewProcessApplicationTask` |
| `backend/internal/candidates/{model,repository,repository_test}.go` | CREATE | Candidate type + pg repo (Create/FindByID/UpdateProfileFields) |
| `backend/internal/applications/{model,repository,service,handler,repository_test,handler_test}.go` | CREATE | Application type, pg repo, intake service (blob upload + enqueue), Fiber handlers |
| `backend/internal/ai/ocr.go` | CREATE | `OCR` interface + `Result{Text, Confidence}` |
| `backend/internal/ai/parser.go` | CREATE | `Parser` interface + `Profile` (F02 schema) |
| `backend/internal/ai/mock.go` | CREATE | deterministic mock OCR + parser (default in dev/CI) |
| `backend/internal/ai/azure_ocr.go` | CREATE | Azure Document Intelligence impl |
| `backend/internal/ai/azure_parser.go` | CREATE | Azure OpenAI GPT-4o impl |
| `backend/internal/ai/factory.go` | CREATE | select mock vs azure from config; the only place provider choice lives |
| `backend/internal/pipeline/process.go` | CREATE | Step 1 OCR → Step 2 Parse → persist; the asynq handler |
| `backend/internal/pipeline/process_test.go` | CREATE | end-to-end pipeline test with mock providers + repo |
| `backend/internal/ai/profile.go` | CREATE | `Profile` struct + JSON validation against F02 |
| `backend/cmd/api/main.go` | UPDATE | build queue client; register intake + job-status routes; add queue health checker |
| `backend/cmd/worker/main.go` | UPDATE | replace heartbeat loop with asynq server consuming `process_application` |
| `backend/internal/applications/routes.go` | CREATE | route registration helper |

## NOT Building (Sprint 2+)
- **Dedup / merge** (F09, pipeline Step 3) — candidate matching is Sprint 2.
- **Scoring engine** (F03, Step 4) + **Must-Have gate** (Step 5).
- **Branch assignment** (F04, Step 6).
- **Notifications** (LINE, Step 7) — pipeline ends at `parsed`, not `notify`.
- **Channel-specific webhooks** (Power Automate, JobBKK/JobDB polling) — only the generic intake API + manual upload in S1.
- **Career Portal / HR Dashboard UI**, PeopleSoft, Azure AI Search, PDPA UI.
- **Real RBAC / row-level security** — mock super_admin from S0 is sufficient for S1.

---

## Step-by-Step Tasks

### Task 1: Pipeline schema migration
- **ACTION**: Create `000003_pipeline_columns.up.sql` / `.down.sql`.
- **IMPLEMENT**: `ALTER TABLE applications ADD COLUMN` for `raw_file_blob_url TEXT`, `raw_file_type VARCHAR(10)`, `ocr_text_blob_url TEXT`, `parsed_profile_blob_url TEXT`, `ocr_confidence NUMERIC(4,3)`, `needs_manual_review BOOLEAN DEFAULT FALSE`, `queue_task_id VARCHAR(120)`, `parsed_at TIMESTAMPTZ`. Index `queue_task_id`. Down drops them.
- **MIRROR**: MIGRATION_NAMING (S0).
- **GOTCHA**: Additive only — never modify 000001/000002 (already applied). `make migrate-up` must stay forward-clean.
- **VALIDATE**: `make migrate-up` then `migrate down 1` round-trips; `\d applications` shows new columns.

### Task 2: Config extension
- **ACTION**: Update `pkg/config/config.go`.
- **IMPLEMENT**: Add `AIProvider` (default `mock`), `AzureOpenAIEndpoint/Key/Deployment`, `AzureDocIntelEndpoint/Key`. Require the Azure vars **only when** `AIProvider=="azure"` (fail-fast in that mode; ignored in mock).
- **MIRROR**: CONFIG_EXTENSION (`config.go:26-50`).
- **GOTCHA**: Default mock so local/CI need zero Azure secrets.
- **VALIDATE**: `AIProvider` unset → mock; `=azure` without keys → clear error. Add a config test.

### Task 3: Blob upload/download
- **ACTION**: Update `pkg/blob/blob.go`.
- **IMPLEMENT**: `Upload(ctx, name string, data []byte, contentType string) (url string, err error)` via `UploadBuffer`; `Download(ctx, name) ([]byte, error)`. Return the blob URL for persistence.
- **MIRROR**: blob error wrapping (`blob.go:34-50`).
- **GOTCHA**: Set content-type; key resumes as `resumes/{application_id}/{filename}` to avoid collisions.
- **VALIDATE**: Round-trip upload+download against Azurite in a test (or covered by pipeline test).

### Task 4: Queue package (asynq)
- **ACTION**: Create `pkg/queue/queue.go` + `tasks.go`.
- **IMPLEMENT**: `redisOpt(url) asynq.RedisClientOpt` from the parsed `REDIS_URL`; `NewClient`, `NewInspector`. Task type const `TypeProcessApplication = "application:process"`. `ProcessApplicationPayload{ApplicationID, BlobName, FileType}` + `NewProcessApplicationTask(payload)` with `asynq.MaxRetry(3)`, `asynq.Timeout(90*time.Second)`.
- **MIRROR**: derive from existing `REDIS_URL` (don't add a new var).
- **GOTCHA**: asynq needs host:port, not a full `redis://` URL — parse it.
- **VALIDATE**: `go build`; enqueue returns a task id.

### Task 5: AI interfaces + profile schema
- **ACTION**: Create `internal/ai/ocr.go`, `parser.go`, `profile.go`.
- **IMPLEMENT**: `OCR.Extract(ctx, fileBytes, fileType) (OCRResult{Text string, Confidence float64}, error)`; `Parser.Parse(ctx, text string, positionCtx string) (Profile, error)`; `Profile` struct exactly matching F02 (personal{name,phone,email,address,age,id_card}, experience[], education[], skills[], languages[], desired_position) + `Validate()`.
- **GOTCHA**: Keep interfaces tiny so mock + azure both satisfy them cleanly.
- **VALIDATE**: compiles; `Profile.Validate()` rejects empty name.

### Task 6: Mock providers
- **ACTION**: Create `internal/ai/mock.go`.
- **IMPLEMENT**: `mockOCR` returns deterministic markdown text + confidence 0.95; `mockParser` returns a fixed valid `Profile` derived from input length (deterministic, no randomness — `Math.random` equivalent banned for reproducibility).
- **GOTCHA**: Determinism is what makes CI assertions stable.
- **VALIDATE**: used by pipeline test.

### Task 7: Azure providers
- **ACTION**: Create `internal/ai/azure_ocr.go`, `azure_parser.go`, `factory.go`.
- **IMPLEMENT**: `azureOCR` calls Document Intelligence prebuilt-layout (markdown output, read overall confidence); `azureParser` calls Azure OpenAI chat completions (Temperature 0, JSON response, system prompt "You are a Thai/English CV parser…"), unmarshal+validate into `Profile`. `factory.New(cfg)` returns `(OCR, Parser)` — mock or azure per `cfg.AIProvider`.
- **GOTCHA**: Don't fail the build/boot in mock mode if Azure keys are absent — factory only constructs azure clients when selected.
- **VALIDATE**: `go build` with and without Azure env; unit-test factory selection.

### Task 8: Candidate domain
- **ACTION**: Create `internal/candidates/{model,repository,repository_test}.go`.
- **IMPLEMENT**: `Candidate` struct (maps 000001 columns); `Repository` with `Create`, `FindByID`, `UpdateProfileFields(ctx,id, name,phone,email,dob,province,...)`. pgx implementation.
- **MIRROR**: REPOSITORY_PATTERN; error wrapping.
- **GOTCHA**: `id_card` UNIQUE — on conflict surface a clear error (full dedup is Sprint 2; for now insert may collide → return typed error).
- **VALIDATE**: repository_test against compose Postgres (build-tagged `integration`) or pgxmock for unit.

### Task 9: Application domain + intake service
- **ACTION**: Create `internal/applications/{model,repository,service,repository_test}.go`.
- **IMPLEMENT**: `Application` struct (incl. new pipeline cols); repo `Create`, `FindByID`, `SetParseResults(...)`, `SetQueueTaskID`. `Service.Intake(ctx, in)` = validate → create candidate (minimal) → upload raw file to Blob → create application(status `pending`) → enqueue asynq task → store `queue_task_id` → return ids.
- **MIRROR**: container/service split (presentational handler stays thin).
- **GOTCHA**: Wrap candidate+application creation so a failed enqueue doesn't orphan a half-written record — create rows, enqueue, and if enqueue fails mark application `failed` (log, return 502).
- **VALIDATE**: service test with mock queue + Azurite.

### Task 10: Intake + status handlers + routes
- **ACTION**: Create `internal/applications/{handler,routes,handler_test}.go`.
- **IMPLEMENT**: `POST /api/v1/applications` (multipart `resume` + form fields `position_id`, `source_channel`, candidate fields) → validate file (≤10MB, pdf/docx/jpg/png) → `Service.Intake` → `Created` with `{application_id, job_id}`. `GET /api/v1/applications/:id`. `GET /api/v1/ai/jobs/:job_id` → inspector state. `RegisterRoutes(app, svc)`.
- **MIRROR**: RESPONSE_ENVELOPE + ERROR_HANDLING (return `fiber.NewError` for validation).
- **GOTCHA**: Reject unsupported content-type with 415; missing file with 400.
- **VALIDATE**: handler_test with httptest/Fiber app + mock service: valid upload → 201; no file → 400; oversize → 413.

### Task 11: Pipeline (asynq handler)
- **ACTION**: Create `internal/pipeline/{process,process_test}.go`.
- **IMPLEMENT**: `Processor{ocr, parser, blob, candidates, applications}`; `HandleProcessApplication(ctx, t *asynq.Task)`: unmarshal payload → download raw file → OCR (set `ocr_confidence`, `needs_manual_review` if `<0.7`) → upload OCR text to Blob → Parse → validate → `candidates.UpdateProfileFields` + upload `parsed_profile.json` → `applications.SetParseResults(status="parsed", parsed_at=now)`. On any step error: log with step context, return err (asynq retries up to 3) and after final failure set status `failed`.
- **MIRROR**: structured logging + `%w` wrapping.
- **GOTCHA**: Idempotency — a retried task must not duplicate blobs/rows; key blobs by application_id and use upserts/updates.
- **VALIDATE**: process_test: mock providers + real Azurite + compose Postgres → application ends `parsed`, profile fields populated, `parsed_profile.json` exists.

### Task 12: Wire api entrypoint
- **ACTION**: Update `cmd/api/main.go`.
- **IMPLEMENT**: After deps, build `queue.NewClient` + `Inspector`; construct candidate/application repos + intake service; `applications.RegisterRoutes(app, svc)`; add a `queue` health checker.
- **MIRROR**: `main.go:41-91` wiring + `app.Get` registration.
- **GOTCHA**: Close the asynq client on shutdown alongside pools.
- **VALIDATE**: `/health` includes `queue: ok`; intake route reachable.

### Task 13: Wire worker entrypoint
- **ACTION**: Update `cmd/worker/main.go`.
- **IMPLEMENT**: Replace the heartbeat `for{}` with `asynq.NewServer(redisOpt, Config{Concurrency:10, Queues:{default:1}})` + `ServeMux` mapping `TypeProcessApplication → processor.HandleProcessApplication`. Keep `/health` on `:8081` (now also reports queue). Graceful `srv.Shutdown()` on SIGTERM.
- **MIRROR**: `worker/main.go:81-115` (health + signal handling).
- **GOTCHA**: asynq server blocks — run it; trigger shutdown on signal. Don't lose the health probe goroutine.
- **VALIDATE**: end-to-end — POST a resume, worker logs the task, application becomes `parsed`.

### Task 14: Docs + Make
- **ACTION**: Update README + add `make` helper for an end-to-end smoke (`curl` upload a sample PDF).
- **VALIDATE**: README intake example works against a fresh `make up && make migrate-up`.

---

## Testing Strategy

### Unit / Integration Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| config azure-required | `AIProvider=azure`, no keys | error | Yes |
| profile validate | empty name | error | Yes |
| factory selection | provider=mock / azure | returns matching impls | No |
| intake service happy | valid file + fields | candidate+application created, task enqueued, ids returned | No |
| intake enqueue failure | queue down | application marked `failed`, 502 | Yes |
| handler no file | multipart w/o `resume` | 400 | Yes |
| handler oversize | 11MB file | 413 | Yes |
| handler bad type | `.exe` | 415 | Yes |
| pipeline happy (mock) | enqueued task | status `parsed`, profile fields set, `parsed_profile.json` in Blob | No |
| pipeline low-confidence | mock OCR conf 0.5 | `needs_manual_review=true`, still parses | Yes |
| pipeline parse failure | mock parser error | retried then status `failed` | Yes |

### Edge Cases Checklist
- [ ] Empty / corrupt file bytes
- [ ] Unsupported content-type (415)
- [ ] File > 10MB (413)
- [ ] Redis/queue down at enqueue
- [ ] OCR confidence < 0.7 → manual-review flag
- [ ] Parser returns invalid JSON → retry → fail
- [ ] Task retried (idempotency — no duplicate blobs/rows)
- [ ] `GET /ai/jobs/:id` for unknown id → 404

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./... && golangci-lint run
```
EXPECT: zero issues.

### Unit + Integration Tests
```bash
make up && make migrate-up          # Azurite + Postgres needed for pipeline/repo tests
cd backend && go test ./... -cover
```
EXPECT: all pass; new packages (ai, pipeline, applications, candidates, queue) covered ≥80% on tested units.

### Build
```bash
cd backend && go build ./... && docker compose build
```
EXPECT: api + worker build.

### End-to-End (the Sprint 1 gate)
```bash
make up && make migrate-up
curl -i -F "resume=@testdata/sample_cv.pdf" -F "position_id=<uuid>" -F "source_channel=walk_in" \
  -F "full_name=สมชาย ใจดี" localhost:8080/api/v1/applications
#   → 201 {application_id, job_id}
sleep 5
curl -s localhost:8080/api/v1/ai/jobs/<job_id>     # → completed
curl -s localhost:8080/api/v1/applications/<id>    # → status "parsed", parsed fields populated
docker compose exec -T postgres psql -U hruser -d hr_db -c \
  "SELECT status, ocr_confidence, needs_manual_review, parsed_profile_blob_url FROM applications;"
```
EXPECT: application reaches `parsed`; profile fields written; `parsed_profile.json` blob exists.

### Manual Validation
- [ ] Upload PDF, DOCX, and an image — all reach `parsed` (mock provider).
- [ ] `AIProvider=azure` with real keys parses a real CV (POC check, optional locally).
- [ ] Worker survives a forced parser error (task retries, then `failed`).

---

## Acceptance Criteria
- [ ] `POST /api/v1/applications` stores the file in Blob + creates candidate/application + enqueues a job.
- [ ] Worker consumes the job, runs OCR + parse, and writes a structured F02 profile.
- [ ] Application transitions `pending → parsed` (or `failed` on exhausted retries).
- [ ] `GET /api/v1/ai/jobs/:job_id` reports task state; `GET /api/v1/applications/:id` shows parsed data.
- [ ] AI providers swap via config (mock default, azure opt-in) with no code change.
- [ ] All validation levels pass; coverage ≥80% on new tested packages.
- [ ] No new secrets committed; mock mode needs zero Azure keys.

## Completion Checklist
- [ ] Mirrors S0 conventions (envelope, error handler, repository, logging, migration naming).
- [ ] Pipeline steps log with step + application_id context.
- [ ] Idempotent task handling.
- [ ] `queue` added to both health endpoints.
- [ ] README intake flow verified.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| asynq RedisClientOpt vs go-redis URL mismatch | Med | Med | Single `redisOpt(url)` parser in `pkg/queue`; unit-test it. |
| Azure SDK surface differs from docs | Med | Med | Isolate behind `ai` interfaces; mock is the default path so build/CI never depend on Azure. |
| Pipeline non-idempotent on retry | Med | High | Key blobs by application_id; use updates/upserts; test the retry path. |
| 60s SLA at load | Low | Med | asynq concurrency=10, per-task 90s timeout; load test deferred to S2/POC. |
| id_card uniqueness collisions pre-dedup | Med | Med | Return typed conflict error now; full dedup/merge is Sprint 2 (F09). |

## Notes
- Pipeline intentionally **stops at `parsed`**. Steps 3–7 (dedup, score, gate, branch-assign, notify) are Sprint 2/later — the `process_application` task will be extended, not replaced.
- The `ai` package is the single seam for provider choice; nothing else imports Azure SDKs directly.
- Reuses `REDIS_URL` for asynq — no new infra in Sprint 1 (Sprint 0 stack is sufficient).
