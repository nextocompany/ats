package applications

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ── shared fakes ────────────────────────────────────────────────────────────

type fakeOnbBlob struct{ uploads int }

func (f *fakeOnbBlob) Upload(context.Context, string, []byte, string) (string, error) {
	f.uploads++
	return "https://blob.example/resumes/onboarding/x.pdf", nil
}
func (f *fakeOnbBlob) SignedURLForStored(string, time.Duration) (string, error) {
	return "https://blob.example/signed", nil
}

// recNotifier, fakeHRDir, and stubCands are defined in notify_test.go / feedback_test.go
// and reused here.

// ── HR onboarding handler ───────────────────────────────────────────────────

// fakeOnbRepo embeds Repository (nil) and overrides only what the HR handler uses.
type fakeOnbRepo struct {
	Repository
	inScope   bool
	app       *Application
	docs      []OnboardingDocument
	reviewed  OnboardingDocument
	reviewErr error
}

func (f *fakeOnbRepo) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeOnbRepo) FindByID(context.Context, uuid.UUID) (*Application, error) { return f.app, nil }
func (f *fakeOnbRepo) ListOnboardingByApplication(context.Context, uuid.UUID) ([]OnboardingDocument, error) {
	return f.docs, nil
}
func (f *fakeOnbRepo) ReviewOnboardingDocument(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, bool, string) (OnboardingDocument, error) {
	return f.reviewed, f.reviewErr
}

var onbRequired = []string{DocIDCard, DocHouseRegistration}

func onbHRTestApp(repo Repository, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterOnboardingRoutes(app, NewOnboardingHandler(repo, &fakeOnbBlob{}, onbRequired))
	return app
}

func reviewPath() string {
	return "/api/v1/applications/" + uuid.NewString() + "/onboarding/documents/" + uuid.NewString() + "/review"
}

func TestOnboardingReview_RoleGate(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "recruiter"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "approve"}); got != fiber.StatusForbidden {
		t.Fatalf("recruiter reviewing should be 403, got %d", got)
	}
}

func TestOnboardingReview_Approve(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true, reviewed: OnboardingDocument{ID: uuid.New(), DocType: DocIDCard, Status: OnbApproved}}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "approve"}); got != fiber.StatusOK {
		t.Fatalf("approve should be 200, got %d", got)
	}
}

func TestOnboardingReview_RejectNoReason(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "reject"}); got != fiber.StatusBadRequest {
		t.Fatalf("reject without reason should be 400, got %d", got)
	}
}

func TestOnboardingReview_BadDecision(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "maybe"}); got != fiber.StatusBadRequest {
		t.Fatalf("unknown decision should be 400, got %d", got)
	}
}

func TestOnboardingReview_NotFound(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true, reviewErr: ErrOnboardingDocNotFound}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "approve"}); got != fiber.StatusNotFound {
		t.Fatalf("unknown document should be 404, got %d", got)
	}
}

func TestOnboardingReview_Conflict(t *testing.T) {
	repo := &fakeOnbRepo{inScope: true, reviewErr: ErrOnboardingDocConflict}
	app := onbHRTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "approve"}); got != fiber.StatusConflict {
		t.Fatalf("concurrent review conflict should be 409, got %d", got)
	}
}

func TestOnboardingReview_NotifiesCandidate(t *testing.T) {
	repo := &fakeOnbRepo{
		inScope:  true,
		app:      &Application{ID: uuid.New(), CandidateID: uuid.New()},
		reviewed: OnboardingDocument{ID: uuid.New(), DocType: DocIDCard, Status: OnbApproved},
	}
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
		return c.Next()
	})
	h := NewOnboardingHandler(repo, &fakeOnbBlob{}, onbRequired)
	notifier := &recNotifier{}
	cand := &candidates.Candidate{LineUserID: "U123", Email: "c@example.com"}
	h.SetNotifier(notifier, stubCands{cand: cand}, "http://portal")
	RegisterOnboardingRoutes(app, h)

	if got := doOffer(t, app, fiber.MethodPost, reviewPath(), OnboardingReviewInput{Decision: "approve"}); got != fiber.StatusOK {
		t.Fatalf("approve should be 200, got %d", got)
	}
	if len(notifier.sent) == 0 {
		t.Fatal("approve should best-effort notify the candidate (LINE + email)")
	}
}

