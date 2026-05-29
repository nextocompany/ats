// Package users serves the current-user endpoint. Full user CRUD is a later
// sprint; Sprint 4a exposes the authenticated identity for the dashboard.
package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

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
	return httpx.OK(c, u)
}
