// Package members is the HR-facing admin layer over career-portal member
// accounts (candidate_accounts). It is distinct from internal/candidateauth (the
// public self-service identity layer): access here is role-gated (super_admin +
// hr_manager), not session-cookie based, and it adds directory, lifecycle, and
// CRM concerns. PII is minimised — raw LINE/Google subs are never exposed, only
// linked-provider booleans.
package members

import (
	"time"

	"github.com/google/uuid"
)

// Account status values.
const (
	StatusActive     = "active"
	StatusSuspended  = "suspended"
	StatusAnonymized = "anonymized"
)

// Member is the admin-facing projection of a candidate_account (no raw subs/keys).
type Member struct {
	ID             uuid.UUID  `json:"id"`
	FullName       string     `json:"full_name"`
	Email          string     `json:"email"`
	Phone          string     `json:"phone"`
	Province       string     `json:"province"`
	EmailVerified  bool       `json:"email_verified"`
	LineLinked     bool       `json:"line_linked"`
	GoogleLinked   bool       `json:"google_linked"`
	EmailLinked    bool       `json:"email_linked"`
	HasResume      bool       `json:"has_resume"`
	ResumeType     string     `json:"resume_file_type"`
	Status         string     `json:"status"`
	PDPAConsent    bool       `json:"pdpa_consent"`
	PDPAVersion    string     `json:"pdpa_version"`
	AppsCount      int        `json:"applications_count"`
	ActiveSessions int        `json:"active_sessions"`
	LastLoginAt    *time.Time `json:"last_login_at"` // newest session created_at (login time, not last activity)
	CreatedAt      time.Time  `json:"created_at"`
	// CandidateID is the account's canonical (non-duplicate) candidate row, the key
	// for the candidate processing lock. Nil for an account with no candidate yet
	// (portal signup who never applied).
	CandidateID *uuid.UUID `json:"candidate_id,omitempty"`
	// Applications is the per-position funnel list, populated only on the detail
	// read (the unified person page). Nil on list rows (omitempty hides it).
	Applications []AccountApplication `json:"applications,omitempty"`
}

// AccountApplication is one row of the unified person detail's applications list:
// a position the person applied to and that application's current funnel status.
// Aggregated across every candidate row linked to the account (an account may
// have multiple per-intake candidate rows), so it mirrors applications_count.
type AccountApplication struct {
	ID            uuid.UUID `json:"id"`
	PositionID    uuid.UUID `json:"position_id"`
	PositionTitle string    `json:"position_title"`
	Status        string    `json:"status"`
	AIScore       *float64  `json:"ai_score"`
	CreatedAt     time.Time `json:"created_at"`
}

// Pagination defaults (mirrors internal/applications).
const (
	defaultLimit = 20
	maxLimit     = 100
)

// ListFilter is the directory query (all fields optional).
type ListFilter struct {
	Search    string // matches full_name / email / phone (ILIKE)
	Provider  string // line | google | email
	Status    string // active | suspended | anonymized
	Tag       string // members carrying this exact tag
	HasResume *bool
	From      *time.Time
	To        *time.Time
	Page      int
	Limit     int
}

func (f *ListFilter) normalize() {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	if f.Page <= 0 {
		f.Page = 1
	}
}

// ProfileUpdate carries the admin-editable member fields (sparse: empty values
// are ignored so a partial save never blanks existing data). Mirrors
// candidateauth.ProfileUpdate but admin-side (email is editable here).
type ProfileUpdate struct {
	FullName string
	Phone    string
	Province string
	Email    string
}

// IsEmpty reports whether the update would change nothing (all fields blank).
func (p ProfileUpdate) IsEmpty() bool {
	return p.FullName == "" && p.Phone == "" && p.Province == "" && p.Email == ""
}

// Stats is the directory summary strip.
type Stats struct {
	Total            int            `json:"total"`
	Active           int            `json:"active"`
	Suspended        int            `json:"suspended"`
	WithApplications int            `json:"with_applications"`
	NewThisWeek      int            `json:"new_this_week"`
	ByProvider       map[string]int `json:"by_provider"` // line | google | email
}

// Note is an HR-only timeline note on a member (never exposed on the public portal).
type Note struct {
	ID          uuid.UUID `json:"id"`
	AuthorEmail string    `json:"author_email"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

// maxTagLen / maxNoteLen bound CRM free-text (defends storage + UI). Tags are
// normalised (trimmed, lowercased) before persistence so "Retail" and "retail"
// don't both exist.
const (
	maxTagLen  = 50
	maxNoteLen = 2000
)
