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

	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/pkg/config"
)

// SessionIssuer turns a verified LINE identity into a candidate session (account-
// first mode). It is satisfied by candidateauth.Service. When nil, the handler
// keeps the legacy behaviour of returning the id_token in the URL fragment.
type SessionIssuer interface {
	// LoginWithLine finds-or-creates the account for a LINE sub and returns a raw
	// session token + its expiry (for the session cookie).
	LoginWithLine(ctx context.Context, sub, name, email string) (token string, expires time.Time, err error)
	// LinkLine attaches the LINE identity to the account behind an existing session.
	LinkLine(ctx context.Context, sessionToken, sub, displayID string) error
}

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

	// Account-first wiring (nil ⇒ legacy fragment-token mode).
	issuer            SessionIssuer
	verifier          auth.Verifier
	sessionCookieName string
}

// NewHandler builds the LINE OAuth handler from config. When issuer is non-nil the
// callback creates/links a candidate account and sets the session cookie (account-
// first); when nil it falls back to returning the id_token in the URL fragment.
func NewHandler(cfg *config.Config, issuer SessionIssuer, verifier auth.Verifier) *Handler {
	return &Handler{
		channelID:         cfg.LINEChannelID,
		channelSecret:     cfg.LINEChannelSecret,
		callbackURL:       cfg.LINELoginCallbackURL,
		portalBaseURL:     strings.TrimRight(cfg.PortalBaseURL, "/"),
		botPrompt:         cfg.LINELoginBotPrompt,
		real:              cfg.UsesRealLINE(),
		secureCookie:      !cfg.IsDevelopment(),
		http:              &http.Client{Timeout: 10 * time.Second},
		issuer:            issuer,
		verifier:          verifier,
		sessionCookieName: cfg.SessionCookieName,
	}
}

// RegisterRoutes mounts the OAuth endpoints under the public group.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/public/line")
	g.Get("/login", h.Login)
	g.Get("/callback", h.Callback)
}

// Login starts the flow: redirect to LINE (real) or bounce back with the stub (mock).
// `mode=link` attaches LINE to the already-logged-in account instead of logging in.
func (h *Handler) Login(c *fiber.Ctx) error {
	ret := h.safeReturn(c.Query("return"))
	mode := c.Query("mode")

	if !h.real {
		// Account-first mock: synthesize a verified identity and finalize directly.
		if h.issuer != nil {
			lineUser, _ := h.verifier.Verify(c.UserContext(), devLineStub)
			return h.finalize(c, ret, mode, lineUser)
		}
		return c.Redirect(ret+"#line_id_token="+devLineStub, fiber.StatusFound)
	}

	state, err := randToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "state generation failed")
	}
	// Bind the CSRF state + return URL + mode in a short-lived httpOnly cookie.
	c.Cookie(&fiber.Cookie{
		Name:     stateCookie,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(state + "\x00" + ret + "\x00" + mode)),
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

// Callback completes the flow: validate state, exchange the code, then either
// (account-first) create/link an account + set the session cookie, or (legacy)
// return the id_token in the URL fragment.
func (h *Handler) Callback(c *fiber.Ctx) error {
	state, ret, mode := h.parseStateCookie(c.Cookies(stateCookie))
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

	// Legacy mode: hand the id_token to the portal in the fragment.
	if h.issuer == nil {
		return c.Redirect(ret+"#line_id_token="+url.QueryEscape(idToken), fiber.StatusFound)
	}

	// Account-first: verify the id_token to extract the LINE identity, then finalize.
	lineUser, verr := h.verifier.Verify(c.UserContext(), idToken)
	if verr != nil {
		log.Warn().Err(verr).Msg("lineauth: id_token verify failed")
		return c.Redirect(ret+"#line_error=verify_failed", fiber.StatusFound)
	}
	return h.finalize(c, ret, mode, lineUser)
}

// finalize either links LINE to the current session (mode=link) or logs in
// (find-or-create account + set session cookie), then redirects to the portal.
func (h *Handler) finalize(c *fiber.Ctx, ret, mode string, lineUser auth.LineUser) error {
	if mode == "link" {
		sessTok := c.Cookies(h.sessionCookieName)
		if sessTok == "" {
			return c.Redirect(ret+"#line_error=not_logged_in", fiber.StatusFound)
		}
		if err := h.issuer.LinkLine(c.UserContext(), sessTok, lineUser.Subject, ""); err != nil {
			log.Warn().Err(err).Msg("lineauth: link line failed")
			return c.Redirect(ret+"#line_error=link_failed", fiber.StatusFound)
		}
		return c.Redirect(ret+"#line_linked=1", fiber.StatusFound)
	}

	tok, exp, err := h.issuer.LoginWithLine(c.UserContext(), lineUser.Subject, lineUser.Name, lineUser.Email)
	if err != nil {
		log.Warn().Err(err).Msg("lineauth: login failed")
		return c.Redirect(ret+"#line_error=login_failed", fiber.StatusFound)
	}
	h.setSessionCookie(c, tok, exp)
	return c.Redirect(ret, fiber.StatusFound)
}

// setSessionCookie writes the candidate session cookie. Must mirror candidateauth's
// attributes: SameSite=None + Secure in prod (cross-site portal↔api).
func (h *Handler) setSessionCookie(c *fiber.Ctx, token string, expires time.Time) {
	sameSite := "Lax"
	if h.secureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     h.sessionCookieName,
		Value:    token,
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: sameSite,
		Expires:  expires,
		Path:     "/",
	})
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

// parseStateCookie decodes the state|return|mode cookie set at /login. Older
// two-part cookies (no mode) still parse with an empty mode.
func (h *Handler) parseStateCookie(raw string) (state, ret, mode string) {
	if raw == "" {
		return "", "", ""
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", ""
	}
	parts := strings.SplitN(string(b), "\x00", 3)
	if len(parts) < 2 {
		return "", "", ""
	}
	if len(parts) == 3 {
		mode = parts[2]
	}
	return parts[0], h.safeReturn(parts[1]), mode
}

func randToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
