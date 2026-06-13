package interview

import "github.com/gofiber/fiber/v2"

// RegisterPublicRoutes mounts the candidate-facing interview endpoints under the
// public group (which is IP rate-limited at the app level).
func RegisterPublicRoutes(app *fiber.App, h *Handler) {
	p := app.Group("/api/v1/public/interview")
	p.Get("/:token", h.Start)
	p.Post("/:token/message", h.Respond)
}

// RegisterDashboardRoutes mounts the HR-facing interview endpoints. These live
// under the authed applications resource.
func RegisterDashboardRoutes(app *fiber.App, h *Handler) {
	a := app.Group("/api/v1/applications/:id/interview")
	a.Post("/", h.Invite)
	a.Get("/", h.Get)
}
