package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/config"
)

func TestAuth_DevPathInjectsSuperAdmin(t *testing.T) {
	h, err := Auth(context.Background(), &config.Config{Env: "development", AuthProvider: "mock"})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(h)
	app.Get("/", func(c *fiber.Ctx) error {
		u, _ := c.Locals(UserContextKey).(DevUser)
		return c.SendString(u.Role)
	})
	resp, _ := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	if got := string(body[:n]); got != "super_admin" {
		t.Fatalf("expected super_admin in dev, got %q", got)
	}
}

func TestBearerToken(t *testing.T) {
	app := fiber.New()
	var got string
	app.Get("/", func(c *fiber.Ctx) error { got = bearerToken(c); return nil })

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc.def.ghi")
	_, _ = app.Test(req)
	if got != "abc.def.ghi" {
		t.Fatalf("expected token extracted, got %q", got)
	}

	req2 := httptest.NewRequest(fiber.MethodGet, "/", nil) // no header
	_, _ = app.Test(req2)
	if got != "" {
		t.Fatalf("expected empty token without header, got %q", got)
	}
}
