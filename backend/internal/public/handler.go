// Package public serves the unauthenticated / LINE-authenticated Career Portal
// API (F14 backend). The Next.js UI (Sprint 4) consumes these endpoints.
package public

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/mail"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/apptimeline"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ConsentRecorder records a candidate's PDPA consent and reports the current
// notice version. pdpa.Repo satisfies it.
type ConsentRecorder interface {
	Record(ctx context.Context, c pdpa.Consent, ip string) error
	CurrentVersion(ctx context.Context) (string, error)
}

// currentConsentVersion resolves the registry's current notice version, falling
// back to "1.0" so an apply never fails on a registry read error.
func (h *Handler) currentConsentVersion(c *fiber.Ctx) string {
	v, err := h.consent.CurrentVersion(c.UserContext())
	if err != nil || v == "" {
		return "1.0"
	}
	return v
}

// AccountResolver resolves a member account from a session token, reads its saved
// resume, and persists PDPA consent — satisfied by candidateauth.Service. Optional
// (nil ⇒ legacy LINE-only apply).
type AccountResolver interface {
	AccountFromSession(ctx context.Context, token string) (*candidateauth.Account, error)
	SavedResumeBytes(ctx context.Context, acct *candidateauth.Account) ([]byte, error)
	// MarkConsented flips the account consent snapshot (no ledger row); the
	// apply's candidate-keyed consent row is the authoritative ledger entry.
	MarkConsented(ctx context.Context, accountID uuid.UUID, version string) error
	// BackfillContact fills the applying member's account phone/email from the
	// apply form when the account lacks them (set-once, collision-safe). LINE
	// accounts in particular start with no email; this is how they acquire one.
	BackfillContact(ctx context.Context, accountID uuid.UUID, phone, email string) error
	// BackfillNames fills the account's Thai/English match names from the apply
	// form when the account lacks them (fill-once), so the worker name-mismatch
	// gate has names to compare the parsed resume against.
	BackfillNames(ctx context.Context, accountID uuid.UUID, nameTH, nameEN string) error
}

const maxResumeBytes = 10 * 1024 * 1024

var contentTypeToFileType = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

// fileTypeToContent maps a stored resume file type back to a representative
// content type + filename extension for quick-apply re-upload.
var fileTypeToContent = map[string][2]string{
	"pdf":   {"application/pdf", "pdf"},
	"docx":  {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx"},
	"image": {"image/jpeg", "jpg"},
}

// Handler serves the public Career API.
type Handler struct {
	intake            *applications.Service
	apps              applications.Repository
	pos               positions.Repository
	verifier          auth.Verifier
	consent           ConsentRecorder
	accounts          AccountResolver
	sessionCookieName string
	notifier          notify.Notifier // optional: apply-received candidate notify
	portalBaseURL     string
}

// SetNotifier wires the candidate "application received" notification (best-effort
// LINE + email on a successful apply). Optional — nil leaves apply silent.
func (h *Handler) SetNotifier(n notify.Notifier, portalBaseURL string) *Handler {
	h.notifier = n
	h.portalBaseURL = portalBaseURL
	return h
}

// notifyApplicationReceived tells the candidate (LINE + email, best-effort) that
// their application for a position was received. No-op when the notifier is unset.
func (h *Handler) notifyApplicationReceived(ctx context.Context, lineUserID, emailAddr, fullName string, positionID uuid.UUID, token string) {
	if h.notifier == nil {
		return
	}
	title := ""
	if p, err := h.pos.FindByID(ctx, positionID); err == nil {
		title = p.TitleTH
	}
	if msg := notify.ApplicationReceivedMessage(lineUserID, fullName, title, h.portalBaseURL, token); msg.Recipient != "" {
		if err := h.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Msg("apply-received notify: line send failed (non-fatal)")
		}
	}
	if em := notify.ApplicationReceivedEmailMessage(emailAddr, fullName, title, h.portalBaseURL, token); em.Recipient != "" {
		if err := h.notifier.Send(ctx, em); err != nil {
			log.Warn().Err(err).Msg("apply-received notify: email send failed (non-fatal)")
		}
	}
}

// NewHandler builds the public handler. accounts/sessionCookieName enable account-
// first apply + quick-apply; pass nil/"" to keep the legacy LINE-only behaviour.
func NewHandler(intake *applications.Service, apps applications.Repository, pos positions.Repository, v auth.Verifier, consent ConsentRecorder, accounts AccountResolver, sessionCookieName string) *Handler {
	return &Handler{intake: intake, apps: apps, pos: pos, verifier: v, consent: consent, accounts: accounts, sessionCookieName: sessionCookieName}
}

