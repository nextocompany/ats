package applications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// onboardingCandidateStore is the narrow repository slice the candidate onboarding
// handler needs. The concrete pgRepository satisfies it.
type onboardingCandidateStore interface {
	FindHiredApplicationByAccount(ctx context.Context, accountID uuid.UUID) (uuid.UUID, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	ListOnboardingByApplication(ctx context.Context, applicationID uuid.UUID) ([]OnboardingDocument, error)
	UpsertOnboardingDocument(ctx context.Context, applicationID uuid.UUID, docType, blobURL, fileName, fileType string, uploadedBy uuid.UUID) (OnboardingDocument, error)
}

// onboardingHRNotify bundles the optional HR-notification deps fired when a
// candidate uploads a document. All zero → no-op.
type onboardingHRNotify struct {
	notifier         notify.Notifier
	hr               HRDirectory
	dashboardBaseURL string
	teamsEnabled     bool
}

// OnboardingCandidateHandler is the membership-authenticated candidate surface:
// list my onboarding checklist and upload/replace a required document. Identity
// comes from the candidateauth session; the hired application is resolved
// server-side (the client never passes an application id).
type OnboardingCandidateHandler struct {
	apps     onboardingCandidateStore
	blob     onboardingBlob
	required []string
	hrNotify onboardingHRNotify
}

// NewOnboardingCandidateHandler builds the candidate onboarding handler.
func NewOnboardingCandidateHandler(apps onboardingCandidateStore, blob onboardingBlob, required []string) *OnboardingCandidateHandler {
	return &OnboardingCandidateHandler{apps: apps, blob: blob, required: required}
}

// SetNotifier wires best-effort HR notification fired when a document is uploaded.
// Unset → no notifications (tests/CI).
func (h *OnboardingCandidateHandler) SetNotifier(n notify.Notifier, hr HRDirectory, dashboardBaseURL string, teamsEnabled bool) {
	h.hrNotify = onboardingHRNotify{notifier: n, hr: hr, dashboardBaseURL: dashboardBaseURL, teamsEnabled: teamsEnabled}
}

// RegisterCandidateOnboardingRoutes mounts the candidate endpoints behind the
// supplied candidate-auth gate (RequireCandidate).
func RegisterCandidateOnboardingRoutes(app *fiber.App, h *OnboardingCandidateHandler, gate fiber.Handler) {
	app.Get("/api/v1/public/auth/onboarding", gate, h.ListMine)
	app.Post("/api/v1/public/auth/onboarding/documents", gate, h.Upload)
}

// ListMine returns the logged-in member's onboarding checklist + progress.
func (h *OnboardingCandidateHandler) ListMine(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	appID, err := h.apps.FindHiredApplicationByAccount(c.UserContext(), acct.ID)
	if errors.Is(err, ErrOnboardingNoHiredApp) {
		return fiber.NewError(fiber.StatusNotFound, "no onboarding in progress")
	}
	if err != nil {
		return err
	}
	docs, err := h.apps.ListOnboardingByApplication(c.UserContext(), appID)
	if err != nil {
		return err
	}
	return httpx.OK(c, buildOnboardingStatus(appID, h.required, docs, h.sign))
}

// Upload stores (or replaces) a required document for the member's hired
// application and returns the refreshed checklist.
func (h *OnboardingCandidateHandler) Upload(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	appID, err := h.apps.FindHiredApplicationByAccount(c.UserContext(), acct.ID)
	if errors.Is(err, ErrOnboardingNoHiredApp) {
		return fiber.NewError(fiber.StatusNotFound, "no onboarding in progress")
	}
	if err != nil {
		return err
	}
	docType := strings.TrimSpace(c.FormValue("doc_type"))
	if !validDocType(docType) {
		return fiber.NewError(fiber.StatusBadRequest, "unknown document type")
	}
	fileHeader, err := c.FormFile("document")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "a document file is required")
	}
	if fileHeader.Size > maxOnboardingBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "document exceeds 10MB limit")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	fileType, ok := onboardingContentTypes[contentType]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported file type")
	}
	data, err := readMultipartFile(fileHeader)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}

	// Stable per-(application, type) key so a re-upload replaces the prior blob;
	// the original filename is kept only as display metadata on the row.
	name := fmt.Sprintf("onboarding/%s/%s%s", appID, docType, extForContentType(contentType))
	storedURL, err := h.blob.Upload(c.UserContext(), name, data, contentType)
	if err != nil {
		return err
	}
	if _, err := h.apps.UpsertOnboardingDocument(c.UserContext(), appID, docType, storedURL, fileHeader.Filename, fileType, acct.ID); err != nil {
		return err
	}

	// Best-effort HR notification of the upload.
	h.notifyDocUploaded(c.UserContext(), appID, docType)

	docs, err := h.apps.ListOnboardingByApplication(c.UserContext(), appID)
	if err != nil {
		return err
	}
	return httpx.OK(c, buildOnboardingStatus(appID, h.required, docs, h.sign))
}

// sign returns a freshly-signed download URL for a stored blob.
func (h *OnboardingCandidateHandler) sign(storedURL string) (string, error) {
	return h.blob.SignedURLForStored(storedURL, onboardingSignedTTL)
}

// notifyDocUploaded best-effort pings store HR (email + Teams) that a document was
// uploaded. No-op when deps are unset or the application has no assigned store.
func (h *OnboardingCandidateHandler) notifyDocUploaded(ctx context.Context, appID uuid.UUID, docType string) {
	d := h.hrNotify
	if d.notifier == nil || d.hr == nil {
		return
	}
	app, err := h.apps.FindByID(ctx, appID)
	if err != nil {
		return // never block the upload on a notify lookup
	}
	emails, err := d.hr.EmailsForStore(ctx, app.AssignedStoreID)
	if err != nil {
		return
	}
	if len(emails) == 0 && !d.teamsEnabled {
		return
	}
	dashURL := d.dashboardBaseURL + "/applications/" + app.ID.String()
	msgs := notify.OnboardingDocUploadedHR(emails, d.teamsEnabled, docType, dashURL)
	dispatchHR(ctx, d.notifier, msgs)
}
