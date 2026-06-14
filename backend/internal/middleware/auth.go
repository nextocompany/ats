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
// AUTH_PROVIDER=real it validates an Azure AD (Entra) bearer token and populates
// the same UserContextKey→DevUser locals every handler already reads; otherwise
// it falls back to MockJWT (dev super_admin). Returns an error if real-auth
// discovery fails (fail fast at startup).
//
// allowAll is the runtime tenant toggle (may be nil ⇒ static allowlist only).
func Auth(ctx context.Context, cfg *config.Config, allowAll AllowAllTenantsReader) (fiber.Handler, error) {
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
		// HR auth gates the HR console only. The health probe, the LINE-authed
		// public career API, and the PeopleSoft machine webhooks are not HR-user
		// requests and bypass Entra validation. (PS webhook auth is a separate
		// machine-to-machine concern — see docs/SECURITY.md.)
		if isUnauthedPath(c.Path()) {
			return c.Next()
		}
		id, err := verifier.Verify(c.UserContext(), bearerToken(c))
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
		}
		c.Locals(UserContextKey, DevUser{
			ID: id.ID, Email: id.Email, Role: id.Role, StoreID: id.StoreID, Subregion: id.Subregion,
		})
		return c.Next()
	}, nil
}

// isUnauthedPath reports whether a path bypasses HR (Entra) auth.
func isUnauthedPath(path string) bool {
	return path == "/health" ||
		strings.HasPrefix(path, "/api/v1/public") ||
		strings.HasPrefix(path, "/api/v1/ps")
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
