package applications

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// defaultShortlistLimit caps the Line-Manager shortlist (Top-N awaiting review).
const defaultShortlistLimit = 5

// ShortlistItem is one ranked candidate on a Line Manager's shortlist.
type ShortlistItem struct {
	ApplicationID   uuid.UUID `json:"application_id"`
	CandidateName   string    `json:"candidate_name"`
	PositionID      string    `json:"position_id"`
	PositionTitle   string    `json:"position_title"`
	AssignedStoreID *int      `json:"assigned_store_id"`
	AIScore         *float64  `json:"ai_score"`
	TAAvgOverall    *float64  `json:"ta_avg_overall"` // mean TA overall_rating (1..5), nil if none
	Composite       float64   `json:"composite"`      // ranking score (0..100)
}

// shortlistStore is the narrow repository slice this handler needs.
type shortlistStore interface {
	Shortlist(ctx context.Context, scope rbac.Scope, limit int) ([]ShortlistItem, error)
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	ListFeedback(ctx context.Context, applicationID uuid.UUID) ([]InterviewFeedback, error)
}

// ShortlistHandler serves the Line-Manager Top-5 shortlist and the per-application
// scorecard summary.
type ShortlistHandler struct {
	apps shortlistStore
}

// NewShortlistHandler builds the shortlist handler.
func NewShortlistHandler(apps shortlistStore) *ShortlistHandler {
	return &ShortlistHandler{apps: apps}
}

// RegisterShortlistRoutes mounts the shortlist + scorecard-summary endpoints.
func RegisterShortlistRoutes(app *fiber.App, h *ShortlistHandler) {
	app.Get("/api/v1/shortlist", h.List)
	app.Get("/api/v1/applications/:id/scorecard-summary", h.ScorecardSummary)
}

// List handles GET /api/v1/shortlist. Returns the top-N shortlisted candidates the
// caller may see; a store-scoped Line Manager (sgm) only sees their own store.
func (h *ShortlistHandler) List(c *fiber.Ctx) error {
	limit := defaultShortlistLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	items, err := h.apps.Shortlist(c.UserContext(), scopeFrom(c), limit)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// ScorecardSummary handles GET /api/v1/applications/:id/scorecard-summary. Combines
// the TA and Line-Manager scorecards plus the composite ranking score.
func (h *ShortlistHandler) ScorecardSummary(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	list, err := h.apps.ListFeedback(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, SummarizeFeedback(list, app.AIScore))
}
