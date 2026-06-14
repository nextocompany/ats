package candidateauth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/email"
)

// BlobStore is the subset of pkg/blob the service needs (resume save + read).
type BlobStore interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	Download(ctx context.Context, name string) ([]byte, error)
}

// Session is an issued candidate session: the raw token (set in the cookie) and
// its expiry. Only the hash is persisted.
type Session struct {
	Token   string
	Expires time.Time
}

// Service orchestrates candidate signup/login: email-OTP, OAuth login (LINE/
// Google), session issuance, profile + resume, and identity linking.
type Service struct {
	repo       Repository
	email      email.Sender
	blob       BlobStore
	otpTTL     time.Duration
	sessionTTL time.Duration
}

// NewService wires the candidateauth service.
func NewService(repo Repository, sender email.Sender, blob BlobStore, otpTTL, sessionTTL time.Duration) *Service {
	return &Service{repo: repo, email: sender, blob: blob, otpTTL: otpTTL, sessionTTL: sessionTTL}
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
	msg := email.Message{
		To:        addr,
		Subject:   "รหัสยืนยันการเข้าสู่ระบบ / Your login code",
		PlainText: fmt.Sprintf("รหัสยืนยันของคุณคือ %s (หมดอายุใน %d นาที)\n\nYour verification code is %s (expires in %d minutes).", code, mins, code, mins),
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

// UpdateProfile applies sparse profile edits.
func (s *Service) UpdateProfile(ctx context.Context, accountID uuid.UUID, p ProfileUpdate) error {
	return s.repo.UpdateProfile(ctx, accountID, p)
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

// SaveConsent records PDPA consent on the account (captured once at signup).
func (s *Service) SaveConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	return s.repo.SetConsent(ctx, accountID, version)
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
