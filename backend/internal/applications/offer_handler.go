package applications

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// OfferHandler is the HR-facing offer management surface: compose, edit (while
// draft), send, and read an application's offer.
type OfferHandler struct {
	lockGuard
	apps   Repository
	notify statusNotifyDeps
}

// NewOfferHandler builds the HR offer handler.
func NewOfferHandler(apps Repository) *OfferHandler {
	return &OfferHandler{apps: apps}
}

// SetNotifier wires best-effort candidate notification fired when an offer is sent.
func (h *OfferHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notify = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}

// RegisterOfferRoutes mounts the HR offer endpoints.
func RegisterOfferRoutes(app *fiber.App, h *OfferHandler) {
	app.Get("/api/v1/applications/:id/offer", h.Get)
	app.Post("/api/v1/applications/:id/offer", h.Create)
	app.Patch("/api/v1/applications/:id/offer", h.Update)
	app.Post("/api/v1/applications/:id/offer/send", h.Send)
	app.Post("/api/v1/applications/:id/offer/reopen", h.Reopen)
	app.Post("/api/v1/applications/:id/offer/withdraw", h.Withdraw)
}

func (h *OfferHandler) scopedAppID(c *fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return uuid.Nil, serr
	} else if !ok {
		return uuid.Nil, fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	return id, nil
}

// guardOfferLock resolves the application's candidate and enforces the processing
// lock — the offer mutations carry only the application id, so the candidate is
// loaded here.
func (h *OfferHandler) guardOfferLock(c *fiber.Ctx, id uuid.UUID) (bool, error) {
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return false, err
	}
	if app == nil {
		return true, nil // unknown application — let the downstream op return 404/conflict
	}
	return h.guardLock(c, app.CandidateID)
}

// Create composes a draft offer for an application in the offer stage.
func (h *OfferHandler) Create(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOffer(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to manage offers")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	if ok, lerr := h.guardLock(c, app.CandidateID); !ok {
		return lerr
	}
	if app.Status != StatusOffer {
		return fiber.NewError(fiber.StatusBadRequest, "an offer can only be created once the candidate reaches the offer stage")
	}
	var in OfferInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	in.Terms = strings.TrimSpace(in.Terms)
	uid, _ := uuid.Parse(u.ID)
	offer, err := h.apps.CreateOffer(c.UserContext(), id, uid, in)
	if errors.Is(err, ErrOfferExists) {
		return fiber.NewError(fiber.StatusConflict, "an offer already exists for this application")
	}
	if err != nil {
		return err
	}
	return httpx.Created(c, offer)
}

// Update edits a still-draft offer.
func (h *OfferHandler) Update(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOffer(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to manage offers")
	}
	if ok, lerr := h.guardOfferLock(c, id); !ok {
		return lerr
	}
	var in OfferInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	in.Terms = strings.TrimSpace(in.Terms)
	offer, err := h.apps.UpdateOffer(c.UserContext(), id, in)
	if errors.Is(err, ErrOfferNotEditable) {
		return fiber.NewError(fiber.StatusConflict, "the offer can no longer be edited (already sent or decided)")
	}
	if err != nil {
		return err
	}
	return httpx.OK(c, offer)
}

// Reopen returns a negotiating offer to draft so HR can revise and re-send it via
// the existing Update + Send path.
func (h *OfferHandler) Reopen(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOffer(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to manage offers")
	}
	if ok, lerr := h.guardOfferLock(c, id); !ok {
		return lerr
	}
	offer, err := h.apps.ReopenOffer(c.UserContext(), id)
	if errors.Is(err, ErrOfferNotEditable) {
		return fiber.NewError(fiber.StatusConflict, "only an offer under negotiation can be reopened")
	}
	if err != nil {
		return err
	}
	return httpx.OK(c, offer)
}

// Withdraw ends a negotiation (or a sent offer) by declining the offer and
// rejecting the application in one transaction.
func (h *OfferHandler) Withdraw(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOffer(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to manage offers")
	}
	if ok, lerr := h.guardOfferLock(c, id); !ok {
		return lerr
	}
	var in OfferResponseInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "a reason is required to end the negotiation")
	}
	offer, err := h.apps.WithdrawOffer(c.UserContext(), id, reason)
	if errors.Is(err, ErrOfferConflict) {
		return fiber.NewError(fiber.StatusConflict, "no active offer to end for this application")
	}
	if err != nil {
		return err
	}
	// Best-effort candidate notification of the rejection.
	h.notify.notifyStatusChange(c.UserContext(), h.apps, id, StatusRejected)
	return httpx.OK(c, offer)
}

// Send transitions a draft offer to sent and notifies the candidate.
func (h *OfferHandler) Send(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOffer(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to manage offers")
	}
	if ok, lerr := h.guardOfferLock(c, id); !ok {
		return lerr
	}
	existing, err := h.apps.GetOfferByApplication(c.UserContext(), id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fiber.NewError(fiber.StatusNotFound, "no offer to send")
	}
	if verr := ValidateOfferForSend(*existing); verr != nil {
		return fiber.NewError(fiber.StatusBadRequest, verr.Error())
	}
	offer, err := h.apps.SendOffer(c.UserContext(), id)
	if errors.Is(err, ErrOfferConflict) {
		return fiber.NewError(fiber.StatusConflict, "the offer has already been sent or decided")
	}
	if err != nil {
		return err
	}
	// Best-effort candidate notification (the application stays at status offer;
	// the offer case was added to notify.statusDoc).
	h.notify.notifyStatusChange(c.UserContext(), h.apps, id, StatusOffer)
	return httpx.OK(c, offer)
}

// Get returns the application's offer, or null when none exists.
func (h *OfferHandler) Get(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	offer, err := h.apps.GetOfferByApplication(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, offer)
}
