package reports

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const (
	exportListLimit = 50
	// Download links must outlive the report's relevance window so a link opened
	// from the list days after generation still works.
	downloadSignedTTL = 7 * 24 * time.Hour
)

// BlobSigner signs stored blob URLs for download links.
type BlobSigner interface {
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// Handler serves the analytics + export endpoints.
type Handler struct {
	repo     *Repo
	exporter *ExportService
	signer   BlobSigner
}

// NewHandler builds the reports handler. exporter and signer may be nil when only
// the read-only analytics endpoints are needed.
func NewHandler(repo *Repo, exporter *ExportService, signer BlobSigner) *Handler {
	return &Handler{repo: repo, exporter: exporter, signer: signer}
}

// RegisterRoutes mounts the analytics + export endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1/reports")
	v1.Get("/funnel", h.Funnel)
	v1.Get("/kpi", h.KPI)
	v1.Get("/sources", h.Sources)
	v1.Get("/by-store", h.WaitingByStore)
	v1.Get("/open-roles", h.OpenRoles)
	v1.Get("/exports", h.ListExports)
	v1.Post("/exports", h.TriggerExport)
}

// operationsLimit caps the dashboard operational panels (top-N stores / roles).
const operationsLimit = 8

func clampLimit(c *fiber.Ctx) int {
	n := c.QueryInt("limit", operationsLimit)
	if n < 1 || n > 50 {
		return operationsLimit
	}
	return n
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

// WaitingByStore handles GET /api/v1/reports/by-store.
func (h *Handler) WaitingByStore(c *fiber.Ctx) error {
	items, err := h.repo.WaitingByStore(c.UserContext(), clampLimit(c))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// OpenRoles handles GET /api/v1/reports/open-roles.
func (h *Handler) OpenRoles(c *fiber.Ctx) error {
	items, err := h.repo.OpenRoles(c.UserContext(), clampLimit(c))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// exportView augments a stored export with short-lived signed download links.
type exportView struct {
	Export
	CSVURL  string `json:"csv_url,omitempty"`
	JSONURL string `json:"json_url,omitempty"`
}

// ListExports handles GET /api/v1/reports/exports.
func (h *Handler) ListExports(c *fiber.Ctx) error {
	items, err := h.repo.ListExports(c.UserContext(), exportListLimit)
	if err != nil {
		return err
	}
	out := make([]exportView, 0, len(items))
	for _, e := range items {
		v := exportView{Export: e}
		if h.signer != nil {
			if u, serr := h.signer.SignedURLForStored(e.CSVBlob, downloadSignedTTL); serr == nil {
				v.CSVURL = u
			} else {
				log.Warn().Err(serr).Str("export_id", e.ID.String()).Msg("reports: sign csv link failed")
			}
			if u, serr := h.signer.SignedURLForStored(e.JSONBlob, downloadSignedTTL); serr == nil {
				v.JSONURL = u
			} else {
				log.Warn().Err(serr).Str("export_id", e.ID.String()).Msg("reports: sign json link failed")
			}
		}
		out = append(out, v)
	}
	return httpx.OK(c, out)
}

// TriggerExport handles POST /api/v1/reports/exports — runs an on-demand export.
func (h *Handler) TriggerExport(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermReportsExport) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to trigger an export")
	}
	if h.exporter == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "export service unavailable")
	}
	y, w := time.Now().UTC().ISOWeek()
	exp, err := h.exporter.Export(c.UserContext(), "ondemand", fmt.Sprintf("%d-W%02d", y, w))
	if err != nil {
		return err
	}
	return httpx.Created(c, exp)
}
