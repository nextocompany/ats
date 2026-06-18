# Plan: ATS 3.8 — Document / Onboarding (post-hire document collection)

## Summary
After a candidate reaches `hired` (via offer accept, slice 3.6), the candidate uploads a checklist of required onboarding documents (ID card, house registration, education certificate, bank book, tax document) through the career-portal, and HR reviews each document — approving or rejecting (with a reason) — until onboarding is complete. Mirrors the established offer/letter slice architecture: a new `onboarding_documents` table (migration `000025`), an account-scoped candidate API + an HR API in package `applications`, blob storage under an `onboarding/` prefix, an HR `OnboardingPanel`, a candidate upload section, and a TH/EN `onboarding` i18n namespace.

## User Story
- **As a newly-hired candidate**, I want to upload my required onboarding documents from the career-portal and see which are still missing or rejected, so that I can complete my pre-employment paperwork without email back-and-forth.
- **As HR**, I want to review each uploaded document and approve or reject it with a reason, and see overall onboarding completion at a glance, so that I can verify a new hire's paperwork before their start date.

## Problem → Solution
**Current state:** the ATS funnel ends at `hired` (offer accept). There is no mechanism for collecting or tracking post-hire onboarding documents — the lifecycle simply stops. → **Desired state:** a self-contained, additive onboarding-document subsystem keyed to hired applications: candidate uploads (account-scoped), HR review (approve/reject), and derived completion progress. The application funnel is untouched (stays lean per the offer/letter precedent).

## Metadata
- **Complexity**: Large (backend domain/repo/handlers/tests + migration + config + 2 frontends + i18n + file upload)
- **Source PRD**: N/A — client Module-3 slice 3.8, free-form
- **PRD Phase**: N/A
- **Estimated Files**: ~24 (backend ~11 incl. 2 notify builders, frontend ~5, career-portal ~5, i18n 4, config 1)

---

## Key Design Decisions (read first)

1. **No new funnel status.** Onboarding is post-`hired` and lives entirely on its own table, exactly like offers/letters keep their lifecycles off the application funnel. The candidate status state machine (`transitions.go`) is **not touched**. Onboarding "completion" is **derived** (every required doc_type has an `approved` row) — no second/parent table, no stored aggregate.
2. **One table:** `onboarding_documents`, `UNIQUE (application_id, doc_type)`. Re-upload is an `ON CONFLICT ... DO UPDATE` upsert that **resets status to `pending`** and clears review fields → cycle is `pending → (HR) rejected → (candidate re-upload) pending → (HR) approved`.
3. **Candidate uploads, HR reviews.** Candidate endpoints under `/api/v1/public/auth/onboarding` resolve the hired application **server-side from the account** (candidate never passes an application_id → no IDOR). HR endpoints under `/api/v1/applications/:id/onboarding` are scope+role gated.
4. **Document types = fixed enum** (DB `CHECK` + Go const list) = the set of *known* types. **Which are *required*** is config-driven via `ONBOARDING_REQUIRED_DOCS` (comma-separated, default = all known types) — cheap real configurability following the `COMPANY_NAME`/`getenv` pattern. The checklist endpoint returns the required set so the UI renders it.
5. **Blob:** reuse the `resumes` container with key `onboarding/{applicationID}/{docType}{ext}`; store the **full URL** (like letters) and serve via `SignedURLForStored` (HR view + candidate view of own docs). Re-upload with a different extension may orphan the old blob (acceptable; noted).
6. **Upload validation mirrors resume:** 10MB max + content types `application/pdf`, `…wordprocessingml.document` (docx), `image/jpeg`, `image/png` — enforced **both** client (reuse `validateFile`) and server (mirror `candidateauth.UploadResume`).
7. **Notifications ARE in scope** (best-effort, inline — matching the existing pattern; NOT asynq):
   - **Candidate uploads a document → notify HR** (HR-facing: `HRDirectory.EmailsForStore(app.AssignedStoreID)` + Teams), cloning the `feedback-recorded → store HR` chain.
   - **HR approves/rejects a document → notify candidate** (candidate-facing: LINE push + ACS email), cloning the `offer-sent → candidate` chain but with a new `approved bool`+`reason` builder.
   - All copy is **Thai-only** (the notify package has no i18n / per-candidate locale — matches every existing notification). Sends are best-effort: failures `log.Warn` and never block the HTTP action.

---

## UX Design

### Before
```
Candidate accepts offer → status "hired" → (nothing). No way to submit documents.
HR sees app at "hired" on the application detail page — no onboarding section.
```

