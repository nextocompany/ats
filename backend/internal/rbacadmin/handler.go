// Package rbacadmin serves the super_admin role/permission management API for
// dynamic RBAC. It lives outside internal/rbac (which is imported widely, incl.
// transitively by the auth middleware) to avoid an import cycle
// (rbac → middleware → hrauth → rbac); this package may import middleware freely.
package rbacadmin

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// roleKeyRe bounds a new role key to a safe slug.
var roleKeyRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// Handler serves /api/v1/admin/rbac/* (gated to the rbac.admin permission).
type Handler struct {
	repo  rbac.Repository
	authz *rbac.Authorizer // may be nil (then changes propagate within the TTL only)
}

// NewHandler builds the rbac admin handler. authz may be nil (no dynamic
// authorizer installed — e.g. matrix not seeded yet).
func NewHandler(repo rbac.Repository, authz *rbac.Authorizer) *Handler {
	return &Handler{repo: repo, authz: authz}
}

// RegisterRoutes mounts the role/permission admin endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/admin/rbac")
	g.Get("/permissions", h.ListPermissions)
	g.Get("/roles", h.ListRoles)
	g.Post("/roles", h.CreateRole)
	g.Patch("/roles/:key", h.UpdateRole)
	g.Delete("/roles/:key", h.DeleteRole)
}

// gate returns false (and the caller writes 403) unless the user holds rbac.admin.
func (h *Handler) gate(c *fiber.Ctx) bool {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return false
	}
	return rbac.Can(u.Role, rbac.PermRBACAdmin)
}

func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	if !h.gate(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage RBAC")
	}
	perms, err := h.repo.ListPermissions(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("rbacadmin: list permissions failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list permissions")
	}
	return httpx.OK(c, perms)
}

func (h *Handler) ListRoles(c *fiber.Ctx) error {
	if !h.gate(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage RBAC")
	}
	roles, err := h.repo.ListRoles(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("rbacadmin: list roles failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list roles")
	}
	return httpx.OK(c, roles)
}

type roleReq struct {
	Key         string   `json:"key"`
	LabelEn     string   `json:"label_en"`
	LabelTh     string   `json:"label_th"`
	ScopeKind   string   `json:"scope_kind"`
	Permissions []string `json:"permissions"`
}

func (h *Handler) CreateRole(c *fiber.Ctx) error {
	if !h.gate(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage RBAC")
	}
	var req roleReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	key := strings.TrimSpace(req.Key)
	if !roleKeyRe.MatchString(key) {
		return httpx.Fail(c, fiber.StatusBadRequest, "role key must match [a-z0-9_]+")
	}
	if !validScope(req.ScopeKind) {
		return httpx.Fail(c, fiber.StatusBadRequest, "scope_kind must be all, subregion, or store")
	}
	if bad := unknownPerms(req.Permissions); bad != "" {
		return httpx.Fail(c, fiber.StatusBadRequest, "unknown permission: "+bad)
	}
	role, err := h.repo.CreateRole(c.UserContext(), key, strings.TrimSpace(req.LabelEn), strings.TrimSpace(req.LabelTh), req.ScopeKind, req.Permissions)
	if err != nil {
		return h.writeErr(c, err)
	}
	h.reload(c)
	return httpx.Created(c, role)
}

func (h *Handler) UpdateRole(c *fiber.Ctx) error {
	if !h.gate(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage RBAC")
	}
	key := c.Params("key")
	var req roleReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	// Guard the super_admin built-in: it must keep all permissions + the broadest
	// scope (the authorizer bypass enforces this anyway, but reject the edit
	// explicitly so the matrix can never misrepresent it).
	if key == rbac.RoleSuperAdmin {
		return httpx.Fail(c, fiber.StatusBadRequest, "the super_admin role cannot be modified")
	}
	in := rbac.RoleInput{}
	if req.LabelEn != "" {
		v := strings.TrimSpace(req.LabelEn)
		in.LabelEn = &v
	}
	if req.LabelTh != "" {
		v := strings.TrimSpace(req.LabelTh)
		in.LabelTh = &v
	}
	if req.ScopeKind != "" {
		if !validScope(req.ScopeKind) {
			return httpx.Fail(c, fiber.StatusBadRequest, "scope_kind must be all, subregion, or store")
		}
		in.ScopeKind = &req.ScopeKind
	}
	if req.Permissions != nil {
		if bad := unknownPerms(req.Permissions); bad != "" {
			return httpx.Fail(c, fiber.StatusBadRequest, "unknown permission: "+bad)
		}
		in.Permissions = &req.Permissions
	}
	role, err := h.repo.UpdateRole(c.UserContext(), key, in)
	if err != nil {
		return h.writeErr(c, err)
	}
	h.reload(c)
	return httpx.OK(c, role)
}

func (h *Handler) DeleteRole(c *fiber.Ctx) error {
	if !h.gate(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage RBAC")
	}
	key := c.Params("key")
	n, err := h.repo.CountUsersWithRole(c.UserContext(), key)
	if err != nil {
		log.Error().Err(err).Msg("rbacadmin: count users with role failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not check role usage")
	}
	if n > 0 {
		return httpx.Fail(c, fiber.StatusConflict, "role is still assigned to users; reassign them first")
	}
	if err := h.repo.DeleteRole(c.UserContext(), key); err != nil {
		return h.writeErr(c, err)
	}
	h.reload(c)
	return httpx.OK(c, fiber.Map{"deleted": key})
}

// reload refreshes the local authorizer snapshot after a write (best-effort).
func (h *Handler) reload(c *fiber.Ctx) {
	if h.authz == nil {
		return
	}
	if err := h.authz.Reload(c.UserContext()); err != nil {
		log.Warn().Err(err).Msg("rbacadmin: authorizer reload after write failed (will refresh on TTL)")
	}
}

func (h *Handler) writeErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, rbac.ErrRoleExists):
		return httpx.Fail(c, fiber.StatusConflict, "a role with that key already exists")
	case errors.Is(err, rbac.ErrRoleNotFound):
		return httpx.Fail(c, fiber.StatusNotFound, "role not found")
	case errors.Is(err, rbac.ErrRoleBuiltin):
		return httpx.Fail(c, fiber.StatusConflict, "built-in roles cannot be deleted")
	default:
		log.Error().Err(err).Msg("rbacadmin: role write failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	}
}

func validScope(s string) bool {
	return s == rbac.KindAll || s == rbac.KindSubregion || s == rbac.KindStore
}

// unknownPerms returns the first permission key not in the catalog, or "".
func unknownPerms(perms []string) string {
	valid := make(map[string]struct{}, len(rbac.AllPermissions))
	for _, p := range rbac.AllPermissions {
		valid[p] = struct{}{}
	}
	for _, p := range perms {
		if _, ok := valid[p]; !ok {
			return p
		}
	}
	return ""
}
