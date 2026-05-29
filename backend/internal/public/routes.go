package public

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts the public Career API.
func RegisterRoutes(app *fiber.App, h *Handler) {
	p := app.Group("/api/v1/public")
	p.Get("/positions", h.ListPositions)
	p.Get("/positions/:id", h.GetPosition)
	p.Post("/apply", h.Apply)
	p.Get("/status/:token", h.Status)
}
