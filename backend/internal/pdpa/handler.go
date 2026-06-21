package pdpa

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the PDPA endpoints.
type Handler struct{ repo *Repo }

// NewHandler builds the PDPA handler.
func NewHandler(repo *Repo) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the PDPA endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1/pdpa")
	v1.Post("/consent", h.RecordConsent)
	v1.Get("/consent/:candidate_id", h.GetConsent)
	// Public: the current privacy/consent notice (the apps stamp this version and
	// render the body on the consent step / privacy page).
	v1.Get("/policy/current", h.CurrentPolicy)
}

// CurrentPolicy handles GET /api/v1/pdpa/policy/current?locale=th|en — the current
// consent document for the requested locale (defaults to th).
func (h *Handler) CurrentPolicy(c *fiber.Ctx) error {
	locale := c.Query("locale", "th")
	if locale != "th" && locale != "en" {
		locale = "th"
	}
	doc, err := h.repo.CurrentDocuments(c.UserContext(), locale)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no current consent document")
	}
	return httpx.OK(c, doc)
}

type consentReq struct {
	CandidateID   string `json:"candidate_id"`
	ConsentGiven  bool   `json:"consent_given"`
	Version       string `json:"consent_version"`
	SourceChannel string `json:"source_channel"`
}

// RecordConsent handles POST /api/v1/pdpa/consent.
func (h *Handler) RecordConsent(c *fiber.Ctx) error {
	var req consentReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	id, err := uuid.Parse(req.CandidateID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid candidate_id is required")
	}
	if err := h.repo.Record(c.UserContext(), Consent{
		CandidateID:   id,
		ConsentGiven:  req.ConsentGiven,
		Version:       req.Version,
		SourceChannel: req.SourceChannel,
	}, c.IP()); err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"candidate_id": id, "consent_given": req.ConsentGiven})
}

// GetConsent handles GET /api/v1/pdpa/consent/:candidate_id.
func (h *Handler) GetConsent(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("candidate_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid candidate_id")
	}
	consent, err := h.repo.Latest(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no consent on record")
	}
	return httpx.OK(c, consent)
}
