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

// statusActive is the only account status that may resolve a session or be
// issued a fresh one. Suspended/anonymized accounts are treated as logged-out.
// (Mirrors internal/members status values without importing it — candidateauth is
// the lower layer.)
const statusActive = "active"

// Sentinel errors.
var (
	// ErrNotFound is returned when no account/session matches the lookup.
	ErrNotFound = errors.New("candidateauth: not found")
	// ErrOTPInvalid is returned when an email OTP does not match a live challenge
	// (wrong code, already consumed, or expired). Kept generic to avoid leaking
	// which case occurred.
	ErrOTPInvalid = errors.New("candidateauth: otp invalid or expired")
	// ErrAccountSuspended is returned when a verified login resolves to a
	// suspended/anonymized account: the identity is valid but the account may not
	// hold a session. The session-resolve path additionally filters these out so
	// an existing cookie also stops working.
	ErrAccountSuspended = errors.New("candidateauth: account not active")
	// ErrResumeLimit is returned when an account already holds MaxResumes CVs and
	// must delete one before uploading another.
	ErrResumeLimit = errors.New("candidateauth: resume limit reached")
	// ErrLineLinkedToOther is returned when linking a LINE identity that already
	// belongs to a DIFFERENT account (line_user_id is unique). The portal surfaces
	// this distinctly so the user knows to log in with that LINE (merging accounts
	// is a separate, deliberate flow — not done here).
	ErrLineLinkedToOther = errors.New("candidateauth: line already linked to another account")
)

// MaxResumes caps the per-account resume library (the portal blocks the 6th
// upload until one is deleted).
const MaxResumes = 5

// Resume is one entry in a candidate's CV history. The blob key is internal and
// never serialized; the client identifies a resume by id.
type Resume struct {
	ID               uuid.UUID `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	FileType         string    `json:"file_type"`
	IsDefault        bool      `json:"is_default"`
	CreatedAt        time.Time `json:"created_at"`
}

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
	Status         string    `json:"-"` // active | suspended | anonymized — gates login, not client-facing
	CreatedAt      time.Time `json:"created_at"`
}

// IsActive reports whether the account may resolve or be issued a session.
func (a *Account) IsActive() bool { return a.Status == statusActive }

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
