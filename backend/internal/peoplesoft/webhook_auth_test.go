package peoplesoft

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/httpx"
)

const testSecret = "testsecret"

// signedApp builds an app whose PS POST routes are HMAC-guarded with testSecret.
func signedApp(h *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, h, testSecret)
	return app
}

func sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}

// postSigned sends a POST with an optional X-PS-Signature header (sig=="" omits it).
func postSigned(t *testing.T, app *fiber.App, path, body, sig string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sig != "" {
		req.Header.Set("X-PS-Signature", sig)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func newGuardedHandler() *Handler {
	return NewHandler(&fakeVac{}, fakePos{knownCode: "CASHIER"}, nil, "real", nil)
}

func TestVerifyHMAC_ValidSignature(t *testing.T) {
	body := `{"ps_vacancy_id":"V-1","position_code":"CASHIER","headcount":1}`
	app := signedApp(newGuardedHandler())
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", body, sign(testSecret, body)); code != fiber.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", code)
	}
}

func TestVerifyHMAC_AcceptsUppercaseHex(t *testing.T) {
	body := `{"ps_vacancy_id":"V-1","position_code":"CASHIER","headcount":1}`
	app := signedApp(newGuardedHandler())
	upper := strings.ToUpper(sign(testSecret, body)) // signer may emit uppercase hex
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", body, upper); code != fiber.StatusOK {
		t.Fatalf("expected 200 for valid uppercase-hex signature, got %d", code)
	}
}

func TestVerifyHMAC_NonHexSignature(t *testing.T) {
	body := `{"ps_vacancy_id":"V-1","headcount":1}`
	app := signedApp(newGuardedHandler())
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", body, "not-hex-zzzz"); code != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 for non-hex signature, got %d", code)
	}
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	body := `{"ps_vacancy_id":"V-1","headcount":1}`
	app := signedApp(newGuardedHandler())
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", body, sign("not-the-secret", body)); code != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 for signature made with wrong secret, got %d", code)
	}
}

func TestVerifyHMAC_TamperedBody(t *testing.T) {
	signed := `{"ps_vacancy_id":"V-1","headcount":1}`
	sent := `{"ps_vacancy_id":"V-1","headcount":999}`
	app := signedApp(newGuardedHandler())
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", sent, sign(testSecret, signed)); code != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 when body differs from signed payload, got %d", code)
	}
}

func TestVerifyHMAC_MissingHeader(t *testing.T) {
	body := `{"ps_vacancy_id":"V-1","headcount":1}`
	app := signedApp(newGuardedHandler())
	if code := postSigned(t, app, "/api/v1/ps/vacancy-opened", body, ""); code != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 for missing signature, got %d", code)
	}
}

func TestVerifyHMAC_GuardsAllPostRoutes(t *testing.T) {
	app := signedApp(newGuardedHandler())
	for _, path := range []string{"/api/v1/ps/vacancy-opened", "/api/v1/ps/vacancy-closed", "/api/v1/ps/sync-hired"} {
		if code := postSigned(t, app, path, `{}`, ""); code != fiber.StatusUnauthorized {
			t.Errorf("expected 401 (no signature) on %s, got %d", path, code)
		}
	}
}

func TestVerifyHMAC_HealthStaysOpen(t *testing.T) {
	app := signedApp(newGuardedHandler())
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/ps/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected /health open (200) even with HMAC guard, got %d", resp.StatusCode)
	}
}
