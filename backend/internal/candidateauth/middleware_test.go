package candidateauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestEnforceOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(EnforceOrigin("https://portal.example.com,http://localhost:3001"))
	app.Post("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	cases := []struct {
		name   string
		method string
		origin string
		want   int
	}{
		{"allowed origin POST", http.MethodPost, "https://portal.example.com", 200},
		{"disallowed origin POST", http.MethodPost, "https://evil.example.com", 403},
		{"no origin POST passes", http.MethodPost, "", 200},
		{"GET always passes", http.MethodGet, "https://evil.example.com", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/x", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != tc.want {
				t.Errorf("origin=%q method=%s: got %d, want %d", tc.origin, tc.method, resp.StatusCode, tc.want)
			}
		})
	}
}
