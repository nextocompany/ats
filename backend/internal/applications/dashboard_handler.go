package applications

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const resumeURLTTL = 15 * time.Minute
const maxBulkIDs = 100

// ResumeSigner produces a short-lived signed URL for a stored blob URL.
type ResumeSigner interface {
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// ActivityWriter records audit entries, with and without actor attribution.
type ActivityWriter interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
	RecordWith(ctx context.Context, a activity.Actor, action, entityType string, entityID uuid.UUID, newValue any) error
}

// CandidateIndexer re-syncs a candidate's search document after a status change.
// No-op by default; the api injects the real (search-backed) one via SetIndexer.
type CandidateIndexer interface {
	Index(ctx context.Context, candidateID uuid.UUID) error
}

type noopReindexer struct{}

func (noopReindexer) Index(context.Context, uuid.UUID) error { return nil }

// DashboardHandler serves the HR inbox read/bulk/resume endpoints.
type DashboardHandler struct {
	lockGuard
	apps       Repository
	signer     ResumeSigner
	activity   ActivityWriter
	indexer    CandidateIndexer
	notifyDeps statusNotifyDeps
	lmNotify   lmShortlistNotify
}

// lmShortlistNotify bundles the optional Line-Manager shortlist-notify deps. All
// zero → no-op.
type lmShortlistNotify struct {
	notifier         notify.Notifier
	hr               HRDirectory
	dashboardBaseURL string
	teamsEnabled     bool
}

// NewDashboardHandler builds the dashboard handler.
func NewDashboardHandler(apps Repository, signer ResumeSigner, act ActivityWriter) *DashboardHandler {
	return &DashboardHandler{apps: apps, signer: signer, activity: act, indexer: noopReindexer{}}
}

// SetIndexer injects a search indexer (no-op by default) so bulk status changes
// keep the search index fresh. Ignored for nil.
func (h *DashboardHandler) SetIndexer(idx CandidateIndexer) {
	if idx != nil {
		h.indexer = idx
	}
}

// SetNotifier wires best-effort candidate notifications on bulk status changes.
// Unset → no notifications. Mirrors SetIndexer.
func (h *DashboardHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notifyDeps = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}

// SetLineManagerNotifier wires the best-effort email/Teams ping to a store's line
// manager(s) when a candidate is bulk-shortlisted. Unset → no notifications.
func (h *DashboardHandler) SetLineManagerNotifier(n notify.Notifier, hr HRDirectory, dashboardBaseURL string, teamsEnabled bool) {
	h.lmNotify = lmShortlistNotify{notifier: n, hr: hr, dashboardBaseURL: dashboardBaseURL, teamsEnabled: teamsEnabled}
}

// notifyShortlistLM best-effort pings the store line manager(s) that a candidate is
// awaiting their shortlist review. No-op when deps are unset or no store assigned.
func (h *DashboardHandler) notifyShortlistLM(ctx context.Context, app *Application) {
	d := h.lmNotify
	if d.notifier == nil || d.hr == nil {
		return
	}
	emails, err := d.hr.LineManagerEmailsForStore(ctx, app.AssignedStoreID)
	if err != nil {
		return // never block the status write
	}
	if len(emails) == 0 && !d.teamsEnabled {
		return
	}
	msgs := notify.ShortlistReadyLM(emails, d.teamsEnabled, "", "", d.dashboardBaseURL+"/shortlist")
	dispatchHR(ctx, d.notifier, msgs)
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
	return rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
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
	Value  string   `json:"value"`  // target status when action=status (only "shortlisted")
	Reason string   `json:"reason"` // required when action=reject
}

// Bulk handles POST /api/v1/applications/bulk. Bulk only supports the two
// transitions that need no per-candidate payload: shortlist and reject (with a
// shared reason). Interview (needs a schedule) and hire/offer are single-record
// actions. Each id is gated individually by the state machine — ids not in a
// valid source state are counted as failed rather than forced.
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

	var target, reason string
	switch req.Action {
	case "reject":
		target = StatusRejected
		reason = strings.TrimSpace(req.Reason)
		if reason == "" {
			return fiber.NewError(fiber.StatusBadRequest, "a rejection reason is required")
		}
	case "status":
		if req.Value != StatusShortlisted {
			return fiber.NewError(fiber.StatusBadRequest, "bulk status only supports shortlisting")
		}
		target = StatusShortlisted
	default:
		return fiber.NewError(fiber.StatusBadRequest, "unsupported action")
	}

	var updated, failed, skippedLocked int
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			failed++
			continue
		}
		// Per-id state-machine gate: skip ids not in a valid source state.
		app, ferr := h.apps.FindByID(c.UserContext(), id)
		if ferr != nil || !CanTransition(app.Status, target) {
			failed++
			continue
		}
		// Processing lock: skip candidates another operator is actively handling,
		// rather than failing the whole batch over one contended record.
		if _, locked := h.lockedByOther(c, app.CandidateID); locked {
			skippedLocked++
			continue
		}
		if target == StatusRejected {
			// An offer-stage application with an active offer must be rejected via
			// WithdrawOffer so the offers row is terminalized in the same tx — a
			// plain SetRejection would orphan a live offer the candidate keeps
			// seeing. ErrOfferConflict (no active offer) falls back to SetRejection.
			rejected := false
			if app.Status == StatusOffer {
				if _, werr := h.apps.WithdrawOffer(c.UserContext(), id, reason); werr == nil {
					rejected = true
				} else if !errors.Is(werr, ErrOfferConflict) {
					failed++
					continue
				}
			}
			if !rejected {
				if err := h.apps.SetRejection(c.UserContext(), id, reason); err != nil {
					failed++
					continue
				}
			}
		} else if err := h.apps.SetStatus(c.UserContext(), id, target); err != nil {
			failed++
			continue
		}
		buid, bip, bua := middleware.AuditActor(c)
		_ = h.activity.RecordWith(c.UserContext(), activity.Actor{UserID: buid, IP: bip, UserAgent: bua}, activity.ActionBulkAction, "application", id, fiber.Map{"status": target})
		// Keep the search index fresh — best-effort, never fails the bulk action.
		_ = h.indexer.Index(c.UserContext(), app.CandidateID)
		// Notify the candidate of the outcome — best-effort (statusDoc has copy for
		// both shortlist progress and rejection).
		h.notifyDeps.notifyStatusChange(c.UserContext(), h.apps, id, target)
		// Notify the store line manager that a candidate awaits shortlist review.
		if target == StatusShortlisted {
			h.notifyShortlistLM(c.UserContext(), app)
		}
		updated++
	}
	return httpx.OK(c, fiber.Map{"updated": updated, "failed": failed, "skipped_locked": skippedLocked, "status": target})
}

// Resume handles GET /api/v1/applications/:id/resume → short-lived signed URL.
func (h *DashboardHandler) Resume(c *fiber.Ctx) error {
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
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	if app.RawFileBlobURL == "" {
		return fiber.NewError(fiber.StatusNotFound, "no resume on file")
	}
	url, err := h.signer.SignedURLForStored(app.RawFileBlobURL, resumeURLTTL)
	if err != nil {
		return err
	}
	// PDPA: record who accessed this candidate's resume (PII), from where.
	uid, ip, ua := middleware.AuditActor(c)
	_ = h.activity.RecordWith(c.UserContext(), activity.Actor{UserID: uid, IP: ip, UserAgent: ua}, activity.ActionViewResume, "application", id, nil)
	return httpx.OK(c, fiber.Map{"url": url, "expires_in_seconds": int(resumeURLTTL.Seconds())})
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
