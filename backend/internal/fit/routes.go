package fit

import "github.com/gofiber/fiber/v2"

// RegisterDashboardRoutes mounts the HR-facing fit-analysis endpoints under the
// authed applications resource.
func RegisterDashboardRoutes(app *fiber.App, h *Handler) {
	a := app.Group("/api/v1/applications/:id/fit-analysis")
	a.Post("/", h.Generate)
	a.Get("/", h.Get)
}
