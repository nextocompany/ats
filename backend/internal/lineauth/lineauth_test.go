package lineauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newApp(h *Handler) *fiber.App {
	app := fiber.New()
	RegisterRoutes(app, h)
	return app
}

func mockHandler() *Handler {
	return &Handler{portalBaseURL: "https://portal.example.com", real: false}
}

func realHandler() *Handler {
	return &Handler{
		channelID:     "2010375490",
		channelSecret: "secret",
		callbackURL:   "https://api.example.com/api/v1/public/line/callback",
		portalBaseURL: "https://portal.example.com",
		botPrompt:     "aggressive",
		real:          true,
	}
}

// loc issues a request and returns (status, Location header).
func loc(t *testing.T, app *fiber.App, req *http.Request) (int, string) {
	t.Helper()
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode, resp.Header.Get("Location")
}

func TestLogin_Mock_BouncesBackWithStub(t *testing.T) {
	app := newApp(mockHandler())
	ret := "https://portal.example.com/jobs/abc/apply"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/login?return="+ret, nil)
	code, location := loc(t, app, req)
	if code != fiber.StatusFound {
		t.Fatalf("want 302, got %d", code)
	}
	if location != ret+"#line_id_token="+devLineStub {
		t.Fatalf("mock should bounce back with stub, got %q", location)
	}
}

func TestLogin_Real_RedirectsToLineWithState(t *testing.T) {
	app := newApp(realHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/login?return=https://portal.example.com/jobs/x/apply", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("want 302, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	for _, want := range []string{authorizeURL, "client_id=2010375490", "scope=openid+profile", "response_type=code", "state=", "bot_prompt=aggressive"} {
		if !strings.Contains(location, want) {
			t.Fatalf("authorize URL missing %q: %s", want, location)
		}
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), stateCookie) {
		t.Fatalf("expected state cookie to be set")
	}
}

func TestLogin_OpenRedirectGuard(t *testing.T) {
	app := newApp(mockHandler())
	// An off-origin return must be ignored → bounce to the portal root.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/login?return=https://evil.example.net/phish", nil)
	_, location := loc(t, app, req)
	if location != "https://portal.example.com#line_id_token="+devLineStub {
		t.Fatalf("off-origin return should fall back to portal root, got %q", location)
	}
}

func TestCallback_StateMismatch_RedirectsWithError(t *testing.T) {
	h := realHandler()
	app := newApp(h)
	// No state cookie present → state mismatch.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/callback?code=abc&state=xyz", nil)
	code, location := loc(t, app, req)
	if code != fiber.StatusFound {
		t.Fatalf("want 302, got %d", code)
	}
	if !strings.Contains(location, "#line_error=state_mismatch") {
		t.Fatalf("expected state_mismatch error redirect, got %q", location)
	}
}

func TestCallback_ProviderError_RedirectsWithError(t *testing.T) {
	app := newApp(realHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/callback?error=access_denied", nil)
	_, location := loc(t, app, req)
	if !strings.Contains(location, "#line_error=access_denied") {
		t.Fatalf("expected access_denied passthrough, got %q", location)
	}
}

func TestSafeReturn(t *testing.T) {
	h := mockHandler()
	cases := map[string]string{
		"https://portal.example.com/jobs/1/apply": "https://portal.example.com/jobs/1/apply",
		"https://evil.com/x":                      "https://portal.example.com",
		"":                                        "https://portal.example.com",
		"https://portal.example.com.evil.com/x":   "https://portal.example.com",
		"not a url ::::":                          "https://portal.example.com",
	}
	for in, want := range cases {
		if got := h.safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStateCookie_RoundTrips(t *testing.T) {
	h := realHandler()
	// Build a cookie value the same way Login does.
	app := newApp(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/line/login?return=https://portal.example.com/jobs/x/apply", nil)
	resp, _ := app.Test(req, -1)
	cookie := resp.Header.Get("Set-Cookie")
	// Extract the cookie value.
	val := ""
	for _, part := range strings.Split(cookie, ";") {
		if strings.HasPrefix(strings.TrimSpace(part), stateCookie+"=") {
			val = strings.TrimPrefix(strings.TrimSpace(part), stateCookie+"=")
		}
	}
	state, ret, _ := h.parseStateCookie(val)
	if state == "" {
		t.Fatalf("expected a non-empty state from the round-tripped cookie")
	}
	if ret != "https://portal.example.com/jobs/x/apply" {
		t.Fatalf("return not preserved: %q", ret)
	}
}
