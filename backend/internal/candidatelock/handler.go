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

// UserResolver maps the authenticated actor's email to their local users.id —
// the lock holder must be a real user row (the Entra OID in the token is not a
// users.id, and locked_by is FK-constrained). Mirrors the requisition/interview
// resolvers.
type UserResolver interface {
	ResolveUser(ctx context.Context, email string) (id uuid.UUID, fullName string, err error)
}

// PickupStamper records that a store HR has begun processing a candidate, which
// stops that candidate's store-specific applications from being swept back to the
// central pool. Acquiring the lock IS the "I'm taking this candidate" action.
type PickupStamper interface {
	MarkPickedUp(ctx context.Context, candidateID, byUser uuid.UUID) (int, error)
}

// Handler serves the candidate processing-lock endpoints.
type Handler struct {
	svc     *Service
	users   UserResolver
	pickups PickupStamper
}

// NewHandler builds the lock handler. The resolver is required to identify the
// holder; without it acquire fails closed.
func NewHandler(svc *Service, users UserResolver) *Handler {
	return &Handler{svc: svc, users: users}
}

// SetPickupStamper wires the pickup recorder invoked on a successful acquire.
// Optional: with no stamper, acquiring the lock does not stamp picked_up_at.
func (h *Handler) SetPickupStamper(p PickupStamper) { h.pickups = p }

// RegisterRoutes mounts the lock endpoints under a candidate.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/candidates/:id/lock")
	g.Get("/", h.Get)
	g.Post("/", h.Acquire)
	g.Delete("/", h.Release)
}

func user(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return middleware.DevUser{}, false
	}
	return u, true
}

func (h *Handler) candidateID(c *fiber.Ctx) (uuid.UUID, error) {
	return uuid.Parse(c.Params("id"))
}

// resolveActor maps the actor's email to a users.id, the lock holder identity.
func (h *Handler) resolveActor(ctx context.Context, email string) (uuid.UUID, error) {
	email = strings.TrimSpace(email)
	if h.users == nil || email == "" {
		return uuid.Nil, errors.New("cannot resolve actor")
	}
	id, _, err := h.users.ResolveUser(ctx, email)
	return id, err
}

// Get returns the current lock state, or null when the candidate is unlocked.
func (h *Handler) Get(c *fiber.Ctx) error {
	if _, ok := user(c); !ok {
		return httpx.Fail(c, fiber.StatusUnauthorized, "authentication required")
	}
	id, err := h.candidateID(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "candidate id must be a valid uuid")
	}
	lock, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "failed to read lock")
	}
	return httpx.OK(c, lock) // nil → JSON null (unlocked)
}

// Acquire takes or refreshes the lock for the authenticated user.
func (h *Handler) Acquire(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok {
		return httpx.Fail(c, fiber.StatusUnauthorized, "authentication required")
	}
	id, err := h.candidateID(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "candidate id must be a valid uuid")
	}
	actor, err := h.resolveActor(c.UserContext(), u.Email)
	if err != nil {
		return httpx.Fail(c, fiber.StatusForbidden, "your account is not provisioned to process candidates")
	}
	lock, err := h.svc.Acquire(c.UserContext(), id, actor)
	if errors.Is(err, ErrLockedByOther) {
		return httpx.Fail(c, fiber.StatusConflict, "candidate is being processed by "+holderLabel(lock))
	}
	if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "failed to acquire lock")
	}
	// Acquiring the lock is the "I'm taking this candidate" action → stamp pickup so
	// the 3-day pool-release sweep no longer counts this candidate as abandoned.
	// Best-effort: a stamping failure must not fail the lock the user already holds.
	if h.pickups != nil {
		if _, perr := h.pickups.MarkPickedUp(c.UserContext(), id, actor); perr != nil {
			log.Warn().Err(perr).Str("candidate_id", id.String()).Msg("candidatelock: pickup stamp failed")
		}
	}
	return httpx.OK(c, lock)
}

// Release drops the lock. The holder may release their own; an admin
// (users.admin) may force-release anyone's.
func (h *Handler) Release(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok {
		return httpx.Fail(c, fiber.StatusUnauthorized, "authentication required")
	}
	id, err := h.candidateID(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "candidate id must be a valid uuid")
	}
	actor, err := h.resolveActor(c.UserContext(), u.Email)
	force := rbac.Can(u.Role, rbac.PermUsersAdmin)
	if err != nil && !force {
		return httpx.Fail(c, fiber.StatusForbidden, "your account is not provisioned to process candidates")
	}
	if err := h.svc.Release(c.UserContext(), id, actor, force); err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "failed to release lock")
	}
	return httpx.OK(c, fiber.Map{"released": true})
}

func holderLabel(l Lock) string {
	if strings.TrimSpace(l.LockedByName) != "" {
		return l.LockedByName
	}
	return "another user"
}
