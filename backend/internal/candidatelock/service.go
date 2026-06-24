// Package candidatelock provides a short-lived, per-candidate processing lock so
// that when several operators (e.g. every store HR + area HR + TA share the
// central pool) can see the same candidate, only one acts at a time. The lock is
// keyed by the canonical candidates.id, auto-expires (default 30 min), and can be
// refreshed by its holder or force-released by an admin.
package candidatelock

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// DefaultTTL is how long a freshly acquired lock stays live without a refresh.
const DefaultTTL = 30 * time.Minute

// ErrLockedByOther is returned when a live lock is held by a different user.
var ErrLockedByOther = errors.New("candidate is locked by another user")

// Lock is the current processing lock on a candidate.
type Lock struct {
	CandidateID  uuid.UUID `json:"candidate_id"`
	LockedBy     uuid.UUID `json:"locked_by"`
	LockedByName string    `json:"locked_by_name,omitempty"`
	LockedAt     time.Time `json:"locked_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Repository is the lock data-access contract.
type Repository interface {
	// Acquire atomically takes or refreshes the lock for byUser. It succeeds when
	// no live lock exists, the existing lock is expired, or byUser already holds it;
	// otherwise it returns ErrLockedByOther along with the live lock's holder.
	Acquire(ctx context.Context, candidateID, byUser uuid.UUID, ttl time.Duration) (Lock, error)
	// Release drops the lock. A non-force release only succeeds for the holder;
	// force (admin) drops it regardless. Releasing an absent lock is a no-op.
	Release(ctx context.Context, candidateID, byUser uuid.UUID, force bool) error
	// Get returns the live lock, or nil when none is held.
	Get(ctx context.Context, candidateID uuid.UUID) (*Lock, error)
}

// Service wraps the repository with the default TTL policy.
type Service struct {
	repo Repository
	ttl  time.Duration
}

// NewService builds the lock service. A non-positive ttl falls back to DefaultTTL.
func NewService(repo Repository, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Service{repo: repo, ttl: ttl}
}

// Acquire takes or refreshes the lock for byUser using the service TTL.
func (s *Service) Acquire(ctx context.Context, candidateID, byUser uuid.UUID) (Lock, error) {
	return s.repo.Acquire(ctx, candidateID, byUser, s.ttl)
}

// Release drops the lock (force lets an admin release another user's lock).
func (s *Service) Release(ctx context.Context, candidateID, byUser uuid.UUID, force bool) error {
	return s.repo.Release(ctx, candidateID, byUser, force)
}

// Get returns the live lock or nil.
func (s *Service) Get(ctx context.Context, candidateID uuid.UUID) (*Lock, error) {
	return s.repo.Get(ctx, candidateID)
}
