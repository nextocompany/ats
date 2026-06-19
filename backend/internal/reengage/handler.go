package reengage

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the manual re-engagement trigger.
type Handler struct{ trigger *Trigger }

// NewHandler builds the re-engagement handler.
func NewHandler(trigger *Trigger) *Handler { return &Handler{trigger: trigger} }

// RegisterRoutes mounts the re-engagement trigger endpoint.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1")
	v1.Post("/positions/:id/reengage", h.Reengage)
}

// Reengage handles POST /api/v1/positions/:id/reengage — enqueues re-engagement
// of matching candidates for the position. Restricted to broad-visibility roles.
func (h *Handler) Reengage(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermReengageTrigger) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for re-engagement")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid position id")
	}
	if err := h.trigger.OnVacancyOpened(c.UserContext(), id); err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"position_id": id, "enqueued": true})
}
