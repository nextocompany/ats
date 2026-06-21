package dsar

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// actionDSARExport is the audit action recorded for a subject access export.
const actionDSARExport = "dsar_export"

// auditWriter records the access event (satisfied by *activity.Log). Optional.
type auditWriter interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
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

// recordAccess audits the export against the subject's own account, capturing the
// request IP (best-effort: a failure must not block the subject's access right).
func (h *Handler) recordAccess(c *fiber.Ctx, accountID uuid.UUID) {
	if h.audit == nil {
		return
	}
	val := fiber.Map{"by": "self", "ip": c.IP()}
	if err := h.audit.Record(c.UserContext(), actionDSARExport, "candidate_account", accountID, val); err != nil {
		log.Warn().Err(err).Str("account_id", accountID.String()).Msg("dsar: export audit record failed")
	}
}
