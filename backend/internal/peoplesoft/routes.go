package peoplesoft

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts the PeopleSoft integration endpoints. When secret != ""
// the state-changing POST webhooks require a valid X-PS-Signature (HMAC-SHA256 of
// the raw body); GET /health stays open as a probe. An empty secret (dev/CI,
// PS_PROVIDER=mock) leaves the group unauthenticated, preserving prior behavior.
func RegisterRoutes(app *fiber.App, h *Handler, secret string) {
	ps := app.Group("/api/v1/ps")
	ps.Get("/health", h.Health) // open probe — registered before the HMAC guard
	if secret != "" {
		ps.Use(VerifyHMAC(secret))
	}
	ps.Post("/vacancy-opened", h.VacancyOpened)
	ps.Post("/vacancy-closed", h.VacancyClosed)
	ps.Post("/sync-hired", h.SyncHired)
}
