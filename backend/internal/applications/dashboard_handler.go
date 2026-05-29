package applications

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const resumeURLTTL = 15 * time.Minute
const maxBulkIDs = 100

// ResumeSigner produces a short-lived signed URL for a stored blob URL.
type ResumeSigner interface {
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// ActivityWriter records audit entries.
type ActivityWriter interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
}

// DashboardHandler serves the HR inbox read/bulk/resume endpoints.
type DashboardHandler struct {
	apps     Repository
	signer   ResumeSigner
	activity ActivityWriter
}

// NewDashboardHandler builds the dashboard handler.
func NewDashboardHandler(apps Repository, signer ResumeSigner, act ActivityWriter) *DashboardHandler {
	return &DashboardHandler{apps: apps, signer: signer, activity: act}
}

// RegisterDashboardRoutes mounts the inbox endpoints.
func RegisterDashboardRoutes(app *fiber.App, h *DashboardHandler) {
	v1 := app.Group("/api/v1")
	v1.Get("/applications", h.List)
	v1.Post("/applications/bulk", h.Bulk)
	v1.Get("/applications/:id/resume", h.Resume)
}

func scopeFrom(c *fiber.Ctx) rbac.Scope {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.New(u.Role, u.StoreID, u.Subregion)
}

// List handles GET /api/v1/applications (ranked, filtered, scoped, paginated).
func (h *DashboardHandler) List(c *fiber.Ctx) error {
	f := ListFilter{
		Status:        c.Query("status"),
		SourceChannel: c.Query("source_channel"),
		Page:          atoiDefault(c.Query("page"), 1),
		Limit:         atoiDefault(c.Query("limit"), DefaultLimit),
	}
	if v := c.Query("min_score"); v != "" {
		s, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid min_score")
		}
		f.MinScore = &s
	}
	if v := c.Query("store_id"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid store_id")
		}
		f.StoreID = &n
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	items, total, err := h.apps.List(c.UserContext(), f, scopeFrom(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Application]{
		Success: true,
		Data:    items,
		Meta:    &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

type bulkReq struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"` // status | reject
	Value  string   `json:"value"`  // target status when action=status
}

// Bulk handles POST /api/v1/applications/bulk.
func (h *DashboardHandler) Bulk(c *fiber.Ctx) error {
	var req bulkReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if len(req.IDs) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "ids is required")
	}
	if len(req.IDs) > maxBulkIDs {
		return fiber.NewError(fiber.StatusBadRequest, "too many ids (max 100)")
	}

	target := req.Value
	switch req.Action {
	case "reject":
		target = StatusRejected
	case "status":
		if !allowedStatuses[target] {
			return fiber.NewError(fiber.StatusBadRequest, "unsupported target status")
		}
	default:
		return fiber.NewError(fiber.StatusBadRequest, "unsupported action")
	}

	var updated, failed int
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			failed++
			continue
		}
		if err := h.apps.SetStatus(c.UserContext(), id, target); err != nil {
			failed++
			continue
		}
		_ = h.activity.Record(c.UserContext(), activity.ActionBulkAction, "application", id, fiber.Map{"status": target})
		updated++
	}
	return httpx.OK(c, fiber.Map{"updated": updated, "failed": failed, "status": target})
}

// Resume handles GET /api/v1/applications/:id/resume → short-lived signed URL.
func (h *DashboardHandler) Resume(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	if app.RawFileBlobURL == "" {
		return fiber.NewError(fiber.StatusNotFound, "no resume on file")
	}
	url, err := h.signer.SignedURLForStored(app.RawFileBlobURL, resumeURLTTL)
	if err != nil {
		return err
	}
	_ = h.activity.Record(c.UserContext(), activity.ActionViewResume, "application", id, nil)
	return httpx.OK(c, fiber.Map{"url": url, "expires_in_seconds": int(resumeURLTTL.Seconds())})
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
