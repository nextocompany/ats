package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// atsReportStore is the narrow slice the ATS report handler needs (the concrete
// *Repo satisfies it; a fake backs the handler tests).
type atsReportStore interface {
	ATSReport(ctx context.Context, scope rbac.Scope, f ATSFilter, requiredDocs []string) (ATSReport, error)
}

// ATSReportHandler serves the HR-facing ATS reports (JSON + CSV).
type ATSReportHandler struct {
	repo         atsReportStore
	requiredDocs []string
}

// NewATSReportHandler builds the ATS report handler. requiredDocs is the configured
// onboarding required-document set (drives onboarding completion).
func NewATSReportHandler(repo atsReportStore, requiredDocs []string) *ATSReportHandler {
	return &ATSReportHandler{repo: repo, requiredDocs: requiredDocs}
}

// RegisterATSRoutes mounts the ATS report endpoints (sit behind global HR auth).
func RegisterATSRoutes(app *fiber.App, h *ATSReportHandler) {
	app.Get("/api/v1/reports/ats", h.Get)
	app.Get("/api/v1/reports/ats.csv", h.ExportCSV)
}

// build runs the role gate + date parse + scoped aggregation shared by both
// endpoints.
func (h *ATSReportHandler) build(c *fiber.Ctx) (ATSReport, error) {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermReportsView) {
		return ATSReport{}, fiber.NewError(fiber.StatusForbidden, "insufficient role to view reports")
	}
	f, err := parseRange(c.Query("from"), c.Query("to"))
	if err != nil {
		return ATSReport{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	scope := rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
	rep, err := h.repo.ATSReport(c.UserContext(), scope, f, h.requiredDocs)
	if err != nil {
		return ATSReport{}, err
	}
	rep.Scope = scopeLabel(scope)
	return rep, nil
}

// Get handles GET /api/v1/reports/ats (JSON).
func (h *ATSReportHandler) Get(c *fiber.Ctx) error {
	rep, err := h.build(c)
	if err != nil {
		return err
	}
	return httpx.OK(c, rep)
}

// ExportCSV handles GET /api/v1/reports/ats.csv (CSV attachment).
func (h *ATSReportHandler) ExportCSV(c *fiber.Ctx) error {
	rep, err := h.build(c)
	if err != nil {
		return err
	}
	b, err := EncodeATSCSV(rep)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="ats-report.csv"`)
	return c.Send(b)
}

// scopeLabel renders a human label for the report's scope (used in JSON + CSV).
func scopeLabel(s rbac.Scope) string {
	switch s.Kind() {
	case rbac.KindAll:
		return "Company"
	case rbac.KindSubregion:
		return "Subregion: " + s.Subregion
	default:
		if s.StoreID != nil {
			return fmt.Sprintf("Store %d", *s.StoreID)
		}
		return "Store (unassigned)"
	}
}

// parseTimeArg accepts an RFC3339 timestamp or a YYYY-MM-DD date (UTC midnight).
// The bool reports whether the input was date-only.
func parseTimeArg(s string) (time.Time, bool, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid date %q (use RFC3339 or YYYY-MM-DD)", s)
}

// parseRange resolves the [from, to) window. Defaults: to=now, from=now-90d. A
// date-only `to` is advanced to end-of-day so that day is fully included.
func parseRange(fromQ, toQ string) (ATSFilter, error) {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -90)
	to := now
	if fromQ != "" {
		t, _, err := parseTimeArg(fromQ)
		if err != nil {
			return ATSFilter{}, err
		}
		from = t
	}
	if toQ != "" {
		t, dateOnly, err := parseTimeArg(toQ)
		if err != nil {
			return ATSFilter{}, err
		}
		if dateOnly {
			t = t.AddDate(0, 0, 1)
		}
		to = t
	}
	if !to.After(from) {
		return ATSFilter{}, fmt.Errorf("'to' must be after 'from'")
	}
	return ATSFilter{From: from, To: to}, nil
}
