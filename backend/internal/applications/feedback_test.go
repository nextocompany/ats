package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

var errStub = errors.New("notify down")

func validFeedback() InterviewFeedback {
	return InterviewFeedback{
		OverallRating:  4,
		Recommendation: RecPass,
		Competencies:   InterviewCompetencies{Communication: 5, Technical: 3, Experience: 4, CultureFit: 0},
	}
}

func TestValidateFeedback(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f *InterviewFeedback)
		wantErr bool
	}{
		{"valid", func(*InterviewFeedback) {}, false},
		{"rating too low", func(f *InterviewFeedback) { f.OverallRating = 0 }, true},
		{"rating too high", func(f *InterviewFeedback) { f.OverallRating = 6 }, true},
		{"unknown recommendation", func(f *InterviewFeedback) { f.Recommendation = "maybe" }, true},
		{"empty recommendation", func(f *InterviewFeedback) { f.Recommendation = "" }, true},
		{"competency too high", func(f *InterviewFeedback) { f.Competencies.Technical = 9 }, true},
		{"competency negative", func(f *InterviewFeedback) { f.Competencies.Experience = -1 }, true},
		{"competency zero allowed", func(f *InterviewFeedback) { f.Competencies = InterviewCompetencies{} }, false},
		{"hold ok", func(f *InterviewFeedback) { f.Recommendation = RecHold }, false},
		{"fail ok", func(f *InterviewFeedback) { f.Recommendation = RecFail }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := validFeedback()
			tc.mutate(&f)
			err := ValidateFeedback(f)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateFeedback() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestCanRecordFeedback(t *testing.T) {
	allowed := map[string]bool{StatusInterview: true, StatusInterviewed: true}
	for _, s := range []string{StatusScored, StatusShortlisted, StatusAIInterviewed, StatusInterview, StatusInterviewed, StatusOffer, StatusRejected} {
		if CanRecordFeedback(s) != allowed[s] {
			t.Fatalf("CanRecordFeedback(%q)=%v, want %v", s, CanRecordFeedback(s), allowed[s])
		}
	}
}

// fakeFeedbackStore is a minimal feedbackStore for handler tests.
type fakeFeedbackStore struct {
	inScope    bool
	app        *Application
	created    InterviewFeedback
	createErr  error
	listResult []InterviewFeedback
}

func (f *fakeFeedbackStore) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeFeedbackStore) FindByID(context.Context, uuid.UUID) (*Application, error) {
	return f.app, nil
}
func (f *fakeFeedbackStore) FindAppointment(context.Context, uuid.UUID) (*Appointment, error) {
	return nil, nil
}
func (f *fakeFeedbackStore) CreateFeedback(_ context.Context, in InterviewFeedback) (InterviewFeedback, error) {
	if f.createErr != nil {
		return InterviewFeedback{}, f.createErr
	}
	in.ID = uuid.New()
	f.created = in
	return in, nil
}
func (f *fakeFeedbackStore) ListFeedback(context.Context, uuid.UUID) ([]InterviewFeedback, error) {
	return f.listResult, nil
}

func feedbackTestApp(store feedbackStore, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterFeedbackRoutes(app, NewFeedbackHandler(store))
	return app
}

func postFeedback(t *testing.T, app *fiber.App, id string, body any) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications/"+id+"/interview-feedback", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestCreateFeedback_RoleGate(t *testing.T) {
	// hr_staff may record the TA scorecard but NOT the line-manager one.
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterview}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	lm := validFeedback()
	lm.Perspective = PerspectiveLineManager
	if got := postFeedback(t, app, uuid.NewString(), lm); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff recording LM scorecard should be forbidden, got %d", got)
	}
}

func TestCreateFeedback_TAPerspectiveAllowed(t *testing.T) {
	// hr_staff (recruiter) may record the TA scorecard (default perspective).
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterview}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	if got := postFeedback(t, app, uuid.NewString(), validFeedback()); got != fiber.StatusCreated {
		t.Fatalf("hr_staff recording TA scorecard should be 201, got %d", got)
	}
	if store.created.Perspective != PerspectiveTA {
		t.Fatalf("expected stored perspective %q, got %q", PerspectiveTA, store.created.Perspective)
	}
}

