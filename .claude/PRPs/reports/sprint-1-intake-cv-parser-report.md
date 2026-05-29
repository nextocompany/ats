# Implementation Report: Sprint 1 — Intake Pipeline + CV Parser + OCR

## Summary
Built the asynchronous intake pipeline on the Sprint 0 foundation. `POST /api/v1/applications` (multipart) stores the resume in Blob, creates `candidate` + `application` rows, and enqueues an **asynq** job; the worker consumes it, runs **OCR (Step 1)** then **CV parse (Step 2)**, writes structured fields back to the candidate, uploads `parsed_profile.json`, and marks the application `parsed`. AI providers sit behind interfaces with a deterministic **mock default** (zero Azure keys) and real Azure REST clients selectable via `AI_PROVIDER=azure`. Verified end-to-end through the live stack.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | Single-pass; 2 small post-e2e refinements |
| Files Changed | ~28 | 25 created + 6 modified |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Pipeline schema migration (000003) | ✅ | 8 columns + index; reversible |
| 2 | Config extension (AI provider + Azure) | ✅ | mock default; azure fail-fast |
| 3 | Blob upload/download | ✅ | `Upload`/`Download`; idempotent by key |
| 4 | Queue package (asynq) | ✅ | client/inspector from REDIS_URL; task + retention |
| 5 | AI interfaces + Profile (F02) | ✅ | `OCR`, `Parser`, `Profile.Validate` |
| 6 | Mock providers | ✅ | deterministic |
| 7 | Azure providers + factory | ✅ | REST over net/http (no uncertain SDK); factory selects |
| 8 | Candidate domain | ✅ | repo: Create/FindByID/UpdateProfileFields |
| 9 | Application domain + intake service | ✅ | enqueue-failure → status `failed` |
| 10 | Handlers + routes | ✅ | intake + get + job-status; size/type validation |
| 11 | Pipeline (asynq handler) | ✅ | OCR→parse→persist; confidence gate; idempotent keys |
| 12 | Wire api | ✅ | queue client/inspector, routes, `queue` health checker, BodyLimit 12MB |
| 13 | Wire worker | ✅ | asynq server replaces heartbeat; keeps /health probe |
| 14 | Docs/Make | ✅ | README intake flow |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; `golangci-lint` → 0 issues |
| Unit Tests | ✅ Pass | ai, applications (handlers), config |
| Build | ✅ Pass | `go build` + `docker compose build` |
| Integration | ✅ Pass | pipeline (happy / low-confidence / parse-failure) against Postgres + Azurite |
| Edge Cases | ✅ Pass | missing file→400, bad type→415, bad position→400; migration 000003 down/up round-trip |

### End-to-end evidence (live stack, mock provider)
```
POST /api/v1/applications → 201 {"application_id","candidate_id","job_id"}
GET  /api/v1/ai/jobs/<job_id>   → {"state":"completed"}
GET  /api/v1/applications/<id>  → status "parsed", ocr_confidence 0.95,
     needs_manual_review false, parsed_profile_blob_url set, parsed_at set,
     raw_file_blob_url .../resumes/<app_id>/sample_cv.pdf
```

## Files Changed
25 created, 6 modified (see git status). New packages: `internal/ai`, `internal/candidates`, `internal/applications`, `internal/pipeline`, `pkg/queue`; migration `000003`.

## Deviations from Plan
1. **Azure providers use REST (net/http) instead of typed SDKs.** WHY: the Go Document Intelligence SDK surface is uncertain; REST keeps the build deterministic. The `ai` package is the only place Azure is referenced; mock is the default path so CI never touches Azure.
2. **Added `asynq.Retention(24h)` to the task** (post-e2e fix). WHY: asynq drops completed tasks immediately, so `GET /ai/jobs/:id` returned 404 the instant a (fast mock) job finished. Retention keeps status queryable.
3. **Blob key no longer prefixed with `resumes/`** (post-e2e fix). WHY: it duplicated the container name → `resumes/resumes/...`. Now `{app_id}/{filename}`.
4. **api `BodyLimit` raised to 12MB.** WHY: Fiber's 4MB default would reject the 10MB resume NFR before our handler's check runs.
5. **`config.Load` requires Azure keys only when `AI_PROVIDER=azure`.** Conditional fail-fast, not unconditional.

## Issues Encountered
- Shell capture of `psql` INSERT included the `INSERT 0 1` command tag as a second line → invalid `position_id` in the first e2e attempt. Resolved with a `SELECT ... | head -1`. (Test-harness issue, not app code.)

## Tests Written
| Test File | Tests | Coverage area |
|---|---|---|
| `internal/ai/ai_test.go` | 3 | profile validate, factory mock/azure selection |
| `internal/applications/handler_test.go` | 3 | intake validation (400/415/400) |
| `internal/pipeline/process_integration_test.go` (tag `integration`) | 3 | happy path, low-confidence review flag, parse-failure → failed |
| `pkg/config/config_test.go` | +2 | azure-required, azure-with-keys |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Open PR (branch `feat/sprint-1-intake-cv-parser`)
- [ ] Sprint 2: dedup/merge (F09), scoring (F03), must-have gate, branch assignment (F04) — extend the same `process_application` task; seed stores/positions.
