// Package profiles serves the HR candidate-facing read API (list, detail with
// applications, timeline). It composes the candidates + applications
// repositories, so neither needs to import the other.
package profiles

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves candidate read endpoints.
type Handler struct {
	cands candidates.Repository
	apps  applications.Repository
}

// NewHandler builds the profiles handler.
func NewHandler(cands candidates.Repository, apps applications.Repository) *Handler {
	return &Handler{cands: cands, apps: apps}
}

// RegisterRoutes mounts the candidate read endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1")
	v1.Get("/candidates", h.List)
	v1.Get("/candidates/:id", h.Detail)
	v1.Get("/candidates/:id/timeline", h.Timeline)
}

func scopeFrom(c *fiber.Ctx) rbac.Scope {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.New(u.Role, u.StoreID, u.Subregion)
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// List handles GET /api/v1/candidates.
func (h *Handler) List(c *fiber.Ctx) error {
	f := candidates.Filter{
		Status: c.Query("status"),
		Page:   atoiDefault(c.Query("page"), 1),
		Limit:  atoiDefault(c.Query("limit"), candidates.DefaultLimit),
	}
	items, total, err := h.cands.List(c.UserContext(), f, scopeFrom(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]candidates.Candidate]{
		Success: true, Data: items, Meta: &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

// Detail handles GET /api/v1/candidates/:id (candidate + applications).
func (h *Handler) Detail(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid candidate id")
	}
	cand, err := h.cands.FindByID(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "candidate not found")
	}
	apps, err := h.apps.ListByCandidate(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"candidate": cand, "applications": apps})
}

// Timeline handles GET /api/v1/candidates/:id/timeline.
func (h *Handler) Timeline(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid candidate id")
	}
	entries, err := h.cands.Timeline(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, entries)
}