### After
```
CANDIDATE (career-portal /account → "Onboarding documents" section, gated to hired):
┌──────────────────────────────────────────────┐
│ Onboarding documents            2 of 5 done   │
│ ─────────────────────────────────────────────│
│ ✓ ID card                       approved      │
│ ⚠ Education certificate  rejected: blurry scan│
│   [ choose file ]  [ upload ]                 │
│ • Bank book                     not uploaded  │
│   [ choose file ]  [ upload ]                 │
│ … (house registration, tax document)          │
└──────────────────────────────────────────────┘

HR (frontend application detail page → OnboardingPanel in the right aside,
    shown only when app.status === "hired"):
┌──────────────────────────────────────────────┐
│ ONBOARDING                      2 / 5 approved│
│ ─────────────────────────────────────────────│
│ ID card           [view]  approved            │
│ Education cert.    [view]  pending            │
│                    [approve] [reject ▸ reason]│
│ Bank book                 not uploaded        │
└──────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Career-portal `/account` | resume section only | + Onboarding documents section (hired only) | inline `role="alert"` errors (no Toaster in CP) |
| HR application detail aside | Approval/Offer/Letters panels | + OnboardingPanel (hired only) | self-gates `return null` |
| Candidate upload | resume only (apply/account) | + per-doc-type onboarding upload | reuse `validateFile`/`ACCEPTED_TYPES`/10MB |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/offer.go` | all | Domain template: status consts, write-roles map, sentinel errors, pointer-nullable struct, validators |
| P0 | `backend/internal/applications/offer_repository.go` | 14-227 | Repo template: `xColumns` const + `scanX`, `COALESCE`/`NULLIF`, `23505`→exists, `pgx.ErrNoRows`→conflict, **one-tx `FOR UPDATE` + guarded UPDATE + `RowsAffected()==0`→conflict** |
| P0 | `backend/internal/applications/letter_repository.go` | 89-152 | `UpsertLetter` `ON CONFLICT (application_id,type) DO UPDATE` — the upsert template for re-upload |
| P0 | `backend/internal/applications/offer_handler.go` | 34-143 | HR handler template: `RegisterXRoutes`, `scopedAppID`, role gate, `errors.Is`→HTTP (409/400/fallthrough 500) |
| P0 | `backend/internal/applications/offer_candidate_handler.go` | all | Candidate handler template: narrow store interface, `candidateauth.CandidateFromCtx`, gate, decision mapping |
| P0 | `backend/internal/candidateauth/handler.go` | 14-21, 157-185 | **Multipart parse template** (`c.FormFile`, `maxResumeBytes`, content-type map, 400/413) + `resumeContentTypes` |
| P0 | `backend/internal/applications/bulk_handler.go` | 134-145 | `readMultipartFile(fh)` helper to copy verbatim |
| P1 | `backend/internal/applications/letter_handler.go` | 20-40, 66-117, 172-178 | Narrow `letterBlob` interface, `blob.Upload` + `SignedURLForStored` usage, best-effort signing in view |
| P1 | `backend/internal/applications/repository.go` | 18, 64-78 | Where to add new methods to the package `Repository` interface |
| P0 | `backend/internal/applications/notify.go` | 16-66 | `statusNotifyDeps`, `notifyStatusChange`, `dispatchHR` — clone for candidate notify |
| P0 | `backend/internal/applications/feedback_handler.go` | 50-64, 175-196 | HR-notify deps struct + `SetNotifier` + `notifyFeedbackRecorded` (EmailsForStore→dispatchHR) |
| P1 | `backend/internal/notify/notify.go` + `message.go` + `hr_message.go` | all | `Notifier`/`Message`, `StatusMessage`/`StatusEmailMessage`, `ApprovalDecidedHR`/`FeedbackRecordedHR`/`hrMessages` |
| P1 | `backend/internal/applications/hr_directory.go` | 14-44, 85-91 | `HRDirectory.EmailsForStore`; nil storeID → no recipients |
| P2 | `backend/internal/candidates/repository.go` + `model.go` | 11-30 | `FindByID` + `Candidate{FullName,Email,LineUserID}` (no locale field — TH copy) |
| P1 | `backend/internal/applications/offer_test.go` | all | Test template: in-package fiber `app.Test`, fake embedding `Repository` (HR) + narrow fake (candidate), locals injection |
| P1 | `backend/migrations/000023_offers.up.sql` + `000024_letters.up.sql` | all | Migration DDL conventions |
| P1 | `backend/pkg/config/config.go` | 152-153, 176-182, 287-305, 508-549 | Config field + `getenv`* + fail-fast validation pattern |
| P1 | `backend/cmd/api/main.go` | 165, 235-262, 317-324 | Route wiring: HR after `authMW`, candidate behind `RequireCandidate(caSvc, cookie)` |
| P0 | `career-portal/components/auth/ResumeUploadStep.tsx` | all | **Candidate upload template** (validateFile, file input classes, busy state) |
| P0 | `career-portal/lib/auth.ts` | 36-40 | `uploadResume` `FormData` + `api.postForm` template |
| P0 | `frontend/components/resume/OfferPanel.tsx` | all | HR panel template (self-gate, STATUS_KEY map, mutate+spinner, error toast) |
| P1 | `frontend/components/resume/LettersPanel.tsx` | all | Per-action spinner via `mutation.variables`; signed-URL `<a>` download |
| P1 | `frontend/lib/queries.ts` | 363-422 | React Query hook template (offer/letter) |
| P1 | `frontend/lib/api.ts` | 51-83 | `postForm`/`requestForm` multipart + `downloadFile` |
| P1 | `career-portal/lib/queries.ts` | 23-39, 59-80 | Candidate hooks + `buildApplyForm` (exported pure FormData builder) |
| P1 | `career-portal/app/offers/page.tsx` | 80-123 | `DocumentsSection` + inline `role="alert"` pattern; session gate |
| P1 | `career-portal/app/account/page.tsx` | 72-77 | Resume `<section>` — where to add the onboarding section |
| P1 | `frontend/app/(app)/applications/[id]/page.tsx` | 51-64 | Aside where OnboardingPanel mounts |
| P2 | `scripts/check-i18n-parity.mjs` | all | Parity rule: identical dotted key sets th↔en per app |

## External Documentation
No external research needed — feature uses established internal patterns (pgx, Fiber, Azure blob via existing `pkg/blob`, next-intl, React Query). All upload/multipart/blob mechanics already exist in-repo.

---

## Patterns to Mirror

### NAMING_CONVENTION — domain file
```go
// SOURCE: backend/internal/applications/offer.go:17-48
const (
	OfferDraft    = "draft"
	OfferSent     = "sent"
	// ...
)
var offerWriteRoles = map[string]bool{"super_admin": true, "hr_manager": true}
func canManageOffer(role string) bool { return offerWriteRoles[role] }
var (
	ErrOfferExists   = errors.New("applications: offer already exists for application")
	ErrOfferConflict = errors.New("applications: offer not in a respondable state")
	ErrOfferNotFound = errors.New("applications: offer not found for this account")
)
```

### REPOSITORY_PATTERN — columns + scan + COALESCE
```go
// SOURCE: backend/internal/applications/offer_repository.go:14-21
const offerColumns = `id, application_id, status, salary, start_date, COALESCE(terms,''), sent_at, responded_at, expires_at, COALESCE(decline_reason,''), created_at`
func scanOffer(row pgx.Row) (Offer, error) {
	var o Offer
	err := row.Scan(&o.ID, &o.ApplicationID, &o.Status, /* ... */ &o.CreatedAt)
	return o, err
}
```

### REPOSITORY_PATTERN — upsert (re-upload)
```go
// SOURCE: backend/internal/applications/letter_repository.go:89-102
const q = `
	INSERT INTO letters (application_id, type, blob_url, created_by)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (application_id, type)
	DO UPDATE SET blob_url = EXCLUDED.blob_url, created_by = EXCLUDED.created_by, created_at = NOW()
	RETURNING ` + letterColumns
```

### REPOSITORY_PATTERN — guarded UPDATE in tx (HR review transition)
```go
// SOURCE: backend/internal/applications/offer_repository.go:105-188 (shape)
tx, err := r.pool.Begin(ctx); /* defer tx.Rollback(ctx) */
// SELECT ... FOR UPDATE OF d  to lock + read ownership/state
tag, err := tx.Exec(ctx, `UPDATE onboarding_documents SET status=$2, ... WHERE id=$1 AND application_id=$3`, ...)
if tag.RowsAffected() == 0 { return Doc{}, ErrOnboardingDocConflict }
// tx.Commit(ctx); re-read
```

### ERROR_HANDLING — sentinel → HTTP (HR handler)
```go
// SOURCE: backend/internal/applications/offer_handler.go:55-85 (shape)
if errors.Is(err, ErrOfferExists) {
	return fiber.NewError(fiber.StatusConflict, "an offer already exists")
}
if err != nil { return err } // global httpx.ErrorHandler → 500
return httpx.Created(c, offer)
```

