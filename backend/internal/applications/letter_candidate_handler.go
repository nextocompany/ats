package applications

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// letterCandidateStore is the narrow repository slice the candidate letter handler
// needs.
type letterCandidateStore interface {
	ListLettersByAccount(ctx context.Context, accountID uuid.UUID) ([]Letter, error)
}

// LetterCandidateHandler lets a logged-in member list and download their letters.
type LetterCandidateHandler struct {
	apps letterCandidateStore
	blob letterBlob
}

// NewLetterCandidateHandler builds the candidate letter handler.
func NewLetterCandidateHandler(apps letterCandidateStore, blob letterBlob) *LetterCandidateHandler {
	return &LetterCandidateHandler{apps: apps, blob: blob}
}

// RegisterCandidateLetterRoutes mounts the candidate letter endpoint behind the
// supplied candidate-auth gate.
func RegisterCandidateLetterRoutes(app *fiber.App, h *LetterCandidateHandler, gate fiber.Handler) {
	app.Get("/api/v1/public/auth/letters", gate, h.ListMine)
}

// ListMine returns the member's letters with freshly-signed download URLs.
func (h *LetterCandidateHandler) ListMine(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	list, err := h.apps.ListLettersByAccount(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	out := make([]LetterView, 0, len(list))
	for _, l := range list {
		url, err := h.blob.SignedURLForStored(l.BlobURL, letterSignedTTL)
		if err != nil {
			log.Warn().Err(err).Str("letter", l.ID.String()).Msg("letter: sign url failed (link omitted)")
		}
		out = append(out, LetterView{ID: l.ID, Type: l.Type, CreatedAt: l.CreatedAt, URL: url})
	}
	return httpx.OK(c, out)
}
