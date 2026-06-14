package fit

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// allowScoper / denyScoper are ScopeChecker test doubles.
type allowScoper struct{}

func (allowScoper) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return true, nil
}

type denyScoper struct{}

func (denyScoper) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return false, nil
}

// newTestApp wires a Fiber app + fit handler over the in-memory service doubles
// (defined in service_test.go), exercising routing + authorization + mapError.
func newTestApp(svc *Service, scoper ScopeChecker) *fiber.App {
	fa := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterDashboardRoutes(fa, NewHandler(svc, scoper))
	return fa
}

func status(t *testing.T, fa *fiber.App, method, path string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	resp, err := fa.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestHandler_InvalidApplicationID(t *testing.T) {
	svc, _ := newSvc(completedApp(), completedSession(), nil)
	fa := newTestApp(svc, allowScoper{})
	if code := status(t, fa, fiber.MethodPost, "/api/v1/applications/not-a-uuid/fit-analysis"); code != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestHandler_OutOfScope404(t *testing.T) {
	svc, _ := newSvc(completedApp(), completedSession(), nil)
	fa := newTestApp(svc, denyScoper{}) // scope denies → 404 before the service runs
	if code := status(t, fa, fiber.MethodGet, "/api/v1/applications/"+uuid.NewString()+"/fit-analysis"); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 for out-of-scope, got %d", code)
	}
}

func TestHandler_GenerateNotScored409(t *testing.T) {
	app := completedApp()
	app.AIScore = nil
	svc, _ := newSvc(app, completedSession(), nil)
	fa := newTestApp(svc, allowScoper{})
	if code := status(t, fa, fiber.MethodPost, "/api/v1/applications/"+app.ID.String()+"/fit-analysis"); code != fiber.StatusConflict {
		t.Fatalf("expected 409 for unscored application, got %d", code)
	}
}

func TestHandler_GetNotFound404(t *testing.T) {
	svc, _ := newSvc(completedApp(), completedSession(), nil) // nothing generated yet
	fa := newTestApp(svc, allowScoper{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/applications/"+uuid.NewString()+"/fit-analysis"); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 when no analysis exists, got %d", code)
	}
}

func TestHandler_GenerateHappyPath200(t *testing.T) {
	app := completedApp()
	pos := []positions.Position{{ID: uuid.New(), TitleTH: "พนักงานขาย"}}
	svc, _ := newSvc(app, completedSession(), pos)
	fa := newTestApp(svc, allowScoper{})
	if code := status(t, fa, fiber.MethodPost, "/api/v1/applications/"+app.ID.String()+"/fit-analysis"); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestNewHandler_NilScoperPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil scoper")
		}
	}()
	NewHandler(nil, nil)
}
