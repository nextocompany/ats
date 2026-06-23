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