### MULTIPART_PARSE — server upload
```go
// SOURCE: backend/internal/candidateauth/handler.go:14-21,163-172
const maxResumeBytes = 10 * 1024 * 1024
var resumeContentTypes = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image", "image/png": "image",
}
fileHeader, err := c.FormFile("resume")
if err != nil { return fiber.NewError(fiber.StatusBadRequest, "resume file is required") }
if fileHeader.Size > maxResumeBytes { return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds 10MB limit") }
contentType := fileHeader.Header.Get("Content-Type")
fileType, ok := resumeContentTypes[contentType]
```
```go
// SOURCE: backend/internal/applications/bulk_handler.go:135-145 — copy verbatim
func readMultipartFile(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open(); if err != nil { return nil, err }
	defer f.Close()
	return io.ReadAll(f)
}
```

### CANDIDATE_AUTH — account-scoped handler + route gate
```go
// SOURCE: backend/internal/applications/offer_candidate_handler.go:38-41,58-...
func RegisterCandidateOnboardingRoutes(app *fiber.App, h *OnboardingCandidateHandler, gate fiber.Handler) {
	app.Get("/api/v1/public/auth/onboarding", gate, h.ListMine)
	app.Post("/api/v1/public/auth/onboarding/documents", gate, h.Upload)
}
acct := candidateauth.CandidateFromCtx(c) // nil → 401
```

### BLOB — narrow interface + signed view
```go
// SOURCE: backend/internal/applications/letter_handler.go:37-40,103-104,172-178
type onboardingBlob interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}
name := fmt.Sprintf("onboarding/%s/%s%s", appID, docType, ext)
storedURL, err := h.blob.Upload(ctx, name, data, contentType)
```

### TEST_STRUCTURE
```go
// SOURCE: backend/internal/applications/offer_test.go:53-79 (shape)
func onboardingTestApp(repo Repository, u middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error { c.Locals(middleware.UserContextKey, u); return c.Next() })
	RegisterOnboardingRoutes(app, NewOnboardingHandler(repo, fakeBlob{}))
	return app
}
// candidate side: c.Locals("candidate_account", acct)
```

### FRONTEND_PANEL — self-gate + mutate + spinner
```tsx
// SOURCE: frontend/components/resume/OfferPanel.tsx (shape)
if (isLoading) return null;
if (app.status !== "hired") return null;            // onboarding gate
const STATUS_KEY: Record<DocStatus, string> = { pending: "status_pending", approved: "status_approved", rejected: "status_rejected" };
review.mutate({ decision: "approve" }, { onSuccess: () => toast.success(t("reviewed")), onError: (e) => toast.error(e instanceof Error ? e.message : t("reviewFailed")) });
{review.isPending && <Loader2 className="size-4 animate-spin" />}
```

### FRONTEND_UPLOAD — candidate (career-portal)
```tsx
// SOURCE: career-portal/components/auth/ResumeUploadStep.tsx:12-31 + lib/auth.ts:36-40
const MAX_RESUME_BYTES = 10 * 1024 * 1024;
const ACCEPTED_TYPES = new Set(["application/pdf","application/vnd.openxmlformats-officedocument.wordprocessingml.document","image/jpeg","image/png"]);
// upload:
const form = new FormData();
form.set("document", file);
form.set("doc_type", docType);
return api.postForm<OnboardingStatus>("/api/v1/public/auth/onboarding/documents", form).then((r) => r.data);
```

### NOTIFY — candidate-facing seam (clone for HR review → candidate)
```go
// SOURCE: backend/internal/applications/notify.go:16-52 (statusNotifyDeps + helper shape)
type statusNotifyDeps struct { notifier notify.Notifier; cands candidates.Repository; portalBaseURL string }
// new method mirrors notifyStatusChange but takes approved+reason and uses a new builder:
func (d statusNotifyDeps) notifyDocumentReviewed(ctx context.Context, apps Repository, appID uuid.UUID, docType string, approved bool, reason string) {
	if d.notifier == nil || d.cands == nil { return }
	app, err := apps.FindByID(ctx, appID); if err != nil { /* log.Warn; return */ }
	cand, err := d.cands.FindByID(ctx, app.CandidateID); if err != nil { /* log.Warn; return */ }
	if msg := notify.DocumentReviewedMessage(cand.LineUserID, cand.FullName, docType, approved, reason, d.portalBaseURL); msg.Recipient != "" {
		if err := d.notifier.Send(ctx, msg); err != nil { /* log.Warn non-fatal */ }
	}
	if em := notify.DocumentReviewedEmailMessage(cand.Email, cand.FullName, docType, approved, reason, d.portalBaseURL); em.Recipient != "" {
		if err := d.notifier.Send(ctx, em); err != nil { /* log.Warn non-fatal */ }
	}
}
// builder in internal/notify/message.go — Thai copy, branch on approved (mirror ApprovalDecidedHR's bool branch)
```

### NOTIFY — HR-facing seam (clone for candidate upload → HR)
```go
// SOURCE: backend/internal/applications/feedback_handler.go:50-55,181-196 + hr_message.go:83-95
type onboardingHRNotify struct { notifier notify.Notifier; hr HRDirectory; dashboardBaseURL string; teamsEnabled bool }
func (h *OnboardingCandidateHandler) notifyDocUploaded(ctx context.Context, app *Application, docType string) {
	d := h.hrNotify
	if d.notifier == nil || d.hr == nil { return }
	emails, err := d.hr.EmailsForStore(ctx, app.AssignedStoreID)
	if err != nil || (len(emails) == 0 && !d.teamsEnabled) { return }
	dashURL := d.dashboardBaseURL + "/applications/" + app.ID.String()
	msgs := notify.OnboardingDocUploadedHR(emails, d.teamsEnabled, docType, dashURL) // returns hrMessages(...)
	dispatchHR(ctx, d.notifier, msgs)
}
```

