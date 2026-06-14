package members

import "github.com/gofiber/fiber/v2"

// RegisterDashboardRoutes mounts the HR member-management endpoints. Access is
// role-gated inside each handler (super_admin + hr_manager); members are global
// (not store-scoped). Fiber's router resolves static segments before parameterised
// ones, so /stats and /:id/resume are matched ahead of /:id regardless of order.
func RegisterDashboardRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/admin/members")
	g.Get("/", h.List)
	g.Get("/stats", h.Stats)
	g.Get("/:id", h.Detail)
	g.Get("/:id/resume", h.Resume)

	// Lifecycle (Phase B). Suspend/reactivate/edit/force-logout gate on
	// super_admin+hr_manager inside the handler; anonymize is super_admin-only.
	g.Patch("/:id", h.UpdateProfile)
	g.Patch("/:id/status", h.SetStatus)
	g.Post("/:id/force-logout", h.ForceLogout)
	g.Post("/:id/anonymize", h.Anonymize)
}
