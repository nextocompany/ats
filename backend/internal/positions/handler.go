package positions

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// ListItem is the slim position projection for UI pickers (e.g. the bulk-upload
// position dropdown) — id + titles only, without the heavy Master JD text.
type ListItem struct {
	ID      uuid.UUID `json:"id"`
	TitleTH string    `json:"title_th"`
	TitleEN string    `json:"title_en"`
}

// positionLister is the narrow slice of the repository the handler needs (accept
// interfaces, return structs). The concrete Repository satisfies it.
type positionLister interface {
	ListAll(ctx context.Context) ([]Position, error)
}

// Handler serves position reference data for the dashboard.
type Handler struct {
	repo positionLister
}

// NewHandler builds the positions handler.
func NewHandler(repo positionLister) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the positions reference endpoint.
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Get("/api/v1/positions", h.List)
}

// List handles GET /api/v1/positions — active positions for a picker. Reference
// data, readable by any authenticated HR user (no role gate).
func (h *Handler) List(c *fiber.Ctx) error {
	all, err := h.repo.ListAll(c.UserContext())
	if err != nil {
		return err
	}
	items := make([]ListItem, 0, len(all))
	for _, p := range all {
		items = append(items, ListItem{ID: p.ID, TitleTH: p.TitleTH, TitleEN: p.TitleEN})
	}
	return httpx.OK(c, items)
}
