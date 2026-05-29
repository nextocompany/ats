// Package dedup implements F09 (Step 3): reconcile a freshly-created candidate
// against existing records using exact contact matches + fuzzy name matching.
package dedup

import (
	"context"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidates"
)

// Dedup state values recorded on the application.
const (
	StateNone          = "none"
	StateAutoMerged    = "auto_merged"
	StatePendingReview = "pending_review"
)

const (
	autoThreshold   = 0.9 // ≥ this ⇒ auto-merge
	reviewThreshold = 0.7 // [reviewThreshold, autoThreshold) ⇒ HR review
	nameMaxDistance = 2   // Levenshtein ≤ 2 counts as a name match (F09)
)

// Decision is the outcome of reconciliation.
type Decision struct {
	CanonicalID uuid.UUID // the candidate the application should point at
	State       string
	Confidence  float64
}

// Service reconciles candidates using the candidate repository.
type Service struct {
	repo candidates.Repository
}

// NewService wires the dedup service.
func NewService(repo candidates.Repository) *Service {
	return &Service{repo: repo}
}

// Reconcile loads candidates sharing a contact field, decides, and on auto-merge
// marks the new candidate as a duplicate of the canonical one. It returns the
// canonical id the application should reference.
func (s *Service) Reconcile(ctx context.Context, newID uuid.UUID, name, phone, email, idCard string) (Decision, error) {
	cands, err := s.repo.FindDuplicates(ctx, newID, idCard, phone, email)
	if err != nil {
		return Decision{}, err
	}

	d := decide(newID, name, phone, email, idCard, cands)
	if d.State == StateAutoMerged {
		if err := s.repo.MarkDuplicateOf(ctx, newID, d.CanonicalID); err != nil {
			return Decision{}, err
		}
	}
	return d, nil
}

// decide is the pure matching logic (unit-tested without a database).
func decide(newID uuid.UUID, name, phone, email, idCard string, cands []candidates.Candidate) Decision {
	best := Decision{CanonicalID: newID, State: StateNone, Confidence: 0}

	for _, c := range cands {
		conf := score(name, phone, email, idCard, c)
		if conf > best.Confidence {
			best.Confidence = conf
			best.CanonicalID = c.ID
		}
	}

	switch {
	case best.Confidence >= autoThreshold:
		best.State = StateAutoMerged
	case best.Confidence >= reviewThreshold:
		best.State = StatePendingReview
		best.CanonicalID = newID // keep separate until HR confirms
	default:
		best.State = StateNone
		best.CanonicalID = newID
	}
	return best
}

// score computes a match confidence between the new values and one candidate.
func score(name, phone, email, idCard string, c candidates.Candidate) float64 {
	if idCard != "" && c.IDCard == idCard {
		return 1.0 // national id is authoritative
	}
	contactMatch := (phone != "" && c.Phone == phone) || (email != "" && c.Email == email)
	nameMatch := name != "" && c.FullName != "" && levenshtein(name, c.FullName) <= nameMaxDistance

	switch {
	case contactMatch && nameMatch:
		return 0.95
	case contactMatch:
		return 0.85
	case nameMatch:
		return 0.5 // name alone is weak — below review threshold
	default:
		return 0
	}
}
