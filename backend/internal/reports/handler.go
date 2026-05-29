package reports

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the analytics endpoints.
type Handler struct{ repo *Repo }

// NewHandler builds the reports handler.
func NewHandler(repo *Repo) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the analytics endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1/reports")
	v1.Get("/funnel", h.Funnel)
	v1.Get("/kpi", h.KPI)
	v1.Get("/sources", h.Sources)
}

// Funnel handles GET /api/v1/reports/funnel.
func (h *Handler) Funnel(c *fiber.Ctx) error {
	f, err := h.repo.Funnel(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, f)
}

// KPI handles GET /api/v1/reports/kpi.
func (h *Handler) KPI(c *fiber.Ctx) error {
	k, err := h.repo.KPI(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, k)
}

// Sources handles GET /api/v1/reports/sources.
func (h *Handler) Sources(c *fiber.Ctx) error {
	s, err := h.repo.Sources(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, s)
}
