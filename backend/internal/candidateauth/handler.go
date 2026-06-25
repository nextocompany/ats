package candidateauth

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// auditWriter records consent events with actor attribution. Optional; satisfied
// by *activity.Log.
type auditWriter interface {
	RecordWith(ctx context.Context, a activity.Actor, action, entityType string, entityID uuid.UUID, newValue any) error
}

const maxResumeBytes = 10 * 1024 * 1024

var resumeContentTypes = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

// Handler serves the candidate membership API under /api/v1/public/auth.
type Handler struct {
	svc        *Service
	cookieName string
	secure     bool // Secure + SameSite=None in prod (cross-site portal↔api)
	audit      auditWriter
}

// NewHandler builds the candidateauth handler. secure should be true outside
// development so the session cookie is sent on cross-site portal requests.
func NewHandler(svc *Service, cookieName string, secure bool) *Handler {
	return &Handler{svc: svc, cookieName: cookieName, secure: secure}
}

// SetAudit wires an optional audit writer used to log consent withdrawals.
func (h *Handler) SetAudit(w auditWriter) { h.audit = w }

// meView is the client-safe account projection (no raw subs / blob keys).
type meView struct {
	ID             string `json:"id"`
	FullName       string `json:"full_name"`
	DisplayName    string `json:"display_name"`
	NameTH         string `json:"name_th"`
	NameEN         string `json:"name_en"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Province       string `json:"province"`
	LineDisplayID  string `json:"line_display_id"`
	LineLinked     bool   `json:"line_linked"`
	GoogleLinked   bool   `json:"google_linked"`
	HasResume      bool   `json:"has_resume"`
	ResumeFileType string `json:"resume_file_type"`
	PDPAConsent    bool   `json:"pdpa_consent"`
}

func toMeView(a *Account) meView {
	return meView{
		ID: a.ID.String(), FullName: a.FullName, DisplayName: a.DisplayName,
		NameTH: a.NameTH, NameEN: a.NameEN, Email: a.Email, Phone: a.Phone,
		Province: a.Province, LineDisplayID: a.LineDisplayID,
		LineLinked: a.LineLinked(), GoogleLinked: a.GoogleLinked(),
		HasResume: a.HasResume(), ResumeFileType: a.ResumeFileType, PDPAConsent: a.PDPAConsent,
	}
}

// StartEmail handles POST /email/start. Always 200 (enumeration-safe): a failure
// to validate/send is logged but not revealed to the caller.
func (h *Handler) StartEmail(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.StartEmailOTP(c.UserContext(), body.Email); err != nil {
		log.Warn().Err(err).Msg("candidateauth: start email otp")
	}
	return httpx.OK(c, fiber.Map{"sent": true})
}

// VerifyEmail handles POST /email/verify. On success sets the session cookie.
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	acct, sess, err := h.svc.VerifyEmailOTP(c.UserContext(), body.Email, body.Code)
	if errors.Is(err, ErrOTPInvalid) {
		return fiber.NewError(fiber.StatusUnauthorized, "รหัสไม่ถูกต้องหรือหมดอายุ")
	}
	if errors.Is(err, ErrAccountSuspended) {
		return fiber.NewError(fiber.StatusForbidden, "บัญชีนี้ถูกระงับการใช้งาน กรุณาติดต่อเจ้าหน้าที่")
	}
	if err != nil {
		return err
	}
	h.setSessionCookie(c, sess)
	return httpx.OK(c, toMeView(acct))
}

// meWithConsent enriches the base view with the consent-version state the portal
// needs to decide whether to show a re-consent prompt (PDPA s.19).
type meWithConsent struct {
	meView
	PDPAVersion        string `json:"pdpa_version"`
	PDPACurrentVersion string `json:"pdpa_current_version"`
	PDPANeedsReconsent bool   `json:"pdpa_needs_reconsent"`
}

// Me handles GET /me (RequireCandidate).
func (h *Handler) Me(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	current := h.svc.CurrentConsentVersion(c.UserContext())
	// A consenting member is prompted to re-consent when a newer notice version is
	// current than the one they accepted. Withdrawn (not-consented) members are not
	// nagged here; they re-consent through the normal consent step on next apply.
	needs := acct.PDPAConsent && acct.PDPAVersion != "" && acct.PDPAVersion != current
	return httpx.OK(c, meWithConsent{
		meView:             toMeView(acct),
		PDPAVersion:        acct.PDPAVersion,
		PDPACurrentVersion: current,
		PDPANeedsReconsent: needs,
	})
}

// WithdrawConsent handles POST /consent/withdraw (RequireCandidate): the member
// revokes PDPA consent. The withdrawal is recorded in the consent ledger + the
// account snapshot. Erasure of data with no other lawful basis is a separate
// self-service action (Phase 3 DSAR); this endpoint only revokes consent.
func (h *Handler) WithdrawConsent(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	version := acct.PDPAVersion
	if version == "" {
		version = h.svc.CurrentConsentVersion(c.UserContext())
	}
	if err := h.svc.WithdrawConsent(c.UserContext(), acct.ID, version); err != nil {
		return err
	}
	// PDPA audit: the subject withdrew consent (actor = self, spoof-resistant IP).
	if h.audit != nil {
		acctID := acct.ID
		a := activity.Actor{UserID: &acctID, IP: middleware.ClientIP(c), UserAgent: c.Get(fiber.HeaderUserAgent)}
		if err := h.audit.RecordWith(c.UserContext(), a, activity.ActionConsentWithdraw, "candidate_account", acct.ID, fiber.Map{"version": version}); err != nil {
			log.Warn().Err(err).Str("account_id", acct.ID.String()).Msg("candidateauth: consent-withdraw audit failed")
		}
	}
	return httpx.OK(c, fiber.Map{"pdpa_consent": false, "pdpa_version": version})
}

// AcceptConsent handles POST /consent/accept (RequireCandidate): the member
// re-consents to the CURRENT notice version (Phase 2 reconsent). Each call records
// a fresh consent event (a new ledger row) + updates the account snapshot to the
// current version, so /me stops signalling pdpa_needs_reconsent. Re-posting writes
// another ledger row (every accept is a real, audited event), which is the
// intended audit-trail behaviour, not a no-op.
func (h *Handler) AcceptConsent(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	version := h.svc.CurrentConsentVersion(c.UserContext())
	if err := h.svc.SaveConsent(c.UserContext(), acct.ID, version); err != nil {
		return err
	}
	// PDPA audit: the subject re-consented (actor = self, spoof-resistant IP).
	if h.audit != nil {
		acctID := acct.ID
		a := activity.Actor{UserID: &acctID, IP: middleware.ClientIP(c), UserAgent: c.Get(fiber.HeaderUserAgent)}
		if err := h.audit.RecordWith(c.UserContext(), a, activity.ActionConsentReaccept, "candidate_account", acct.ID, fiber.Map{"version": version}); err != nil {
			log.Warn().Err(err).Str("account_id", acct.ID.String()).Msg("candidateauth: consent-reaccept audit failed")
		}
	}
	return httpx.OK(c, fiber.Map{"pdpa_consent": true, "pdpa_version": version})
}

// Logout handles POST /logout: revoke the session + clear the cookie.
func (h *Handler) Logout(c *fiber.Ctx) error {
	if tok := c.Cookies(h.cookieName); tok != "" {
		if err := h.svc.Logout(c.UserContext(), tok); err != nil {
			log.Warn().Err(err).Msg("candidateauth: logout")
		}
	}
	h.clearSessionCookie(c)
	return httpx.OK(c, fiber.Map{"ok": true})
}

// UpdateProfile handles PATCH /profile (RequireCandidate). Also records PDPA
// consent when consent_given=true.
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	var body struct {
		FullName       string `json:"full_name"`
		NameTH         string `json:"name_th"`
		NameEN         string `json:"name_en"`
		DisplayName    string `json:"display_name"`
		Phone          string `json:"phone"`
		Email          string `json:"email"`
		LineDisplayID  string `json:"line_display_id"`
		Province       string `json:"province"`
		ConsentGiven   bool   `json:"consent_given"`
		ConsentVersion string `json:"consent_version"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.UpdateProfile(c.UserContext(), acct.ID, ProfileUpdate{
		FullName: body.FullName, NameTH: body.NameTH, NameEN: body.NameEN, DisplayName: body.DisplayName,
		Phone: body.Phone, Email: body.Email, LineDisplayID: body.LineDisplayID, Province: body.Province,
	}); err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			return fiber.NewError(fiber.StatusConflict, "อีเมลนี้ถูกใช้กับบัญชีอื่นแล้ว")
		case errors.Is(err, ErrInvalidEmail):
			return fiber.NewError(fiber.StatusBadRequest, "อีเมลไม่ถูกต้อง")
		default:
			return err
		}
	}
	if body.ConsentGiven {
		version := body.ConsentVersion
		if version == "" {
			version = h.svc.CurrentConsentVersion(c.UserContext())
		}
		if err := h.svc.SaveConsent(c.UserContext(), acct.ID, version); err != nil {
			return err
		}
	}
	updated, err := h.svc.repo.GetByID(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, toMeView(updated))
}

