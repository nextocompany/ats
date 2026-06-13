// Package lineauth implements the LINE Login OAuth 2.1 web flow for the public
// career portal. The portal sends the candidate to /login; we redirect to LINE,
// LINE returns to /callback with an authorization code, and we exchange it
// (server-side, with the channel secret) for an OpenID id_token. The id_token is
// handed back to the portal in the URL *fragment* (never logged) where the apply
// flow sends it as X-LINE-IdToken — verified by internal/auth's realVerifier.
//
// Mock mode (LINE_PROVIDER != real) skips LINE entirely and bounces straight back
// with the dev stub token, so local/CI need no LINE credentials.
package lineauth

import (
	"context"
	"crypto/rand"
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
	authorizeURL = "https://access.line.me/oauth2/v2.1/authorize"
	exchangeURL  = "https://api.line.me/oauth2/v2.1/token"
	stateCookie  = "line_oauth"
	stateTTL     = 10 * time.Minute
	// devLineStub is what mock mode returns; the mock verifier accepts any non-empty value.
	devLineStub = "dev-line-id-token"
)

// Handler serves the LINE Login OAuth endpoints.
type Handler struct {
	channelID     string
	channelSecret string
	callbackURL   string
	portalBaseURL string
	botPrompt     string
	real          bool
	secureCookie  bool
	http          *http.Client
}

// NewHandler builds the LINE OAuth handler from config.
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		channelID:     cfg.LINEChannelID,
		channelSecret: cfg.LINEChannelSecret,
		callbackURL:   cfg.LINELoginCallbackURL,
		portalBaseURL: strings.TrimRight(cfg.PortalBaseURL, "/"),
		botPrompt:     cfg.LINELoginBotPrompt,
		real:          cfg.UsesRealLINE(),
		secureCookie:  !cfg.IsDevelopment(),
		http:          &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterRoutes mounts the OAuth endpoints under the public group.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/public/line")
	g.Get("/login", h.Login)
	g.Get("/callback", h.Callback)
}

// Login starts the flow: redirect to LINE (real) or bounce back with the stub (mock).
func (h *Handler) Login(c *fiber.Ctx) error {
	ret := h.safeReturn(c.Query("return"))

	if !h.real {
		return c.Redirect(ret+"#line_id_token="+devLineStub, fiber.StatusFound)
	}

	state, err := randToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "state generation failed")
	}
	// Bind the CSRF state + the return URL together in a short-lived httpOnly cookie.
	c.Cookie(&fiber.Cookie{
		Name:     stateCookie,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(state + "\x00" + ret)),
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: "Lax", // survives the top-level GET redirect back from LINE
		MaxAge:   int(stateTTL.Seconds()),
		Path:     "/",
	})

	q := url.Values{
		"response_type": {"code"},
		"client_id":     {h.channelID},
		"redirect_uri":  {h.callbackURL},
		"state":         {state},
		"scope":         {"openid profile"},
	}
	// Prompt the candidate to add the linked Official Account as a friend during
	// login, so status/interview LINE pushes can actually reach them. Requires the
	// channels to be linked in the LINE console; LINE ignores it otherwise.
	if h.botPrompt != "" {
		q.Set("bot_prompt", h.botPrompt)
	}
	return c.Redirect(authorizeURL+"?"+q.Encode(), fiber.StatusFound)
}

// Callback completes the flow: validate state, exchange the code, and redirect
// back to the portal with the id_token (or an error) in the URL fragment.
func (h *Handler) Callback(c *fiber.Ctx) error {
	state, ret := h.parseStateCookie(c.Cookies(stateCookie))
	if ret == "" {
		ret = h.portalBaseURL
	}
	// Clear the one-time state cookie.
	c.Cookie(&fiber.Cookie{Name: stateCookie, Value: "", HTTPOnly: true, Secure: h.secureCookie, SameSite: "Lax", MaxAge: -1, Path: "/"})

	if e := c.Query("error"); e != "" {
		return c.Redirect(ret+"#line_error="+url.QueryEscape(e), fiber.StatusFound)
	}
	qState := c.Query("state")
	if state == "" || qState == "" || qState != state {
		return c.Redirect(ret+"#line_error=state_mismatch", fiber.StatusFound)
	}
	code := c.Query("code")
	if code == "" {
		return c.Redirect(ret+"#line_error=missing_code", fiber.StatusFound)
	}

	idToken, err := h.exchange(c.UserContext(), code)
	if err != nil {
		log.Warn().Err(err).Msg("lineauth: token exchange failed")
		return c.Redirect(ret+"#line_error=exchange_failed", fiber.StatusFound)
	}
	return c.Redirect(ret+"#line_id_token="+url.QueryEscape(idToken), fiber.StatusFound)
}

// exchange swaps the authorization code for an id_token at LINE's token endpoint.
func (h *Handler) exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {h.callbackURL},
		"client_id":     {h.channelID},
		"client_secret": {h.channelSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exchangeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("lineauth: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("lineauth: token call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("lineauth: token status %d: %s", resp.StatusCode, string(raw))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", fmt.Errorf("lineauth: token decode: %w", err)
	}
	if tok.IDToken == "" {
		return "", fmt.Errorf("lineauth: no id_token in response")
	}
	return tok.IDToken, nil
}

// safeReturn guards against open redirects: the return URL must be on the portal
// origin (scheme + host), else we fall back to the portal root.
func (h *Handler) safeReturn(ret string) string {
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

// parseStateCookie decodes the state|return cookie set at /login.
func (h *Handler) parseStateCookie(raw string) (state, ret string) {
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

func randToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
