package peoplesoft

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts the PeopleSoft integration endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	ps := app.Group("/api/v1/ps")
	ps.Post("/vacancy-opened", h.VacancyOpened)
	ps.Post("/vacancy-closed", h.VacancyClosed)
	ps.Post("/sync-hired", h.SyncHired)
	ps.Get("/health", h.Health)
}
