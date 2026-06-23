package positions

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/internal/scoring"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// fakeReader seeds the authorizer: super_admin (builtin, all perms incl
// settings.admin) and hr_staff (no perms).
type fakeReader struct{}

func (fakeReader) ListRoles(context.Context) ([]rbac.Role, error) {
	return []rbac.Role{
		{Key: "super_admin", ScopeKind: rbac.KindAll, IsBuiltin: true},
		{Key: "hr_staff", ScopeKind: rbac.KindStore},
	}, nil
}

func init() {
	a := rbac.NewAuthorizer(fakeReader{}, 0)
	_ = a.Reload(context.Background())
	rbac.SetDefault(a)
}

type fakeLister struct {
	items      []Position
	detail     *Position
	gotWeights *scoring.Weights
}

func (f *fakeLister) ListAll(context.Context) ([]Position, error) { return f.items, nil }

func (f *fakeLister) FindByID(_ context.Context, id uuid.UUID) (*Position, error) {
	if f.detail != nil && f.detail.ID == id {
		return f.detail, nil
	}
	return nil, pgx.ErrNoRows
}

func (f *fakeLister) UpdateScoreWeights(_ context.Context, _ uuid.UUID, w scoring.Weights) error {
	f.gotWeights = &w
	return nil
}

func appWithUser(role string, repo positionLister) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	if role != "" {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
			return c.Next()
		})
	}
	RegisterRoutes(app, NewHandler(repo))
	return app
}

func TestPositionsList_Shape(t *testing.T) {
	id := uuid.New()
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, NewHandler(&fakeLister{items: []Position{
		{ID: id, TitleTH: "พนักงานขาย", TitleEN: "Sales", Responsibilities: "heavy text dropped"},
	}}))

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/positions", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var env struct {
		Data []ListItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 1 || env.Data[0].TitleTH != "พนักงานขาย" || env.Data[0].ID != id {
		t.Fatalf("unexpected list payload: %+v", env.Data)
	}
}

func TestPositionsDetail_IncludesJD(t *testing.T) {
	id := uuid.New()
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, NewHandler(&fakeLister{detail: &Position{
		ID: id, TitleTH: "พนักงานขาย", TitleEN: "Sales", Level: "officer",
		Responsibilities: "ดูแลลูกค้า", Qualifications: "วุฒิ ปวส.", Benefits: "ประกันสุขภาพ",
	}}))

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/positions/"+id.String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var env struct {
		Data DetailItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Responsibilities != "ดูแลลูกค้า" || env.Data.Qualifications != "วุฒิ ปวส." || env.Data.Benefits != "ประกันสุขภาพ" {
		t.Fatalf("detail JD not surfaced: %+v", env.Data)
	}
	// No stored weights -> response carries the effective DefaultWeights.
	if env.Data.ScoreWeights != scoring.DefaultWeights() {
		t.Fatalf("expected default weights, got %+v", env.Data.ScoreWeights)
	}
}

func TestPositionsDetail_NotFound(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, NewHandler(&fakeLister{}))

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/positions/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	bad := httptest.NewRequest(fiber.MethodGet, "/api/v1/positions/not-a-uuid", nil)
	bresp, _ := app.Test(bad)
	if bresp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for bad id, got %d", bresp.StatusCode)
	}
}

func putWeights(t *testing.T, app *fiber.App, id, body string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPut, "/api/v1/positions/"+id+"/score-weights", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

func TestUpdateScoreWeights_Valid(t *testing.T) {
	repo := &fakeLister{}
	app := appWithUser("super_admin", repo)
	body := `{"experience":40,"skills":20,"education":10,"language":10,"location":20}`
	if got := putWeights(t, app, uuid.NewString(), body); got != fiber.StatusOK {
		t.Fatalf("valid weights: got %d want 200", got)
	}
	if repo.gotWeights == nil || repo.gotWeights.Experience != 40 || repo.gotWeights.Sum() != 100 {
		t.Fatalf("weights not captured/correct: %+v", repo.gotWeights)
	}
}

func TestUpdateScoreWeights_BadSum(t *testing.T) {
	app := appWithUser("super_admin", &fakeLister{})
	body := `{"experience":34,"skills":22,"education":11,"language":11,"location":11}` // sum 89
	if got := putWeights(t, app, uuid.NewString(), body); got != fiber.StatusBadRequest {
		t.Fatalf("bad-sum weights: got %d want 400", got)
	}
}

func TestUpdateScoreWeights_Forbidden(t *testing.T) {
	app := appWithUser("hr_staff", &fakeLister{})
	body := `{"experience":34,"skills":22,"education":11,"language":11,"location":22}`
	if got := putWeights(t, app, uuid.NewString(), body); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff weights: got %d want 403", got)
	}
}
