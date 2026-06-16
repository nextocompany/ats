# Plan: Bulk CV intake + load testing

## Summary
Add an HR-facing **bulk CV upload** (one position, many resume files in a single
submission) that creates N applications and enqueues N pipeline jobs, plus a
**load-test harness** to validate the system under concurrent intake and to measure
async pipeline throughput. Reuses the existing single-intake `Service.Intake` and
asynq pipeline — the candidate name is filled from the parsed profile, so bulk
uploads need no per-file data entry.

## User Story
As an **HR user**, I want to drag-and-drop a batch of CVs for a position and have
them all screened automatically, so that I can process 20–30+ real CVs at once
instead of one at a time — and as an **operator**, I want a repeatable load test so
I know the system holds up under concurrent / bulk use.

## Problem → Solution
Intake is single-file only (`internal/applications/handler.go:62`), candidate-facing
(career-portal), and there is no load harness. → A dashboard bulk-upload endpoint +
UI creates many applications at once; a k6 script + a pipeline-throughput
measurement prove behaviour under load.

## Metadata
- **Complexity**: Large
- **Source PRD**: `.claude/PRPs/plans/delivery-scope-roadmap.md` (PRP-2)
- **PRD Phase**: PRP-2 (P0) — unblocks PRP-3 (feeds real CVs + drives load)
- **Estimated Files**: ~16 (backend + dashboard frontend + loadtest)

---

## Key enabling fact (verified)
The pipeline **already overwrites** candidate name/phone/email/address from the
parsed profile (`internal/pipeline/process.go:177` `UpdateProfileFields`). So bulk
intake can create each candidate with a **placeholder name = filename**, and the
real name appears after parse. No per-file data entry, no schema change.

---

## UX Design

### Before
```
HR → (no upload path on dashboard). Candidates self-apply on career-portal,
     one CV at a time, typing name/phone/email per submission.
```

### After
```
HR dashboard → /applications/bulk
  1. pick Position (dropdown)
  2. drag-drop N resume files (pdf/docx/jpg/png, ≤10MB each, ≤50/batch)
  3. Submit → progress
  4. result table: ✓ created (filename → app link) / ✗ failed (filename → reason)
Each file flows through the normal OCR→parse→score→assign pipeline; the parsed
name replaces the placeholder. Notifications (PRP-1) fire per scored application.
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| HR adds candidates | none (self-apply only) | bulk upload page | position-scoped batch |
| Position selection | n/a | new `GET /positions` dropdown | active positions |
| Throughput visibility | none | load harness + drain metric | ops artifact |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/handler.go` | 53-114 | Single Intake multipart parse + type/size validation — mirror per-file |
| P0 | `backend/internal/applications/service.go` | 25-140 | `Intake` / `IntakeInput` to reuse per file (placeholder name) |
| P0 | `backend/pkg/queue/tasks.go` | 27-60 | Task payload + enqueue policy (each file → one task) |
| P0 | `backend/internal/pipeline/process.go` | 142-184 | Pipeline fills candidate name from parse (placeholder is fine) |
| P0 | `backend/internal/members/handler.go` | 26-36, 72-112 | Role-gate pattern (`memberAdminRoles`, Forbidden) to mirror for bulk auth |
| P1 | `backend/internal/applications/dashboard_handler.go` | 71-82 | `RegisterDashboardRoutes` + `scopeFrom` — where to mount routes |
| P1 | `backend/internal/positions/model.go` | all | Position fields (TitleTH/TitleEN/id) for the list endpoint |
| P1 | `backend/internal/positions/repository.go` | all | Existing position reads to add a `ListActive` method |
| P1 | `backend/cmd/api/main.go` | 181-316 | Wiring (positionRepo exists at 183; mount new handlers) |
| P1 | `backend/cmd/worker/main.go` | 42, 150 | `workerConcurrency = 10` const → make env-configurable for load tuning |
| P0 | `career-portal/lib/api.ts` | 55-80 | Multipart upload client (`form: FormData`) — mirror into dashboard api.ts |
| P0 | `career-portal/lib/queries.ts` | 30-60 | `buildApplyForm` FormData builder pattern |
| P1 | `frontend/lib/api.ts` | 21-56 | Dashboard client is JSON-only — add a `postForm` method |
| P1 | `frontend/lib/queries.ts` | 86-115 | Query/mutation patterns (useApplications/useApplication) to mirror |
| P1 | `frontend/components/resume/ScheduleInterviewDialog.tsx` | all | Dashboard form + Select + toast pattern to mirror for the upload UI |
| P2 | `frontend/lib/roles.ts` | all | Role-gate helpers (mirror `canRecordInterviewFeedback`) for UI gating |
| P2 | `backend/e2e/harness_test.go` | all | Existing e2e stack — reference for a smoke/throughput check |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| k6 load testing | grafana.com/docs/k6 | `http.post` with `http.file()` for multipart; `options.scenarios` for VUs/duration; `thresholds` to assert p95 + error rate (fails the run → CI-gateable) |
| asynq concurrency | github.com/hibiken/asynq | Server `Concurrency` caps in-flight tasks; raising it only helps until the downstream (Azure AI) rate-limits |
| Azure OpenAI TPM | Azure docs + project memory | The async bottleneck is Azure OpenAI TPM + DocIntel rate, NOT the HTTP intake. Fit analysis already needed a TPM bump (gpt4omini-gs 10→50). Expect to tune TPM + worker concurrency together. |

