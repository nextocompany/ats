package executive

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the executive overview endpoint.
type Handler struct {
	svc Service
}

// NewHandler builds the executive handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the executive overview endpoint.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1/executive")
	v1.Get("/overview", h.Overview)
}

// Overview handles GET /api/v1/executive/overview.
func (h *Handler) Overview(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermExecutiveView) {
		return fiber.NewError(fiber.StatusForbidden, "executive overview is restricted to leadership roles")
	}
	ov, err := h.svc.Overview(c.UserContext())
	if err != nil {
		return err
	}
	ov.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return httpx.OK(c, ov)
}
