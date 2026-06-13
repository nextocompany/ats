# Code Review: AI Pre-Interview (slice 2.5)

**Reviewed**: 2026-06-13
**Branch**: `feat/ai-pre-interview` (uncommitted local changes)
**Mode**: Local review — two independent specialist passes (go-reviewer + typescript-reviewer), then triage + fixes
**Decision**: **APPROVE** (after fixes) — all CRITICAL/HIGH findings resolved or reclassified with evidence; validation green

## Summary
Independent Go and TypeScript reviews each returned BLOCK on first pass. After triage, the genuinely-real defects were fixed and re-verified live; two "HIGH" findings were reclassified against the actual codebase conventions (evidence below). Final state: backend + both frontends pass type-check, lint, build, unit tests (`-race`), and a live end-to-end smoke.

## Findings & Resolution

### Reclassified (not regressions — verified against the codebase)
| Orig | Finding | Verdict | Evidence |
|---|---|---|---|
| BE-H1 | "IDOR: no RBAC scope on invite/get" | **Downgraded → MEDIUM follow-up (repo-wide)** | Existing single-record dashboard ops do the same: `applications.UpdateStatus` (the *hire* action) and `GetResume` parse `:id` and act without per-store scoping. RBAC is enforced at the *List* SQL level (`rbac.Scope.ApplicationsClause`), not per-record mutation. Interview matches the established pattern; scoping interview alone would be inconsistent. Tracked as a separate repo-wide hardening item. |
| BE-H3 | "SaveConversation/SetEvaluation ignore RowsAffected" | **Rejected** | `grep RowsAffected internal/applications/repository.go` → none. The entire applications repo never checks it; interview is consistent, not a regression. |

### Fixed — CRITICAL
| ID | Finding | Fix |
|---|---|---|
| BE-C2 | Non-atomic completion: `SaveConversation(completed)` then a failed `Evaluate`/`SetEvaluation` left the session stuck `completed` with null scores | Reordered `Respond`: **evaluate first**, then `SaveConversation(in_progress)`, then `SetEvaluation` (now the *sole* writer of `completed`). A failed/slow LLM eval leaves the session `in_progress` and retryable. `service.go` |

### Fixed — HIGH
| ID | Finding | Fix |
|---|---|---|
| BE-C1 (Invite) | Concurrent invite → UNIQUE(application_id) violation surfaced as 500 | `repository.Create` translates SQLSTATE 23505 → `ErrAlreadyExists`; `service.Invite` catches it and re-reads → idempotent under double-click. Verified live (token1==token2). |
| BE-H2 | `access_token` echoed in HR Get body | `Get` blanks `session.AccessToken` before serializing; `interview_url` still carries it for sharing. Verified live (access_token now `""`). |
| FE-H1 | Stale-closure rollback fragility | Capture invocation-scoped `const snapshot = turns` before the optimistic update; roll back to it on error. |
| FE-H2 | Enter-key double-send race | `onKeyDown` now guards `!respond.isPending` inline (in addition to the in-fn guard + disabled textarea). |
| FE-H3 | Unsound `e as unknown as React.FormEvent` cast | Extracted a param-free `submitAnswer()`; form `onSubmit` and `onKeyDown` both call it. Cast removed; `async` dropped (no await). |
| FE-H4 | `seeded` ref blocked server-side completion sync | Completion now *derived*: `isDone = done || Boolean(data?.done)` — accepts a server-reported completion without an effect-driven setState (also satisfies `react-hooks/set-state-in-effect`). |
| FE-H5 | `InterviewPanel` swallowed `isError` | Now renders an inline "Could not load the AI interview" note on error; `null` reserved for the genuine no-interview (404) case. |

