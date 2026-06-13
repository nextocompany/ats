package candidateauth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func mkIDToken(t *testing.T, claims googleClaims) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".sig"
}

func TestDecodeIDToken(t *testing.T) {
	tok := mkIDToken(t, googleClaims{Sub: "abc", Email: "User@X.com", EmailVerified: true, Name: "U"})
	cl, err := decodeIDToken(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cl.Sub != "abc" || cl.Email != "user@x.com" || !cl.EmailVerified {
		t.Fatalf("unexpected claims: %+v", cl)
	}
}

func TestDecodeIDTokenMalformed(t *testing.T) {
	if _, err := decodeIDToken("not-a-jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestGoogleMockLoginSetsSession(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	h := &GoogleHandler{
		portalBaseURL: "https://portal.example.com",
		cookieName:    "cp_session",
		real:          false,
		secureCookie:  false,
		svc:           svc,
	}
	app := fiber.New()
	RegisterGoogleRoutes(app, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/auth/google/login?return=https://portal.example.com/account", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://portal.example.com/account" {
		t.Fatalf("unexpected redirect: %q", loc)
	}
	if sc := resp.Header.Get("Set-Cookie"); !strings.Contains(sc, "cp_session=") {
		t.Fatalf("expected session cookie, got %q", sc)
	}
}

func TestGoogleMockLoginOpenRedirectGuarded(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	h := &GoogleHandler{portalBaseURL: "https://portal.example.com", cookieName: "cp_session", svc: svc}
	app := fiber.New()
	RegisterGoogleRoutes(app, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/auth/google/login?return=https://evil.example.com/x", nil)
	resp, _ := app.Test(req, -1)
	if loc := resp.Header.Get("Location"); loc != "https://portal.example.com" {
		t.Fatalf("open redirect not guarded: %q", loc)
	}
}