// ── Candidate onboarding handler ────────────────────────────────────────────

type fakeOnbCandStore struct {
	hiredErr  error
	hiredID   uuid.UUID
	app       *Application
	docs      []OnboardingDocument
	upserts   int
	upsertDoc OnboardingDocument
}

func (f *fakeOnbCandStore) FindHiredApplicationByAccount(context.Context, uuid.UUID) (uuid.UUID, error) {
	return f.hiredID, f.hiredErr
}
func (f *fakeOnbCandStore) FindByID(context.Context, uuid.UUID) (*Application, error) {
	return f.app, nil
}
func (f *fakeOnbCandStore) ListOnboardingByApplication(context.Context, uuid.UUID) ([]OnboardingDocument, error) {
	return f.docs, nil
}
func (f *fakeOnbCandStore) UpsertOnboardingDocument(context.Context, uuid.UUID, string, string, string, string, uuid.UUID) (OnboardingDocument, error) {
	f.upserts++
	return f.upsertDoc, nil
}

func onbCandTestApp(store onboardingCandidateStore, blob onboardingBlob, acct *candidateauth.Account) (*fiber.App, *OnboardingCandidateHandler) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler, BodyLimit: 20 * 1024 * 1024})
	app.Use(func(c *fiber.Ctx) error {
		if acct != nil {
			c.Locals("candidate_account", acct)
		}
		return c.Next()
	})
	h := NewOnboardingCandidateHandler(store, blob, onbRequired)
	passthrough := func(c *fiber.Ctx) error { return c.Next() }
	RegisterCandidateOnboardingRoutes(app, h, passthrough)
	return app, h
}

func doUpload(t *testing.T, app *fiber.App, fields map[string]string, fileField, fileName, contentType string, fileBytes []byte) int {
	t.Helper()
	body, ct := multipartBody(t, fields, fileField, fileName, contentType, fileBytes)
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/public/auth/onboarding/documents", body)
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestOnboardingUpload_Unauthed(t *testing.T) {
	app, _ := onbCandTestApp(&fakeOnbCandStore{}, &fakeOnbBlob{}, nil)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "id.pdf", "application/pdf", []byte("%PDF-1.4")); got != fiber.StatusUnauthorized {
		t.Fatalf("no session should be 401, got %d", got)
	}
}

func TestOnboardingUpload_NoHiredApp(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{hiredErr: ErrOnboardingNoHiredApp}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "id.pdf", "application/pdf", []byte("%PDF-1.4")); got != fiber.StatusNotFound {
		t.Fatalf("no hired application should be 404, got %d", got)
	}
}

func TestOnboardingUpload_InvalidDocType(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{hiredID: uuid.New()}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	if got := doUpload(t, app, map[string]string{"doc_type": "passport"}, "document", "p.pdf", "application/pdf", []byte("%PDF-1.4")); got != fiber.StatusBadRequest {
		t.Fatalf("unknown doc_type should be 400, got %d", got)
	}
}

func TestOnboardingUpload_MissingFile(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{hiredID: uuid.New()}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "", "", "", nil); got != fiber.StatusBadRequest {
		t.Fatalf("missing file should be 400, got %d", got)
	}
}

func TestOnboardingUpload_WrongType(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{hiredID: uuid.New()}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "x.txt", "text/plain", []byte("hello")); got != fiber.StatusUnsupportedMediaType {
		t.Fatalf("unsupported content type should be 415, got %d", got)
	}
}

func TestOnboardingUpload_Oversize(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{hiredID: uuid.New()}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	big := make([]byte, maxOnboardingBytes+1)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "big.pdf", "application/pdf", big); got != fiber.StatusRequestEntityTooLarge {
		t.Fatalf("oversize file should be 413, got %d", got)
	}
}

