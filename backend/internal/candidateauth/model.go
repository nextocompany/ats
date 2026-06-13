// Package candidateauth owns the persistent career-portal candidate identity:
// accounts (signup via LINE / Google / email-OTP), httpOnly sessions, and the
// passwordless email-OTP challenge. It sits above the per-application candidates
// rows the scoring pipeline operates on (members link via candidates.account_id).
package candidateauth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors.
var (
	// ErrNotFound is returned when no account/session matches the lookup.
	ErrNotFound = errors.New("candidateauth: not found")
	// ErrOTPInvalid is returned when an email OTP does not match a live challenge
	// (wrong code, already consumed, or expired). Kept generic to avoid leaking
	// which case occurred.
	ErrOTPInvalid = errors.New("candidateauth: otp invalid or expired")
)

// Account is a persistent candidate identity. Any one of email / line_user_id /
// google_sub may be present; the others fill in as the user links providers.
type Account struct {
	ID             uuid.UUID `json:"id"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	EmailVerified  bool      `json:"email_verified"`
	Phone          string    `json:"phone"`
	LineUserID     string    `json:"-"` // verified LINE sub — never serialized to the client
	LineDisplayID  string    `json:"line_display_id"`
	GoogleSub      string    `json:"-"` // verified Google sub — never serialized
	Province       string    `json:"province"`
	ResumeBlobURL  string    `json:"-"` // internal blob URL — never serialized
	ResumeFileType string    `json:"resume_file_type"`
	PDPAConsent    bool      `json:"pdpa_consent"`
	PDPAVersion    string    `json:"pdpa_version"`
	CreatedAt      time.Time `json:"created_at"`
}

// HasResume reports whether a saved resume is attached (enables quick-apply).
func (a *Account) HasResume() bool { return a.ResumeBlobURL != "" }

// LineLinked reports whether a verified LINE identity is attached (enables push).
func (a *Account) LineLinked() bool { return a.LineUserID != "" }

// GoogleLinked reports whether a verified Google identity is attached.
func (a *Account) GoogleLinked() bool { return a.GoogleSub != "" }

// ProfileUpdate carries the user-editable profile fields (sparse: empty values
// are ignored so a partial save never blanks existing data).
type ProfileUpdate struct {
	FullName      string
	Phone         string
	LineDisplayID string
	Province      string
}
