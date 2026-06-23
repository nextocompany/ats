package positions

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// ListItem is the slim position projection for UI pickers (e.g. the bulk-upload
// position dropdown) — id + titles only, without the heavy Master JD text.
type ListItem struct {
	ID      uuid.UUID `json:"id"`
	TitleTH string    `json:"title_th"`
	TitleEN string    `json:"title_en"`
}

// DetailItem is the full position projection for the requisition form. It carries
// the Master JD text so the dashboard can prefill responsibilities / qualifications
// / benefits when HR picks a position.
type DetailItem struct {
	ID               uuid.UUID `json:"id"`
	TitleTH          string    `json:"title_th"`
	TitleEN          string    `json:"title_en"`
	Level            string    `json:"level"`
	Responsibilities string    `json:"responsibilities"`
	Qualifications   string    `json:"qualifications"`
	Benefits         string    `json:"benefits"`
}

// positionLister is the narrow slice of the repository the handler needs (accept
// interfaces, return structs). The concrete Repository satisfies it.
type positionLister interface {
	ListAll(ctx context.Context) ([]Position, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Position, error)
}

// Handler serves position reference data for the dashboard.
type Handler struct {
	repo positionLister
}

// NewHandler builds the positions handler.
func NewHandler(repo positionLister) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the positions reference endpoints. The static list route
// is declared before the parameterised detail route so Fiber routing is unambiguous.
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Get("/api/v1/positions", h.List)
	app.Get("/api/v1/positions/:id", h.Detail)
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

// Detail handles GET /api/v1/positions/:id — a single position with its Master JD
// text, used by the requisition form to prefill responsibilities / qualifications /
// benefits. Reference data, readable by any authenticated HR user (no role gate),
// matching List.
func (h *Handler) Detail(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid position id")
	}
	p, err := h.repo.FindByID(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Fail(c, fiber.StatusNotFound, "position not found")
		}
		return err
	}
	return httpx.OK(c, DetailItem{
		ID:               p.ID,
		TitleTH:          p.TitleTH,
		TitleEN:          p.TitleEN,
		Level:            p.Level,
		Responsibilities: p.Responsibilities,
		Qualifications:   p.Qualifications,
		Benefits:         p.Benefits,
	})
}
