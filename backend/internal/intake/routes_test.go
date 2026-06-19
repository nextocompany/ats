package intake

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/positions"
)

func sign(secret, ts, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func nowTS() string { return strconv.FormatInt(time.Now().Unix(), 10) }

func mountedApp(t *testing.T, secret string) *fiber.App {
	t.Helper()
	pos := &positions.Position{ID: uuid.New()}
	app := fiber.New()
	RegisterRoutes(app, NewHandler(&fakeIntake{}, &fakePos{byID: pos}), secret)
	return app
}

func TestRegisterRoutes_DisabledWithoutSecret(t *testing.T) {
	app := mountedApp(t, "")
	req := httptest.NewRequest("POST", "/api/v1/intake/seek", strings.NewReader("{}"))
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 (route not mounted) without secret, got %d", resp.StatusCode)
	}
}

func TestRegisterRoutes_HMAC(t *testing.T) {
	const secret = "s3cr3t"
	app := mountedApp(t, secret)
	body := `{"position_id":"x"}`

	// Missing headers → 401.
	req := httptest.NewRequest("POST", "/api/v1/intake/seek", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("missing headers: expected 401, got %d", resp.StatusCode)
	}

	// Wrong signature → 401.
	ts := nowTS()
	req = httptest.NewRequest("POST", "/api/v1/intake/seek", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(signatureHeader, sign("wrong", ts, body))
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("wrong signature: expected 401, got %d", resp.StatusCode)
	}

	// Stale timestamp (10min old) → 401, even with a valid signature for that ts.
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	req = httptest.NewRequest("POST", "/api/v1/intake/seek", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(timestampHeader, stale)
	req.Header.Set(signatureHeader, sign(secret, stale, body))
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("stale timestamp: expected 401, got %d", resp.StatusCode)
	}

	// Fresh timestamp + correct signature → passes auth (reaches handler; not 401).
	req = httptest.NewRequest("POST", "/api/v1/intake/seek", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(signatureHeader, sign(secret, ts, body))
	resp, _ = app.Test(req)
	_, _ = io.ReadAll(resp.Body)
	if resp.StatusCode == fiber.StatusUnauthorized {
		t.Fatalf("correct signature should pass the HMAC guard, got 401")
	}
}
