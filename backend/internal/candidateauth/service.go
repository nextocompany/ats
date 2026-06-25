package candidateauth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/email"
	"github.com/nexto/hr-ats/pkg/emailtmpl"
)

// BlobStore is the subset of pkg/blob the service needs (resume save + read +
// delete + view). SignedURL takes no ctx — that mirrors *blob.Client.
type BlobStore interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	Download(ctx context.Context, name string) ([]byte, error)
	Delete(ctx context.Context, name string) error
	SignedURL(name string, ttl time.Duration) (string, error)
}

// resumeViewTTL bounds how long a candidate's resume-view link stays valid (the
// browser opens it immediately, so a short window is enough). Mirrors the HR
// dashboard's resumeURLTTL.
const resumeViewTTL = 15 * time.Minute

// Session is an issued candidate session: the raw token (set in the cookie) and
// its expiry. Only the hash is persisted.
type Session struct {
	Token   string
	Expires time.Time
}

// ConsentPolicy reports the current PDPA notice version (satisfied by *pdpa.Repo).
// Optional: nil falls back to the historical "1.0".
type ConsentPolicy interface {
	CurrentVersion(ctx context.Context) (string, error)
}

// Service orchestrates candidate signup/login: email-OTP, OAuth login (LINE/
// Google), session issuance, profile + resume, and identity linking.
type Service struct {
	repo       Repository
	email      email.Sender
	blob       BlobStore
	otpTTL     time.Duration
	sessionTTL time.Duration
	policy     ConsentPolicy
}

// NewService wires the candidateauth service.
func NewService(repo Repository, sender email.Sender, blob BlobStore, otpTTL, sessionTTL time.Duration) *Service {
	return &Service{repo: repo, email: sender, blob: blob, otpTTL: otpTTL, sessionTTL: sessionTTL}
}

// NewProvisioner returns a Service wired with only the repository, for callers
// (the worker pipeline) that need silent at-intake account provisioning
// (EnsureAccountByEmail) without the email/blob/session machinery.
func NewProvisioner(repo Repository) *Service {
	return &Service{repo: repo}
}

// EnsureAccountByEmail silently ensures an (unverified) candidate account exists
// for rawEmail and returns its id. ok is false (with a nil error) when the email
// is empty or invalid, so the caller cleanly skips no-email candidates without
// treating it as a failure. Never sends any notification.
func (s *Service) EnsureAccountByEmail(ctx context.Context, rawEmail string) (uuid.UUID, bool, error) {
	email, ok := normalizeEmail(rawEmail)
	if !ok {
		return uuid.Nil, false, nil
	}
	acct, err := s.repo.FindOrCreateUnverifiedByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, false, err
	}
	return acct.ID, true, nil
}

// WithConsentPolicy wires the current-version source for consent stamping and the
// re-consent prompt, returning the service for chaining.
func (s *Service) WithConsentPolicy(p ConsentPolicy) *Service {
	s.policy = p
	return s
}

// CurrentConsentVersion returns the registry's current notice version, falling
// back to "1.0" when no policy is wired or the lookup fails.
func (s *Service) CurrentConsentVersion(ctx context.Context) string {
	if s.policy == nil {
		return "1.0"
	}
	v, err := s.policy.CurrentVersion(ctx)
	if err != nil || v == "" {
		return "1.0"
	}
	return v
}

// WithdrawConsent records a consent withdrawal for the account (ledger + snapshot).
func (s *Service) WithdrawConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	return s.repo.WithdrawConsent(ctx, accountID, version)
}

// normalizeEmail lowercases + trims; returns ("", false) when not a valid address.
func normalizeEmail(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" {
		return "", false
	}
	if _, err := mail.ParseAddress(e); err != nil {
		return "", false
	}
	return e, true
}