### CONFIG
```go
// SOURCE: backend/pkg/config/config.go:289,508 (shape)
OnboardingRequiredDocs string  // struct field, comma-separated doc types
OnboardingRequiredDocs: getenv("ONBOARDING_REQUIRED_DOCS", "id_card,house_registration,education_certificate,bank_book,tax_document,photo,health_check"),
// validate each token ∈ known types in Load() (fail fast)
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000025_onboarding_documents.up.sql` | CREATE | `onboarding_documents` table |
| `backend/migrations/000025_onboarding_documents.down.sql` | CREATE | `DROP TABLE IF EXISTS onboarding_documents;` |
| `backend/internal/applications/onboarding.go` | CREATE | Domain: doc-type consts, status consts, write-roles, sentinels, structs, validators, required-set helper |
| `backend/internal/applications/onboarding_repository.go` | CREATE | Repo methods on `pgRepository` |
| `backend/internal/applications/onboarding_handler.go` | CREATE | HR handler + `RegisterOnboardingRoutes` |
| `backend/internal/applications/onboarding_candidate_handler.go` | CREATE | Candidate handler + `RegisterCandidateOnboardingRoutes` |
| `backend/internal/applications/onboarding_test.go` | CREATE | HR + candidate handler tests |
| `backend/internal/applications/repository.go` | UPDATE | Add onboarding methods to `Repository` interface |
| `backend/internal/notify/message.go` | UPDATE | `DocumentReviewedMessage` (LINE) + `DocumentReviewedEmailMessage` — Thai, approved/rejected+reason |
| `backend/internal/notify/hr_message.go` | UPDATE | `OnboardingDocUploadedHR(...)` builder → `hrMessages(...)` |
| `backend/pkg/config/config.go` | UPDATE | `OnboardingRequiredDocs` field + load + validate |
| `backend/cmd/api/main.go` | UPDATE | Wire HR + candidate onboarding routes + both `SetNotifier` calls |
| `frontend/lib/types.ts` | UPDATE | `OnboardingDoc`, `DocStatus`, `OnboardingStatus` types |
| `frontend/lib/queries.ts` | UPDATE | `useOnboarding(appId)`, `useReviewOnboardingDoc(...)` |
| `frontend/components/resume/OnboardingPanel.tsx` | CREATE | HR panel |
| `frontend/app/(app)/applications/[id]/page.tsx` | UPDATE | Mount `<OnboardingPanel>` in aside |
| `frontend/messages/{th,en}.json` | UPDATE | `onboarding` namespace |
| `career-portal/lib/types.ts` | UPDATE | `OnboardingDoc`/`OnboardingStatus`/`DocStatus` |
| `career-portal/lib/queries.ts` | UPDATE | `useMyOnboarding()`, `useUploadOnboardingDoc()` + pure `buildOnboardingDocForm` |
| `career-portal/components/onboarding/OnboardingSection.tsx` | CREATE | Candidate upload+status section |
| `career-portal/app/account/page.tsx` | UPDATE | Render `<OnboardingSection>` when hired |
| `career-portal/messages/{th,en}.json` | UPDATE | `onboarding` namespace |

## NOT Building
- **DB-driven / per-application configurable checklist** — required set is a global env subset of fixed types. No admin UI for configuring it.
- **New funnel status / "onboarding complete" application transition** — completion is derived, funnel untouched.
- **Document versioning / history** — one current doc per (application, doc_type); re-upload replaces.
- **HR-side upload on behalf of candidate** — candidate uploads only.
- **Document expiry / e-signature / OCR validation** of uploaded docs.
- **Standalone career-portal `/onboarding` route** — section lives on `/account` (single hub; nav stays simple).

---

## Step-by-Step Tasks

### Task 1: Migration 000025
- **ACTION**: Create `backend/migrations/000025_onboarding_documents.up.sql` and `.down.sql`.
- **IMPLEMENT**:
```sql
-- Post-hire onboarding documents (Module-3 3.8). One current document per
-- (application, doc_type); the file lives in blob, this row is the record +
-- review state. Candidate uploads, HR reviews. Additive; funnel unaffected.
CREATE TABLE IF NOT EXISTS onboarding_documents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    doc_type       TEXT NOT NULL CHECK (doc_type IN ('id_card','house_registration','education_certificate','bank_book','tax_document','photo','health_check','military_certificate','name_change')),
    status         TEXT NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    blob_url       TEXT NOT NULL,
    file_name      TEXT,
    file_type      TEXT,
    review_reason  TEXT,
    uploaded_by    UUID REFERENCES candidate_accounts(id),
    reviewed_by    UUID REFERENCES users(id),
    uploaded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, doc_type)
);
CREATE INDEX IF NOT EXISTS idx_onboarding_documents_application ON onboarding_documents (application_id);
```
  down: `DROP TABLE IF EXISTS onboarding_documents;`
- **MIRROR**: `000024_letters.up.sql` (CHECK enum + composite UNIQUE + `idx_{table}_application`).
- **GOTCHA**: `uploaded_by` FKs `candidate_accounts(id)` (the uploader is a candidate account, not a `users` row); `reviewed_by` FKs `users(id)`. Confirm next number is `000025` (`ls backend/migrations | tail`).
- **VALIDATE**: file pair present; SQL parses by inspection (local `migrate up` is blocked by Docker disk-full — operator applies on staging/prod).

### Task 2: Domain — `onboarding.go`
- **ACTION**: Create `backend/internal/applications/onboarding.go`.
- **IMPLEMENT**:
  - Doc-type consts (9 known): `DocIDCard="id_card"`, `DocHouseRegistration="house_registration"`, `DocEducationCertificate="education_certificate"`, `DocBankBook="bank_book"`, `DocTaxDocument="tax_document"`, `DocPhoto="photo"`, `DocHealthCheck="health_check"`, `DocMilitaryCertificate="military_certificate"`, `DocNameChange="name_change"`; `var knownDocTypes = map[string]bool{...}` (all 9) + `func validDocType(t string) bool`.
  - Status consts: `OnbPending="pending"`, `OnbApproved="approved"`, `OnbRejected="rejected"`; decision verbs `OnbDecisionApprove="approve"`, `OnbDecisionReject="reject"`; `func validOnbDecision(d string) bool`.
  - `var onboardingWriteRoles = map[string]bool{"super_admin":true,"hr_manager":true,"hr_staff":true,"sgm":true}` + `func canManageOnboarding(role string) bool` (mirror letters' wider HR set, not offers').
  - Sentinels: `ErrOnboardingNoHiredApp`, `ErrOnboardingDocNotFound`, `ErrOnboardingDocConflict`, `ErrInvalidDocType`.
  - Structs: `OnboardingDocument{ ID, ApplicationID uuid.UUID; DocType, Status string; BlobURL string json:"-"; FileName, FileType string json:",omitempty"; ReviewReason string json:",omitempty"; ReviewedBy *uuid.UUID json:"-"; UploadedAt time.Time; ReviewedAt *time.Time }`. View `OnboardingDocView{ embeds OnboardingDocument minus BlobURL via separate struct; URL string }` — follow `LetterView` (expose freshly-signed `URL`, hide `BlobURL`). `OnboardingStatus{ ApplicationID uuid.UUID; Required []string; Documents []OnboardingDocView; ApprovedCount, RequiredCount int; Complete bool }`.
  - Content-type map: copy `resumeContentTypes` as `onboardingContentTypes` (+ `maxOnboardingBytes = 10*1024*1024`).
  - Helper `func extForContentType(ct string) string` → `.pdf/.docx/.jpg/.png` (for blob key).
  - `func computeComplete(required []string, docs []OnboardingDocView) (approved int, complete bool)` — pure; `complete` = every required type has an approved doc.
- **MIRROR**: `offer.go` (consts/roles/sentinels/validators) + `letter.go` (wider role set, `*View` with signed URL).
- **GOTCHA**: required-set is **not** here — it comes from config (Task 8) and is passed in. Keep `knownDocTypes` as the DB-CHECK mirror only.
- **VALIDATE**: `go build ./internal/applications/`.

