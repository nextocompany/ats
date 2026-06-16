package positions

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

type fakeLister struct{ items []Position }

func (f fakeLister) ListAll(context.Context) ([]Position, error) { return f.items, nil }

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