> GOTCHA: Load-testing the **intake API** mostly measures upload+enqueue (fast). The
> real capacity question is **pipeline drain rate** (OCR+LLM per CV), which is
> async in the worker. The harness must measure both: (a) API p95/error under
> concurrent uploads, (b) time for a batch to reach a terminal status (scored/
> rejected) → CVs/min.

---

## Patterns to Mirror

### MULTIPART_INTAKE (per-file validation)
```go
// SOURCE: backend/internal/applications/handler.go:62-95
fileHeader, err := c.FormFile("resume")
if fileHeader.Size > maxResumeBytes { return fiber.NewError(fiber.StatusRequestEntityTooLarge, ...) }
contentType := fileHeader.Header.Get("Content-Type")
fileType, ok := contentTypeToFileType[contentType]   // pdf/docx/jpeg/png allowlist
positionID, err := uuid.Parse(c.FormValue("position_id"))
f, _ := fileHeader.Open(); data, _ := io.ReadAll(f)
```

### REUSE_INTAKE_SERVICE
```go
// SOURCE: backend/internal/applications/service.go:64
res, err := svc.Intake(ctx, applications.IntakeInput{
	CandidateName: placeholder,        // filename; pipeline overwrites from parse
	SourceChannel: "bulk_upload",
	PositionID:    positionID,
	FileName:      fileHeader.Filename, FileType: fileType,
	ContentType:   contentType, FileBytes: data,
})
```

### ROLE_GATE
```go
// SOURCE: backend/internal/members/handler.go:28,72-112
var bulkIntakeRoles = map[string]bool{"super_admin": true, "hr_manager": true, "sgm": true, "hr_staff": true}
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
if !bulkIntakeRoles[u.Role] { return fiber.NewError(fiber.StatusForbidden, "insufficient role to upload CVs") }
```

### ROUTE_REGISTRATION
```go
// SOURCE: backend/internal/applications/dashboard_handler.go:71-77 + feedback_handler.go
func RegisterBulkRoutes(app *fiber.App, h *BulkHandler) {
	app.Post("/api/v1/applications/bulk-intake", h.BulkIntake)
}
```

### FRONTEND_MULTIPART_CLIENT
```ts
// SOURCE: career-portal/lib/api.ts:55-80 (mirror into frontend/lib/api.ts)
async postForm<T>(path: string, form: FormData): Promise<{ data: T }> {
  const res = await fetch(`${API_URL}${path}`, { method: "POST", credentials: "include", body: form });
  // ...same error handling as request()...
}
```

