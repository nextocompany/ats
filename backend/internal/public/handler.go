// Package public serves the unauthenticated / LINE-authenticated Career Portal
// API (F14 backend). The Next.js UI (Sprint 4) consumes these endpoints.
package public

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ConsentRecorder records a candidate's PDPA consent. pdpa.Repo satisfies it.
type ConsentRecorder interface {
	Record(ctx context.Context, c pdpa.Consent, ip string) error
}

const maxResumeBytes = 10 * 1024 * 1024

var contentTypeToFileType = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

// Handler serves the public Career API.
type Handler struct {
	intake   *applications.Service
	apps     applications.Repository
	pos      positions.Repository
	verifier auth.Verifier
	consent  ConsentRecorder
}

// NewHandler builds the public handler.
func NewHandler(intake *applications.Service, apps applications.Repository, pos positions.Repository, v auth.Verifier, consent ConsentRecorder) *Handler {
	return &Handler{intake: intake, apps: apps, pos: pos, verifier: v, consent: consent}
}

// ListPositions handles GET /api/v1/public/positions (only positions with open vacancies).
func (h *Handler) ListPositions(c *fiber.Ctx) error {
	list, err := h.pos.ListPublic(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, list)
}

// GetPosition handles GET /api/v1/public/positions/:id (public projection).
func (h *Handler) GetPosition(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid position id")
	}
	p, err := h.pos.FindByID(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "position not found")
	}
	return httpx.OK(c, fiber.Map{
		"id": p.ID, "title_th": p.TitleTH, "title_en": p.TitleEN, "level": p.Level,
	})
}

// Apply handles POST /api/v1/public/apply (LINE-authenticated multipart).
func (h *Handler) Apply(c *fiber.Ctx) error {
	idToken := c.Get("X-LINE-IdToken")
	if idToken == "" {
		idToken = c.FormValue("line_id_token")
	}
	if _, err := h.verifier.Verify(c.UserContext(), idToken); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "LINE authentication required")
	}

	fileHeader, err := c.FormFile("resume")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "resume file is required")
	}
	if fileHeader.Size > maxResumeBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds 10MB limit")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	fileType, ok := contentTypeToFileType[contentType]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported file type")
	}
	positionID, err := uuid.Parse(c.FormValue("position_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}
	name := c.FormValue("full_name")
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "full_name is required")
	}
	// PDPA consent is mandatory before any candidate data is stored (F13).
	if c.FormValue("consent_given") != "true" {
		return fiber.NewError(fiber.StatusBadRequest, "PDPA consent is required")
	}
	consentVersion := c.FormValue("consent_version")
	if consentVersion == "" {
		consentVersion = "1.0"
	}

	f, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}

	result, err := h.intake.Intake(c.UserContext(), applications.IntakeInput{
		CandidateName: name,
		Phone:         c.FormValue("phone"),
		Email:         c.FormValue("email"),
		IDCard:        c.FormValue("id_card"),
		Province:      c.FormValue("province"),
		SourceChannel: "career_portal",
		PositionID:    positionID,
		FileName:      fileHeader.Filename,
		FileType:      fileType,
		ContentType:   contentType,
		FileBytes:     data,
	})
	if err != nil {
		return err
	}

	token, err := newPublicToken()
	if err != nil {
		return err
	}
	if err := h.apps.SetPublicToken(c.UserContext(), result.ApplicationID, token); err != nil {
		return err
	}

	// Record PDPA consent against the (canonical) candidate. A failure here must
	// not lose the application — log and continue (consent is retryable).
	if err := h.consent.Record(c.UserContext(), pdpa.Consent{
		CandidateID:   result.CandidateID,
		ConsentGiven:  true,
		Version:       consentVersion,
		SourceChannel: "career_portal",
	}, c.IP()); err != nil {
		log.Warn().Err(err).Str("candidate_id", result.CandidateID.String()).Msg("failed to record PDPA consent")
	}

	return httpx.Created(c, fiber.Map{"status_token": token})
}

// Status handles GET /api/v1/public/status/:token — minimal projection only.
func (h *Handler) Status(c *fiber.Ctx) error {
	app, err := h.apps.FindByPublicToken(c.UserContext(), c.Params("token"))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	out := fiber.Map{"status": app.Status, "applied_at": app.CreatedAt}
	if p, err := h.pos.FindByID(c.UserContext(), app.PositionID); err == nil {
		out["position"] = p.TitleTH
	}
	return httpx.OK(c, out)
}

// newPublicToken returns a URL-safe opaque token (never the application UUID).
func newPublicToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, "token generation failed")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
