package applications

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidatelock"
)

// lockGuard is embedded by every handler that mutates a candidate so the
// candidate processing lock is enforced uniformly across the operate surface.
// The enforcer is optional: when unset (tests, or enforcement not wired) the
// guard is a no-op and the mutation proceeds — keeping existing behaviour when
// the feature is not configured.
type lockGuard struct{ lockEnforcer *candidatelock.Enforcer }

// SetLockEnforcer wires the shared lock enforcer (called once at startup).
func (g *lockGuard) SetLockEnforcer(e *candidatelock.Enforcer) { g.lockEnforcer = e }

// guardLock enforces the processing lock for the actor on candidateID. It
// returns (proceed, err); when proceed is false the HTTP response is already
// written and the caller must return err.
func (g *lockGuard) guardLock(c *fiber.Ctx, candidateID uuid.UUID) (bool, error) {
	return g.lockEnforcer.Guard(c, candidateID)
}

// lockedByOther reports a lock held by a different actor (used by bulk to skip
// contended candidates rather than fail the whole batch).
func (g *lockGuard) lockedByOther(c *fiber.Ctx, candidateID uuid.UUID) (*candidatelock.Lock, bool) {
	return g.lockEnforcer.Held(c, candidateID)
}
