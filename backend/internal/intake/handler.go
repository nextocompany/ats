// Package intake exposes a generic inbound webhook that lets external sources
// (MS Forms, SEEK, JobsDB — via their own automation/middleware) submit candidate
// applications into the same pipeline as the career portal. Each source posts a
// normalized JSON payload with the resume inline as base64 (no URL fetch, so no
// SSRF surface); the handler maps it onto applications.Service.Intake with the
// source as the source_channel. Authentication is a shared-secret HMAC (see
// routes.go); the endpoint is disabled unless INTAKE_WEBHOOK_SECRET is set.
package intake

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// maxResumeBytes caps a decoded resume. Kept under the app's 12MB body limit with
// headroom for base64's ~33% inflation (8MB → ~10.7MB encoded).
const maxResumeBytes = 8 * 1024 * 1024

// allowedSources are the recognised intake channels; the :source path segment is
// validated against this set and becomes the candidate's source_channel.
var allowedSources = map[string]bool{
	"ms_forms": true,
	"seek":     true,
	"jobsdb":   true,
}

// contentTypeToFileType maps an inbound resume MIME type to the pipeline's file
// type. Mirrors the public apply handler's accepted set.
var contentTypeToFileType = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

var fileTypeDefaultExt = map[string]string{"pdf": ".pdf", "docx": ".docx", "image": ".jpg"}

// intakeService is the subset of applications.Service the handler needs.
type intakeService interface {
	Intake(ctx context.Context, in applications.IntakeInput) (applications.IntakeResult, error)
}

// positionResolver resolves a position by internal id or PeopleSoft code.
type positionResolver interface {
	FindByID(ctx context.Context, id uuid.UUID) (*positions.Position, error)
	FindByPSCode(ctx context.Context, code string) (*positions.Position, error)
}

// Handler serves the intake webhook.
type Handler struct {
	intake intakeService
	pos    positionResolver
}

// NewHandler builds the intake webhook handler.
func NewHandler(intake intakeService, pos positionResolver) *Handler {
	return &Handler{intake: intake, pos: pos}
}

type resumePayload struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Base64      string `json:"base64"`
}

type submitReq struct {
	PositionID  string        `json:"position_id"`  // internal uuid (optional if position_ref set)
	PositionRef string        `json:"position_ref"` // ps_position_code (optional if position_id set)
	FullName    string        `json:"full_name"`
	Phone       string        `json:"phone"`
	Email       string        `json:"email"`
	Province    string        `json:"province"`
	ExternalRef string        `json:"external_ref"` // source's own id (traceability only)
	Resume      resumePayload `json:"resume"`
}

// Submit handles POST /api/v1/intake/:source. Validates the source + payload,
// resolves the position, decodes the resume, and feeds applications.Service.Intake.
func (h *Handler) Submit(c *fiber.Ctx) error {
	source := strings.ToLower(strings.TrimSpace(c.Params("source")))
	if !allowedSources[source] {
		return fiber.NewError(fiber.StatusNotFound, "unknown intake source")
	}

	var req submitReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(req.FullName) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "full_name is required")
	}

	pos, err := h.resolvePosition(c.UserContext(), req)
	if err != nil {
		return err
	}

	fileType, ok := contentTypeToFileType[strings.TrimSpace(req.Resume.ContentType)]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "resume content_type must be pdf, docx, jpeg, or png")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Resume.Base64))
	if err != nil || len(data) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "resume.base64 must be valid, non-empty base64")
	}
	if len(data) > maxResumeBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds the size limit")
	}

	res, err := h.intake.Intake(c.UserContext(), applications.IntakeInput{
		CandidateName: strings.TrimSpace(req.FullName),
		Phone:         strings.TrimSpace(req.Phone),
		Email:         strings.TrimSpace(req.Email),
		Province:      strings.TrimSpace(req.Province),
		SourceChannel: source,
		PositionID:    pos.ID,
		FileName:      safeFilename(req.Resume.Filename, fileType),
		FileType:      fileType,
		ContentType:   strings.TrimSpace(req.Resume.ContentType),
		FileBytes:     data,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, res)
}

// resolvePosition resolves by internal uuid (preferred) or ps_position_code.
func (h *Handler) resolvePosition(ctx context.Context, req submitReq) (*positions.Position, error) {
	switch {
	case strings.TrimSpace(req.PositionID) != "":
		id, err := uuid.Parse(strings.TrimSpace(req.PositionID))
		if err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest, "position_id is not a valid uuid")
		}
		pos, err := h.pos.FindByID(ctx, id)
		return notFoundIfMissing(pos, err, "position not found")
	case strings.TrimSpace(req.PositionRef) != "":
		pos, err := h.pos.FindByPSCode(ctx, strings.TrimSpace(req.PositionRef))
		return notFoundIfMissing(pos, err, "position not found for position_ref")
	default:
		return nil, fiber.NewError(fiber.StatusBadRequest, "position_id or position_ref is required")
	}
}

// notFoundIfMissing maps a repo lookup to a 404 fiber error when the row is absent.
// The positions repo returns a wrapped pgx.ErrNoRows (not a nil pointer) on no-row,
// so both forms are handled.
func notFoundIfMissing(pos *positions.Position, err error, msg string) (*positions.Position, error) {
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && pos == nil) {
		return nil, fiber.NewError(fiber.StatusNotFound, msg)
	}
	if err != nil {
		return nil, err
	}
	return pos, nil
}

// safeFilename strips any path components and unexpected characters from the
// source-provided filename, defaulting to resume<ext> when missing/unsafe.
func safeFilename(name, fileType string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, " ", "_")
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		default:
			return -1
		}
	}, base)
	// Need a real stem before the extension (reject "", ".", ".pdf", "noext").
	if cleaned == "" || strings.HasPrefix(cleaned, ".") || !strings.Contains(cleaned, ".") {
		return "resume" + fileTypeDefaultExt[fileType]
	}
	return cleaned
}