// StartEmailOTP generates and emails a one-time code for the address. It is
// enumeration-safe at the handler layer (always 200); here it returns an error
// only for an invalid address or a delivery failure.
func (s *Service) StartEmailOTP(ctx context.Context, rawEmail string) error {
	addr, ok := normalizeEmail(rawEmail)
	if !ok {
		return fmt.Errorf("candidateauth: invalid email")
	}
	code, err := genOTP()
	if err != nil {
		return err
	}
	if err := s.repo.CreateOTP(ctx, addr, hashSecret(code), time.Now().Add(s.otpTTL)); err != nil {
		return err
	}
	mins := int(s.otpTTL.Minutes())
	// Branded, bilingual (TH/EN). The code lives in a detail row and is never put
	// in the subject; the mock sender logs only PlainText (and only in development),
	// never HTML, so a misconfigured prod cannot leak the code.
	doc := emailtmpl.Doc{
		Title:      "รหัสยืนยันการเข้าสู่ระบบ / Your login code",
		Paragraphs: []string{"ใช้รหัสด้านล่างเพื่อเข้าสู่ระบบ", "Use the code below to sign in."},
		Details:    []emailtmpl.DetailRow{{Label: "รหัสยืนยัน / Code", Value: code}},
		Outro:      fmt.Sprintf("รหัสหมดอายุใน %d นาที / expires in %d minutes", mins, mins),
	}
	msg := email.Message{
		To:        addr,
		Subject:   "รหัสยืนยันการเข้าสู่ระบบ / Your login code",
		PlainText: doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
	if err := s.email.Send(ctx, msg); err != nil {
		return fmt.Errorf("candidateauth: send otp: %w", err)
	}
	return nil
}

// VerifyEmailOTP consumes the code, finds-or-creates the account, and issues a
// session. Returns ErrOTPInvalid when the code does not match a live challenge.
func (s *Service) VerifyEmailOTP(ctx context.Context, rawEmail, code string) (*Account, Session, error) {
	addr, ok := normalizeEmail(rawEmail)
	if !ok {
		return nil, Session{}, fmt.Errorf("candidateauth: invalid email")
	}
	if strings.TrimSpace(code) == "" {
		return nil, Session{}, ErrOTPInvalid
	}
	if err := s.repo.ConsumeOTP(ctx, addr, hashSecret(code)); err != nil {
		return nil, Session{}, err
	}
	acct, err := s.repo.FindOrCreateByEmail(ctx, addr)
	if err != nil {
		return nil, Session{}, err
	}
	sess, err := s.issueSessionFor(ctx, acct)
	if err != nil {
		return nil, Session{}, err
	}
	return acct, sess, nil
}

// LoginWithLine implements lineauth.SessionIssuer: find-or-create by LINE sub and
// issue a session. Returns the raw session token + expiry for the cookie.
func (s *Service) LoginWithLine(ctx context.Context, sub, name, email string) (string, time.Time, error) {
	// Normalize the LINE email (lower+trim, drop if invalid) so it matches stored
	// emails for unify/backfill — mirrors the Google path, which already lowercases.
	if e, ok := normalizeEmail(email); ok {
		email = e
	} else {
		email = ""
	}
	acct, err := s.repo.FindOrCreateByLineSub(ctx, sub, name, email)
	if err != nil {
		return "", time.Time{}, err
	}
	sess, err := s.issueSessionFor(ctx, acct)
	if err != nil {
		return "", time.Time{}, err
	}
	return sess.Token, sess.Expires, nil
}

// LoginWithGoogle finds-or-creates by Google sub and issues a session.
func (s *Service) LoginWithGoogle(ctx context.Context, sub, name, email string) (string, time.Time, error) {
	acct, err := s.repo.FindOrCreateByGoogleSub(ctx, sub, name, email)
	if err != nil {
		return "", time.Time{}, err
	}
	sess, err := s.issueSessionFor(ctx, acct)
	if err != nil {
		return "", time.Time{}, err
	}
	return sess.Token, sess.Expires, nil
}

// LinkLine implements lineauth.SessionIssuer's link path: resolve the account
// from an existing session token, then attach the verified LINE identity.
func (s *Service) LinkLine(ctx context.Context, sessionToken, sub, displayID string) error {
	acct, err := s.AccountFromSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	return s.repo.LinkLine(ctx, acct.ID, sub, displayID)
}

// AccountFromSession resolves the account behind a live session token, or ErrNotFound.
func (s *Service) AccountFromSession(ctx context.Context, token string) (*Account, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNotFound
	}
	return s.repo.FindAccountBySessionHash(ctx, hashSecret(token))
}

// Logout revokes the session behind the token (idempotent).
func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.repo.RevokeSession(ctx, hashSecret(token))
}

// UpdateProfile applies sparse profile edits. A non-empty email is normalized and
// validated here; the repo enforces set-once + collision (ErrEmailTaken).
func (s *Service) UpdateProfile(ctx context.Context, accountID uuid.UUID, p ProfileUpdate) error {
	if strings.TrimSpace(p.Email) != "" {
		addr, ok := normalizeEmail(p.Email)
		if !ok {
			return ErrInvalidEmail
		}
		p.Email = addr
	}
	return s.repo.UpdateProfile(ctx, accountID, p)
}

// BackfillContact best-effort fills phone/email onto an account from an apply.
// The email is normalized; an invalid one is dropped (the apply already validated
// it for the candidate row, but normalize defensively) so backfill never fails.
func (s *Service) BackfillContact(ctx context.Context, accountID uuid.UUID, phone, rawEmail string) error {
	email := ""
	if addr, ok := normalizeEmail(rawEmail); ok {
		email = addr
	}
	return s.repo.BackfillContact(ctx, accountID, phone, email)
}

// BackfillNames best-effort fills the Thai/English match names onto an account
// from an apply submission (fill-once; signup/profile names stay authoritative).
func (s *Service) BackfillNames(ctx context.Context, accountID uuid.UUID, nameTH, nameEN string) error {
	return s.repo.BackfillNames(ctx, accountID, strings.TrimSpace(nameTH), strings.TrimSpace(nameEN))
}