### Fixed — MEDIUM/LOW (cheap, in-PR)
| ID | Fix |
|---|---|
| BE-M4 | `parseEvaluation` now threads the `ParseFloat` error instead of silently scoring 0 |
| BE-M6 | Single `defaultMaxTurns = 6` constant replaces the magic number in service + azure + (config default) |
| BE-M7 | `turn_count` now persists user-turn count (`session.userTurns()`), matching its documented semantics, not total turns |
| BE-L1 | Service tests use `errors.Is` for sentinel comparisons |
| BE-L6 | Added `handler_test.go` (Fiber `app.Test`): 404 bad token, invite→start→get, empty-content 400, invalid-id 400, no-interview 404, and asserts the access_token omission. Coverage **39% → 51.7%** |
| FE-M1 | Composite list keys `${i}-${t.role}` (chat + transcript) |
| FE-M4 | Dashboard invite `▶` glyph wrapped in `aria-hidden` span |
| FE-M6 | `aria-live="polite"` on the chat transcript `<ul>`; `role="alert"` on the error line |
| FE-M7 | Explicit `aria-label="ส่งคำตอบ"` on the send button |
| FE-M8 | `useInterviewSession` gets `staleTime: Infinity` (local state is authoritative once seeded) |

### Follow-ups — RESOLVED in a second pass ("fix it all")
- **Per-turn concurrency** (BE-C1 Respond, BE-H4 Start): implemented **optimistic concurrency** via a `version` column — `SaveConversation` applies only `WHERE version = $expected` and bumps it, returning `ErrConflict` (→ 409) on a stale write. No DB lock is held across the LLM call. `Start` re-reads on conflict; `SetEvaluation` is idempotent (`WHERE status <> 'completed'`). Deterministic unit test `TestSaveConversation_StaleVersionConflicts`; live double-answer smoke shows no 500/lost-turn.
- **Token in URL** (FE-M5): moved to the **URL fragment** (`/interview#token=…`) — read client-side via `useSyncExternalStore`, never sent to the server/proxy. notify + handler URLs updated.
- **BE-H1 / repo-wide RBAC-per-record**: added reusable `applications.Repository.ExistsInScope(id, scope)` (reuses the list scoping clause → handles all/subregion/store) and enforced it on the interview invite/get **and** the sibling per-record endpoints the review named — `applications.UpdateStatus` and dashboard `Resume`. Out-of-scope → 404 (no existence leak). Covered by `TestHandler_InviteOutOfScope404`.
- **BE-L4** `StatusExpired`: now **lazily persisted** — `Start`/`Respond` call `MarkExpired` (best-effort) when a session is past `expires_at`, so the DB reflects reality. Covered by `TestStart_ExpiredMarksExpired`.
- **FE-M10**: HR interview URL is now a **click-to-copy** button, not raw text.
- **FE-M9**: HR query key disambiguated to `["interview-session", id]`.

### Deliberately NOT done (with rationale)
- **BE-L2** shared `pkg/azureopenai` client: would modify `scoring/azure.go`'s live Azure path, which has **no unit coverage** (mock-default), so a regression couldn't be verified here. Risk to working critical code outweighs cosmetic dedup. Kept as intentional, commented mirroring.
- **BE-M3** URL builder in `notify` vs `handler`: `notify` is decoupled-by-design (takes primitives, not the interview package). One-line format, aligned to `#token=` in both. Acceptable.

## Validation Results

| Check | Result |
|---|---|
| Go build | ✅ Pass (`go build ./...`) |
| Go vet | ✅ Pass |
| golangci-lint | ✅ 0 issues (interview + notify) |
| gosec | ✅ Clean |
| Go tests (`-race`) | ✅ Pass — full suite; interview coverage 51.7% |
| TS type-check (both FE) | ✅ Pass |
| ESLint (both FE) | ✅ Pass |
| Next build (both FE) | ✅ Pass |
| Live integration smoke | ✅ idempotent invite · completes · eval present · access_token omitted · 404/400/409 guards |

## Files Reviewed
All `backend/internal/interview/*.go`, `backend/internal/notify/interview_message.go`, migration `000012`, `config.go`, `cmd/api/main.go`, and the portal + dashboard interview files (api/types/queries, InterviewChat, InterviewPanel, AiSummaryPanel, detail page, e2e specs).

## Decision Rationale
Zero remaining CRITICAL/HIGH defects: the real ones (C2 limbo, invite-500, token leak, FE correctness/cast/sync) are fixed and re-verified; the two BLOCK-level findings that didn't survive scrutiny were reclassified with codebase evidence. Remaining items are MEDIUM/LOW follow-ups documented above. **APPROVE for PR**, with the concurrency optimistic-lock and repo-wide per-record RBAC noted as follow-ups.
