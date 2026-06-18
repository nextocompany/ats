# Code Review: ATS Scorecard (TA/LM split) + Top-5 Shortlist (local, uncommitted)

**Reviewed**: 2026-06-18
**Author**: Ittiporn Roongmaneephan
**Branch**: `feat/ats-scorecard-shortlist` → `main`
**Decision**: APPROVE with comments

## Summary
Clean, pattern-faithful implementation of the first ATS-lifecycle slice (3.4 + 3.1). No security issues, no secrets/debug, all files well under size limits, validation green. One ranking-consistency bug was found and fixed during review; one spec-intent gap is flagged (non-blocking).

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
1. **Shortlist "AI + TA rating" composite effectively reduces to AI score** (`repository.go` Shortlist + `feedback.go` CanRecordFeedback). The shortlist lists candidates in status `shortlisted`, but TA scorecards can only be recorded at the `interview`/`interviewed` stage (`CanRecordFeedback`). Since shortlisting happens *before* interview, shortlisted rows have **no TA scorecard yet**, so `ta.avg_overall` is always NULL and the composite = AI score in practice. The "+ TA rating" half of spec 3.1 does not contribute at the shortlist stage. **Not a code bug** — the composite is meaningful on the post-interview detail page (ScorecardSummary). **Recommendation (follow-up, not blocking):** either accept composite≈AI for the shortlist v1, or add a lightweight TA review-rating recordable at the shortlist stage (distinct from the interview scorecard) if the client wants AI+TA blending pre-interview. Flag to the client.
2. **Composite SQL/Go divergence — FIXED during review.** The SQL `ORDER BY` blended `ai*0.6 + ta*0.4` while Go `CompositeScore` collapses to plain AI when no TA rating exists, so the displayed composite could disagree with the sort order when mixing rated/unrated rows. Aligned the SQL to the same `CASE WHEN ta IS NULL` collapse so order always matches the displayed number.

### LOW
1. **LM-notify fan-out**: bulk-shortlisting N candidates for one store sends the LM N emails + N `LineManagerEmailsForStore` queries (best-effort, ≤100). Acceptable v1; debounce/digest is a documented follow-up.
2. **Single-record status PATCH to `shortlisted` does not notify the LM** — only the bulk path does (the documented HR workflow). Follow-up.
3. **`/shortlist` API has no role allowlist** (relies on RBAC scope); a store-scoped `hr_manager` could call it directly and get their store's shortlist even though the nav/page is gated to sgm. Harmless — same visibility they already have via the inbox; not a data-exposure issue.
4. **Style**: `AggCard` uses an inline `import("@/lib/types").PerspectiveAgg` type annotation; a top-level import would read cleaner. Cosmetic.

## Validation Results
| Check | Result |
|---|---|
| Type check (`tsc --noEmit`) | Pass (exit 0) |
| Lint (eslint, changed files) | Pass (pre-existing `AppHeader.tsx` errors untouched) |
| Go build / vet / gofmt | Pass (3 gofmt-flagged files pre-existing, not in this change) |
| Tests (`go test ./...`) | Pass (+14 new; full suite exit 0) |
| Build (`next build`) | Pass (`/shortlist` route present) |
| i18n parity | Pass (49 keys th/en) |

## Security Review
- No secrets/credentials; `perspective` is validated against an allowlist before use.
- SQL is parameterized; the RBAC scope clause is appended with correct placeholder indexing — a store-scoped `sgm` only ever receives their own store's shortlist (verified by test). nil store → `1=0` (no rows).
- Per-perspective write gates are server-enforced (TA: super_admin/hr_manager/hr_staff; LM: super_admin/sgm); covered by tests (403 for hr_staff on LM scorecard).
- Migration 000021 is additive (default 'ta'); no data loss.

## Files Reviewed
- Backend (Added): `scorecard.go`, `shortlist_handler.go`, `scorecard_test.go`, `shortlist_handler_test.go`, `migrations/000021_*`
- Backend (Modified): `feedback.go`, `feedback_handler.go`, `repository.go` (+SQL fix), `hr_directory.go`, `dashboard_handler.go`, `cmd/api/main.go`, `notify/hr_message.go`, `feedback_test.go`
- Frontend (Added): `app/(app)/shortlist/page.tsx`, `components/resume/Scorecards.tsx`
- Frontend (Modified): `applications/[id]/page.tsx`, `nav-config.tsx`, `lib/{types,queries,roles}.ts`, `messages/{en,th}.json`
- Frontend (Deleted): `components/resume/InterviewFeedbackPanel.tsx` (replaced — no dead code)
- Docs: plan archived to `completed/`
