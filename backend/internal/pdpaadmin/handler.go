package pdpaadmin

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// RetentionInfo is the retention-policy summary surfaced on the overview (read
// from config, not the DB).
type RetentionInfo struct {
	Days    int
	Enabled bool
}

// auditWriter records console actions with actor attribution (satisfied by
// *activity.Log). Optional.
type auditWriter interface {
	RecordWith(ctx context.Context, a activity.Actor, action, entityType string, entityID uuid.UUID, newValue any) error
}

// dpoLister resolves the published DPO directory dynamically (officers flagged
// is_dpo). Satisfied by *pdpa.Repo.
type dpoLister interface {
	ListDPOOfficers(ctx context.Context) ([]pdpa.DPOOfficer, error)
}

// Handler serves /api/v1/pdpa/admin, gated entirely to pdpa.admin. The console is
// company-wide (no RBAC data scope). The DPO directory is resolved dynamically;
// retention + company come from config.
type Handler struct {
	repo      Repository
	dpo       dpoLister
	company   string
	retention RetentionInfo
	audit     auditWriter
}

// NewHandler builds the PDPA-console handler. audit may be nil.
func NewHandler(repo Repository, dpo dpoLister, company string, retention RetentionInfo, audit auditWriter) *Handler {
	return &Handler{repo: repo, dpo: dpo, company: company, retention: retention, audit: audit}
}

// RegisterRoutes mounts the console endpoints. Static segments precede the
// parameterised ones so Fiber does not capture them as :id.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/pdpa/admin")
	g.Get("/overview", h.Overview)
	g.Get("/dsar-requests", h.ListDSAR)
	g.Post("/dsar-requests/:id/complete", h.CompleteDSAR)
	g.Post("/dsar-requests/:id/reject", h.RejectDSAR)
	g.Get("/consents", h.LookupConsents)
}

func authz(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" || !rbac.Can(u.Role, rbac.PermPDPAAdmin) {
		return middleware.DevUser{}, false
	}
	return u, true
}

func forbidden(c *fiber.Ctx) error {
	return httpx.Fail(c, fiber.StatusForbidden, "insufficient role for the PDPA console")
}

func (h *Handler) Overview(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	pending, open, overdue, version, err := h.repo.Counts(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("pdpaadmin: overview counts failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not load overview")
	}
	officers, err := h.dpo.ListDPOOfficers(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("pdpaadmin: dpo officers failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not load overview")
	}
	return httpx.OK(c, Overview{
		DSARPending:     pending,
		BreachesOpen:    open,
		BreachesOverdue: overdue,
		ConsentVersion:  version,
		RetentionDays:   h.retention.Days,
		RetentionOn:     h.retention.Enabled,
		DPO:             pdpa.DPODirectory{Company: h.company, Officers: officers},
	})
}

func (h *Handler) ListDSAR(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	f := DSARListFilter{Status: c.Query("status")}
	if f.Status != "" && f.Status != DSARStatusPending && f.Status != DSARStatusCompleted && f.Status != DSARStatusRejected {
		return httpx.Fail(c, fiber.StatusBadRequest, "status must be one of pending|completed|rejected")
	}
	if v := c.QueryInt("page"); v > 0 {
		f.Page = v
	}
	if v := c.QueryInt("limit"); v > 0 {
		f.Limit = v
	}
	items, total, err := h.repo.ListDSAR(c.UserContext(), f)
	if err != nil {
		log.Error().Err(err).Msg("pdpaadmin: dsar list failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list DSAR requests")
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]DSARRequest]{
		Success: true,
		Data:    items,
		Meta:    &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

func (h *Handler) CompleteDSAR(c *fiber.Ctx) error {
	return h.resolveDSAR(c, DSARStatusCompleted, activity.ActionDSARComplete, "")
}

type rejectReq struct {
	Reason string `json:"reason"`
}

func (h *Handler) RejectDSAR(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	var req rejectReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return httpx.Fail(c, fiber.StatusBadRequest, "a rejection reason is required")
	}
	if len(reason) > maxReason {
		reason = reason[:maxReason]
	}
	return h.resolveDSAR(c, DSARStatusRejected, activity.ActionDSARReject, reason)
}

// resolveDSAR is the shared complete/reject path: authorize, move the pending
// request to the target status (stamping the resolver), and audit.
func (h *Handler) resolveDSAR(c *fiber.Ctx, status, action, reason string) error {
	u, ok := authz(c)
	if !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request id")
	}
	var actor *uuid.UUID
	if aid, perr := uuid.Parse(u.ID); perr == nil {
		actor = &aid
	}
	d, err := h.repo.ResolveDSAR(c.UserContext(), id, status, reason, actor)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return httpx.Fail(c, fiber.StatusNotFound, "DSAR request not found")
		case errors.Is(err, ErrBadState):
			return httpx.Fail(c, fiber.StatusConflict, "DSAR request is already resolved")
		default:
			log.Error().Err(err).Msg("pdpaadmin: dsar resolve failed")
			return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
		}
	}
	h.record(c, action, d.AccountID, fiber.Map{"request_id": d.ID.String(), "reason": reason})
	return httpx.OK(c, d)
}

func (h *Handler) LookupConsents(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	var accountID, candidateID *uuid.UUID
	if v := c.Query("account_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "account_id must be a valid uuid")
		}
		accountID = &id
	}
	if v := c.Query("candidate_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "candidate_id must be a valid uuid")
		}
		candidateID = &id
	}
	if accountID == nil && candidateID == nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "provide account_id or candidate_id")
	}
	items, err := h.repo.LookupConsents(c.UserContext(), accountID, candidateID)
	if err != nil {
		log.Error().Err(err).Msg("pdpaadmin: consent lookup failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not look up consents")
	}
	return httpx.OK(c, items)
}

// record audits a console mutation, attributed to the DPO caller. Best-effort.
func (h *Handler) record(c *fiber.Ctx, action string, entityID uuid.UUID, detail any) {
	if h.audit == nil {
		return
	}
	uid, ip, ua := middleware.AuditActor(c)
	if err := h.audit.RecordWith(c.UserContext(), activity.Actor{UserID: uid, IP: ip, UserAgent: ua}, action, "dsar_request", entityID, detail); err != nil {
		log.Warn().Err(err).Str("action", action).Msg("pdpaadmin: audit record failed")
	}
}
