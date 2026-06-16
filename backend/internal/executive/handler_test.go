package executive

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// newTestApp builds a Fiber app that injects the given role before the
// executive routes, so we can exercise the role gate without real auth.
func newTestApp(role string) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{Role: role})
		return c.Next()
	})
	RegisterRoutes(app, NewHandler(&mockService{pool: nil}))
	return app
}

func TestHandler_Overview_AllowsLeadership(t *testing.T) {
	for _, role := range []string{"super_admin", "regional_director", "auditor"} {
		req := httptest.NewRequest(fiber.MethodGet, "/api/v1/executive/overview", nil)
		resp, err := newTestApp(role).Test(req)
		if err != nil {
			t.Fatalf("%s: test: %v", role, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("%s: status = %d, want 200", role, resp.StatusCode)
		}
	}
}

func TestHandler_Overview_DeniesNonLeadership(t *testing.T) {
	for _, role := range []string{"hr_staff", "hr_manager", "sgm", ""} {
		req := httptest.NewRequest(fiber.MethodGet, "/api/v1/executive/overview", nil)
		resp, err := newTestApp(role).Test(req)
		if err != nil {
			t.Fatalf("%q: test: %v", role, err)
		}
		if resp.StatusCode != fiber.StatusForbidden {
			t.Fatalf("%q: status = %d, want 403", role, resp.StatusCode)
		}
	}
}
