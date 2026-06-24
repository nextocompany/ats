package candidatelock

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// enforceRepo is a configurable lock repository for the enforcement scenarios.
type enforceRepo struct {
	acquireErr   error
	getLock      *Lock
	getErr       error
	acquireCalls int
}

func (f *enforceRepo) Acquire(_ context.Context, candidateID, byUser uuid.UUID, ttl time.Duration) (Lock, error) {
	f.acquireCalls++
	if f.acquireErr != nil {
		return Lock{LockedByName: "Other Operator"}, f.acquireErr
	}
	return Lock{CandidateID: candidateID, LockedBy: byUser, ExpiresAt: time.Now().Add(ttl)}, nil
}
func (f *enforceRepo) Release(context.Context, uuid.UUID, uuid.UUID, bool) error { return nil }
func (f *enforceRepo) Get(context.Context, uuid.UUID) (*Lock, error)             { return f.getLock, f.getErr }

type enforceResolver struct {
	id  uuid.UUID
	err error
}

func (r enforceResolver) ResolveUser(context.Context, string) (uuid.UUID, string, error) {
	return r.id, "Resolved Name", r.err
}

type enforceStamper struct{ calls int }

func (s *enforceStamper) MarkPickedUp(context.Context, uuid.UUID, uuid.UUID) (int, error) {
	s.calls++
	return 1, nil
}

func guardApp(e *Enforcer, u middleware.DevUser, candID uuid.UUID) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, u)
		return c.Next()
	})
	app.Post("/g", func(c *fiber.Ctx) error {
		ok, err := e.Guard(c, candID)
		if !ok {
			return err
		}
		return httpx.OK(c, fiber.Map{"ok": true})
	})
	return app
}

func statusFor(t *testing.T, app *fiber.App) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("POST", "/g", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

const operatorRole = "hr_store" // a real operate role, not an admin

func TestGuard_NilEnforcerProceeds(t *testing.T) {
	var e *Enforcer // not wired
	u := middleware.DevUser{ID: uuid.NewString(), Email: "a@b.co", Role: operatorRole}
	if got := statusFor(t, guardApp(e, u, uuid.New())); got != fiber.StatusOK {
		t.Errorf("nil enforcer: status = %d, want 200", got)
	}
}

func TestGuard_NoAuthIsUnauthorized(t *testing.T) {
	e := NewEnforcer(NewService(&enforceRepo{}, 0), enforceResolver{id: uuid.New()})
	if got := statusFor(t, guardApp(e, middleware.DevUser{}, uuid.New())); got != fiber.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want 401", got)
	}
}

func TestGuard_SuperAdminBypassesWithoutAcquiring(t *testing.T) {
	repo := &enforceRepo{acquireErr: ErrLockedByOther} // would 409 if consulted
	e := NewEnforcer(NewService(repo, 0), enforceResolver{id: uuid.New()})
	u := middleware.DevUser{ID: uuid.NewString(), Email: "admin@b.co", Role: "super_admin"}
	if got := statusFor(t, guardApp(e, u, uuid.New())); got != fiber.StatusOK {
		t.Errorf("super_admin: status = %d, want 200 (bypass)", got)
	}
	if repo.acquireCalls != 0 {
		t.Errorf("super_admin must not touch the lock; Acquire called %d times", repo.acquireCalls)
	}
}

func TestGuard_UnresolvableActorForbidden(t *testing.T) {
	e := NewEnforcer(NewService(&enforceRepo{}, 0), enforceResolver{err: errors.New("not provisioned")})
	u := middleware.DevUser{ID: uuid.NewString(), Email: "ghost@b.co", Role: operatorRole}
	if got := statusFor(t, guardApp(e, u, uuid.New())); got != fiber.StatusForbidden {
		t.Errorf("unresolvable actor: status = %d, want 403", got)
	}
}

func TestGuard_LockedByOtherConflicts(t *testing.T) {
	e := NewEnforcer(NewService(&enforceRepo{acquireErr: ErrLockedByOther}, 0), enforceResolver{id: uuid.New()})
	u := middleware.DevUser{ID: uuid.NewString(), Email: "a@b.co", Role: operatorRole}
	if got := statusFor(t, guardApp(e, u, uuid.New())); got != fiber.StatusConflict {
		t.Errorf("locked by other: status = %d, want 409", got)
	}
}

func TestGuard_HolderProceedsAndStampsPickup(t *testing.T) {
	stamper := &enforceStamper{}
	e := NewEnforcer(NewService(&enforceRepo{}, 0), enforceResolver{id: uuid.New()})
	e.SetPickupStamper(stamper)
	u := middleware.DevUser{ID: uuid.NewString(), Email: "a@b.co", Role: operatorRole}
	if got := statusFor(t, guardApp(e, u, uuid.New())); got != fiber.StatusOK {
		t.Errorf("holder: status = %d, want 200", got)
	}
	if stamper.calls != 1 {
		t.Errorf("pickup stamp called %d times, want 1", stamper.calls)
	}
}

func TestHeld_BulkScenarios(t *testing.T) {
	actor := uuid.New()
	other := uuid.New()
	cand := uuid.New()
	mk := func(repo *enforceRepo) *Enforcer {
		return NewEnforcer(NewService(repo, 0), enforceResolver{id: actor})
	}
	app := func(e *Enforcer, role string) (held bool) {
		a := fiber.New()
		a.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Email: "a@b.co", Role: role})
			return c.Next()
		})
		a.Get("/h", func(c *fiber.Ctx) error {
			_, h := e.Held(c, cand)
			held = h
			return c.SendStatus(fiber.StatusOK)
		})
		_, _ = a.Test(httptest.NewRequest("GET", "/h", nil))
		return held
	}

	if app(mk(&enforceRepo{getLock: nil}), operatorRole) {
		t.Error("unlocked: Held should be false")
	}
	if !app(mk(&enforceRepo{getLock: &Lock{LockedBy: other}}), operatorRole) {
		t.Error("locked by other: Held should be true")
	}
	if app(mk(&enforceRepo{getLock: &Lock{LockedBy: actor}}), operatorRole) {
		t.Error("locked by self: Held should be false")
	}
	if app(mk(&enforceRepo{getLock: &Lock{LockedBy: other}}), "super_admin") {
		t.Error("super_admin: Held should be false (never blocked)")
	}
}
