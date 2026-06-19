package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/pkg/config"
)

// AllowAllTenantsReader exposes the runtime, admin-managed "allow all Entra
// tenants" toggle. The settings.Service satisfies it; defined here so this package
// does not import settings (which imports this package for DevUser).
type AllowAllTenantsReader interface {
	AllowAllTenants(ctx context.Context) bool
}

// HRAuthCookieName is the httpOnly cookie carrying the opaque HR password-session
// token. Defined here (not in hrauth) so the auth middleware can read it without
// importing hrauth — hrauth imports this package, so the dependency only runs one
// way. hrauth references this constant when writing the cookie.
const HRAuthCookieName = "hr_auth"

// HRSessionValidator resolves an HR password-session token to an authenticated
// identity. The hrauth.Service satisfies it; defined here (like AllowAllTenantsReader)
// to keep the dependency one-directional. A nil validator ⇒ password sessions are
// not accepted (Entra-only deployment).
type HRSessionValidator interface {
	ValidateSession(ctx context.Context, token string) (auth.Identity, bool)
}

// entraTenantPolicy is the authorisation gate for which directories may sign in:
// the static AZURE_AD_ALLOWED_TENANTS allowlist, OR — when the admin toggle is on
// — any tenant. The verifier has already validated the token cryptographically
// and bound the issuer to the tenant before this runs.
type entraTenantPolicy struct {
	allowed map[string]struct{}   // lower-cased static allowlist
	toggle  AllowAllTenantsReader // runtime "allow all"; nil ⇒ allowlist only
}

func (p entraTenantPolicy) AllowsTenant(ctx context.Context, tenantID string) bool {
	if _, ok := p.allowed[tenantID]; ok {
		return true
	}
	return p.toggle != nil && p.toggle.AllowAllTenants(ctx)
}

// Auth returns the authentication middleware selected by config. When
// AUTH_PROVIDER=real it accepts EITHER an Azure AD (Entra) bearer token OR a local
// HR password session (the hr_auth httpOnly cookie, resolved via sessions), and
// populates the same UserContextKey→DevUser locals every handler already reads;
// otherwise it falls back to MockJWT (dev super_admin). Returns an error if
// real-auth discovery fails (fail fast at startup).
//
// allowAll is the runtime tenant toggle (may be nil ⇒ static allowlist only).
// sessions is the HR password-session validator (may be nil ⇒ Entra-only).
func Auth(ctx context.Context, cfg *config.Config, allowAll AllowAllTenantsReader, sessions HRSessionValidator) (fiber.Handler, error) {
	if !cfg.UsesRealAuth() {
		return MockJWT(cfg.IsDevelopment()), nil
	}
	allowed := make(map[string]struct{})
	for _, t := range cfg.AllowedTenantList() {
		allowed[strings.ToLower(t)] = struct{}{}
	}
	policy := entraTenantPolicy{allowed: allowed, toggle: allowAll}
	verifier, err := auth.NewEntraVerifier(ctx, cfg, policy)
	if err != nil {
		return nil, err
	}
	return func(c *fiber.Ctx) error {
		// HR auth gates the HR console only. The health probe, the password
		// login/logout endpoints, the LINE-authed public career API, and the
		// PeopleSoft machine webhooks bypass token validation. (PS webhook auth is
		// a separate machine-to-machine concern — see docs/SECURITY.md.)
		if isUnauthedPath(c.Path()) {
			return c.Next()
		}
		// Path 1: Entra bearer token (SSO).
		if tok := bearerToken(c); tok != "" {
			if id, vErr := verifier.Verify(c.UserContext(), tok); vErr == nil {
				setUser(c, id)
				return c.Next()
			}
		}
		// Path 2: local HR password session (httpOnly cookie).
		if sessions != nil {
			if id, ok := sessions.ValidateSession(c.UserContext(), c.Cookies(HRAuthCookieName)); ok {
				setUser(c, id)
				return c.Next()
			}
		}
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}, nil
}

// setUser stores the resolved identity on the request locals.
func setUser(c *fiber.Ctx, id auth.Identity) {
	c.Locals(UserContextKey, DevUser{
		ID: id.ID, Email: id.Email, Role: id.Role, StoreID: id.StoreID, Subregion: id.Subregion,
	})
}

// isUnauthedPath reports whether a path bypasses HR auth. The password
// login/logout endpoints must be reachable before a session exists.
func isUnauthedPath(path string) bool {
	return path == "/health" ||
		path == "/api/v1/auth/login" ||
		path == "/api/v1/auth/logout" ||
		strings.HasPrefix(path, "/api/v1/public") ||
		strings.HasPrefix(path, "/api/v1/ps") ||
		strings.HasPrefix(path, "/api/v1/intake/") // HMAC-authed external intake webhook (trailing slash: no prefix bleed)
}

// bearerToken extracts the token from an "Authorization: Bearer <jwt>" header.
func bearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
