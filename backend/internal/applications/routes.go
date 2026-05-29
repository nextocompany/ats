package applications

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts the Sprint 1 application + job-status endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1")
	v1.Post("/applications", h.Intake)
	v1.Get("/applications/:id", h.Get)
	v1.Patch("/applications/:id/status", h.UpdateStatus)
	v1.Get("/ai/jobs/:job_id", h.JobStatus)
}
