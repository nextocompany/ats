// Package users serves the current-user endpoint. Full user CRUD is a later
// sprint; Sprint 4a exposes the authenticated identity for the dashboard.
package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// meResponse is the authenticated identity plus its resolved RBAC capabilities,
// so the dashboard gates UI on permissions (not hardcoded role lists).
type meResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// LocalID is the local users.id (uniform across SSO + password; ID is the Entra
	// OID for SSO). The UI uses it to tell whether a candidate lock is held by the
	// current user. Empty for an unprovisioned SSO identity.
	LocalID     string   `json:"local_id"`
	Role        string   `json:"role"`
	StoreID     *int     `json:"store_id"`
	Subregion   string   `json:"subregion"`
	Permissions []string `json:"permissions"`
	Scope       string   `json:"scope"`
}

// Handler serves user endpoints.
type Handler struct{}

// NewHandler builds the users handler.
func NewHandler() *Handler { return &Handler{} }

// RegisterRoutes mounts the user endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Group("/api/v1/users").Get("/me", h.Me)
}

// Me handles GET /api/v1/users/me — the authenticated user from context.
func (h *Handler) Me(c *fiber.Ctx) error {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "not authenticated")
	}
	return httpx.OK(c, meResponse{
		ID:          u.ID,
		LocalID:     u.LocalID,
		Email:       u.Email,
		Role:        u.Role,
		StoreID:     u.StoreID,
		Subregion:   u.Subregion,
		Permissions: rbac.Permissions(u.Role),
		Scope:       rbac.ScopeKindFor(u.Role),
	})
}
