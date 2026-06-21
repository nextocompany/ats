package dsar

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Audit action names for subject-initiated DSAR events.
const (
	actionDSARExport    = activity.ActionDSARExport
	actionDSARErase     = activity.ActionDSARErase     // erased immediately
	actionDSAREraseHeld = activity.ActionDSAREraseHeld // queued for HR (legal hold)
)

// auditWriter records the access event with actor attribution (satisfied by
// *activity.Log). Optional.
type auditWriter interface {
	RecordWith(ctx context.Context, a activity.Actor, action, entityType string, entityID uuid.UUID, newValue any) error
}

// Handler serves the portal DSAR endpoints (RequireCandidate-gated).
type Handler struct {
	svc   *Service
	audit auditWriter
}

// NewHandler builds the DSAR handler. audit may be nil (the access is then not
// logged, but the export still works).
func NewHandler(svc *Service, audit auditWriter) *Handler {
	return &Handler{svc: svc, audit: audit}
}

// RegisterRoutes mounts the DSAR endpoints under the portal auth group. gate is
// candidateauth.RequireCandidate so every route is scoped to the caller's session.
func RegisterRoutes(app *fiber.App, h *Handler, gate fiber.Handler) {
	g := app.Group("/api/v1/public/auth/me")
	g.Get("/export", gate, h.Export)
	g.Post("/erase", gate, h.RequestErasure)
}

// RequestErasure handles POST /api/v1/public/auth/me/erase - the subject erases
// their own data (PDPA s.33). Erased immediately unless a legal hold (hired)
// applies, in which case the request is queued for HR/DPO. Strictly scoped to the
// caller's own account; the action is audited.
func (h *Handler) RequestErasure(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	result, err := h.svc.RequestErasure(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	if result == ErasureHeld {
		h.recordErase(c, acct.ID, actionDSAREraseHeld)
		// One message covers both the legal-hold case (hired) and a queued partial
		// erasure: the subject's request is received and handled by staff.
		return httpx.OK(c, fiber.Map{
			"status":  string(result),
			"message": "คำขอลบข้อมูลของคุณถูกบันทึกและจะดำเนินการโดยเจ้าหน้าที่ บางข้อมูลอาจต้องเก็บไว้ตามกฎหมาย",
		})
	}
	h.recordErase(c, acct.ID, actionDSARErase)
	return httpx.OK(c, fiber.Map{"status": string(result)})
}

// recordErase audits a self-service erasure outcome against the subject's account.
func (h *Handler) recordErase(c *fiber.Ctx, accountID uuid.UUID, action string) {
	h.recordEvent(c, action, accountID)
}

// recordEvent records a subject-initiated DSAR event. The subject is both actor
// and entity (they act on their own account); the IP is spoof-resistant
// (middleware.ClientIP). Best-effort: a failure must not block the subject's
// right, so it is only logged.
func (h *Handler) recordEvent(c *fiber.Ctx, action string, accountID uuid.UUID) {
	if h.audit == nil {
		return
	}
	acct := accountID
	a := activity.Actor{UserID: &acct, IP: middleware.ClientIP(c), UserAgent: c.Get(fiber.HeaderUserAgent)}
	if err := h.audit.RecordWith(c.UserContext(), a, action, "candidate_account", accountID, fiber.Map{"by": "self"}); err != nil {
		log.Warn().Err(err).Str("account_id", accountID.String()).Str("action", action).Msg("dsar: audit record failed")
	}
}

// Export handles GET /api/v1/public/auth/me/export - a JSON download of the
// authenticated subject's complete personal data (PDPA s.30 + s.31). Strictly
// scoped to the caller's own account; the access is audited.
func (h *Handler) Export(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	data, err := h.svc.Export(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	h.recordAccess(c, acct.ID)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="my-data.json"`)
	return httpx.OK(c, data)
}

// recordAccess audits the export against the subject's own account.
func (h *Handler) recordAccess(c *fiber.Ctx, accountID uuid.UUID) {
	h.recordEvent(c, actionDSARExport, accountID)
}
