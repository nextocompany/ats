// Package public serves the unauthenticated / LINE-authenticated Career Portal
// API (F14 backend). The Next.js UI (Sprint 4) consumes these endpoints.
package public

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ConsentRecorder records a candidate's PDPA consent. pdpa.Repo satisfies it.
type ConsentRecorder interface {
	Record(ctx context.Context, c pdpa.Consent, ip string) error
}

// AccountResolver resolves a member account from a session token, reads its saved
// resume, and persists PDPA consent — satisfied by candidateauth.Service. Optional
// (nil ⇒ legacy LINE-only apply).
type AccountResolver interface {
	AccountFromSession(ctx context.Context, token string) (*candidateauth.Account, error)
	SavedResumeBytes(ctx context.Context, acct *candidateauth.Account) ([]byte, error)
	SaveConsent(ctx context.Context, accountID uuid.UUID, version string) error
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
	return httpx.OK(c, fiber.Map{
		"id": p.ID, "title_th": p.TitleTH, "title_en": p.TitleEN, "level": p.Level,
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
	name := c.FormValue("full_name")
	if name == "" && acct != nil {
		name = acct.FullName
	}
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "full_name is required")
	}
	// PDPA consent is mandatory and must be REAL — never fabricated. A guest ticks
	// the box; a member must have actually consented at signup (don't assume).
	consentVersion := c.FormValue("consent_version")
	if consentVersion == "" {
		consentVersion = "1.0"
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
			if err := h.accounts.SaveConsent(c.UserContext(), acct.ID, consentVersion); err != nil {
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
		Phone:         formOr(c, "phone", acctPhone(acct)),
		Email:         formOr(c, "email", acctEmail(acct)),
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
	return h.finalizeApplication(c, result, consentVersion)
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
		consentVersion = "1.0"
	}
	if !acct.PDPAConsent {
		// Not consented at signup — require an explicit consent on this apply, then
		// persist it (so the member isn't re-prompted next time).
		if !body.ConsentGiven {
			return fiber.NewError(fiber.StatusBadRequest, "PDPA consent is required")
		}
		if err := h.accounts.SaveConsent(c.UserContext(), acct.ID, consentVersion); err != nil {
			log.Warn().Err(err).Str("account_id", acct.ID.String()).Msg("failed to persist member PDPA consent")
		}
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
	return h.finalizeApplication(c, result, consentVersion)
}

// finalizeApplication issues the opaque status token and records PDPA consent.
func (h *Handler) finalizeApplication(c *fiber.Ctx, result applications.IntakeResult, consentVersion string) error {
	token, err := newPublicToken()
	if err != nil {
		return err
	}
	if err := h.apps.SetPublicToken(c.UserContext(), result.ApplicationID, token); err != nil {
		return err
	}
	// Record PDPA consent against the candidate. A failure here must not lose the
	// application — log and continue (consent is retryable).
	if err := h.consent.Record(c.UserContext(), pdpa.Consent{
		CandidateID:   result.CandidateID,
		ConsentGiven:  true,
		Version:       consentVersion,
		SourceChannel: "career_portal",
	}, c.IP()); err != nil {
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
