package candidateauth

import (
	"errors"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/httpx"
)

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
}

// NewHandler builds the candidateauth handler. secure should be true outside
// development so the session cookie is sent on cross-site portal requests.
func NewHandler(svc *Service, cookieName string, secure bool) *Handler {
	return &Handler{svc: svc, cookieName: cookieName, secure: secure}
}

// meView is the client-safe account projection (no raw subs / blob keys).
type meView struct {
	ID             string `json:"id"`
	FullName       string `json:"full_name"`
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
		ID: a.ID.String(), FullName: a.FullName, Email: a.Email, Phone: a.Phone,
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
	if err != nil {
		return err
	}
	h.setSessionCookie(c, sess)
	return httpx.OK(c, toMeView(acct))
}

// Me handles GET /me (RequireCandidate).
func (h *Handler) Me(c *fiber.Ctx) error {
	acct := CandidateFromCtx(c)
	if acct == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "login required")
	}
	return httpx.OK(c, toMeView(acct))
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
		Phone          string `json:"phone"`
		LineDisplayID  string `json:"line_display_id"`
		Province       string `json:"province"`
		ConsentGiven   bool   `json:"consent_given"`
		ConsentVersion string `json:"consent_version"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.UpdateProfile(c.UserContext(), acct.ID, ProfileUpdate{
		FullName: body.FullName, Phone: body.Phone, LineDisplayID: body.LineDisplayID, Province: body.Province,
	}); err != nil {
		return err
	}
	if body.ConsentGiven {
		version := body.ConsentVersion
		if version == "" {
			version = "1.0"
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
	if err := h.svc.SaveResume(c.UserContext(), acct.ID, fileHeader.Filename, fileType, contentType, data); err != nil {
		return err
	}
	updated, err := h.svc.repo.GetByID(c.UserContext(), acct.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, toMeView(updated))
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
