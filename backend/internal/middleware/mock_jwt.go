package middleware

import "github.com/gofiber/fiber/v2"

// UserContextKey is the Locals key under which the authenticated user is stored.
const UserContextKey = "user"

// DevUser is the fixed identity injected during local development in place of
// real Azure AD SSO. It must NEVER be active outside ENV=development.
type DevUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// LocalID is the local users.id (uniform across SSO + password; empty for an
	// unprovisioned SSO identity). Use it — not ID — for area/requisition scope.
	LocalID   string `json:"local_id"`
	Role      string `json:"role"`
	StoreID   *int   `json:"store_id"`
	Subregion string `json:"subregion"`
}

// MockJWT injects a fixed super_admin user so handlers can read auth context
// locally without Azure AD. When enabled is false it is a no-op, which
// guarantees it cannot leak into production.
func MockJWT(enabled bool) fiber.Handler {
	dev := DevUser{
		ID:      "00000000-0000-0000-0000-000000000001",
		LocalID: "00000000-0000-0000-0000-000000000001",
		Email:   "dev.superadmin@local.test",
		Role:    "super_admin", // sees all; role scoping is unit-tested across roles
	}
	return func(c *fiber.Ctx) error {
		if enabled {
			c.Locals(UserContextKey, dev)
		}
		return c.Next()
	}
}
