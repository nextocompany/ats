package requisitions

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const maxHeadcount = 999

// Handler serves /api/v1/requisitions, gated to requisition.manage /
// requisition.approve. RBAC scope (store/subregion/all) bounds every read.
type Handler struct {
	repo Repository
}

// NewHandler builds the requisitions handler.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// RegisterRoutes mounts the requisition endpoints. Static segments are declared
// before the parameterised ones so Fiber does not capture them as :id.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/requisitions")
	g.Get("/", h.List)
	g.Post("/", h.Create)
	g.Patch("/:id", h.Update)
	g.Post("/:id/approve", h.Approve)
	g.Post("/:id/close", h.Close)
	g.Delete("/:id", h.Delete)
}

// user returns the authenticated DevUser, or false if absent.
func user(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return middleware.DevUser{}, false
	}
	return u, true
}

func scopeFrom(u middleware.DevUser) rbac.Scope {
	return rbac.New(u.Role, u.StoreID, u.Subregion)
}

func (h *Handler) List(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionManage) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage requisitions")
	}
	f := ListFilter{Status: c.Query("status")}
	if v := c.Query("store_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.StoreID = &n
		}
	}
	if v := c.Query("position_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.PositionID = &id
		}
	}
	if v := c.QueryInt("page"); v > 0 {
		f.Page = v
	}
	if v := c.QueryInt("limit"); v > 0 {
		f.Limit = v
	}
	items, total, err := h.repo.List(c.UserContext(), f, scopeFrom(u))
	if err != nil {
		log.Error().Err(err).Msg("requisitions: list failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list requisitions")
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Requisition]{
		Success: true,
		Data:    items,
		Meta:    &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

type createReq struct {
	PositionID string `json:"position_id"`
	StoreID    int    `json:"store_id"`
	Headcount  int    `json:"headcount"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionManage) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage requisitions")
	}
	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	positionID, err := uuid.Parse(req.PositionID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "position_id must be a valid uuid")
	}
	if req.StoreID <= 0 {
		return httpx.Fail(c, fiber.StatusBadRequest, "store_id is required")
	}
	if req.Headcount < 1 || req.Headcount > maxHeadcount {
		return httpx.Fail(c, fiber.StatusBadRequest, "headcount must be between 1 and 999")
	}
	creator, err := uuid.Parse(u.ID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "authenticated user has no internal id")
	}
	req2, err := h.repo.Create(c.UserContext(), CreateInput{PositionID: positionID, StoreID: req.StoreID, Headcount: req.Headcount}, creator)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.Created(c, req2)
}

type updateReq struct {
	PositionID *string `json:"position_id"`
	StoreID    *int    `json:"store_id"`
	Headcount  *int    `json:"headcount"`
}

func (h *Handler) Update(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionManage) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage requisitions")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid requisition id")
	}
	if ok, err := h.repo.ExistsInScope(c.UserContext(), id, scopeFrom(u)); err != nil {
		log.Error().Err(err).Msg("requisitions: scope check failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	} else if !ok {
		return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	}
	var req updateReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	in := UpdateInput{StoreID: req.StoreID, Headcount: req.Headcount}
	if req.PositionID != nil {
		pid, perr := uuid.Parse(*req.PositionID)
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "position_id must be a valid uuid")
		}
		in.PositionID = &pid
	}
	if req.Headcount != nil && (*req.Headcount < 1 || *req.Headcount > maxHeadcount) {
		return httpx.Fail(c, fiber.StatusBadRequest, "headcount must be between 1 and 999")
	}
	updated, err := h.repo.Update(c.UserContext(), id, in)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, updated)
}

func (h *Handler) Approve(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionApprove) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to approve requisitions")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid requisition id")
	}
	if ok, err := h.repo.ExistsInScope(c.UserContext(), id, scopeFrom(u)); err != nil {
		log.Error().Err(err).Msg("requisitions: scope check failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	} else if !ok {
		return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	}
	approver, err := uuid.Parse(u.ID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "authenticated user has no internal id")
	}
	req, err := h.repo.Approve(c.UserContext(), id, approver)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, req)
}

func (h *Handler) Close(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionManage) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage requisitions")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid requisition id")
	}
	if ok, err := h.repo.ExistsInScope(c.UserContext(), id, scopeFrom(u)); err != nil {
		log.Error().Err(err).Msg("requisitions: scope check failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	} else if !ok {
		return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	}
	req, err := h.repo.Close(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, req)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermRequisitionManage) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage requisitions")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid requisition id")
	}
	if ok, err := h.repo.ExistsInScope(c.UserContext(), id, scopeFrom(u)); err != nil {
		log.Error().Err(err).Msg("requisitions: scope check failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	} else if !ok {
		return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	}
	if err := h.repo.Delete(c.UserContext(), id); err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, fiber.Map{"deleted": id.String()})
}

func (h *Handler) writeErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return httpx.Fail(c, fiber.StatusNotFound, "requisition not found")
	case errors.Is(err, ErrBadState):
		return httpx.Fail(c, fiber.StatusConflict, "requisition is not in a state that allows this action")
	default:
		log.Error().Err(err).Msg("requisitions: write failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	}
}
