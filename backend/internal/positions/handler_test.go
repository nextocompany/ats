package positions

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/pkg/httpx"
)

type fakeLister struct {
	items  []Position
	detail *Position
}

func (f fakeLister) ListAll(context.Context) ([]Position, error) { return f.items, nil }

func (f fakeLister) FindByID(_ context.Context, id uuid.UUID) (*Position, error) {
	if f.detail != nil && f.detail.ID == id {
		return f.detail, nil
	}
	return nil, pgx.ErrNoRows
}

func TestPositionsList_Shape(t *testing.T) {
	id := uuid.New()
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, NewHandler(fakeLister{items: []Position{
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
	RegisterRoutes(app, NewHandler(fakeLister{detail: &Position{
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
}

func TestPositionsDetail_NotFound(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, NewHandler(fakeLister{}))

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
