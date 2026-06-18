package applications

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// offerCandidateStore is the narrow repository slice the candidate offer handler
// needs. The concrete pgRepository satisfies it.
type offerCandidateStore interface {
	ListOffersByAccount(ctx context.Context, accountID uuid.UUID) ([]OfferView, error)
	GetOfferByID(ctx context.Context, id uuid.UUID) (*Offer, error)
	RespondOffer(ctx context.Context, offerID, accountID uuid.UUID, accept bool, reason string) (Offer, error)
}

// OfferCandidateHandler is the membership-authenticated candidate surface: list my
// offers and accept/decline. Identity comes from the candidateauth session.
type OfferCandidateHandler struct {
	apps  offerCandidateStore
	hired HiredSyncer // best-effort PeopleSoft push on accept; nil-safe
}

// NewOfferCandidateHandler builds the candidate offer handler.
func NewOfferCandidateHandler(apps offerCandidateStore, hired HiredSyncer) *OfferCandidateHandler {
	return &OfferCandidateHandler{apps: apps, hired: hired}
}

// RegisterCandidateOfferRoutes mounts the candidate endpoints behind the supplied
// candidate-auth gate (RequireCandidate).
func RegisterCandidateOfferRoutes(app *fiber.App, h *OfferCandidateHandler, gate fiber.Handler) {
	app.Get("/api/v1/public/auth/offers", gate, h.ListMine)
	app.Post("/api/v1/public/auth/offers/:id/respond", gate, h.Respond)
}

// ListMine returns the logged-in member's offers.
func (h *OfferCandidateHandler) ListMine(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	offers, err := h.apps.ListOffersByAccount(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, offers)
}

// Respond records the member's accept/decline. Accept advances the application to
// hired and best-effort pushes to PeopleSoft; decline rejects it with the reason.
func (h *OfferCandidateHandler) Respond(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	offerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid offer id")
	}
	var in OfferResponseInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	decision := strings.TrimSpace(in.Decision)
	if !validOfferDecision(decision) {
		return fiber.NewError(fiber.StatusBadRequest, "decision must be accept or decline")
	}
	reason := strings.TrimSpace(in.Reason)
	if decision == OfferDecisionDecline && reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "a reason is required to decline")
	}

	accept := decision == OfferDecisionAccept
	offer, err := h.apps.RespondOffer(c.UserContext(), offerID, acct.ID, accept, reason)
	if errors.Is(err, ErrOfferNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "offer not found")
	}
	if errors.Is(err, ErrOfferConflict) {
		return fiber.NewError(fiber.StatusConflict, "this offer can no longer be responded to (expired or already decided)")
	}
	if err != nil {
		return err
	}

	if accept && h.hired != nil {
		// Best-effort: an accepted hire is committed regardless of PeopleSoft.
		if serr := h.hired.SyncHired(c.UserContext(), offer.ApplicationID); serr != nil {
			log.Warn().Err(serr).Str("application", offer.ApplicationID.String()).Msg("offer accept: peoplesoft sync failed (non-fatal)")
		}
	}
	return httpx.OK(c, offer)
}
