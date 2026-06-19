package stores

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// ListItem is the slim store projection for UI pickers (the placement/reassign
// dropdown) — number, name, and locality, without coordinates.
type ListItem struct {
	StoreNo   int    `json:"store_no"`
	StoreName string `json:"store_name"`
	Subregion string `json:"subregion"`
	Province  string `json:"province"`
}

// storeLister is the narrow slice of the repository the handler needs.
type storeLister interface {
	List(ctx context.Context) ([]Store, error)
}

// Handler serves store reference data for the dashboard.
type Handler struct {
	repo storeLister
}

// NewHandler builds the stores handler.
func NewHandler(repo storeLister) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the stores reference endpoint.
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Get("/api/v1/stores", h.List)
}

// List handles GET /api/v1/stores — reference data, readable by any authenticated
// HR user (no role gate), used to populate the placement/reassign picker.
func (h *Handler) List(c *fiber.Ctx) error {
	rows, err := h.repo.List(c.UserContext())
	if err != nil {
		return err
	}
	out := make([]ListItem, 0, len(rows))
	for _, s := range rows {
		out = append(out, ListItem{StoreNo: s.StoreNo, StoreName: s.StoreName, Subregion: s.Subregion, Province: s.Province})
	}
	return httpx.OK(c, out)
}
