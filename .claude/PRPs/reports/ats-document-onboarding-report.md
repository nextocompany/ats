# Implementation Report: ATS 3.8 — Document / Onboarding

## Summary
Built the post-hire onboarding-document lifecycle as a self-contained, additive slice mirroring the offer/letter slice architecture. A hired candidate uploads a checklist of required documents (9 known types; 7 required by default) from the career-portal; HR reviews each (approve / reject with reason); onboarding completion is derived (every required type approved). Best-effort notifications fire both ways (candidate upload → store HR via email/Teams; HR review → candidate via LINE/email). No funnel/state-machine change.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 9/10 | Single-pass — no rework of approach |
| Files Changed | ~24 | 31 (20 modified, 11 created) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000025 (up/down) | ✅ | `onboarding_documents`, UNIQUE(application_id, doc_type), 9-type CHECK |
| 2 | Domain `onboarding.go` | ✅ | consts/roles/sentinels/structs/validators/computeComplete/extForContentType |
| 3 | Repo `onboarding_repository.go` | ✅ | upsert, list, getByID, review (tx + RowsAffected), FindHiredApplicationByAccount |
| 4 | HR handler `onboarding_handler.go` | ✅ | Get + Review, scope/role gate, sentinel→HTTP, candidate notify, buildOnboardingStatus |
| 5 | Candidate handler `onboarding_candidate_handler.go` | ✅ | ListMine + Upload (multipart, validation), HR notify |
| 6 | Wiring `cmd/api/main.go` | ✅ | HR + candidate routes + both SetNotifier |
| 7 | Repository interface | ✅ | 5 onboarding methods added |
| 8 | Config `ONBOARDING_REQUIRED_DOCS` | ✅ | field + load + fail-fast validation + accessor; default 7 |
| 8b | Notify builders | ✅ | DocumentReviewed{Message,EmailMessage} + OnboardingDocUploadedHR (Thai), docTypeLabel map |
| 9 | Backend tests | ✅ | 21 handler/helper tests + 5 notify builder tests |
| 10 | FE types + hooks | ✅ | OnboardingDoc/Status, useOnboarding (404→null), useReviewOnboardingDoc |
| 11 | HR `OnboardingPanel.tsx` | ✅ | self-gates hired+role, per-row approve/reject + reason, per-action spinner |
| 12 | CP types/hooks/section | ✅ | useMyOnboarding (404→null), useUploadOnboardingDoc, buildOnboardingDocForm, OnboardingSection, shared lib/upload.ts |
| 13 | i18n `onboarding` namespace (4 files) | ✅ | parity green both apps |
| 14 | Full validation sweep | ✅ | see below |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go build ./...`, `go vet ./...`, `gofmt -l` clean; both apps `tsc --noEmit` + `eslint` clean (no new errors) |
| Unit Tests | ✅ Pass | Full backend suite green (`go test ./...`); +26 new tests |
| Build | ✅ Pass | dashboard `next build` exit 0; career-portal `next build` exit 0 |
| Integration | N/A | DB-backed integration not runnable locally (Docker disk-full — migration validated by inspection; operator applies on staging/prod) |
| Edge Cases | ✅ Pass | invalid doc_type 400, missing file 400, oversize 413, wrong type 415, no-hired-app 404, unauthed 401, review conflict 409, role gate 403, derived completion |
| i18n parity | ✅ Pass | frontend 138 keys, career-portal 59 keys, th/en in parity |

## Files Changed

### Created (11)
| File | Lines |
|---|---|
| `backend/migrations/000025_onboarding_documents.up.sql` | +21 |
| `backend/migrations/000025_onboarding_documents.down.sql` | +1 |
| `backend/internal/applications/onboarding.go` | +175 |
| `backend/internal/applications/onboarding_repository.go` | +160 |
| `backend/internal/applications/onboarding_handler.go` | +165 |
| `backend/internal/applications/onboarding_candidate_handler.go` | +175 |
| `backend/internal/applications/onboarding_test.go` | +320 |
| `backend/internal/notify/onboarding_message_test.go` | +70 |
| `frontend/components/resume/OnboardingPanel.tsx` | +185 |
| `career-portal/components/onboarding/OnboardingSection.tsx` | +150 |
| `career-portal/lib/upload.ts` | +30 |

### Modified (20)
backend: `cmd/api/main.go`, `internal/applications/notify.go`, `internal/applications/repository.go`, `internal/notify/hr_message.go`, `internal/notify/message.go`, `pkg/config/config.go`, `pkg/config/config_test.go`.
dashboard: `app/(app)/applications/[id]/page.tsx`, `lib/queries.ts`, `lib/roles.ts`, `lib/types.ts`, `messages/{en,th}.json`.
career-portal: `app/account/page.tsx`, `components/auth/ResumeUploadStep.tsx`, `lib/auth.ts`, `lib/queries.ts`, `lib/types.ts`, `messages/{en,th}.json`.

## Deviations from Plan
- **Reused existing test fakes** (`recNotifier`, `fakeHRDir`, `stubCands`) instead of defining new ones — they already existed in `notify_test.go`/`feedback_test.go` (Go disallows redeclaration in a package). WHY: avoid duplicate type declarations; same behavior.
- **HR handler holds full `Repository`** (not a narrow store interface as the plan sketched). WHY: the candidate-facing notify seam (`notifyDocumentReviewed`) needs `Repository.FindByID`; matches the OfferHandler precedent which also holds the full `Repository`. Test fake embeds `Repository` (same as `fakeOfferRepo`).
- **Extracted shared upload constants to `career-portal/lib/upload.ts`** and refactored `ResumeUploadStep` to consume them (error codes mapped to its existing Thai copy). WHY: DRY the accepted-type/size source of truth without changing resume-upload behavior.
- **Candidate UI uses `useTranslations`** (full i18n) even though the surrounding account page is hardcoded-Thai — follows the better offers-page precedent and keeps parity meaningful.

## Issues Encountered
- `gofmt` realigned the Thai `docTypeLabelTH` map in `notify/message.go` — applied `gofmt -w`.
- First test build failed on `fakeHRDir` redeclaration — resolved by reusing the existing fakes (see Deviations).
- Oversize-upload test needed the test Fiber app's `BodyLimit` raised to 20MB so an 11MB body reaches the handler's own 413 check (default Fiber BodyLimit is 4MB, below the 10MB rule).

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/applications/onboarding_test.go` | 21 | HR review (approve/reject/role/404/409/notify), candidate upload (happy/notify/401/404/400/413/415/nil-notifier), computeComplete, extForContentType |
| `internal/notify/onboarding_message_test.go` | 5 | candidate review builders (approved/rejected/empty-recipient), HR uploaded builder (email+teams / no-teams) |
| `pkg/config/config_test.go` | +2 | default 7 required docs; invalid token rejected |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] PR via `/prp-pr` (fresh off main — NO stacking)
- [ ] Operator: apply migration `000025`; rebuild/roll **api** + **dashboard** (5 Entra build-args) + **career-portal**. Worker/scheduler unaffected (notify is inline best-effort). Notifications only fire where `NOTIFY_PROVIDER=real`/`TEAMS_WEBHOOK_URL` set (already on prod). Set `ONBOARDING_REQUIRED_DOCS` only to override the default seven.
- [ ] Human browser UAT: candidate upload each type → pending; HR approve/reject+reason → candidate sees outcome → re-upload resets to pending; progress reaches N/N → complete.
