package fit

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ScopeChecker reports whether an application is visible to an RBAC scope. The
// applications repository satisfies it; it keeps per-record authorization on the
// HR fit endpoints consistent with the scoping used elsewhere.
type ScopeChecker interface {
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
}

// Handler exposes the HR-facing fit-analysis endpoints.
type Handler struct {
	svc    *Service
	scoper ScopeChecker
}

// NewHandler builds the fit HTTP handler. A nil scoper is a wiring bug — every
// fit endpoint is per-application authorized, so we fail fast rather than risk
// silently serving cross-scope data.
func NewHandler(svc *Service, scoper ScopeChecker) *Handler {
	if scoper == nil {
		panic("fit: ScopeChecker must not be nil")
	}
	return &Handler{svc: svc, scoper: scoper}
}

func scopeFrom(c *fiber.Ctx) rbac.Scope {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
}

// authorizeApplication returns a 404 (not 403, to avoid leaking existence) when
// the caller's scope cannot see the application.
func (h *Handler) authorizeApplication(c *fiber.Ctx, id uuid.UUID) error {
	ok, err := h.scoper.ExistsInScope(c.UserContext(), id, scopeFrom(c))
	if err != nil {
		return err
	}
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	return nil
}

// generatedBy parses the authenticated user's id, or nil when unavailable.
func generatedBy(c *fiber.Ctx) *uuid.UUID {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return nil
	}
	return &id
}

// Generate handles POST /api/v1/applications/:id/fit-analysis (HR action).
func (h *Handler) Generate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if err := h.authorizeApplication(c, id); err != nil {
		return err
	}
	a, err := h.svc.Generate(c.UserContext(), id, generatedBy(c))
	if err != nil {
		return mapError(err)
	}
	return httpx.OK(c, fiber.Map{"analysis": a})
}

// Get handles GET /api/v1/applications/:id/fit-analysis (HR view).
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if err := h.authorizeApplication(c, id); err != nil {
		return err
	}
	a, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return mapError(err)
	}
	return httpx.OK(c, fiber.Map{"analysis": a})
}

// mapError translates service/repository errors into HTTP errors.
func mapError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "no fit analysis for this application")
	case errors.Is(err, ErrNotScored):
		return fiber.NewError(fiber.StatusConflict, "ต้องผ่านการ Screening ก่อนจึงจะวิเคราะห์ความเหมาะสมได้")
	case errors.Is(err, ErrInterviewIncomplete):
		return fiber.NewError(fiber.StatusConflict, "ต้องทำ AI Interview ให้เสร็จก่อนจึงจะวิเคราะห์ความเหมาะสมได้")
	default:
		return err
	}
}
