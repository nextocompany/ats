package members

import "github.com/gofiber/fiber/v2"

// RegisterDashboardRoutes mounts the HR member-management endpoints. Access is
// role-gated inside each handler (super_admin + hr_manager); members are global
// (not store-scoped).
//
// Route ORDER matters here: this Fiber setup matches in registration order, so
// every static segment (/stats, /export.csv, /bulk) MUST be registered before the
// parameterised /:id routes — otherwise "export.csv" is captured as an :id and the
// Detail handler 400s on the bad UUID (same lesson as candidates/search in Phase A).
func RegisterDashboardRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/admin/members")

	// Static segments first.
	g.Get("/", h.List)
	g.Get("/stats", h.Stats)
	g.Get("/export.csv", h.Export) // CRM (Phase C)
	g.Post("/bulk", h.Bulk)        // CRM (Phase C)

	// Parameterised routes.
	g.Get("/:id", h.Detail)
	g.Get("/:id/resume", h.Resume)

	// Lifecycle (Phase B). Suspend/reactivate/edit/force-logout gate on
	// super_admin+hr_manager inside the handler; anonymize is super_admin-only.
	g.Patch("/:id", h.UpdateProfile)
	g.Patch("/:id/status", h.SetStatus)
	g.Post("/:id/force-logout", h.ForceLogout)
	g.Post("/:id/anonymize", h.Anonymize)

	// CRM (Phase C): per-member HR notes + tags.
	g.Get("/:id/notes", h.ListNotes)
	g.Post("/:id/notes", h.AddNote)
	g.Get("/:id/tags", h.ListTags)
	g.Post("/:id/tags", h.AddTag)
	g.Delete("/:id/tags", h.RemoveTag)
}
