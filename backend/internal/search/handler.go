package search

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves candidate search.
type Handler struct{ searcher Searcher }

// NewHandler builds the search handler.
func NewHandler(s Searcher) *Handler { return &Handler{searcher: s} }

// RegisterRoutes mounts the candidate search endpoint.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1")
	v1.Get("/candidates/search", h.Search)
}

func scopeFrom(c *fiber.Ctx) rbac.Scope {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
}

// Search handles GET /api/v1/candidates/search?q=&status=&min_score=&page=&limit=.
func (h *Handler) Search(c *fiber.Ctx) error {
	q := Query{
		Text:   c.Query("q"),
		Status: c.Query("status"),
		Page:   atoiDefault(c.Query("page"), 1),
		Limit:  atoiDefault(c.Query("limit"), DefaultLimit),
	}
	if q.Text == "" {
		return fiber.NewError(fiber.StatusBadRequest, "q is required")
	}
	if v := c.Query("min_score"); v != "" {
		s, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid min_score")
		}
		q.MinScore = &s
	}

	hits, total, err := h.searcher.Search(c.UserContext(), q, scopeFrom(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Hit]{
		Success: true,
		Data:    hits,
		Meta:    &httpx.Meta{Total: total, Page: q.Page, Limit: q.Limit},
	})
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
