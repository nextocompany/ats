package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/config"
)

func TestAuth_DevPathInjectsSuperAdmin(t *testing.T) {
	h, err := Auth(context.Background(), &config.Config{Env: "development", AuthProvider: "mock"}, nil)
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

type fakeToggle struct{ on bool }

func (f fakeToggle) AllowAllTenants(context.Context) bool { return f.on }

func TestEntraTenantPolicy_AllowsTenant(t *testing.T) {
	const allowed = "tid-allowed"
	const other = "tid-other"

	cases := []struct {
		name   string
		toggle AllowAllTenantsReader
		tid    string
		want   bool
	}{
		{"on allowlist, toggle off", fakeToggle{on: false}, allowed, true},
		{"off allowlist, toggle off", fakeToggle{on: false}, other, false},
		{"off allowlist, toggle on ⇒ allowed", fakeToggle{on: true}, other, true},
		{"off allowlist, nil toggle ⇒ denied", nil, other, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := entraTenantPolicy{
				allowed: map[string]struct{}{allowed: {}},
				toggle:  tc.toggle,
			}
			if got := p.AllowsTenant(context.Background(), tc.tid); got != tc.want {
				t.Fatalf("AllowsTenant(%q)=%v want %v", tc.tid, got, tc.want)
			}
		})
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
