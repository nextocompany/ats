package reengage

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

type fakeEnqueuer struct{ calls int }

func (f *fakeEnqueuer) Enqueue(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.calls++
	return &asynq.TaskInfo{}, nil
}

// testApp builds an app that injects the given role as the authenticated user,
// mirroring how MockJWT populates locals in development.
func testApp(role string, enq *fakeEnqueuer) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		if role != "" {
			c.Locals(middleware.UserContextKey, middleware.DevUser{Role: role})
		}
		return c.Next()
	})
	RegisterRoutes(app, NewHandler(NewTrigger(enq)))
	return app
}

func do(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestReengageHandler_AuthorizedRoleEnqueues(t *testing.T) {
	enq := &fakeEnqueuer{}
	code := do(t, testApp("super_admin", enq), "/api/v1/positions/"+uuid.NewString()+"/reengage")
	if code != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}
	if enq.calls != 1 {
		t.Fatalf("expected enqueue called once, got %d", enq.calls)
	}
}

func TestReengageHandler_ForbiddenRole(t *testing.T) {
	enq := &fakeEnqueuer{}
	code := do(t, testApp("hr_staff", enq), "/api/v1/positions/"+uuid.NewString()+"/reengage")
	if code != fiber.StatusForbidden {
		t.Fatalf("expected 403 for hr_staff, got %d", code)
	}
	if enq.calls != 0 {
		t.Fatalf("expected no enqueue for forbidden role, got %d", enq.calls)
	}
}

func TestReengageHandler_NoUserFailsClosed(t *testing.T) {
	enq := &fakeEnqueuer{}
	if code := do(t, testApp("", enq), "/api/v1/positions/"+uuid.NewString()+"/reengage"); code != fiber.StatusForbidden {
		t.Fatalf("expected 403 when no auth user present, got %d", code)
	}
}

func TestReengageHandler_BadUUID(t *testing.T) {
	enq := &fakeEnqueuer{}
	if code := do(t, testApp("super_admin", enq), "/api/v1/positions/not-a-uuid/reengage"); code != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for bad uuid, got %d", code)
	}
}
