package areas

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// Handler serves the area-management admin API, gated to area.admin.
type Handler struct{ repo Repository }

// NewHandler builds the areas handler.
func NewHandler(repo Repository) *Handler { return &Handler{repo: repo} }

// RegisterRoutes mounts the area-management endpoints.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/areas")
	g.Get("/", h.List)
	g.Post("/", h.Create)
	g.Get("/:id", h.Get)
	g.Patch("/:id", h.Update)
	g.Delete("/:id", h.Delete)
	g.Put("/:id/stores", h.SetStores)
	g.Put("/:id/members", h.SetMembers)

	// User-side area coverage (the area picker in user admin). Distinct path from
	// /users/me so it does not collide with the users handler.
	u := app.Group("/api/v1/users/:id/areas")
	u.Get("/", h.UserAreas)
	u.Put("/", h.SetUserAreas)
}

// requireAdmin gates every endpoint on the area.admin permission.
func (h *Handler) requireAdmin(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" || !rbac.Can(u.Role, rbac.PermAreaAdmin) {
		return middleware.DevUser{}, false
	}
	return u, true
}

func (h *Handler) idParam(c *fiber.Ctx) (uuid.UUID, error) { return uuid.Parse(c.Params("id")) }

func (h *Handler) List(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	list, err := h.repo.List(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("areas: list failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list areas")
	}
	return httpx.OK(c, list)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	id, err := h.idParam(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "area id must be a valid uuid")
	}
	a, err := h.repo.Get(c.UserContext(), id)
	if errors.Is(err, ErrNotFound) {
		return httpx.Fail(c, fiber.StatusNotFound, "area not found")
	}
	if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not load area")
	}
	return httpx.OK(c, a)
}

type nameReq struct {
	Name string `json:"name"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	var req nameReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 120 {
		return httpx.Fail(c, fiber.StatusBadRequest, "name is required (max 120 chars)")
	}
	a, err := h.repo.Create(c.UserContext(), name)
	if err != nil {
		log.Error().Err(err).Msg("areas: create failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not create area")
	}
	return httpx.Created(c, a)
}

type updateReq struct {
	Name   *string `json:"name"`
	Active *bool   `json:"active"`
}

func (h *Handler) Update(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	id, err := h.idParam(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "area id must be a valid uuid")
	}
	var req updateReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len([]rune(n)) > 120 {
			return httpx.Fail(c, fiber.StatusBadRequest, "name must be 1-120 chars")
		}
		req.Name = &n
	}
	a, err := h.repo.Update(c.UserContext(), id, req.Name, req.Active)
	if errors.Is(err, ErrNotFound) {
		return httpx.Fail(c, fiber.StatusNotFound, "area not found")
	}
	if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not update area")
	}
	return httpx.OK(c, a)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	id, err := h.idParam(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "area id must be a valid uuid")
	}
	if err := h.repo.Delete(c.UserContext(), id); errors.Is(err, ErrNotFound) {
		return httpx.Fail(c, fiber.StatusNotFound, "area not found")
	} else if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not delete area")
	}
	return httpx.OK(c, fiber.Map{"deleted": true})
}

type storesReq struct {
	StoreNos []int `json:"store_nos"`
}

func (h *Handler) SetStores(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	id, err := h.idParam(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "area id must be a valid uuid")
	}
	var req storesReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.repo.SetStores(c.UserContext(), id, req.StoreNos); err != nil {
		log.Error().Err(err).Msg("areas: set stores failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not set stores")
	}
	return httpx.OK(c, fiber.Map{"area_id": id, "store_count": len(req.StoreNos)})
}

type membersReq struct {
	UserIDs []string `json:"user_ids"`
}

func (h *Handler) SetMembers(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	id, err := h.idParam(c)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "area id must be a valid uuid")
	}
	var req membersReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	ids := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, s := range req.UserIDs {
		uid, perr := uuid.Parse(strings.TrimSpace(s))
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "user_ids must be valid uuids")
		}
		ids = append(ids, uid)
	}
	if err := h.repo.SetMembers(c.UserContext(), id, ids); err != nil {
		log.Error().Err(err).Msg("areas: set members failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not set members")
	}
	return httpx.OK(c, fiber.Map{"area_id": id, "member_count": len(ids)})
}

// UserAreas returns the area ids a user covers.
func (h *Handler) UserAreas(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	uid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "user id must be a valid uuid")
	}
	ids, err := h.repo.AreaIDsForUser(c.UserContext(), uid)
	if err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not load user areas")
	}
	return httpx.OK(c, ids)
}

// SetUserAreas replaces the set of areas a user covers.
func (h *Handler) SetUserAreas(c *fiber.Ctx) error {
	if _, ok := h.requireAdmin(c); !ok {
		return httpx.Fail(c, fiber.StatusForbidden, "area administration not permitted")
	}
	uid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "user id must be a valid uuid")
	}
	var req struct {
		AreaIDs []string `json:"area_ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	ids := make([]uuid.UUID, 0, len(req.AreaIDs))
	for _, s := range req.AreaIDs {
		aid, perr := uuid.Parse(strings.TrimSpace(s))
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "area_ids must be valid uuids")
		}
		ids = append(ids, aid)
	}
	if err := h.repo.SetUserAreas(c.UserContext(), uid, ids); err != nil {
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not set user areas")
	}
	return httpx.OK(c, fiber.Map{"user_id": uid, "area_count": len(ids)})
}
