package candidateauth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// EnforceOrigin rejects state-changing (non-safe-method) cross-origin requests
// whose Origin is not in the allowlist. This is the CSRF defense for cookie-authed
// endpoints: the session cookie is SameSite=None in prod (portal↔api are cross-
// site), and "simple" requests like multipart uploads bypass CORS preflight — so
// an Origin check is required to stop a malicious site from forging resume uploads
// or applications with the victim's cookie. Safe methods and requests with no
// Origin header (non-browser clients) pass through.
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

// candidateLocalsKey is where the resolved *Account is stored on the request.
const candidateLocalsKey = "candidate_account"

// RequireCandidate gates a route behind a valid candidate session cookie. It
// resolves the account and stores it in c.Locals, or returns 401.
func RequireCandidate(svc *Service, cookieName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		acct := resolveAccount(c, svc, cookieName)
		if acct == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "login required")
		}
		c.Locals(candidateLocalsKey, acct)
		return c.Next()
	}
}

// OptionalCandidate resolves the account when a valid session is present but never
// blocks the request.
func OptionalCandidate(svc *Service, cookieName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if acct := resolveAccount(c, svc, cookieName); acct != nil {
			c.Locals(candidateLocalsKey, acct)
		}
		return c.Next()
	}
}

// CandidateFromCtx returns the account set by the middleware, or nil.
func CandidateFromCtx(c *fiber.Ctx) *Account {
	if a, ok := c.Locals(candidateLocalsKey).(*Account); ok {
		return a
	}
	return nil
}

func resolveAccount(c *fiber.Ctx, svc *Service, cookieName string) *Account {
	tok := c.Cookies(cookieName)
	if tok == "" {
		return nil
	}
	acct, err := svc.AccountFromSession(c.UserContext(), tok)
	if err != nil {
		return nil
	}
	return acct
}