### FRONTEND_FORM_UI
```tsx
// SOURCE: frontend/components/resume/ScheduleInterviewDialog.tsx
// Select (position) + file input + Button + toast.success/error + useMutation
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/applications/bulk_handler.go` | CREATE | Multipart bulk-intake handler (role-gated, per-file loop, partial-failure result) |
| `backend/internal/applications/bulk_handler_test.go` | CREATE | Role gate, bad position, mixed valid/invalid files, count caps |
| `backend/internal/positions/handler.go` | CREATE | `GET /api/v1/positions` list for the picker |
| `backend/internal/positions/repository.go` | UPDATE | Add `ListActive(ctx) ([]Position, error)` |
| `backend/internal/positions/handler_test.go` | CREATE | List shape |
| `backend/cmd/api/main.go` | UPDATE | Mount bulk + positions routes |
| `backend/cmd/worker/main.go` | UPDATE | `WORKER_CONCURRENCY` env (default 10) for load tuning |
| `backend/pkg/config/config.go` | UPDATE | `WorkerConcurrency int` env |
| `frontend/lib/api.ts` | UPDATE | Add `postForm` multipart method |
| `frontend/lib/types.ts` | UPDATE | `Position`, `BulkIntakeResult` types |
| `frontend/lib/queries.ts` | UPDATE | `usePositions`, `useBulkIntake` |
| `frontend/lib/roles.ts` | UPDATE | `canBulkUpload(role)` mirror |
| `frontend/app/(app)/applications/bulk/page.tsx` | CREATE | Bulk upload page (position + dropzone + results) |
| `frontend/components/applications/BulkUpload.tsx` | CREATE | Upload component (multi-file, progress, result table) |
| `frontend/components/shell/nav-config.tsx` | UPDATE | Add "Bulk upload" nav entry (role-gated) |
| `loadtest/intake-load.js` | CREATE | k6 script: concurrent intake, thresholds |
| `loadtest/README.md` | CREATE | Run recipe + targets + pipeline-drain measurement + bottleneck notes |

## NOT Building
- No CSV/Excel/ZIP archive import (multi-file form upload only).
- No per-candidate metadata entry in bulk (name/phone/email come from parse).
- No new async "bulk job" entity — each file enqueues a normal pipeline task.
- No auto position detection from CV content (HR picks one position per batch).
- No dedup-preview UI at upload (existing pipeline dedup still runs per application).
- No change to the career-portal candidate apply flow.
- Load test is an **operator artifact** (run on demand) — not wired into CI gating in this PRP.

---

## Step-by-Step Tasks

