package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/pkg/config"
)

// Auth returns the authentication middleware selected by config. When
// AUTH_PROVIDER=real it validates an Azure AD (Entra) bearer token and populates
// the same UserContextKey→DevUser locals every handler already reads; otherwise
// it falls back to MockJWT (dev super_admin). Returns an error if real-auth
// discovery fails (fail fast at startup).
func Auth(ctx context.Context, cfg *config.Config) (fiber.Handler, error) {
	if !cfg.UsesRealAuth() {
		return MockJWT(cfg.IsDevelopment()), nil
	}
	verifier, err := auth.NewEntraVerifier(ctx, cfg)
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