func TestOnboardingUpload_HappyPathAndNotifiesHR(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	storeNo := 7
	store := &fakeOnbCandStore{
		hiredID:   uuid.New(),
		app:       &Application{ID: uuid.New(), AssignedStoreID: &storeNo},
		upsertDoc: OnboardingDocument{ID: uuid.New(), DocType: DocIDCard, Status: OnbPending},
		docs:      []OnboardingDocument{{ID: uuid.New(), DocType: DocIDCard, Status: OnbPending}},
	}
	blob := &fakeOnbBlob{}
	app, h := onbCandTestApp(store, blob, acct)
	notifier := &recNotifier{}
	h.SetNotifier(notifier, fakeHRDir{emails: []string{"hr@example.com"}}, "http://dash", true)

	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "id.pdf", "application/pdf", []byte("%PDF-1.4 data")); got != fiber.StatusOK {
		t.Fatalf("happy upload should be 200, got %d", got)
	}
	if blob.uploads != 1 {
		t.Fatalf("upload should write exactly one blob, got %d", blob.uploads)
	}
	if store.upserts != 1 {
		t.Fatalf("upload should upsert exactly one document, got %d", store.upserts)
	}
	if len(notifier.sent) == 0 {
		t.Fatal("upload should best-effort notify store HR (email + Teams)")
	}
}

func TestOnboardingUpload_NilNotifierStillOK(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOnbCandStore{
		hiredID:   uuid.New(),
		app:       &Application{ID: uuid.New()},
		upsertDoc: OnboardingDocument{ID: uuid.New(), DocType: DocIDCard, Status: OnbPending},
	}
	app, _ := onbCandTestApp(store, &fakeOnbBlob{}, acct)
	if got := doUpload(t, app, map[string]string{"doc_type": DocIDCard}, "document", "id.pdf", "application/pdf", []byte("%PDF-1.4")); got != fiber.StatusOK {
		t.Fatalf("upload without a notifier should still be 200, got %d", got)
	}
}

// ── pure helpers ────────────────────────────────────────────────────────────

func TestComputeComplete(t *testing.T) {
	required := []string{DocIDCard, DocBankBook}
	tests := []struct {
		name     string
		docs     []OnboardingDocView
		approved int
		complete bool
	}{
		{"none uploaded", nil, 0, false},
		{"one approved", []OnboardingDocView{{DocType: DocIDCard, Status: OnbApproved}}, 1, false},
		{"one approved one pending", []OnboardingDocView{{DocType: DocIDCard, Status: OnbApproved}, {DocType: DocBankBook, Status: OnbPending}}, 1, false},
		{"all approved", []OnboardingDocView{{DocType: DocIDCard, Status: OnbApproved}, {DocType: DocBankBook, Status: OnbApproved}}, 2, true},
		{"extra non-required approved", []OnboardingDocView{{DocType: DocIDCard, Status: OnbApproved}, {DocType: DocBankBook, Status: OnbApproved}, {DocType: DocPhoto, Status: OnbApproved}}, 2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotApproved, gotComplete := computeComplete(required, tc.docs)
			if gotApproved != tc.approved || gotComplete != tc.complete {
				t.Fatalf("computeComplete = (%d, %v), want (%d, %v)", gotApproved, gotComplete, tc.approved, tc.complete)
			}
		})
	}
}

func TestComputeComplete_EmptyRequiredNotComplete(t *testing.T) {
	if approved, complete := computeComplete(nil, nil); approved != 0 || complete {
		t.Fatalf("empty required set must not report complete, got (%d, %v)", approved, complete)
	}
}

func TestExtForContentType(t *testing.T) {
	cases := map[string]string{
		"application/pdf": ".pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"text/plain": "",
	}
	for ct, want := range cases {
		if got := extForContentType(ct); got != want {
			t.Fatalf("extForContentType(%q) = %q, want %q", ct, got, want)
		}
	}
}
