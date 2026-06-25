package applications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// offerCandidateStore is the narrow repository slice the candidate offer handler
// needs. The concrete pgRepository satisfies it.
type offerCandidateStore interface {
	ListOffersByAccount(ctx context.Context, accountID uuid.UUID) ([]OfferView, error)
	GetOfferByID(ctx context.Context, id uuid.UUID) (*Offer, error)
	RespondOffer(ctx context.Context, offerID, accountID uuid.UUID, accept bool, reason string) (Offer, error)
	NegotiateOffer(ctx context.Context, offerID, accountID uuid.UUID, counter *float64, note string, maxRounds int) (Offer, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
}

// offerNegotiateNotify bundles the optional HR-notification deps fired when a
// candidate counters an offer. All zero → no-op.
type offerNegotiateNotify struct {
	notifier         notify.Notifier
	hr               HRDirectory
	dashboardBaseURL string
	teamsEnabled     bool
}

// OfferCandidateHandler is the membership-authenticated candidate surface: list my
// offers and accept/decline/negotiate. Identity comes from the candidateauth
// session.
type OfferCandidateHandler struct {
	apps      offerCandidateStore
	hired     HiredSyncer // best-effort PeopleSoft push on accept; nil-safe
	maxRounds int         // negotiation cap (config NEGOTIATION_MAX_ROUNDS)
	hrNotify  offerNegotiateNotify
}

// NewOfferCandidateHandler builds the candidate offer handler. maxRounds caps the
// number of candidate counter-offers (config NEGOTIATION_MAX_ROUNDS).
func NewOfferCandidateHandler(apps offerCandidateStore, hired HiredSyncer, maxRounds int) *OfferCandidateHandler {
	return &OfferCandidateHandler{apps: apps, hired: hired, maxRounds: maxRounds}
}

// SetNegotiateNotifier wires best-effort HR notification fired when a candidate
// counters an offer. Unset → no notifications (tests/CI).
func (h *OfferCandidateHandler) SetNegotiateNotifier(n notify.Notifier, hr HRDirectory, dashboardBaseURL string, teamsEnabled bool) {
	h.hrNotify = offerNegotiateNotify{notifier: n, hr: hr, dashboardBaseURL: dashboardBaseURL, teamsEnabled: teamsEnabled}
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
		return fiber.NewError(fiber.StatusBadRequest, "decision must be accept, decline, or negotiate")
	}

	// Negotiate: the candidate counters with a new figure. The offer pauses at
	// 'negotiating' (application stays at 'offer') awaiting an HR revise & re-send.
	if decision == OfferDecisionNegotiate {
		if in.CounterSalary == nil || *in.CounterSalary <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "a positive counter amount is required to negotiate")
		}
		note := strings.TrimSpace(in.Note)
		offer, err := h.apps.NegotiateOffer(c.UserContext(), offerID, acct.ID, in.CounterSalary, note, h.maxRounds)
		if errors.Is(err, ErrOfferNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "offer not found")
		}
		if errors.Is(err, ErrNegotiationClosed) {
			return fiber.NewError(fiber.StatusConflict, "the negotiation limit has been reached, please accept or reject")
		}
		if errors.Is(err, ErrOfferConflict) {
			return fiber.NewError(fiber.StatusConflict, "this offer can no longer be negotiated (expired or already decided)")
		}
		if err != nil {
			return err
		}
		h.notifyNegotiated(c.UserContext(), offer)
		return httpx.OK(c, offer)
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

// notifyNegotiated best-effort pings store HR (email + Teams) that a candidate
// countered their offer. No-op when deps are unset or the application has no
// assigned store. Never blocks the response.
func (h *OfferCandidateHandler) notifyNegotiated(ctx context.Context, offer Offer) {
	d := h.hrNotify
	if d.notifier == nil || d.hr == nil {
		return
	}
	app, err := h.apps.FindByID(ctx, offer.ApplicationID)
	if err != nil || app == nil {
		return
	}
	emails, err := d.hr.EmailsForStore(ctx, app.AssignedStoreID)
	if err != nil {
		return
	}
	if len(emails) == 0 && !d.teamsEnabled {
		return
	}
	counterText := "-"
	if offer.CounterSalary != nil {
		counterText = fmt.Sprintf("%.0f บาท", *offer.CounterSalary)
	}
	dashURL := d.dashboardBaseURL + "/applications/" + app.ID.String()
	msgs := notify.OfferNegotiatedHR(emails, d.teamsEnabled, "", counterText, offer.NegotiationNote, dashURL)
	dispatchHR(ctx, d.notifier, msgs)
}
