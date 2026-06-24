package candidatelock

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Enforcer turns the advisory candidate lock into a hard gate: a mutating
// operation on a candidate succeeds only when the actor can take or already
// holds the lock. It reuses Acquire (atomic take-or-refresh) so a single call
// both enforces exclusivity and refreshes/stamps the actor's claim — there is
// no read-then-check window (no TOCTOU) and an active operator's lock is
// auto-extended on every action.
type Enforcer struct {
	svc     *Service
	users   UserResolver
	pickups PickupStamper
}

// NewEnforcer builds the lock enforcer. svc and users are required; without a
// resolver the actor cannot be mapped to a users.id and every guarded mutation
// fails closed.
func NewEnforcer(svc *Service, users UserResolver) *Enforcer {
	return &Enforcer{svc: svc, users: users}
}

// SetPickupStamper wires the pickup recorder stamped on a successful guard.
// Taking the lock IS the "I'm taking this candidate" action, exactly as on the
// explicit acquire endpoint — so an operate action stops the pool-release timer.
func (e *Enforcer) SetPickupStamper(p PickupStamper) { e.pickups = p }

// Guard enforces the processing lock for the fiber actor over candidateID.
//
// It returns (proceed, err):
//   - proceed == true:  the actor holds (or just took) the lock — continue; err is nil.
//   - proceed == false: Guard has already written the HTTP response
//     (401/403/409/500) and the caller must `return err`.
//
// A nil receiver (enforcer not wired, e.g. in tests) and a super_admin actor
// both proceed without touching the lock — admin bypass mirrors force-release.
func (e *Enforcer) Guard(c *fiber.Ctx, candidateID uuid.UUID) (bool, error) {
	if e == nil { // not wired → enforcement disabled, behave as before
		return true, nil
	}
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return false, httpx.Fail(c, fiber.StatusUnauthorized, "authentication required")
	}
	// Admins bypass the per-candidate lock (same authority as force-release).
	if rbac.Can(u.Role, rbac.PermUsersAdmin) {
		return true, nil
	}
	actor, err := e.resolveActor(c.UserContext(), u.Email)
	if err != nil {
		return false, httpx.Fail(c, fiber.StatusForbidden, "your account is not provisioned to process candidates")
	}
	lock, err := e.svc.Acquire(c.UserContext(), candidateID, actor)
	if errors.Is(err, ErrLockedByOther) {
		return false, httpx.Fail(c, fiber.StatusConflict, "candidate is being processed by "+holderLabel(lock))
	}
	if err != nil {
		return false, httpx.Fail(c, fiber.StatusInternalServerError, "failed to verify the processing lock")
	}
	// Best-effort pickup stamp: a stamping failure must not fail a lock the actor
	// already holds.
	if e.pickups != nil {
		if _, perr := e.pickups.MarkPickedUp(c.UserContext(), candidateID, actor); perr != nil {
			log.Warn().Err(perr).Str("candidate_id", candidateID.String()).Msg("candidatelock: enforce pickup stamp failed")
		}
	}
	return true, nil
}

// Held reports whether candidateID is currently locked by someone OTHER than the
// fiber actor, returning that holder's lock. Unlike Guard it never acquires —
// callers that process many candidates in one request (bulk) use it to skip the
// contended ones without taking a lock on the rest.
func (e *Enforcer) Held(c *fiber.Ctx, candidateID uuid.UUID) (*Lock, bool) {
	if e == nil {
		return nil, false
	}
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok {
		return nil, false
	}
	if rbac.Can(u.Role, rbac.PermUsersAdmin) {
		return nil, false // admins are never blocked
	}
	lock, err := e.svc.Get(c.UserContext(), candidateID)
	if err != nil || lock == nil {
		return nil, false // unlocked, or fail-open on read error (mutation still scope-gated)
	}
	actor, err := e.resolveActor(c.UserContext(), u.Email)
	if err != nil {
		return lock, true // cannot prove ownership → treat as held by other
	}
	if lock.LockedBy == actor {
		return nil, false // the actor holds it
	}
	return lock, true
}

func (e *Enforcer) resolveActor(ctx context.Context, email string) (uuid.UUID, error) {
	email = strings.TrimSpace(email)
	if e.users == nil || email == "" {
		return uuid.Nil, errors.New("cannot resolve actor")
	}
	id, _, err := e.users.ResolveUser(ctx, email)
	return id, err
}
