package applications

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Letter generation (Module-3 3.3). HR generates a PDF interview-invitation or
// offer letter; it is stored in blob and downloadable by HR and the candidate.
// The lifecycle is trivial — one current letter per (application, type), regenerate
// overwrites. The actual PDF rendering lives in internal/letters.

// Letter types (mirror letters.TypeInterview / letters.TypeOffer).
const (
	LetterInterview = "interview"
	LetterOffer     = "offer"
)

// canManageLetter may generate letters — now resolved via dynamic RBAC
// (rbac.PermLetterWrite).
func canManageLetter(role string) bool { return rbac.Can(role, rbac.PermLetterWrite) }

func validLetterType(tp string) bool { return tp == LetterInterview || tp == LetterOffer }

// ErrLetterPreconditions signals the source data for a letter is missing (no
// interview scheduled, or no sendable offer) — mapped to HTTP 400.
var ErrLetterPreconditions = errors.New("applications: letter preconditions not met")

// Letter is the stored letter record. BlobURL is internal; the API exposes a
// freshly-signed URL via LetterView.
type Letter struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	Type          string    `json:"type"`
	BlobURL       string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

// LetterView is a letter plus a short-lived signed download URL.
type LetterView struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url"`
}
