# Implementation Report: AI Pre-Interview (HR-Human Conversational Screening)

## Summary
Implemented an AI-conducted conversational screening interview as the post-scoring stage. HR invites a candidate from the dashboard ("Send AI interview"); the candidate completes an adaptive Thai/English text chat via an opaque token on the career portal; the AI produces a transcript + structured evaluation (score, recommendation, strengths, concerns, summary) that HR reviews in the candidate detail view. Decisions remain manual. Turns are synchronous API calls (no worker); the interviewer LLM sits behind a mock-default provider seam with real Azure OpenAI behind config.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | Single-pass — no rework needed |
| Files Changed | ~28 | 29 (19 created, 10 updated) + plan/report |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration `000012_interview_sessions` | ✅ Complete | Applied + verified live on local DB |
| 2 | `interview/model.go` (Session, Turn, Evaluation, token) | ✅ Complete | |
| 3 | `interviewer.go` interface + InterviewContext | ✅ Complete | |
| 4 | `mock.go` deterministic interviewer | ✅ Complete | Drives CI + local |
| 5 | `azure.go` Azure OpenAI multi-turn + evaluator | ✅ Complete | `[[END]]` sentinel; json.Number score parse |
| 6 | `repository.go` (pgx) | ✅ Complete | JSONB conversation/strengths/concerns |
| 7 | `service.go` Invite/Start/Respond | ✅ Complete | Idempotent invite, MaxTurns cap, expiry guards |
| 8 | `notify/interview_message.go` | ✅ Complete | Best-effort LINE invite, empty-recipient skip |
| 9 | `handler.go` + `routes.go` | ✅ Complete | Public (start/respond) + admin (invite/get) |
| 10 | Config `InterviewMaxTurns` | ✅ Complete | Reuses `UsesAzureAI()`, no new required secret |
| 11 | Wire into `cmd/api/main.go` | ✅ Complete | Synchronous in api; worker untouched |
| 12 | Backend tests + gates | ✅ Complete | 11 interview tests; vet/lint/gosec/race all green |
| 13 | Portal api.post + types + queries | ✅ Complete | Added JSON `post` to portal client |
| 14 | Portal chat UI (`/interview` + InterviewChat) | ✅ Complete | Optimistic send + rollback, completion state |
| 15 | Dashboard types + queries + invite button | ✅ Complete | 404→null for "not invited" state |
| 16 | Dashboard `InterviewPanel` + detail page | ✅ Complete | Score hero, recommendation, transcript |
| 17 | Frontend e2e specs | ✅ Complete | Portal guard tests run standalone; full flows gated on stack |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | gofmt clean · `go vet` clean · golangci-lint 0 issues · gosec clean · tsc clean (both FE) · eslint clean |
| Unit Tests | ✅ Pass | `go test -race ./internal/interview/... ./internal/notify/... ./pkg/config/...` — 11 interview tests + suites |
| Build | ✅ Pass | `next build` succeeds for career-portal (`/interview` route) and dashboard |
| Integration | ✅ Pass | Migration applied; api rebuilt; live invite→start→respond→complete→HR-view verified in mock mode |
| Edge Cases | ✅ Pass | 404 bad token · idempotent invite (same token) · 409 post-completion · 400 empty answer |
| Full backend suite | ✅ Pass | `go test ./...` — no regressions |

### Live smoke evidence (mock mode, local stack)
```
GET  /api/v1/public/interview/bogus            → HTTP 404 {"success":false,"error":"interview not found"}
POST /api/v1/applications/{id}/interview       → access_token=…, status=invited (re-invite → same token)
GET  /api/v1/public/interview/{token}          → status=in_progress, 1 assistant turn (Thai)
POST …/interview/{token}/message  ×4           → done=true at turn 4
GET  /api/v1/applications/{id}/interview        → status=completed, score=75, recommend, จุดแข็ง/ข้อสังเกต/summary, 9 turns
POST …/message (after completion)              → HTTP 409
POST …/message (empty content)                 → HTTP 400
```

## Files Changed

| File | Action |
|---|---|
| `backend/migrations/000012_interview_sessions.{up,down}.sql` | CREATED |
| `backend/internal/interview/{model,interviewer,mock,factory,azure,repository,service,handler,routes}.go` | CREATED (9) |
| `backend/internal/interview/{service_test,azure_test}.go` | CREATED (2) |
| `backend/internal/notify/interview_message.go` | CREATED |
| `backend/pkg/config/config.go` | UPDATED (+InterviewMaxTurns) |
| `backend/cmd/api/main.go` | UPDATED (construct + register routes) |
| `career-portal/lib/{api,types,queries}.ts` | UPDATED |
| `career-portal/app/interview/page.tsx` | CREATED |
| `career-portal/components/InterviewChat.tsx` | CREATED |
| `career-portal/e2e/interview.spec.ts` | CREATED |
| `frontend/lib/{types,queries}.ts` | UPDATED |
| `frontend/components/resume/AiSummaryPanel.tsx` | UPDATED (Send AI interview button) |
| `frontend/components/resume/InterviewPanel.tsx` | CREATED |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATED (mount InterviewPanel) |
| `frontend/e2e/interview.spec.ts` | CREATED |
| `.env.example` | UPDATED (INTERVIEW_MAX_TURNS) |

## Deviations from Plan
- **Reader interfaces import concrete domain types**: the service's `appReader`/`positionReader`/`candidateReader` reference `applications.Application` / `positions.Position` / `candidates.Candidate` directly (interview→those packages; no cycle since none import interview). The plan allowed this; chosen over fully primitive structs for clarity. WHY: simpler, no mapping layer, and those packages never import interview.
- **`ProfileSummary` = `app.AISummary`** rather than re-parsing the resume blob. WHY: the resume LLM summary is already a concise profile summary available on the application; avoids a blob fetch+parse per turn.
- **Token generator copied, not extracted**: `newAccessToken()` mirrors `public.newPublicToken` (base64 raw-url, 24 bytes) rather than importing it (the public one is unexported). WHY: avoids exporting/relocating existing code for a 5-line helper.

## Issues Encountered
- None blocking. `gofmt` realigned const blocks/struct tags on first write (expected). Build/lint/test all passed on first full run after implementation.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `backend/internal/interview/service_test.go` | 8 | invite idempotency, no-LINE skip, start-seeds-question, run-to-completion+evaluate, empty-answer, not-started, expired, unknown-token |
| `backend/internal/interview/azure_test.go` | 4 | score-as-number, score-as-string, score clamp, prompt grounds in JD |
| `career-portal/e2e/interview.spec.ts` | 3 | no-token guard, unknown-token not-found, full flow (gated on stack) |
| `frontend/e2e/interview.spec.ts` | 1 | detail exposes Send AI interview action (gated on stack) |

## Production Deployment Notes (CRITICAL)
- **Apply migration `000012` to prod BEFORE deploying api code** that reads `interview_sessions` (prod does not auto-run migrations — matches the slice 2.3 apply-500 lesson). Use the live-DB recipe: `db-url` secret + temp PG firewall rule.
- Interview reuses the existing Azure OpenAI deployment; set nothing new to keep mock mode, or `AI_PROVIDER=azure` (already live) makes interviews real automatically. `INTERVIEW_MAX_TURNS` optional (default 6).
- Candidate access is token-only (no LINE/Entra login); public routes inherit the per-IP rate limiter.

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create stacked PR(s) via `/prp-pr` (suggest: backend → portal → dashboard, or one slice PR)
- [ ] Apply migration 000012 to prod before the api rollout
- [ ] Optional: a candidate `interview` status surface in the inbox + interview-aware filtering (future slice)
