package applications

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// onboardingSignedTTL is how long an onboarding-document download link stays valid.
const onboardingSignedTTL = 24 * time.Hour

// onboardingBlob is the blob subset the onboarding handlers need (mirrors
// letterBlob).
type onboardingBlob interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// OnboardingHandler is the HR-facing onboarding surface: read the document
// checklist + progress for an application, and approve/reject each document.
type OnboardingHandler struct {
	apps     Repository
	blob     onboardingBlob
	required []string
	notify   statusNotifyDeps
}

// NewOnboardingHandler builds the HR onboarding handler. required is the
// config-driven list of document types a hired candidate must submit.
func NewOnboardingHandler(apps Repository, blob onboardingBlob, required []string) *OnboardingHandler {
	return &OnboardingHandler{apps: apps, blob: blob, required: required}
}

// SetNotifier wires best-effort candidate notification fired when a document is
// reviewed. Unset → no notifications (tests/CI).
func (h *OnboardingHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notify = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}

// RegisterOnboardingRoutes mounts the HR onboarding endpoints.
func RegisterOnboardingRoutes(app *fiber.App, h *OnboardingHandler) {
	app.Get("/api/v1/applications/:id/onboarding", h.Get)
	app.Post("/api/v1/applications/:id/onboarding/documents/:docId/review", h.Review)
}

func (h *OnboardingHandler) scopedAppID(c *fiber.Ctx) (uuid.UUID, error) {
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

// Get returns the application's onboarding checklist + progress.
func (h *OnboardingHandler) Get(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOnboarding(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to view onboarding")
	}
	docs, err := h.apps.ListOnboardingByApplication(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, buildOnboardingStatus(id, h.required, docs, h.sign))
}

// Review records an HR approve/reject for one document and notifies the candidate.
func (h *OnboardingHandler) Review(c *fiber.Ctx) error {
	id, err := h.scopedAppID(c)
	if err != nil {
		return err
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canManageOnboarding(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to review onboarding")
	}
	docID, err := uuid.Parse(c.Params("docId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid document id")
	}
	var in OnboardingReviewInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	decision := strings.TrimSpace(in.Decision)
	if !validOnbDecision(decision) {
		return fiber.NewError(fiber.StatusBadRequest, "decision must be approve or reject")
	}
	reason := strings.TrimSpace(in.Reason)
	if decision == OnbDecisionReject && reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "a reason is required to reject a document")
	}
	reviewerID, err := uuid.Parse(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid actor identity")
	}

	approve := decision == OnbDecisionApprove
	// A reason is only meaningful on rejection; never persist a stray reason an
	// approve request may have carried.
	if approve {
		reason = ""
	}
	doc, err := h.apps.ReviewOnboardingDocument(c.UserContext(), docID, id, reviewerID, approve, reason)
	if errors.Is(err, ErrOnboardingDocNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "onboarding document not found")
	}
	if errors.Is(err, ErrOnboardingDocConflict) {
		return fiber.NewError(fiber.StatusConflict, "the document could not be reviewed (please retry)")
	}
	if err != nil {
		return err
	}

	// Best-effort candidate notification of the review outcome.
	h.notify.notifyDocumentReviewed(c.UserContext(), h.apps, id, doc.DocType, approve, reason)

	url, serr := h.sign(doc.BlobURL)
	if serr != nil {
		log.Warn().Err(serr).Str("onboarding_doc", doc.ID.String()).Msg("onboarding: sign url failed (link omitted)")
	}
	return httpx.OK(c, onboardingDocView(doc, url))
}

// sign returns a freshly-signed download URL for a stored blob.
func (h *OnboardingHandler) sign(storedURL string) (string, error) {
	return h.blob.SignedURLForStored(storedURL, onboardingSignedTTL)
}

// onboardingDocView maps a document to its API projection with a signed URL.
func onboardingDocView(d OnboardingDocument, url string) OnboardingDocView {
	return OnboardingDocView{
		ID: d.ID, DocType: d.DocType, Status: d.Status, FileName: d.FileName,
		FileType: d.FileType, ReviewReason: d.ReviewReason, UploadedAt: d.UploadedAt,
		ReviewedAt: d.ReviewedAt, URL: url,
	}
}

// buildOnboardingStatus assembles the checklist + progress, signing each
// document's blob URL best-effort (a signing failure yields an empty URL — logged
// — rather than failing the whole response). Shared by the HR and candidate lists.
func buildOnboardingStatus(appID uuid.UUID, required []string, docs []OnboardingDocument, sign func(string) (string, error)) OnboardingStatus {
	views := make([]OnboardingDocView, 0, len(docs))
	for _, d := range docs {
		url, err := sign(d.BlobURL)
		if err != nil {
			log.Warn().Err(err).Str("onboarding_doc", d.ID.String()).Msg("onboarding: sign url failed (link omitted)")
		}
		views = append(views, onboardingDocView(d, url))
	}
	approved, complete := computeComplete(required, views)
	return OnboardingStatus{
		ApplicationID: appID,
		Required:      required,
		Documents:     views,
		ApprovedCount: approved,
		RequiredCount: len(required),
		Complete:      complete,
	}
}