// ListPositions handles GET /api/v1/public/positions (only positions with open vacancies).
func (h *Handler) ListPositions(c *fiber.Ctx) error {
	list, err := h.pos.ListPublic(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, list)
}

// GetPosition handles GET /api/v1/public/positions/:id (public projection).
func (h *Handler) GetPosition(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid position id")
	}
	p, err := h.pos.FindByID(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "position not found")
	}
	// Role-generic Master JD (responsibilities/qualifications/benefits) is safe to
	// expose on the public posting; opening-specific/internal fields never are.
	return httpx.OK(c, fiber.Map{
		"id": p.ID, "title_th": p.TitleTH, "title_en": p.TitleEN, "level": p.Level,
		"responsibilities": p.Responsibilities,
		"qualifications":   p.Qualifications,
		"benefits":         p.Benefits,
	})
}

// sessionAccount resolves the logged-in member from the session cookie, or nil.
func (h *Handler) sessionAccount(c *fiber.Ctx) *candidateauth.Account {
	if h.accounts == nil || h.sessionCookieName == "" {
		return nil
	}
	tok := c.Cookies(h.sessionCookieName)
	if tok == "" {
		return nil
	}
	acct, err := h.accounts.AccountFromSession(c.UserContext(), tok)
	if err != nil {
		return nil
	}
	return acct
}

