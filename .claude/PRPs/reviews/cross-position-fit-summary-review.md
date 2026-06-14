# Code Review: AI Cross-Position Fit Summary (local changes)

**Reviewed**: 2026-06-14
**Branch**: `feat/cross-position-fit-summary`
**Mode**: Local (uncommitted changes) — 3 independent reviewer agents (go-reviewer, typescript-reviewer, security-reviewer) + validation
**Decision**: ✅ APPROVE (after remediation)

## Summary
Self-contained vertical mirroring the `interview` package. No CRITICAL or unresolved HIGH issues. All actionable HIGH/MEDIUM findings were fixed in this pass; a few cross-cutting items are deferred as follow-ups (they are consistent with the existing `interview`/`scoring` code, not regressions introduced here).

## Findings & Resolution

### CRITICAL — None.

### HIGH (all fixed)
- **`GeneratedAt` inconsistency** (go) — `Generate` set the timestamp in-memory then returned the pre-write struct, diverging from `Get`. **Fixed**: `Generate` now re-reads via `FindByApplicationID` after upsert; service no longer stamps the time.
- **`io.ReadAll` error discarded** (go, `azure.go`) — **Fixed**: error now checked and wrapped.
- **`scoper == nil` silent authorization bypass** (security, `handler.go`) — **Fixed**: `NewHandler` panics on nil scoper; the dead nil-guard in `authorizeApplication` removed.
- **`overallTone` left `"weak"` grey** (ts) — **Fixed**: `weak` → `--score-mid`, `none` → `--score-low`.
- **`onClick={generate}` passed the event** (ts) — **Fixed**: wrapped as `onClick={() => generate()}` (both buttons).
- **Index keys on regenerable LLM lists** (ts) — **Partially addressed**: nested reasons now keyed `${position_id}-${i}`; strengths/concerns keep index keys (consistent with the reference `InterviewPanel`).

### MEDIUM (fixed)
- nil slices serialising as JSON `null` → `parseFit` now wraps `strengths`/`concerns`/`reasons` with `nonNilStr`.
- duplicate `interview.NewRepository(pool)` in `main.go` → extracted shared `interviewRepo`.
- integration test `err != ErrNotFound` → `errors.Is`.
- `MaxTokens` 1200 → 2000; added `maxCatalogue` (120) cap with `log.Warn`; per-transcript-turn truncation (`maxTurnChars` 2000) — also mitigates prompt-injection/token-exhaustion via candidate free-text.
- candidate-lookup error swallowed → now `log.Warn` (matches `interview.buildContext`).
- missing handler tests → added `handler_test.go` (400 bad id, 404 out-of-scope, 409 unscored, 404 not-found, 200 happy, nil-scoper panic).

### Deferred (noted, not blocking — consistent with existing code)
- **No per-user rate limit** on the synchronous LLM `POST` — same as `interview` Invite; track as a cross-cutting follow-up for all authed LLM endpoints.
- **`scopeFrom` / `chatRequest`+`chatResponse`+`openAIAPIVersion` duplicated** across `fit`/`interview`/`scoring` — extract a shared `internal/llm` (or `pkg/azureopenai`) package in a future refactor.
- **Azure error body embedded in returned error** (logged, masked from client by `httpx.ErrorHandler`) — matches `interview`/`scoring`; harmless to clients. Revisit if logs become broadly queryable.
- **`generated_by` FK-less** — intentional (mock/dev users absent from `users`); confirmed acceptable by security review.

## Validation Results
| Check | Result |
|---|---|
| Go vet | ✅ Pass |
| Go unit tests (`-race`) | ✅ Pass (fit: parse/mock/service/handler) |
| Go integration tests | ✅ Pass (repo upsert/read/regenerate/not-found) |
| Go build (`./...`) | ✅ Pass |
| Go full suite | ✅ Pass (no FAIL) |
| TS type-check | ✅ Pass |
| ESLint | ✅ Pass |
| Frontend build | ✅ Pass |

## Files Reviewed
- Added: `backend/internal/fit/*` (model, summarizer, factory, mock, azure, repository, service, handler, routes + 4 test files), `migrations/000015_*`, `frontend/components/resume/FitAnalysisPanel.tsx`
- Modified: `backend/internal/positions/model.go`, `backend/cmd/api/main.go`, `backend/internal/peoplesoft/webhook_test.go`, `frontend/lib/{queries,types}.ts`, `frontend/app/(app)/applications/[id]/page.tsx`