func TestCreateFeedback_LMPerspectiveBySgm(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterviewed}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "sgm"})
	lm := validFeedback()
	lm.Perspective = PerspectiveLineManager
	if got := postFeedback(t, app, uuid.NewString(), lm); got != fiber.StatusCreated {
		t.Fatalf("sgm recording LM scorecard should be 201, got %d", got)
	}
	if store.created.Perspective != PerspectiveLineManager {
		t.Fatalf("expected stored perspective %q, got %q", PerspectiveLineManager, store.created.Perspective)
	}
}

func TestCreateFeedback_InvalidPerspective(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterview}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "super_admin"})
	bad := validFeedback()
	bad.Perspective = "manager"
	if got := postFeedback(t, app, uuid.NewString(), bad); got != fiber.StatusBadRequest {
		t.Fatalf("invalid perspective should be 400, got %d", got)
	}
}

func TestCreateFeedback_StatusGate(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusShortlisted}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "sgm"})
	got := postFeedback(t, app, uuid.NewString(), validFeedback())
	if got != fiber.StatusBadRequest {
		t.Fatalf("feedback before interview stage should be 400, got %d", got)
	}
}

func TestCreateFeedback_OutOfScope(t *testing.T) {
	store := &fakeFeedbackStore{inScope: false, app: &Application{Status: StatusInterview}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "super_admin"})
	got := postFeedback(t, app, uuid.NewString(), validFeedback())
	if got != fiber.StatusNotFound {
		t.Fatalf("out-of-scope application should be 404, got %d", got)
	}
}

func TestCreateFeedback_Validation(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterview}}
	app := feedbackTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	bad := validFeedback()
	bad.OverallRating = 0
	got := postFeedback(t, app, uuid.NewString(), bad)
	if got != fiber.StatusBadRequest {
		t.Fatalf("invalid rating should be 400, got %d", got)
	}
}

// fakeHRDir returns a fixed recipient set for HR-notify tests.
type fakeHRDir struct{ emails []string }

func (f fakeHRDir) EmailsForStore(context.Context, *int) ([]string, error) { return f.emails, nil }
func (f fakeHRDir) LineManagerEmailsForStore(context.Context, *int) ([]string, error) {
	return f.emails, nil
}

func TestCreateFeedback_HRNotifyNonFatal(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterviewed, AssignedStoreID: ptrInt(5)}}
	h := NewFeedbackHandler(store)
	// Notifier always errors → must NOT break the 201.
	h.SetNotifier(&recNotifier{err: errStub}, fakeHRDir{emails: []string{"hr@x.com"}}, "https://dash", true)
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Email: "gm@x.com", Role: "sgm"})
		return c.Next()
	})
	RegisterFeedbackRoutes(app, h)
	lm := validFeedback()
	lm.Perspective = PerspectiveLineManager // sgm records the LM scorecard
	got := postFeedback(t, app, uuid.NewString(), lm)
	if got != fiber.StatusCreated {
		t.Fatalf("HR-notify failure must not break create; got %d", got)
	}
}

func ptrInt(n int) *int { return &n }

func TestCreateFeedback_HappyPath(t *testing.T) {
	store := &fakeFeedbackStore{inScope: true, app: &Application{Status: StatusInterviewed}}
	uid := uuid.NewString()
	app := feedbackTestApp(store, middleware.DevUser{ID: uid, Email: "gm@cp.test", Role: "sgm"})
	lm := validFeedback()
	lm.Perspective = PerspectiveLineManager // sgm records the LM scorecard
	got := postFeedback(t, app, uuid.NewString(), lm)
	if got != fiber.StatusCreated {
		t.Fatalf("valid feedback should be 201, got %d", got)
	}
	if store.created.Recommendation != RecPass {
		t.Fatalf("expected stored recommendation %q, got %q", RecPass, store.created.Recommendation)
	}
	if store.created.InterviewerID == nil || store.created.InterviewerID.String() != uid {
		t.Fatalf("interviewer id not stamped from auth context")
	}
}
