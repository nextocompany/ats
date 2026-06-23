package positions

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/internal/scoring"
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
	// ScoreWeights is the EFFECTIVE per-position screening weights (the stored
	// config, or scoring.DefaultWeights when unset) — always present so the UI can
	// render the active weights without knowing the default.
	ScoreWeights scoring.Weights `json:"score_weights"`
}

// positionLister is the narrow slice of the repository the handler needs (accept
// interfaces, return structs). The concrete Repository satisfies it.
type positionLister interface {
	ListAll(ctx context.Context) ([]Position, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Position, error)
	UpdateScoreWeights(ctx context.Context, id uuid.UUID, w scoring.Weights) error
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
	app.Put("/api/v1/positions/:id/score-weights", h.UpdateScoreWeights)
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
	weights := scoring.DefaultWeights()
	if p.ScoreWeights != nil && p.ScoreWeights.Valid() {
		weights = *p.ScoreWeights
	}
	return httpx.OK(c, DetailItem{
		ID:               p.ID,
		TitleTH:          p.TitleTH,
		TitleEN:          p.TitleEN,
		Level:            p.Level,
		Responsibilities: p.Responsibilities,
		Qualifications:   p.Qualifications,
		Benefits:         p.Benefits,
		ScoreWeights:     weights,
	})
}

// userFrom returns the authenticated DevUser, or false if absent.
func userFrom(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return middleware.DevUser{}, false
	}
	return u, true
}

// UpdateScoreWeights handles PUT /api/v1/positions/:id/score-weights — set the
// per-position screening weights. Gated to settings.admin. Body is a Weights JSON
// object; weights must each be 0-100 and sum to 100.
func (h *Handler) UpdateScoreWeights(c *fiber.Ctx) error {
	u, ok := userFrom(c)
	if !ok || !rbac.Can(u.Role, rbac.PermSettingsAdmin) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to edit scoring weights")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid position id")
	}
	var w scoring.Weights
	if err := c.BodyParser(&w); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if !w.Valid() {
		return httpx.Fail(c, fiber.StatusBadRequest, "each weight must be 0-100 and the five must sum to 100")
	}
	if err := h.repo.UpdateScoreWeights(c.UserContext(), id, w); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Fail(c, fiber.StatusNotFound, "position not found")
		}
		return err
	}
	return httpx.OK(c, w)
}
