package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// EnforceOrigin rejects state-changing (non-safe-method) requests whose Origin
// header is present but not in the allowlist. This is the CSRF defense for the
// HR API once cookie-based sessions exist: the hr_auth cookie is SameSite=None in
// prod (dashboard↔api are cross-site under the apps.io public suffix), so a
// browser would otherwise attach it to a forged cross-site POST. Safe methods and
// requests with no Origin header (server-to-server, machine webhooks) pass — those
// are not browser-driven CSRF vectors. Bearer-authed requests are unaffected
// because the dashboard origin is always in the allowlist.
func EnforceOrigin(allowedOrigins string) fiber.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(allowedOrigins, ",") {
		if t := strings.TrimSpace(o); t != "" {
			allowed[t] = true
		}
	}
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		if origin := c.Get("Origin"); origin != "" && !allowed[origin] {
			return fiber.NewError(fiber.StatusForbidden, "cross-origin request rejected")
		}
		return c.Next()
	}
}
