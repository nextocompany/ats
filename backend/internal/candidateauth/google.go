package candidateauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/config"
)

const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleStateCookie  = "google_oauth"
	googleStateTTL     = 10 * time.Minute
	// devGoogleSub/Email are the deterministic mock identity (no Google call).
	devGoogleSub   = "google-dev-sub"
	devGoogleEmail = "dev.google@example.com"
	devGoogleName  = "Dev Google"
)

// GoogleHandler serves the Google Login OAuth web flow for candidate membership.
// It mirrors internal/lineauth and issues a candidate session on success.
type GoogleHandler struct {
	clientID      string
	clientSecret  string
	callbackURL   string
	portalBaseURL string
	cookieName    string
	real          bool
	secureCookie  bool
	http          *http.Client
	svc           *Service
}

// NewGoogleHandler builds the Google OAuth handler from config.
func NewGoogleHandler(cfg *config.Config, svc *Service) *GoogleHandler {
	return &GoogleHandler{
		clientID:      cfg.GoogleClientID,
		clientSecret:  cfg.GoogleClientSecret,
		callbackURL:   cfg.GoogleCallbackURL,
		portalBaseURL: strings.TrimRight(cfg.PortalBaseURL, "/"),
		cookieName:    cfg.SessionCookieName,
		real:          cfg.UsesRealGoogle(),
		secureCookie:  !cfg.IsDevelopment(),
		http:          &http.Client{Timeout: 10 * time.Second},
		svc:           svc,
	}
}

// RegisterGoogleRoutes mounts the Google OAuth endpoints under the auth group.
func RegisterGoogleRoutes(app *fiber.App, h *GoogleHandler) {
	g := app.Group("/api/v1/public/auth/google")
	g.Get("/login", h.Login)
	g.Get("/callback", h.Callback)
}

// Login starts the flow: redirect to Google (real) or finalize the mock identity.
func (h *GoogleHandler) Login(c *fiber.Ctx) error {
	ret := h.safeReturn(c.Query("return"))

	if !h.real {
		return h.finalize(c, ret, devGoogleSub, devGoogleName, devGoogleEmail)
	}

	state, err := randToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "state generation failed")
	}
	c.Cookie(&fiber.Cookie{
		Name:     googleStateCookie,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(state + "\x00" + ret)),
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: "Lax",
		MaxAge:   int(googleStateTTL.Seconds()),
		Path:     "/",
	})

	q := url.Values{
		"response_type": {"code"},
		"client_id":     {h.clientID},
		"redirect_uri":  {h.callbackURL},
		"state":         {state},
		"scope":         {"openid email profile"},
		"access_type":   {"online"},
		"prompt":        {"select_account"},
	}
	return c.Redirect(googleAuthorizeURL+"?"+q.Encode(), fiber.StatusFound)
}

// Callback validates state, exchanges the code, decodes the id_token, and logs in.
func (h *GoogleHandler) Callback(c *fiber.Ctx) error {
	state, ret := h.parseStateCookie(c.Cookies(googleStateCookie))
	if ret == "" {
		ret = h.portalBaseURL
	}
	c.Cookie(&fiber.Cookie{Name: googleStateCookie, Value: "", HTTPOnly: true, Secure: h.secureCookie, SameSite: "Lax", MaxAge: -1, Path: "/"})

	if e := c.Query("error"); e != "" {
		return c.Redirect(ret+"#auth_error="+url.QueryEscape(e), fiber.StatusFound)
	}
	qState := c.Query("state")
	if state == "" || qState == "" || qState != state {
		return c.Redirect(ret+"#auth_error=state_mismatch", fiber.StatusFound)
	}
	code := c.Query("code")
	if code == "" {
		return c.Redirect(ret+"#auth_error=missing_code", fiber.StatusFound)
	}

	idToken, err := h.exchange(c.UserContext(), code)
	if err != nil {
		log.Warn().Err(err).Msg("googleauth: token exchange failed")
		return c.Redirect(ret+"#auth_error=exchange_failed", fiber.StatusFound)
	}
	claims, err := decodeIDToken(idToken)
	if err != nil {
		log.Warn().Err(err).Msg("googleauth: id_token decode failed")
		return c.Redirect(ret+"#auth_error=token_invalid", fiber.StatusFound)
	}
	if claims.Sub == "" || !claims.EmailVerified {
		return c.Redirect(ret+"#auth_error=email_unverified", fiber.StatusFound)
	}
	return h.finalize(c, ret, claims.Sub, claims.Name, claims.Email)
}

func (h *GoogleHandler) finalize(c *fiber.Ctx, ret, sub, name, email string) error {
	tok, exp, err := h.svc.LoginWithGoogle(c.UserContext(), sub, name, email)
	if err != nil {
		log.Warn().Err(err).Msg("googleauth: login failed")
		return c.Redirect(ret+"#auth_error=login_failed", fiber.StatusFound)
	}
	writeSessionCookie(c, h.cookieName, h.secureCookie, Session{Token: tok, Expires: exp})
	return c.Redirect(ret, fiber.StatusFound)
}

// exchange swaps the authorization code for an id_token at Google's token endpoint.
func (h *GoogleHandler) exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {h.callbackURL},
		"client_id":     {h.clientID},
		"client_secret": {h.clientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("googleauth: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("googleauth: token call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("googleauth: token status %d: %s", resp.StatusCode, string(raw))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", fmt.Errorf("googleauth: token decode: %w", err)
	}
	if tok.IDToken == "" {
		return "", fmt.Errorf("googleauth: no id_token in response")
	}
	return tok.IDToken, nil
}

// googleClaims are the OpenID claims we read from the id_token payload.
type googleClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// decodeIDToken reads the JWT payload WITHOUT verifying the signature. This is
// safe because the id_token was obtained directly from Google's token endpoint
// over TLS in the server-to-server exchange (Google's documented exception), so
// it cannot have been tampered with in transit.
func decodeIDToken(idToken string) (googleClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return googleClaims{}, fmt.Errorf("googleauth: malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return googleClaims{}, fmt.Errorf("googleauth: decode payload: %w", err)
	}
	var cl googleClaims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return googleClaims{}, fmt.Errorf("googleauth: unmarshal payload: %w", err)
	}
	cl.Email = strings.ToLower(strings.TrimSpace(cl.Email))
	return cl, nil
}

// safeReturn guards against open redirects: the return URL must be on the portal
// origin, else fall back to the portal root.
func (h *GoogleHandler) safeReturn(ret string) string {
	if ret == "" {
		return h.portalBaseURL
	}
	u, err := url.Parse(ret)
	if err != nil {
		return h.portalBaseURL
	}
	base, err := url.Parse(h.portalBaseURL)
	if err != nil || u.Scheme != base.Scheme || u.Host != base.Host {
		return h.portalBaseURL
	}
	return ret
}

func (h *GoogleHandler) parseStateCookie(raw string) (state, ret string) {
	if raw == "" {
		return "", ""
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(b), "\x00", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], h.safeReturn(parts[1])
}