### Task 3: Repository — `onboarding_repository.go` + interface
- **ACTION**: Create `backend/internal/applications/onboarding_repository.go`; add methods to `Repository` interface in `repository.go`.
- **IMPLEMENT** methods:
  - `UpsertOnboardingDocument(ctx, applicationID uuid.UUID, docType string, blobURL, fileName, fileType string, uploadedBy uuid.UUID) (OnboardingDocument, error)` — `INSERT ... ON CONFLICT (application_id, doc_type) DO UPDATE SET blob_url=EXCLUDED.blob_url, file_name=EXCLUDED.file_name, file_type=EXCLUDED.file_type, status='pending', review_reason=NULL, reviewed_by=NULL, reviewed_at=NULL, uploaded_by=EXCLUDED.uploaded_by, uploaded_at=NOW(), updated_at=NOW() RETURNING ` + `onboardingColumns`. (Re-upload resets review state — the cycle.)
  - `ListOnboardingByApplication(ctx, applicationID uuid.UUID) ([]OnboardingDocument, error)` — `WHERE application_id=$1 ORDER BY doc_type`; `make([]…,0)`, check `rows.Err()`.
  - `ReviewOnboardingDocument(ctx, docID, applicationID, reviewerID uuid.UUID, approve bool, reason string) (OnboardingDocument, error)` — **tx**: `Begin`/`defer Rollback`; `SELECT id FROM onboarding_documents WHERE id=$1 AND application_id=$2 FOR UPDATE` (→ `pgx.ErrNoRows` → `ErrOnboardingDocNotFound`); guarded `UPDATE ... SET status=$n, review_reason=NULLIF($n,''), reviewed_by=$n, reviewed_at=NOW(), updated_at=NOW() WHERE id=$1 AND application_id=$2`, `tag.RowsAffected()==0 → ErrOnboardingDocConflict`; `Commit`; re-read.
  - `FindHiredApplicationByAccount(ctx, accountID uuid.UUID) (uuid.UUID, error)` — `SELECT a.id FROM applications a JOIN candidates c ON c.id=a.candidate_id WHERE c.account_id=$1 AND a.status=$2 ORDER BY a.hired_at DESC NULLS LAST LIMIT 1` with `StatusHired`; `pgx.ErrNoRows → ErrOnboardingNoHiredApp`. (Resolves the candidate's hired app server-side — no IDOR.)
  - `onboardingColumns` const + `scanOnboardingDoc` helper using `COALESCE(file_name,'')`, `COALESCE(file_type,'')`, `COALESCE(review_reason,'')`.
- **MIRROR**: `offer_repository.go` (`RespondOffer` tx, `scanOffer`, COALESCE) + `letter_repository.go` (`UpsertLetter`).
- **IMPORTS**: `context`, `errors`, `fmt`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgconn` (if you map 23505 anywhere — not strictly needed since upsert avoids it).
- **GOTCHA**: review allows correcting a prior decision (approved↔rejected) — guard is `id+application_id` scope, **not** `status='pending'`. Reason required on reject is enforced in the handler.
- **VALIDATE**: `go build ./internal/applications/`.

### Task 4: HR handler — `onboarding_handler.go`
- **ACTION**: Create `backend/internal/applications/onboarding_handler.go`.
- **IMPLEMENT**:
  - `type onboardingStore interface` (narrow): `ExistsInScope`, `GetByID` (for status gate? use `app` only if needed — offer handler reads app via repo; here gate on `app.status=="hired"` by reading the application). Reuse `scopedAppID` (returns ok after `ExistsInScope`). To check hired status, fetch the application: include `FindByID`/`GetApplication` in the narrow interface (see how offer handler reads `h.apps` for the app — it calls a repo method; mirror it).
  - `type onboardingBlob interface { Upload(...); SignedURLForStored(...) }`.
  - `OnboardingHandler{ apps onboardingStore; blob onboardingBlob; required []string }` — `required` injected from config (Task 8).
  - `NewOnboardingHandler(apps Repository, blob onboardingBlob, required []string) *OnboardingHandler`.
  - Routes: `RegisterOnboardingRoutes(app, h)` → `GET /api/v1/applications/:id/onboarding` (h.Get), `POST /api/v1/applications/:id/onboarding/documents/:docId/review` (h.Review).
  - `Get`: `scopedAppID` → role gate `canManageOnboarding` → `ListOnboardingByApplication` → build `[]OnboardingDocView` signing each `BlobURL` via `SignedURLForStored(b, onboardingSignedTTL)` best-effort (log+empty URL on failure, like letter `view`) → compute `Required/ApprovedCount/RequiredCount/Complete` from `h.required` → `httpx.OK(c, status)`.
  - `Review`: parse `:id` + `:docId`, `ExistsInScope`, role gate; parse body `{decision, reason}`; `validOnbDecision` else 400; reject requires non-empty reason else 400; `ReviewOnboardingDocument(...)`; map `ErrOnboardingDocNotFound`→404, `ErrOnboardingDocConflict`→409; **then best-effort `h.notify.notifyDocumentReviewed(ctx, h.apps, appID, doc.DocType, approve, reason)`** (candidate notify); success `httpx.OK(c, view)`.
  - `const onboardingSignedTTL = 24 * time.Hour`.
  - **Candidate-facing notifier seam**: embed `notify statusNotifyDeps` in `OnboardingHandler` + `func (h *OnboardingHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string)` (mirror `OfferHandler.SetNotifier` offer_handler.go:29-31). Add `notifyDocumentReviewed` to `notify.go` (per the NOTIFY pattern). nil-safe so tests without a notifier pass.
- **MIRROR**: `offer_handler.go` (scopedAppID, role gate, sentinel→HTTP, SetNotifier) + `letter_handler.go` (narrow blob interface, best-effort signing in view).
- **GOTCHA**: do NOT require app.status hired for HR `Get` to render (HR may inspect any time), but the **panel** self-gates client-side. Keep HR endpoints status-agnostic except where it would create an inconsistent doc (HR review of a doc whose app is no longer hired is still fine — additive).
- **VALIDATE**: `go build ./internal/applications/`.

### Task 5: Candidate handler — `onboarding_candidate_handler.go`
- **ACTION**: Create `backend/internal/applications/onboarding_candidate_handler.go`.
- **IMPLEMENT**:
  - `type onboardingCandidateStore interface` (narrow): `FindHiredApplicationByAccount`, `FindByID` (for `app.AssignedStoreID` on HR notify), `ListOnboardingByApplication`, `UpsertOnboardingDocument`.
  - `OnboardingCandidateHandler{ apps onboardingCandidateStore; blob onboardingBlob; required []string; hrNotify onboardingHRNotify }` + `NewOnboardingCandidateHandler(apps Repository, blob onboardingBlob, required []string)` + `func (h *OnboardingCandidateHandler) SetNotifier(n notify.Notifier, hr HRDirectory, dashboardBaseURL string, teamsEnabled bool)` (mirror `FeedbackHandler.SetNotifier`).
  - `RegisterCandidateOnboardingRoutes(app, h, gate)` → `GET /api/v1/public/auth/onboarding` (h.ListMine), `POST /api/v1/public/auth/onboarding/documents` (h.Upload).
  - `ListMine`: `acct := candidateauth.CandidateFromCtx(c)` (nil→401); `FindHiredApplicationByAccount(acct.ID)` (map `ErrOnboardingNoHiredApp`→404); list → sign each URL → compute status → `httpx.OK`.
  - `Upload`: acct (401); resolve hired app (404 if none); `docType := c.FormValue("doc_type")` → `validDocType` else 400 (`ErrInvalidDocType`); `fileHeader, err := c.FormFile("document")` → 400 if missing; `fileHeader.Size > maxOnboardingBytes` → 413; `contentType := fileHeader.Header.Get("Content-Type")`; `fileType, ok := onboardingContentTypes[contentType]` else 400; `data, _ := readMultipartFile(fileHeader)`; `ext := extForContentType(contentType)`; `name := fmt.Sprintf("onboarding/%s/%s%s", appID, docType, ext)`; `storedURL, _ := h.blob.Upload(ctx, name, data, contentType)`; `UpsertOnboardingDocument(ctx, appID, docType, storedURL, fileHeader.Filename, fileType, acct.ID)`; **then best-effort HR notify: load `app, _ := h.apps.FindByID(ctx, appID)` and `h.notifyDocUploaded(ctx, app, docType)`** (per NOTIFY HR pattern); respond with the refreshed `OnboardingStatus` (re-list + sign + compute) so the UI updates in one round-trip.
- **MIRROR**: `offer_candidate_handler.go` (CandidateFromCtx, gate, narrow store) + `candidateauth/handler.go:157-185` (multipart parse, size/type checks) + `feedback_handler.go:181-196` (HR notify, EmailsForStore, dispatchHR).
- **IMPORTS**: `mime/multipart` not needed if you reuse `readMultipartFile`; add `time`, `fmt`, `github.com/…/candidateauth`, `httpx`, `fiber`.
- **GOTCHA**: candidate never sends application_id — always resolved from the account. Sanitize/ignore client-provided filename for the blob **key** (use stable `docType+ext`); keep original `fileHeader.Filename` only in `file_name` for display.
- **VALIDATE**: `go build ./internal/applications/`.

### Task 6: Wire routes — `cmd/api/main.go`
- **ACTION**: Update `backend/cmd/api/main.go`.
- **IMPLEMENT** (near offer/letter wiring ~317-324 for HR, ~259-262 for candidate):
```go
// HR (after app.Use(authMW)):
onboardingHandler := applications.NewOnboardingHandler(appRepo, blobClient, cfg.OnboardingRequiredDocs())
onboardingHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL) // review → candidate
applications.RegisterOnboardingRoutes(app, onboardingHandler)
// Candidate (with the candidate gate):
onboardingCandHandler := applications.NewOnboardingCandidateHandler(appRepo, blobClient, cfg.OnboardingRequiredDocs())
onboardingCandHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "") // upload → HR
applications.RegisterCandidateOnboardingRoutes(app, onboardingCandHandler,
	candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName))