### Task 1: Positions list repo + endpoint
- **ACTION**: Add `ListActive(ctx) ([]Position, error)` to positions repo; create `positions.Handler` with `GET /api/v1/positions` returning `[{id,title_th,title_en}]`.
- **IMPLEMENT**: Query active positions (mirror existing repo queries). Handler returns `httpx.OK`. Read-open to any authed HR (no role gate needed — it's reference data).
- **MIRROR**: ROUTE_REGISTRATION; existing repo query style.
- **IMPORTS**: fiber, httpx, pgx.
- **GOTCHA**: Master JD has duplicate `title_en` on prod (known) — return `id` so the picker keys on id, and show `title_th` primarily.
- **VALIDATE**: `go test ./internal/positions/...`; `go build ./...`.

### Task 2: Bulk intake handler
- **ACTION**: Create `BulkHandler` with `BulkIntake` — role-gated, parses multipart `position_id` + repeated `resumes`, loops per file calling `svc.Intake` with placeholder name.
- **IMPLEMENT**: `form, _ := c.MultipartForm(); files := form.File["resumes"]`. Enforce `len(files) >= 1 && <= maxBulkFiles (50)`. Per file: validate type/size (reuse `contentTypeToFileType` + `maxResumeBytes`); on per-file error append to `failed`, continue; on success append `{filename, application_id}`. Placeholder name = filename without extension (fallback "ผู้สมัคร"). Return `{total, succeeded, failed_count, created:[], failed:[]}`.
- **MIRROR**: MULTIPART_INTAKE, REUSE_INTAKE_SERVICE, ROLE_GATE.
- **IMPORTS**: fiber, uuid, io, path/filepath, middleware, httpx, the intake `*Service`.
- **GOTCHA**: One bad file must NOT abort the batch (collect + continue). Cap total request body — set Fiber `BodyLimit` high enough on the route group or rely on per-file 10MB × 50 ≈ 500MB → set an explicit `maxBulkFiles` and document. `SourceChannel="bulk_upload"`.
- **VALIDATE**: handler test (below).

### Task 3: Mount routes (api)
- **ACTION**: In `cmd/api/main.go` register `positions.RegisterRoutes` + `applications.RegisterBulkRoutes` (bulk handler built from `intakeSvc`).
- **IMPLEMENT**: `applications.RegisterBulkRoutes(app, applications.NewBulkHandler(intakeSvc))` and `positions.RegisterRoutes(app, positions.NewHandler(positionRepo))`. `intakeSvc` + `positionRepo` already exist (lines 197, 183).
- **MIRROR**: existing registrations (lines 200, 287, 296).
- **GOTCHA**: Route ordering — `/applications/bulk-intake` is a fixed segment; safe alongside `/applications/:id/...`. EnforceOrigin (global) already covers the POST.
- **VALIDATE**: `go build ./...`; route returns 401 unauth (auth gate), 403 wrong role.

### Task 4: Worker concurrency env
- **ACTION**: Replace the `workerConcurrency = 10` const with `cfg.WorkerConcurrency` (`WORKER_CONCURRENCY`, default 10).
- **IMPLEMENT**: config field + `getenvInt`; use in `asynq.Config{Concurrency: cfg.WorkerConcurrency}`.
- **MIRROR**: CONFIG_VAR pattern (config.go).
- **GOTCHA**: Raising concurrency past the Azure OpenAI TPM ceiling causes 429s, not speedups — document in loadtest README.
- **VALIDATE**: `go build ./cmd/worker`.

### Task 5: Dashboard multipart client
- **ACTION**: Add `postForm` to `frontend/lib/api.ts` (fetch with FormData body, `credentials: "include"`, same error mapping as `request`).
- **IMPLEMENT**: Mirror career-portal `api.ts` upload. Do NOT set Content-Type (browser sets the multipart boundary).
- **MIRROR**: FRONTEND_MULTIPART_CLIENT.
- **GOTCHA**: Reuse the existing `ApiError` + non-2xx handling; include credentials so the HR session cookie is sent.
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 6: Frontend types + queries + role gate
- **ACTION**: Add `Position` + `BulkIntakeResult` types; `usePositions` (GET) + `useBulkIntake` (postForm) hooks; `canBulkUpload(role)` in roles.ts.
- **IMPLEMENT**: `usePositions` → `["positions"]`; `useBulkIntake` → mutation taking `{positionId, files}` → builds FormData (`position_id` + each `resumes`), invalidates `["applications"]` on success. `canBulkUpload` mirrors `bulkIntakeRoles`.
- **MIRROR**: queries.ts patterns; roles.ts `canRecordInterviewFeedback`.
- **VALIDATE**: `tsc`.

### Task 7: Bulk upload UI
- **ACTION**: Create `BulkUpload.tsx` + `/applications/bulk` page: position Select, multi-file `<input type="file" multiple>` (drag-drop optional), submit, per-file result table (✓ link to `/applications/{id}` / ✗ reason). Gate the page/nav on `canBulkUpload(me.role)`.
- **IMPLEMENT**: Use `useMe` for role gate, `usePositions` for the dropdown, `useBulkIntake` for submit; toast on done with `succeeded/total`. Disable submit while pending.
- **MIRROR**: FRONTEND_FORM_UI (ScheduleInterviewDialog), InterviewFeedbackPanel role-gating.
- **GOTCHA**: Validate client-side file count (≤50) + types before submit for a fast error; the server re-validates. Show partial results even when some failed.
- **VALIDATE**: `tsc` + `next build`.

### Task 8: Nav entry
- **ACTION**: Add a role-gated "Bulk upload" link to `nav-config.tsx`.
- **MIRROR**: existing nav entries + role gating.
- **VALIDATE**: `next build`.

### Task 9: Load-test harness
- **ACTION**: Create `loadtest/intake-load.js` (k6) + `loadtest/README.md`.
- **IMPLEMENT**: k6 script: `options.scenarios` ramping VUs (e.g., 10→30), each VU POSTs a sample CV to `/api/v1/applications/bulk-intake` (or single intake) using `http.file()`; `thresholds`: `http_req_duration p(95)<2000`, `http_req_failed rate<0.01`. Parameterize target URL + auth cookie via env. README: run recipe, **initial targets** (30 concurrent uploads, API p95 < 2s, error < 1%), and the **pipeline-drain measurement** (enqueue a known batch via bulk, poll `GET /api/v1/applications?status=scored|rejected` counts over time → CVs/min) + bottleneck guidance (Azure OpenAI TPM, DocIntel rate, `WORKER_CONCURRENCY`).
- **MIRROR**: external k6 docs; sample CV from `backend/internal/ai/mock.go` fixtures or a small committed PDF.
- **GOTCHA**: Run load tests against a **staging/throwaway** dataset, never seeding junk into the real prod hr_db. Document a cleanup query. Auth: the endpoints require an HR session — script must carry a valid cookie/token (document how to obtain).
- **VALIDATE**: `k6 run loadtest/intake-load.js` against a local/staging stack completes and prints thresholds.

### Task 10: Backend tests
- **ACTION**: `bulk_handler_test.go` (role gate 403, missing position 400, mixed valid/invalid files → partial result, >50 files 400) + `positions/handler_test.go` (list shape). Use a fake intake service / narrow interface so no DB.
- **IMPLEMENT**: Define a narrow `bulkIntaker` interface (`Intake(ctx, IntakeInput) (IntakeResult, error)`) the handler accepts (Service satisfies it) → fake in tests. Mirror `feedback_test.go` test-app + injected DevUser.
- **MIRROR**: `feedback_test.go` (feedbackTestApp + postFeedback helpers).
- **GOTCHA**: Reuse the `multipartBody` helper style from `handler_test.go:16` but for multiple `resumes` parts.
- **VALIDATE**: `go test ./internal/applications/... ./internal/positions/...`.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Bulk role gate | hr_staff allowed / auditor | 403 for auditor | yes |
| Bulk missing position | no position_id | 400 | yes |
| Bulk empty files | 0 resumes | 400 | yes |
| Bulk over cap | 51 files | 400 | yes |
| Bulk mixed | 2 ok + 1 .exe | 207-style: succeeded=2, failed=1 | yes |
| Bulk placeholder name | valid file | Intake called with non-empty name | — |
| Positions list | — | array of {id,title_th} | — |

### Edge Cases Checklist
- [ ] One corrupt/oversized file in a batch (others still created)
- [ ] Unsupported type in a batch (reported, not fatal)
- [ ] Max batch size (50) accepted; 51 rejected
- [ ] Duplicate CVs in a batch (pipeline dedup handles downstream)
- [ ] Concurrent batches from two HR users (no shared state)
- [ ] Auth cookie missing → 401; wrong role → 403
- [ ] Azure TPM 429 under load (pipeline retries; surfaced as slower drain, not data loss)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./...
cd frontend && pnpm exec tsc --noEmit
```
EXPECT: zero errors

### Unit Tests
```bash
cd backend && go test ./internal/applications/... ./internal/positions/...
```
EXPECT: pass

### Build
```bash
cd backend && go build ./...
cd frontend && pnpm exec next build
```
EXPECT: clean

### Load test (staging)
```bash
k6 run -e TARGET=https://<staging-api> -e COOKIE="hr_auth=..." loadtest/intake-load.js
```
EXPECT: thresholds pass (p95 < 2s, error < 1%); record CVs/min drain rate.

### Manual Validation
- [ ] HR logs in → /applications/bulk → pick position → drop 20–30 real CVs → submit
- [ ] Result table shows per-file ✓/✗; created apps appear in the inbox
- [ ] After pipeline runs, candidate names are the **parsed** names (not filenames)
- [ ] Scored applications trigger PRP-1 HR notifications
- [ ] auditor role: no bulk nav/page

---

## Acceptance Criteria
- [ ] Bulk endpoint creates N applications + enqueues N jobs, partial-failure safe
- [ ] Positions list endpoint powers the picker
- [ ] Dashboard bulk upload UI works, role-gated
- [ ] Parsed name replaces the placeholder after pipeline
- [ ] Load harness runs + documents API p95/error + pipeline CVs/min + bottleneck
- [ ] `go vet`/tests/build + `tsc`/`next build` green

## Completion Checklist
- [ ] Reuses `Service.Intake` (no duplicated intake logic)
- [ ] Per-file failures collected, never abort the batch
- [ ] Role gate mirrors backend allowlist on both layers
- [ ] Multipart client added without breaking JSON `request`
- [ ] Load test never writes to real prod data (staging + cleanup documented)
- [ ] `WORKER_CONCURRENCY` configurable for tuning

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Azure OpenAI/DocIntel TPM throttles under bulk | High | Med | Document as expected bottleneck; tune TPM + WORKER_CONCURRENCY; pipeline retries (no data loss) |
| Large multipart bodies (50×10MB) | Med | Med | `maxBulkFiles=50` cap; document; consider per-file streaming if needed |
| Load test pollutes prod data | Med | High | Staging only + cleanup query; never point at prod hr_db |
| Placeholder names visible briefly before parse | Low | Low | Inbox shows filename until pipeline completes (seconds–minutes); acceptable, note in UI |
| Duplicate title_en in Master JD picker | Known | Low | Key on id, show title_th |

## Notes
- This PRP intentionally feeds **PRP-3** (Validation/UAT): the 20–30 real-CV parsing-
  accuracy check and the load measurement both run through this bulk path.
- The async pipeline is the real capacity constraint; the k6 API test is necessary
  but not sufficient — the pipeline-drain metric is the headline number for "load จริง".
- If stakeholders later want CSV/ZIP import or background batch jobs, that is a
  follow-up PRP, not this one.
```
