package executive

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the executive overview endpoint.
type Handler struct {
	svc Service
}

// NewHandler builds the executive handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the executive overview + Recruitment ROI endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	v1 := app.Group("/api/v1/executive")
	v1.Get("/overview", h.Overview)
	v1.Get("/roi", h.ROI)
	v1.Get("/cost-config", h.GetCostConfig)
	v1.Put("/cost-config", h.SetCostConfig)
}

// Overview handles GET /api/v1/executive/overview.
func (h *Handler) Overview(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermExecutiveView) {
		return fiber.NewError(fiber.StatusForbidden, "executive overview is restricted to leadership roles")
	}
	ov, err := h.svc.Overview(c.UserContext())
	if err != nil {
		return err
	}
	ov.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return httpx.OK(c, ov)
}

// ROI handles GET /api/v1/executive/roi — the Recruitment ROI & Performance view.
// Query params: period (month|quarter|year), dimension (branch|region|position),
// store (int), region (area uuid), position (uuid). Gated on executive.view.
func (h *Handler) ROI(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermExecutiveView) {
		return fiber.NewError(fiber.StatusForbidden, "executive ROI is restricted to leadership roles")
	}
	f := ExecFilters{
		Period:    c.Query("period", "quarter"),
		Dimension: c.Query("dimension", "branch"),
		Region:    c.Query("region"),
		Position:  c.Query("position"),
	}
	if s := c.Query("store"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			f.Store = &n
		}
	}
	view, err := h.svc.ROI(c.UserContext(), f)
	if err != nil {
		return err
	}
	view.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return httpx.OK(c, view)
}

// GetCostConfig handles GET /api/v1/executive/cost-config (executive.view).
func (h *Handler) GetCostConfig(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermExecutiveView) {
		return fiber.NewError(fiber.StatusForbidden, "executive cost config is restricted to leadership roles")
	}
	cfg, err := h.svc.GetCostConfig(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, cfg)
}

// SetCostConfig handles PUT /api/v1/executive/cost-config — writing the cost
// assumptions is gated on settings.admin (stricter than reading).
func (h *Handler) SetCostConfig(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermSettingsAdmin) {
		return fiber.NewError(fiber.StatusForbidden, "editing cost assumptions requires settings admin")
	}
	var body CostConfig
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.SetCostConfig(c.UserContext(), body, u.Email); err != nil {
		if errors.Is(err, ErrNegativeCost) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return err
	}
	cfg, err := h.svc.GetCostConfig(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, cfg)
}