// UploadResume handles POST /resume (RequireCandidate, multipart "resume").
func (h *Handler) UploadResume(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	fileHeader, err := c.FormFile("resume")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "resume file is required")
	}
	if fileHeader.Size > maxResumeBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds 10MB limit")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	fileType, ok := resumeContentTypes[contentType]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported file type")
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
	if err := h.svc.AddResume(c.UserContext(), acct.ID, fileHeader.Filename, fileType, contentType, data); err != nil {
		if errors.Is(err, ErrResumeLimit) {
			return fiber.NewError(fiber.StatusConflict, "resume limit reached (max 5) - delete one first")
		}
		return err
	}
	return h.respondResumes(c, acct.ID)
}

// respondResumes returns the account's current CV history (newest first). Shared
// by upload / set-default / delete so the client always gets the fresh list.
func (h *Handler) respondResumes(c *fiber.Ctx, accountID uuid.UUID) error {
	list, err := h.svc.ListResumes(c.UserContext(), accountID)
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"resumes": list})
}

// ListResumes handles GET /resumes (RequireCandidate).
func (h *Handler) ListResumes(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	return h.respondResumes(c, acct.ID)
}

// ViewResume handles GET /resumes/:id/file (RequireCandidate): a short-lived
// signed URL for the candidate's own CV. Account-scoped lookup → no IDOR.
func (h *Handler) ViewResume(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid resume id")
	}
	url, err := h.svc.ResumeViewURL(c.UserContext(), acct.ID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "resume not found")
		}
		return err
	}
	return httpx.OK(c, fiber.Map{"url": url, "expires_in_seconds": int(resumeViewTTL.Seconds())})
}

