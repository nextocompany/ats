package applications

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// fakeShortlistStore records the scope/limit it was called with and returns canned data.
type fakeShortlistStore struct {
	gotScope rbac.Scope
	gotLimit int
	items    []ShortlistItem
	app      *Application
	feedback []InterviewFeedback
	inScope  bool
}

func (f *fakeShortlistStore) Shortlist(_ context.Context, scope rbac.Scope, limit int) ([]ShortlistItem, error) {
	f.gotScope, f.gotLimit = scope, limit
	return f.items, nil
}
func (f *fakeShortlistStore) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeShortlistStore) FindByID(context.Context, uuid.UUID) (*Application, error) {
	return f.app, nil
}
func (f *fakeShortlistStore) ListFeedback(context.Context, uuid.UUID) ([]InterviewFeedback, error) {
	return f.feedback, nil
}

func shortlistTestApp(store shortlistStore, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterShortlistRoutes(app, NewShortlistHandler(store))
	return app
}

func TestShortlist_ScopedToSgmStore(t *testing.T) {
	store := &fakeShortlistStore{items: []ShortlistItem{{CandidateName: "A"}, {CandidateName: "B"}}}
	app := shortlistTestApp(store, middleware.DevUser{Role: "sgm", StoreID: ptrInt(7)})
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/shortlist", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d want 200", resp.StatusCode)
	}
	// sgm is store-scoped → the repo must receive a store-kind scope for store 7.
	if store.gotScope.Kind() != rbac.KindStore || store.gotScope.StoreID == nil || *store.gotScope.StoreID != 7 {
		t.Fatalf("expected store-7 scope, got %+v", store.gotScope)
	}
	if store.gotLimit != defaultShortlistLimit {
		t.Fatalf("default limit = %d want %d", store.gotLimit, defaultShortlistLimit)
	}
}

func TestShortlist_LimitParam(t *testing.T) {
	store := &fakeShortlistStore{}
	app := shortlistTestApp(store, middleware.DevUser{Role: "super_admin"})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/shortlist?limit=3", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d want 200", resp.StatusCode)
	}
	if store.gotLimit != 3 {
		t.Fatalf("limit = %d want 3", store.gotLimit)
	}
}

func TestScorecardSummary_OK(t *testing.T) {
	store := &fakeShortlistStore{
		inScope:  true,
		app:      &Application{AIScore: f64(80)},
		feedback: []InterviewFeedback{fbTA(4)},
	}
	app := shortlistTestApp(store, middleware.DevUser{Role: "sgm", StoreID: ptrInt(1)})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/applications/"+uuid.NewString()+"/scorecard-summary", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d want 200", resp.StatusCode)
	}
	var env httpx.Envelope[ScorecardSummary]
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if !env.Success || env.Data.TA == nil || env.Data.CompositeScore == nil {
		t.Fatalf("expected TA agg + composite, got %+v", env.Data)
	}
}

func TestScorecardSummary_OutOfScope(t *testing.T) {
	store := &fakeShortlistStore{inScope: false}
	app := shortlistTestApp(store, middleware.DevUser{Role: "sgm", StoreID: ptrInt(1)})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/applications/"+uuid.NewString()+"/scorecard-summary", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("out-of-scope should be 404, got %d", resp.StatusCode)
	}
}
