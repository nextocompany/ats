# Implementation Report: ATS Scorecard (TA/LM split) + Top-5 Shortlist

## Summary
First ATS-lifecycle slice (Module 3 items 3.4 + 3.1). Split the single interview scorecard into TA and Line-Manager **perspectives** (each with its own competency subset, gated per role), added a combined **aggregate summary** + **composite ranking score**, and built a **Top-5 shortlist** endpoint + LM-only `/shortlist` page (store-scoped, composite-ranked) with an email notification to the store line manager when a candidate is shortlisted. LM approve/decline reuse the existing state machine.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | XL | XL — as expected |
| Confidence | 8/10 | Single-pass; one design self-correction (sgm↔TA gate) caught by tests |
| Files Changed | ~20 | 23 (1 migration pair, 10 backend, 1 backend renamed/expanded FE, 8 FE, 2 i18n) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000021 (perspective col + index) | ✅ | additive, default 'ta' |
| 2 | Extend scorecard model + perspective + validation | ✅ | competency superset (7 dims) |
| 3 | Per-perspective role gate | ✅ | taRecordRoles / lmRecordRoles |
| 4 | Repo perspective insert/select | ✅ | COALESCE('ta') on read for legacy |
| 5 | Aggregate computation (scorecard.go) | ✅ | pure `SummarizeFeedback` + `CompositeScore` |
| 6 | Shortlist repo query | ✅ | composite-ranked, scope-clause, LIMIT |
| 7 | Shortlist + summary handler + routes | ✅ | `/shortlist`, `/scorecard-summary` |
| 8 | LM directory resolver + notify builder + trigger | ✅ | fires on bulk→shortlisted (email; Teams if enabled) |
| 9 | Frontend types | ✅ | perspective + dims + ScorecardSummary + ShortlistItem |
| 10 | Frontend hooks | ✅ | useShortlist, useScorecardSummary, invalidations |
| 11 | Frontend role gates | ✅ | canRecordTaScorecard / isLineManager / canRecordLmScorecard |
| 12 | Scorecards component (split + aggregate) | ✅ | replaces InterviewFeedbackPanel |
| 13 | Wire panels into detail page | ✅ | dead InterviewFeedbackPanel.tsx removed |
| 14 | Shortlist page (LM Top-5) | ✅ | role-gated, ranked rows |
| 15 | Nav entry | ✅ | SHORTLIST_NAV gated to sgm |
| 16 | i18n keys (both catalogs) | ✅ | nav.shortlist + shortlist.* |
| 17 | Scorecard aggregate tests | ✅ | 6 tests |
| 18 | Shortlist + perspective handler tests | ✅ | 4 shortlist + 4 perspective gate tests |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go build`/`go vet`/`gofmt` clean (3 flagged files pre-existing, not mine); `tsc --noEmit` exit 0; eslint clean on changed files |
| Unit Tests | ✅ Pass | +14 backend tests; `go test ./internal/applications/...` ok |
| Build | ✅ Pass | `next build` ✓ (`/shortlist` route present); full `go build ./...` ok |
| Integration | ◑ Partial | Endpoints + role/perspective gates + scope exercised via Fiber `app.Test`. Live-server browser run deferred (needs Postgres+Redis + migration 000021) |
| Edge Cases | ✅ Pass | legacy rows→TA, no-TA composite=AI, no-AI composite=nil, sgm null store, invalid perspective 400, role gates |

## Files Changed

| File | Action | Lines |
|---|---|---|
| `backend/migrations/000021_scorecard_perspective.{up,down}.sql` | CREATED | +9 |
| `backend/internal/applications/feedback.go` | UPDATED | +43/-? |
| `backend/internal/applications/feedback_handler.go` | UPDATED | per-perspective gate + DTO |
| `backend/internal/applications/repository.go` | UPDATED | perspective + Shortlist query + iface |
| `backend/internal/applications/scorecard.go` | CREATED | +120 |
| `backend/internal/applications/shortlist_handler.go` | CREATED | +91 |
| `backend/internal/applications/hr_directory.go` | UPDATED | LM resolver |
| `backend/internal/applications/dashboard_handler.go` | UPDATED | LM-notify on shortlist |
| `backend/internal/notify/hr_message.go` | UPDATED | ShortlistReadyLM |
| `backend/cmd/api/main.go` | UPDATED | wire shortlist + LM notifier |
| `backend/internal/applications/{scorecard,shortlist_handler,feedback}_test.go` | CREATED/UPDATED | +14 tests |
| `frontend/components/resume/Scorecards.tsx` | CREATED (replaces InterviewFeedbackPanel.tsx) | +178 |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATED | swap panels |
| `frontend/app/(app)/shortlist/page.tsx` | CREATED | +88 |
| `frontend/components/shell/nav-config.tsx` | UPDATED | SHORTLIST_NAV |
| `frontend/lib/{types,queries,roles}.ts` | UPDATED | types/hooks/gates |
| `frontend/messages/{en,th}.json` | UPDATED | i18n |

## Deviations from Plan
- **`sgm` excluded from TA scorecard** (design self-correction): the plan's `taRecordRoles` = {super_admin, hr_manager, hr_staff} means `sgm` cannot record a TA scorecard (only LM). Two legacy feedback tests used `sgm` + default(ta) and correctly began returning 403; updated them to post the LM perspective. This is the intended behavior, not a regression — surfaced by the validation loop.
- **LM-notify wired into the bulk-shortlist path only** (dashboard `Bulk` handler), the documented HR shortlist workflow. A single-record status PATCH to `shortlisted` does not notify — noted as a follow-up (debounce/digest also a follow-up).
- **Old `InterviewFeedbackPanel.tsx` removed** (no dead code) — fully replaced by `Scorecards.tsx`.

## Issues Encountered
- `Repository` interface needed `Shortlist` added (build caught it) — fixed.
- `fakeHRDir` test stub needed the new `LineManagerEmailsForStore` method (vet caught it) — fixed.
- Both resolved within the validation loop; no broken state carried forward.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `scorecard_test.go` | 6 | composite formula, TA-only/both/none/no-AI aggregate, legacy→TA |
| `shortlist_handler_test.go` | 4 | sgm store-scope, limit param, summary OK, out-of-scope 404 |
| `feedback_test.go` (added) | 4 | hr_staff TA allowed, hr_staff LM forbidden, sgm LM allowed, invalid perspective 400 |

## Next Steps
- [ ] `/code-review`
- [ ] `/prp-pr` → PR (note: deploy needs **migration 000021** via operator migrate recipe)
- [ ] Next ATS PRPs: 3.5 Approval Workflow → 3.6 Offer Management → 3.3 Interview Letter → 3.8 Document/Onboarding → 3.9 ATS Reports
