package settings

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// adminRolesAllowed may read or change system settings. Restricted to the
// highest-privilege role — this gates who can open sign-in to other tenants.
var adminRolesAllowed = map[string]bool{"super_admin": true}

// Handler serves the admin system-settings endpoints.
type Handler struct{ svc *Service }

// NewHandler builds the settings handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mounts the admin settings endpoints under /api/v1/admin/settings.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/admin/settings")
	g.Get("/", h.Get)
	g.Patch("/", h.Update)
}

// dto is the wire shape for the settings the admin console manages.
type dto struct {
	AllowAllTenants bool `json:"allow_all_tenants"`
}

// Get handles GET /api/v1/admin/settings — returns the current flags.
func (h *Handler) Get(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for system settings")
	}
	v, err := h.svc.GetAllowAll(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, dto{AllowAllTenants: v})
}

// Update handles PATCH /api/v1/admin/settings — persists the flags.
func (h *Handler) Update(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !adminRolesAllowed[u.Role] {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for system settings")
	}
	var body dto
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.SetAllowAll(c.UserContext(), body.AllowAllTenants, u.Email); err != nil {
		return err
	}
	return httpx.OK(c, body)
}

func (h *Handler) authorized(c *fiber.Ctx) bool {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return adminRolesAllowed[u.Role]
}