```
- **MIRROR**: existing offer/letter wiring + `offerHandler.SetNotifier` (main.go:320) + `feedbackHandler.SetNotifier` (main.go:309).
- **GOTCHA**: `cfg.OnboardingRequiredDocs()` returns `[]string` (parsed in config). `blobClient` / `notifier` / `candidateRepo` / `pool` are existing locals in `main()`. `NewHRDirectory(pool)` mirrors the feedback wiring.
- **VALIDATE**: `go build ./cmd/api/`.

### Task 7: Repository interface update
- **ACTION**: Add the 4 onboarding methods to the `Repository` interface in `backend/internal/applications/repository.go` (after the letter methods block ~78).
- **VALIDATE**: `go build ./internal/applications/`.

### Task 8: Config — `ONBOARDING_REQUIRED_DOCS`
- **ACTION**: Update `backend/pkg/config/config.go`.
- **IMPLEMENT**:
  - Struct field grouped near `CompanyName` (~152): `OnboardingRequiredDocsRaw string` (doc: comma-separated subset of the 9 known onboarding doc types; default = 7 required; `military_certificate` + `name_change` are known but NOT default-required — they're optional/conditional, operator opts in via env).
  - In `Load()` (~289): `OnboardingRequiredDocsRaw: getenv("ONBOARDING_REQUIRED_DOCS", "id_card,house_registration,education_certificate,bank_book,tax_document,photo,health_check")`.
  - Validation block (near other validations): split on `,`, trim, ensure each ∈ the known set; on unknown token `return nil, fmt.Errorf("config: invalid ONBOARDING_REQUIRED_DOCS token %q", tok)`.
  - Method `func (c *Config) OnboardingRequiredDocs() []string` — split/trim/return (deterministic order as listed).
- **MIRROR**: `CompanyName` field + `getenv` + the provider allowlist fail-fast validation style.
- **GOTCHA**: the config package must not import `applications` (no import cycle) — define the known-type allowlist inline in config (small string set) rather than referencing `applications.knownDocTypes`.
- **VALIDATE**: `go build ./pkg/config/`; `go vet ./...`.

### Task 8b: Notify message builders — `notify` package
- **ACTION**: Update `backend/internal/notify/message.go` (candidate) + `backend/internal/notify/hr_message.go` (HR).
- **IMPLEMENT** (Thai-only copy, `fmt.Sprintf`, mirror existing builders):
  - `message.go`: `DocumentReviewedMessage(lineUserID, fullName, docType string, approved bool, reason, portalBaseURL string) Message` → `{Channel: ChannelLINE, Recipient: lineUserID, Subject/Body: Thai}`; branch on `approved` ("เอกสาร … ได้รับการอนุมัติแล้ว" vs "ถูกตีกลับ: <reason> กรุณาอัปโหลดใหม่"); include `portalBaseURL + "/account"`. `DocumentReviewedEmailMessage(email, fullName, docType, approved, reason, portalBaseURL) Message` → `{Channel: ChannelEmail, Recipient: email, Subject, Body}`. Empty `Recipient` when contact missing (caller checks `!= ""`).
  - `hr_message.go`: `OnboardingDocUploadedHR(toEmails []string, teamsEnabled bool, docType, dashURL string) []Message` → Thai subject "มีการอัปโหลดเอกสาร onboarding ใหม่" + body w/ docType + dashURL → `return hrMessages(toEmails, teamsEnabled, subject, body)`.
  - Optionally a small Thai `docType` label map in the notify package (e.g. `docTypeLabelTH`) for human copy — keep it local to notify (avoid importing `applications`).
- **MIRROR**: `StatusMessage`/`StatusEmailMessage` (message.go:15-49) for candidate; `ApprovalDecidedHR` (hr_message.go:56-68, the `approved bool` branch) + `FeedbackRecordedHR` (hr_message.go:23-31) + `hrMessages` (83-95) for HR.
- **GOTCHA**: notify package is TH-only by design — no i18n. Do not import `applications` (the doc-type label map stays local).
- **VALIDATE**: `go build ./internal/notify/` ; `go test ./internal/notify/`.

### Task 9: Backend tests — `onboarding_test.go`
- **ACTION**: Create `backend/internal/applications/onboarding_test.go`.
- **IMPLEMENT** (in-package, fiber `app.Test`):
  - HR fake embedding `Repository` overriding `ExistsInScope`, `ListOnboardingByApplication`, `ReviewOnboardingDocument`, the app-fetch method; `fakeBlob` (Upload/SignedURLForStored counting).
  - Candidate narrow fake implementing `onboardingCandidateStore`.
  - `fakeNotifier` implementing `notify.Notifier` (counts `Send` calls) + `fakeHRDirectory` implementing `HRDirectory` (returns canned emails).
  - Tests: HR review approve (200), reject-without-reason (400), bad decision (400), doc not found (404), conflict (409), role gate (403); candidate upload happy (200 + blob.uploads==1 + upsert called), invalid doc_type (400), missing file (400), oversize (413), wrong content type (400), no hired app (404), unauth (401); **notify wired: HR review with SetNotifier → notifier.Send called (candidate); candidate upload with SetNotifier → notifier.Send called (HR); both still 200 when notifier nil** (best-effort); pure helpers `computeComplete` + `extForContentType` table-driven. Add `notify` builder tests in `internal/notify` (approved vs rejected branch, empty recipient).
- **MIRROR**: `offer_test.go` (`offerTestApp`, `doOffer`, locals injection, narrow candidate fake, `fakeHired`); feedback_test for HR-notify assertions.
- **GOTCHA**: candidate auth faked via `c.Locals("candidate_account", acct)`; build multipart bodies with `multipart.NewWriter` like `handler_test.go:16` / `bulk_handler_test.go:36`.
- **VALIDATE**: `go test ./internal/applications/ -run Onboarding -race`.

### Task 10: Frontend types + hooks
- **ACTION**: Update `frontend/lib/types.ts` + `frontend/lib/queries.ts`.
- **IMPLEMENT**:
  - types: `type DocStatus = "pending"|"approved"|"rejected"`; `interface OnboardingDoc { id: string; doc_type: string; status: DocStatus; file_name?: string; review_reason?: string; uploaded_at: string; reviewed_at?: string; url?: string }`; `interface OnboardingStatus { application_id: string; required: string[]; documents: OnboardingDoc[]; approved_count: number; required_count: number; complete: boolean }`.
  - hooks: `useOnboarding(appId)` → `GET /api/v1/applications/${appId}/onboarding` (`retry:false`, returns the status); `useReviewOnboardingDoc(appId)` → `POST /api/v1/applications/${appId}/onboarding/documents/${docId}/review` body `{decision, reason?}`, `onSuccess: invalidate ["onboarding", appId]`.
- **MIRROR**: offer/letter hooks `queries.ts:363-422`; types `types.ts:189-226`.
- **VALIDATE**: `cd frontend && pnpm exec tsc --noEmit`.

### Task 11: HR `OnboardingPanel.tsx`
- **ACTION**: Create `frontend/components/resume/OnboardingPanel.tsx`; mount in `frontend/app/(app)/applications/[id]/page.tsx` aside.
- **IMPLEMENT**: `"use client"`; props `{ applicationId: string; app: Application }`; `useTranslations("onboarding")`; self-gate `if (app.status !== "hired") return null` and role gate; `useOnboarding(applicationId)`; render `<section className="mt-6 border-t border-hairline pt-5">` with eyebrow title + `{approved_count}/{required_count}` badge; iterate `required` types, find matching doc; per doc: `view` link (`<a href={doc.url} target="_blank" rel="noopener noreferrer">`), status label via typed `STATUS_KEY`, approve/reject buttons with `useReviewOnboardingDoc` `mutate` + reject reason input + `Loader2` spinner gated on `isPending` (scope to the in-flight doc via `review.variables`); "not uploaded" row for missing types; error via `toast.error`.
- **MIRROR**: `OfferPanel.tsx` + `LettersPanel.tsx` (per-action spinner via `variables`).
- **GOTCHA**: add `<OnboardingPanel applicationId={app.id} app={app} />` to the aside (page.tsx:51-64) after `<LettersPanel ... />`.
- **VALIDATE**: `pnpm exec tsc --noEmit && pnpm exec eslint app components lib`.

### Task 12: Career-portal types + hooks + section
- **ACTION**: Update `career-portal/lib/types.ts`, `career-portal/lib/queries.ts`; create `career-portal/components/onboarding/OnboardingSection.tsx`; render in `career-portal/app/account/page.tsx`.
- **IMPLEMENT**:
  - types: mirror `OnboardingDoc`/`OnboardingStatus`/`DocStatus`.
  - hooks: `useMyOnboarding()` → `GET /api/v1/public/auth/onboarding` (`retry:false`); `useUploadOnboardingDoc()` → `useMutation` calling `api.postForm("/api/v1/public/auth/onboarding/documents", buildOnboardingDocForm(input))`; export pure `buildOnboardingDocForm({docType, file})` (set `doc_type` + `document`). Invalidate `["my-onboarding"]` on success.
  - `OnboardingSection`: `"use client"`; reuse `validateFile`/`ACCEPTED_TYPES`/`MAX_RESUME_BYTES` (extract to `career-portal/lib/upload.ts` shared by ResumeUploadStep + here, OR copy verbatim — prefer extracting to a shared module, DRY); render checklist with per-type `<Input type="file">` + upload button; inline `role="alert"` errors (NO toaster in CP); `{approved_count}/{required_count}` progress; `mutate` not `mutateAsync` (the section is not an async submit handler — use a per-type pending state from `upload.isPending && upload.variables?.docType === t`).
  - account page: render `<OnboardingSection />` (the section self-gates: it calls `useMyOnboarding()` which 404s when no hired app → render nothing).
- **MIRROR**: `ResumeUploadStep.tsx`, `lib/auth.ts:36-40`, `offers/page.tsx` DocumentsSection + inline alert, `buildApplyForm`.
- **GOTCHA**: career-portal has **no `<Toaster>`** — use inline `role="alert"` blocks. Session gate: the account page already requires candidate; the section additionally renders nothing if `useMyOnboarding()` resolves null/404.
- **VALIDATE**: `cd career-portal && pnpm exec tsc --noEmit && pnpm exec eslint app components lib`.

### Task 13: i18n — `onboarding` namespace (4 files)
- **ACTION**: Add an `onboarding` namespace to `frontend/messages/{th,en}.json` and `career-portal/messages/{th,en}.json`.
- **IMPLEMENT** keys (identical dotted set per app, th + en):
  - shared: `title`, `progress` ("{approved} of {required} done" / ICU), `status_pending`, `status_approved`, `status_rejected`, `notUploaded`, `doc_id_card`, `doc_house_registration`, `doc_education_certificate`, `doc_bank_book`, `doc_tax_document`, `doc_photo`, `doc_health_check`, `doc_military_certificate`, `doc_name_change` (label all 9 known types so optional ones render if enabled).
  - frontend (HR) extra: `view`, `approve`, `reject`, `reason`, `reasonPlaceholder`, `reviewed`, `reviewFailed`, `complete`, `incomplete`.
  - career-portal (candidate) extra: `uploadCta`, `chooseFile`, `uploaded`, `uploadFailed`, `fileTooLarge`, `fileTypeInvalid`, `rejectedReason` ("Rejected: {reason}" / ICU).
- **MIRROR**: offer/letters namespaces.
- **GOTCHA**: keys present in one locale must exist in the other (same app) — parity script flattens dotted paths. HR and candidate apps may have different key sets (parity is per-app, th↔en).
- **VALIDATE**: `node scripts/check-i18n-parity.mjs` (exit 0).

### Task 14: Full validation sweep
- **ACTION**: Run the complete gate.
- **VALIDATE**: see Validation Commands below.

---

## Testing Strategy

### Unit / Handler Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| HR review approve | valid docId+app, role hr_manager | 200, status approved | no |
| HR reject no reason | decision=reject, reason="" | 400 | yes |
| HR bad decision | decision="meh" | 400 | yes |
| HR review doc not found | unknown docId | 404 | yes |
| HR review conflict | repo returns ErrOnboardingDocConflict | 409 | yes |
| HR role gate | role recruiter | 403 | yes |
| Candidate upload happy | doc_type+pdf file, hired app | 200, blob.uploads==1, upsert called | no |
| Candidate invalid doc_type | doc_type="passport" | 400 | yes |
| Candidate missing file | no `document` part | 400 | yes |
| Candidate oversize | >10MB file | 413 | yes |
| Candidate wrong type | text/plain | 400 | yes |
| Candidate no hired app | account with no hired app | 404 | yes |
| Candidate unauth | no candidate locals | 401 | yes |
| computeComplete | required=[a,b], docs a=approved | approved=1, complete=false | yes |
| extForContentType | each known ct | correct ext | no |

### Edge Cases Checklist
- [ ] Empty doc_type / unknown doc_type → 400
- [ ] Oversize (10MB) → 413; wrong content type → 400
- [ ] Re-upload after reject resets to pending (upsert)
- [ ] Candidate with no hired application → 404 (both list + upload)
- [ ] Concurrent HR review → guarded UPDATE `RowsAffected()==0` → 409
- [ ] Account-scope: candidate cannot target another account's application (resolved server-side)
- [ ] Permission denied (HR role not in onboardingWriteRoles) → 403

---

## Validation Commands

### Backend
```bash
cd backend && go build ./... && go vet ./... && gofmt -l internal/applications internal/notify pkg/config cmd/api
go test ./internal/applications/ ./internal/notify/ ./pkg/config/ -race
```
EXPECT: build/vet clean; `gofmt -l` prints nothing new (pre-existing dirty: `cmd/seedresumes/main.go` — NOT mine); tests pass.

### Frontend (dashboard)
```bash
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
```
EXPECT: zero type errors; eslint clean except pre-existing errors in `components/shell/AppHeader.tsx`, `LocaleSwitcher.tsx` (NOT mine); build green.

### Career-portal
```bash
cd career-portal && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
```
EXPECT: clean; build green.

### i18n parity
```bash
node scripts/check-i18n-parity.mjs
```
EXPECT: exit 0.

### Database
```bash
ls backend/migrations | tail   # confirm 000025 pair present, contiguous
```
EXPECT: `000025_onboarding_documents.{up,down}.sql`. NOTE: local `migrate up` blocked by Docker disk-full — operator applies on staging/prod.

### Manual Validation (post-deploy, operator)
- [ ] Candidate (hired) → /account → upload each doc type → shows pending
- [ ] HR → application detail (hired) → OnboardingPanel → view PDF (signed URL opens), approve one, reject one with reason
- [ ] Candidate sees rejected reason → re-uploads → back to pending
- [ ] Notify fires (prod NOTIFY=real): candidate upload → HR email/Teams; HR review → candidate LINE/email
- [ ] Progress reaches N/N → Complete when all required approved

---

## Acceptance Criteria
- [ ] Migration 000025 pair created (contiguous, mirrors letters DDL)
- [ ] Candidate can upload/replace each required doc type (account-scoped, validated 10MB/type)
- [ ] HR can approve/reject (reason required) with 409 on concurrent conflict
- [ ] Best-effort notify: candidate upload → HR (email/Teams); HR review → candidate (LINE/email); failures never block the action
- [ ] Derived completion + progress correct
- [ ] All validation commands pass; tests written and passing; no type/lint errors (beyond pre-existing)
- [ ] i18n parity green; both apps TH default

## Completion Checklist
- [ ] Code follows discovered offer/letter patterns (indistinguishable from existing slices)
- [ ] Sentinel→HTTP mapping (409/400/404/403/413) matches handler style
- [ ] Account-scope enforced server-side (no application_id from candidate)
- [ ] Blob full-URL stored + signed on read; `BlobURL` `json:"-"`
- [ ] `mutate` not `mutateAsync`; typed status maps (no `as`); per-action spinner
- [ ] No new funnel status; `transitions.go` untouched
- [ ] config defaults safe; no import cycle (config ⊥ applications)
- [ ] Self-contained — no questions needed during implementation

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Local `migrate up` can't validate (Docker disk-full) | High | Low | Inspect SQL; operator applies on staging/prod (known workflow) |
| Candidate has >1 hired application | Low | Medium | Resolve newest by `hired_at DESC`; document limitation; one hire is the norm |
| Re-upload with different file ext orphans old blob | Medium | Low | Acceptable; key is `docType+ext`; note for future cleanup sweep |
| Shared `validateFile` extraction touches ResumeUploadStep | Low | Low | Extract to `lib/upload.ts` carefully or copy verbatim; tsc/build catches regressions |
| Config import cycle if referencing `applications` consts | Low | Medium | Inline the known-type allowlist in config package |

## Notes
- **Slice independence:** built fresh off `main` (05dcfe2) — NO stacking (avoids the 3.5/3.6/3.3 stacked-PR auto-close gotcha). One branch `feat/ats-document-onboarding`, one PR.
- **Deploy (operator, post-merge):** apply migration `000025`; rebuild/roll **api** (new routes + config + notify) + **dashboard** (5 Entra build-args) + **career-portal**. **Worker/scheduler unaffected** — notifications are sent **inline best-effort by the api process** (no asynq task, no new job). Notifications only actually fire if `NOTIFY_PROVIDER=real` (LINE/email) and/or `TEAMS_WEBHOOK_URL` set — both already configured on prod. Set `ONBOARDING_REQUIRED_DOCS` only to override the default seven (e.g. add `military_certificate`/`name_change`).
- **Recurring review lessons applied:** `mutate` not `mutateAsync`; typed status-key maps not `as`; per-type pending spinner; conflict→409 not 500; COALESCE nullable text in scans; check `RowsAffected` on guarded UPDATEs; best-effort signing/notify logged.