// SaveResume stores the resume under an account-scoped blob key and records it.
// The stored value is the blob KEY (not a URL) so quick-apply can Download it.
func (s *Service) SaveResume(ctx context.Context, accountID uuid.UUID, fileName, fileType, contentType string, data []byte) error {
	key := fmt.Sprintf("accounts/%s/%s", accountID, fileName)
	if _, err := s.blob.Upload(ctx, key, data, contentType); err != nil {
		return fmt.Errorf("candidateauth: upload resume: %w", err)
	}
	return s.repo.SetResume(ctx, accountID, key, fileType)
}

// AddResume adds a CV to the account's library (newest first). The account's
// FIRST resume becomes the default. Blocks at MaxResumes with ErrResumeLimit -
// the candidate must delete one first. Stores the blob KEY (not a URL) so
// quick-apply can Download the default.
func (s *Service) AddResume(ctx context.Context, accountID uuid.UUID, fileName, fileType, contentType string, data []byte) error {
	count, err := s.repo.CountResumes(ctx, accountID)
	if err != nil {
		return err
	}
	if count >= MaxResumes {
		return ErrResumeLimit
	}
	id := uuid.New()
	key := fmt.Sprintf("accounts/%s/%s/%s", accountID, id, fileName)
	if _, err := s.blob.Upload(ctx, key, data, contentType); err != nil {
		return fmt.Errorf("candidateauth: upload resume: %w", err)
	}
	if err := s.repo.InsertResume(ctx, accountID, id, key, fileName, fileType, count == 0); err != nil {
		// Best-effort cleanup of the orphaned blob; the DB row is the source of truth.
		_ = s.blob.Delete(ctx, key)
		return err
	}
	return nil
}

// ListResumes returns the account's CV history, newest first.
func (s *Service) ListResumes(ctx context.Context, accountID uuid.UUID) ([]Resume, error) {
	return s.repo.ListResumes(ctx, accountID)
}

// SetDefaultResume marks one resume the default (used for quick-apply).
func (s *Service) SetDefaultResume(ctx context.Context, accountID, resumeID uuid.UUID) error {
	return s.repo.SetDefaultResume(ctx, accountID, resumeID)
}

// ResumeViewURL returns a short-lived signed URL for one of the account's resumes
// so the candidate can open the file in the browser. The lookup is account-scoped
// (ErrNotFound for a resume the account does not own), so it cannot resolve another
// member's CV. PDFs/images render inline; docx downloads under its original name.
func (s *Service) ResumeViewURL(ctx context.Context, accountID, resumeID uuid.UUID) (string, error) {
	key, _, err := s.repo.ResumeBlobKey(ctx, accountID, resumeID)
	if err != nil {
		return "", err
	}
	return s.blob.SignedURL(key, resumeViewTTL)
}

// DeleteResume removes a resume and its blob (best-effort). Deleting the default
// promotes the newest remaining resume.
func (s *Service) DeleteResume(ctx context.Context, accountID, resumeID uuid.UUID) error {
	key, err := s.repo.DeleteResume(ctx, accountID, resumeID)
	if err != nil {
		return err
	}
	if key != "" {
		_ = s.blob.Delete(ctx, key) // row already gone; blob cleanup is best-effort
	}
	return nil
}

// SaveConsent records PDPA consent on the account + a ledger row (portal
// profile/signup path, where no candidate row exists yet).
func (s *Service) SaveConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	return s.repo.SetConsent(ctx, accountID, version)
}

// MarkConsented updates only the account consent snapshot (no ledger row). The
// apply flow uses this; the candidate-keyed apply consent row is the ledger record.
func (s *Service) MarkConsented(ctx context.Context, accountID uuid.UUID, version string) error {
	return s.repo.MarkConsented(ctx, accountID, version)
}

// SavedResumeBytes downloads the account's saved resume for quick-apply.
func (s *Service) SavedResumeBytes(ctx context.Context, acct *Account) ([]byte, error) {
	if !acct.HasResume() {
		return nil, fmt.Errorf("candidateauth: no saved resume")
	}
	return s.blob.Download(ctx, acct.ResumeBlobURL) // stores the blob key
}

// issueSessionFor refuses a suspended/anonymized account a fresh session: the
// provider identity is valid, but the account may not log in. This is the
// fresh-login half of the suspension enforcement (the session-resolve query in
// the repository handles existing cookies).
func (s *Service) issueSessionFor(ctx context.Context, acct *Account) (Session, error) {
	if !acct.IsActive() {
		return Session{}, ErrAccountSuspended
	}
	return s.issueSession(ctx, acct.ID)
}

func (s *Service) issueSession(ctx context.Context, accountID uuid.UUID) (Session, error) {
	tok, err := randToken()
	if err != nil {
		return Session{}, err
	}
	exp := time.Now().Add(s.sessionTTL)
	if err := s.repo.CreateSession(ctx, accountID, hashSecret(tok), exp); err != nil {
		return Session{}, err
	}
	return Session{Token: tok, Expires: exp}, nil
}