// Apply handles POST /api/v1/public/apply (multipart). Account-first: a logged-in
// member is identified by the session cookie (form fields prefill/override the
// saved profile). Falls back to LINE id-token identity for the legacy/guest path.
func (h *Handler) Apply(c *fiber.Ctx) error {
	acct := h.sessionAccount(c)

	var lineUserID string
	var accountID *uuid.UUID
	if acct != nil {
		lineUserID = acct.LineUserID
		id := acct.ID
		accountID = &id
	} else {
		idToken := c.Get("X-LINE-IdToken")
		if idToken == "" {
			idToken = c.FormValue("line_id_token")
		}
		lineUser, err := h.verifier.Verify(c.UserContext(), idToken)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
		}
		lineUserID = lineUser.Subject
	}

	fileHeader, err := c.FormFile("resume")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "resume file is required")
	}
	if fileHeader.Size > maxResumeBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds 10MB limit")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	fileType, ok := contentTypeToFileType[contentType]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported file type")
	}
	positionID, err := uuid.Parse(c.FormValue("position_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}
	// Names: the Thai name is canonical (becomes CandidateName + account full_name);
	// both Thai and English are matched against the parsed resume by the worker.
	// Prefer the form values, fall back to the account (account-first), then to a
	// legacy single full_name field for older clients.
	nameTH := strings.TrimSpace(formOr(c, "name_th", acctNameTH(acct)))
	nameEN := strings.TrimSpace(formOr(c, "name_en", acctNameEN(acct)))
	if nameTH == "" {
		nameTH = strings.TrimSpace(c.FormValue("full_name"))
	}
	if nameTH == "" && acct != nil {
		nameTH = acct.FullName
	}
	if nameTH == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	name := nameTH
	// Phone and email are both required at apply: an email-OTP account already
	// carries an email so only phone is new, but a LINE-only account often has
	// neither, so the form must supply them (prefilled from LINE when available).
	phone := strings.TrimSpace(formOr(c, "phone", acctPhone(acct)))
	if phone == "" {
		return fiber.NewError(fiber.StatusBadRequest, "phone is required")
	}
	email := strings.TrimSpace(formOr(c, "email", acctEmail(acct)))
	if !validEmail(email) {
		return fiber.NewError(fiber.StatusBadRequest, "a valid email is required")
	}
	// PDPA consent is mandatory and must be REAL — never fabricated. A guest ticks
	// the box; a member must have actually consented at signup (don't assume).
	consentVersion := c.FormValue("consent_version")
	if consentVersion == "" {
		consentVersion = h.currentConsentVersion(c)
	}
	if acct != nil {
		switch {
		case acct.PDPAConsent:
			// Consented at signup — reuse the recorded version.
			if acct.PDPAVersion != "" {
				consentVersion = acct.PDPAVersion
			}
		case c.FormValue("consent_given") == "true":
			// Member consenting now (e.g. signed up via OAuth, which skips the signup
			// consent step). Persist it so later applies don't re-prompt. A failure
			// to persist must not lose the application — the legal record is still
			// written by finalizeApplication below.
			if err := h.accounts.MarkConsented(c.UserContext(), acct.ID, consentVersion); err != nil {
				log.Warn().Err(err).Str("account_id", acct.ID.String()).Msg("failed to persist member PDPA consent")
			}
		default:
			return fiber.NewError(fiber.StatusBadRequest, "PDPA consent is required")
		}
	} else if c.FormValue("consent_given") != "true" {
		return fiber.NewError(fiber.StatusBadRequest, "PDPA consent is required")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}

	result, err := h.intake.Intake(c.UserContext(), applications.IntakeInput{
		CandidateName: name,
		Phone:         phone,
		Email:         email,
		IDCard:        c.FormValue("id_card"),
		Province:      formOr(c, "province", acctProvince(acct)),
		SourceChannel: "career_portal",
		LineUserID:    lineUserID,
		AccountID:     accountID,
		PositionID:    positionID,
		FileName:      fileHeader.Filename,
		FileType:      fileType,
		ContentType:   contentType,
		FileBytes:     data,
	})
	if err != nil {
		return err
	}
	// Persist the typed phone/email back onto the applying member's account when it
	// lacks them (LINE accounts start with no email). Best-effort: a failure must
	// not lose the application — the candidate row already carries the contact.
	if accountID != nil {
		if err := h.accounts.BackfillContact(c.UserContext(), *accountID, phone, email); err != nil {
			log.Warn().Err(err).Str("account_id", accountID.String()).Msg("failed to backfill account contact")
		}
		// Persist the match names too, so the worker name-mismatch gate has the
		// Thai/English names to compare the parsed resume against (fill-once).
		if err := h.accounts.BackfillNames(c.UserContext(), *accountID, nameTH, nameEN); err != nil {
			log.Warn().Err(err).Str("account_id", accountID.String()).Msg("failed to backfill account names")
		}
	}
	// Mint the status token BEFORE notifying so the apply notification carries the
	// /status?token=… deep link (notify previously ran before the token existed).
	token, err := h.issuePublicToken(c.UserContext(), result.ApplicationID)
	if err != nil {
		return err
	}
	h.notifyApplicationReceived(c.UserContext(), lineUserID, email, name, positionID, token)
	return h.finalizeApplication(c, result, token, consentVersion, accountID)
}

// QuickApply handles POST /api/v1/public/apply/quick (RequireCandidate). A member
// applies to a position using their saved profile + saved resume in one step.
func (h *Handler) QuickApply(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	if acct.FullName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "complete your profile before applying")
	}
	if !acct.HasResume() {
		return fiber.NewError(fiber.StatusBadRequest, "no saved resume — upload one first")
	}
	var body struct {
		PositionID   string `json:"position_id"`
		ConsentGiven bool   `json:"consent_given"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	positionID, err := uuid.Parse(body.PositionID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}
	consentVersion := acct.PDPAVersion
	if consentVersion == "" {
		consentVersion = h.currentConsentVersion(c)
	}
	if !acct.PDPAConsent {
		// Not consented at signup — require an explicit consent on this apply, then
		// persist it (so the member isn't re-prompted next time).
		if !body.ConsentGiven {
			return fiber.NewError(fiber.StatusBadRequest, "PDPA consent is required")
		}
		if err := h.accounts.MarkConsented(c.UserContext(), acct.ID, consentVersion); err != nil {
			log.Warn().Err(err).Str("account_id", acct.ID.String()).Msg("failed to persist member PDPA consent")
		}
	}
	// Quick-apply reuses the saved profile, so it must already hold phone + email
	// (the frontend hides this path until the profile is complete; enforce here too).
	if strings.TrimSpace(acct.Phone) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "phone is required — please complete your profile")
	}
	if !validEmail(acct.Email) {
		return fiber.NewError(fiber.StatusBadRequest, "a valid email is required — please complete your profile")
	}
	meta, ok := fileTypeToContent[acct.ResumeFileType]
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "saved resume type is unsupported")
	}
	data, err := h.accounts.SavedResumeBytes(c.UserContext(), acct)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read saved resume")
	}
	id := acct.ID
	result, err := h.intake.Intake(c.UserContext(), applications.IntakeInput{
		CandidateName: acct.FullName,
		Phone:         acct.Phone,
		Email:         acct.Email,
		Province:      acct.Province,
		SourceChannel: "career_portal",
		LineUserID:    acct.LineUserID,
		AccountID:     &id,
		PositionID:    positionID,
		FileName:      "resume." + meta[1],
		FileType:      acct.ResumeFileType,
		ContentType:   meta[0],
		FileBytes:     data,
	})
	if err != nil {
		return err
	}
	token, err := h.issuePublicToken(c.UserContext(), result.ApplicationID)
	if err != nil {
		return err
	}
	h.notifyApplicationReceived(c.UserContext(), acct.LineUserID, acct.Email, acct.FullName, positionID, token)
	return h.finalizeApplication(c, result, token, consentVersion, &id)
}

// finalizeApplication issues the opaque status token and records PDPA consent.
// accountID is the applying member's portal account (nil for guest/LINE-only
// applies); it is stamped on the ledger row so the apply consent correlates to
// the account without a join through candidates.
// issuePublicToken mints the opaque status-page token and stores it on the
// application. Called by Apply/QuickApply BEFORE the apply notification so the
// notification's CTA can carry the /status?token=… deep link.
func (h *Handler) issuePublicToken(ctx context.Context, appID uuid.UUID) (string, error) {
	token, err := newPublicToken()
	if err != nil {
		return "", err
	}
	if err := h.apps.SetPublicToken(ctx, appID, token); err != nil {
		return "", err
	}
	return token, nil
}

func (h *Handler) finalizeApplication(c *fiber.Ctx, result applications.IntakeResult, token, consentVersion string, accountID *uuid.UUID) error {
	// Record PDPA consent against the candidate (+ account when known). A failure
	// here must not lose the application — log and continue (consent is retryable).
	if err := h.consent.Record(c.UserContext(), pdpa.Consent{
		CandidateID:   result.CandidateID,
		AccountID:     accountID,
		ConsentGiven:  true,
		Version:       consentVersion,
		SourceChannel: "career_portal",
	}, middleware.ClientIP(c)); err != nil {
		log.Warn().Err(err).Str("candidate_id", result.CandidateID.String()).Msg("failed to record PDPA consent")
	}
	return httpx.Created(c, fiber.Map{"status_token": token})
}

// formOr returns the form value when present, else the fallback (member profile).
func formOr(c *fiber.Ctx, key, fallback string) string {
	if v := c.FormValue(key); v != "" {
		return v
	}
	return fallback
}

// validEmail reports whether s is a non-empty, syntactically valid email address.
func validEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

func acctPhone(a *candidateauth.Account) string {
	if a != nil {
		return a.Phone
	}
	return ""
}
func acctEmail(a *candidateauth.Account) string {
	if a != nil {
		return a.Email
	}
	return ""
}
func acctProvince(a *candidateauth.Account) string {
	if a != nil {
		return a.Province
	}
	return ""
}

func acctNameTH(a *candidateauth.Account) string {
	if a != nil {
		return a.NameTH
	}
	return ""
}

func acctNameEN(a *candidateauth.Account) string {
	if a != nil {
		return a.NameEN
	}
	return ""
}

// MyApplications handles GET /api/v1/public/me/applications (RequireCandidate):
// the logged-in member's own application history across every linked candidate
// row, newest first. Candidate-facing projection only (no AI score / internals).
func (h *Handler) MyApplications(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	items, err := h.apps.ListByAccountForPortal(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	if items == nil {
		items = []applications.PortalApplication{}
	}
	return httpx.OK(c, items)
}

// ApplicationTimelineResponse is the curated candidate-facing status timeline.
type ApplicationTimelineResponse struct {
	Position   string                  `json:"position"`
	Milestones []apptimeline.Milestone `json:"milestones"`
}

// MyApplicationTimeline handles
// GET /api/v1/public/me/applications/:token/timeline (RequireCandidate): the
// curated milestone timeline for one of the logged-in member's applications,
// keyed by its public token. Account-scoped — an unknown OR unowned token both
// return 404 (no IDOR oracle). This is the richer, login-gated counterpart of
// the public /status/:token current-status view.
func (h *Handler) MyApplicationTimeline(c *fiber.Ctx) error {
	acct := candidateauth.CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	tl, err := h.apps.PortalTimelineByToken(c.UserContext(), c.Params("token"), acct.ID)
	if errors.Is(err, applications.ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	if err != nil {
		return err
	}
	events := make([]apptimeline.Event, len(tl.Events))
	for i, e := range tl.Events {
		events[i] = apptimeline.Event{To: e.To, At: e.At}
	}
	return httpx.OK(c, ApplicationTimelineResponse{
		Position:   tl.Position,
		Milestones: apptimeline.Build(events, tl.CreatedAt, tl.Status),
	})
}

// Status handles GET /api/v1/public/status/:token — minimal projection only.
func (h *Handler) Status(c *fiber.Ctx) error {
	app, err := h.apps.FindByPublicToken(c.UserContext(), c.Params("token"))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	out := fiber.Map{"status": app.Status, "applied_at": app.CreatedAt}
	if p, err := h.pos.FindByID(c.UserContext(), app.PositionID); err == nil {
		out["position"] = p.TitleTH
	}
	return httpx.OK(c, out)
}

// newPublicToken returns a URL-safe opaque token (never the application UUID).
func newPublicToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, "token generation failed")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