// SetDefaultResume handles POST /resumes/:id/default (RequireCandidate).
func (h *Handler) SetDefaultResume(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid resume id")
	}
	if err := h.svc.SetDefaultResume(c.UserContext(), acct.ID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "resume not found")
		}
		return err
	}
	return h.respondResumes(c, acct.ID)
}

// DeleteResume handles DELETE /resumes/:id (RequireCandidate).
func (h *Handler) DeleteResume(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid resume id")
	}
	if err := h.svc.DeleteResume(c.UserContext(), acct.ID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "resume not found")
		}
		return err
	}
	return h.respondResumes(c, acct.ID)
}

func (h *Handler) setSessionCookie(c *fiber.Ctx, sess Session) {
	writeSessionCookie(c, h.cookieName, h.secure, sess)
}

func (h *Handler) clearSessionCookie(c *fiber.Ctx) {
	clearSessionCookie(c, h.cookieName, h.secure)
}

// cookieSameSite returns the SameSite policy for the session cookie. In prod
// (secure) it must be "None" so the cookie is sent on cross-site portal→api
// fetches (azurecontainerapps.io is a public suffix → portal and api are cross-site).
func cookieSameSite(secure bool) string {
	if secure {
		return "None"
	}
	return "Lax"
}

// writeSessionCookie sets the httpOnly candidate session cookie (package-level so
// the OAuth handlers can reuse it).
func writeSessionCookie(c *fiber.Ctx, name string, secure bool, sess Session) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    sess.Token,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: cookieSameSite(secure),
		Expires:  sess.Expires,
		Path:     "/",
	})
}

func clearSessionCookie(c *fiber.Ctx, name string, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    "",
		HTTPOnly: true,
		Secure:   secure,
		SameSite: cookieSameSite(secure),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Path:     "/",
	})
}
