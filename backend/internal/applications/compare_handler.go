package applications

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// defaultCompareLimit caps how many ranked candidates the compare view returns.
const (
	defaultCompareLimit = 50
	maxCompareLimit     = 200
)

// CompareResponse is the per-position comparison payload: the resolved position
// plus the ranked, eligible candidates (highest composite first).
type CompareResponse struct {
	PositionID    string        `json:"position_id"`
	PositionTitle string        `json:"position_title"`
	Candidates    []CompareItem `json:"candidates"`
}

// compareStore is the narrow repository slice this handler needs.
type compareStore interface {
	CompareByPosition(ctx context.Context, positionID uuid.UUID, scope rbac.Scope, limit int) ([]CompareItem, error)
}

// CompareHandler serves the per-position Compare Candidates ranking.
type CompareHandler struct {
	apps compareStore
	pos  positions.Repository
}

// NewCompareHandler builds the compare handler.
func NewCompareHandler(apps compareStore, pos positions.Repository) *CompareHandler {
	return &CompareHandler{apps: apps, pos: pos}
}

// RegisterCompareRoutes mounts the compare endpoint.
func RegisterCompareRoutes(app *fiber.App, h *CompareHandler) {
	app.Get("/api/v1/compare", h.List)
}

// List handles GET /api/v1/compare?position_id=&lt;uuid&gt;&limit=&lt;n&gt;. Returns the
// eligible candidates for the position the caller may see, ranked by the
// screening+AI-interview composite. Visibility is enforced by the RBAC scope
// inside the query (a store-scoped user only sees their store's candidates).
func (h *CompareHandler) List(c *fiber.Ctx) error {
	pid, err := uuid.Parse(c.Query("position_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}
	limit := defaultCompareLimit
	if v := c.Query("limit"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 && n <= maxCompareLimit {
			limit = n
		}
	}
	items, err := h.apps.CompareByPosition(c.UserContext(), pid, scopeFrom(c), limit)
	if err != nil {
		return err
	}
	if items == nil {
		items = []CompareItem{}
	}
	// Position title is best-effort (English preferred, Thai fallback); an empty
	// title never fails the request.
	title := ""
	if p, perr := h.pos.FindByID(c.UserContext(), pid); perr == nil && p != nil {
		if p.TitleEN != "" {
			title = p.TitleEN
		} else {
			title = p.TitleTH
		}
	}
	return httpx.OK(c, CompareResponse{
		PositionID:    pid.String(),
		PositionTitle: title,
		Candidates:    items,
	})
}
