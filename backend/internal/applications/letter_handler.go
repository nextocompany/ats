package applications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/letters"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// letterSignedTTL is how long a letter download link stays valid.
const letterSignedTTL = 24 * time.Hour

// letterStore is the narrow repository slice the HR letter handler needs.
type letterStore interface {
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
	GatherLetterData(ctx context.Context, applicationID uuid.UUID, letterType string) (letters.LetterData, error)
	UpsertLetter(ctx context.Context, applicationID, createdBy uuid.UUID, letterType, blobURL string) (Letter, error)
	GetLettersByApplication(ctx context.Context, applicationID uuid.UUID) ([]Letter, error)
	GetLetterByID(ctx context.Context, id uuid.UUID) (*Letter, error)
}

// letterRenderer renders a letter to PDF bytes.
type letterRenderer interface {
	Render(d letters.LetterData) ([]byte, error)
}

// letterBlob is the blob subset the handler needs (mirrors reports.BlobStore).
type letterBlob interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// LetterHandler generates and serves PDF letters for HR.
type LetterHandler struct {
	apps     letterStore
	renderer letterRenderer
	blob     letterBlob
}

// NewLetterHandler builds the HR letter handler.
func NewLetterHandler(apps letterStore, renderer letterRenderer, blob letterBlob) *LetterHandler {
	return &LetterHandler{apps: apps, renderer: renderer, blob: blob}
}

// RegisterLetterRoutes mounts the HR letter endpoints.
func RegisterLetterRoutes(app *fiber.App, h *LetterHandler) {
	app.Get("/api/v1/applications/:id/letters", h.List)
	app.Post("/api/v1/applications/:id/letters", h.Generate)
	app.Get("/api/v1/applications/:id/letters/:letterID", h.Download)
}

type letterReq struct {
	Type string `json:"type"`
}

// Generate renders a letter to PDF, stores it in blob, and upserts the record.
func (h *LetterHandler) Generate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageLetter(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to generate letters")
	}
	var req letterReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if !validLetterType(req.Type) {
		return fiber.NewError(fiber.StatusBadRequest, "type must be interview or offer")
	}

	data, err := h.apps.GatherLetterData(c.UserContext(), id, req.Type)
	if errors.Is(err, ErrLetterPreconditions) {
		if req.Type == LetterInterview {
			return fiber.NewError(fiber.StatusBadRequest, "schedule an interview before generating an interview letter")
		}
		return fiber.NewError(fiber.StatusBadRequest, "create and send an offer before generating an offer letter")
	}
	if err != nil {
		return err
	}

	pdf, err := h.renderer.Render(data)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("letters/%s-%s.pdf", id, req.Type)
	storedURL, err := h.blob.Upload(c.UserContext(), name, pdf, "application/pdf")
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid actor identity")
	}
	letter, err := h.apps.UpsertLetter(c.UserContext(), id, uid, req.Type, storedURL)
	if err != nil {
		return err
	}
	return httpx.Created(c, h.view(letter))
}

// List returns the application's letters with freshly-signed download URLs.
func (h *LetterHandler) List(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	list, err := h.apps.GetLettersByApplication(c.UserContext(), id)
	if err != nil {
		return err
	}
	out := make([]LetterView, 0, len(list))
	for _, l := range list {
		out = append(out, h.view(l))
	}
	return httpx.OK(c, out)
}

// Download returns a signed URL for a single letter.
func (h *LetterHandler) Download(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	letterID, err := uuid.Parse(c.Params("letterID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid letter id")
	}
	letter, err := h.apps.GetLetterByID(c.UserContext(), letterID)
	if err != nil {
		return err
	}
	if letter == nil || letter.ApplicationID != id {
		return fiber.NewError(fiber.StatusNotFound, "letter not found")
	}
	url, err := h.blob.SignedURLForStored(letter.BlobURL, letterSignedTTL)
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"url": url, "expires_in_seconds": int(letterSignedTTL.Seconds())})
}

// view signs a letter's blob URL into a LetterView (best-effort: a signing failure
// yields an empty URL — logged — rather than failing the whole list).
func (h *LetterHandler) view(l Letter) LetterView {
	url, err := h.blob.SignedURLForStored(l.BlobURL, letterSignedTTL)
	if err != nil {
		log.Warn().Err(err).Str("letter", l.ID.String()).Msg("letter: sign url failed (link omitted)")
	}
	return LetterView{ID: l.ID, Type: l.Type, CreatedAt: l.CreatedAt, URL: url}
}
