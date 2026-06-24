package requisitions

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// resolveHiringManager maps the actor's email to a local users.id for the
// requisition's hiring-manager link. Returns nil (no link) when there is no
// resolver, no email, or the lookup fails — never blocking the create.
func (h *Handler) resolveHiringManager(ctx context.Context, email string) *uuid.UUID {
	email = strings.TrimSpace(email)
	if h.users == nil || email == "" {
		return nil
	}
	id, _, err := h.users.ResolveUser(ctx, email)
	if err != nil {
		log.Warn().Err(err).Msg("requisitions: resolve hiring manager failed; leaving link null")
		return nil
	}
	return &id
}

const maxHeadcount = 999

// Handler serves /api/v1/requisitions, gated to requisition.manage /
// requisition.approve. RBAC scope (store/subregion/all) bounds every read.
// UserResolver maps an authenticated actor's email to their local users.id, so a
// requisition can record its owning hiring manager as a resolvable user (the
// Entra OID in the token is not a users.id). Mirrors the interview scheduler's
// resolver. Optional: with no resolver the hiring-manager link is left null.
type UserResolver interface {
	ResolveUser(ctx context.Context, email string) (id uuid.UUID, fullName string, err error)
}

type Handler struct {
	repo  Repository
	users UserResolver
}

// NewHandler builds the requisitions handler.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// SetUserResolver wires the actor email → users.id resolver used to stamp the
// requisition's hiring manager. Safe to leave unset (link stays null).
func (h *Handler) SetUserResolver(u UserResolver) { h.users = u }

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
	return rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
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
	PositionID       string `json:"position_id"`
	StoreID          int    `json:"store_id"`
	Headcount        int    `json:"headcount"`
	Responsibilities string `json:"responsibilities"`
	Qualifications   string `json:"qualifications"`
	Benefits         string `json:"benefits"`
	OtherDetails     string `json:"other_details"`
	EmploymentType   string `json:"employment_type"`
	SalaryMin        *int   `json:"salary_min"`
	SalaryMax        *int   `json:"salary_max"`
	Priority         string `json:"priority"`
	OpenReason       string `json:"open_reason"`
}

// textTooLong reports whether s exceeds the per-field JD cap (rune-counted for Thai).
func textTooLong(s string) bool { return len([]rune(s)) > maxJDTextLen }

// validateSalary checks an optional salary range: each bound non-negative, and
// max >= min when both are present.
func validateSalary(min, max *int) error {
	if min != nil && *min < 0 {
		return errBadField("salary_min must be zero or greater")
	}
	if max != nil && *max < 0 {
		return errBadField("salary_max must be zero or greater")
	}
	if min != nil && max != nil && *max < *min {
		return errBadField("salary_max must be greater than or equal to salary_min")
	}
	return nil
}

// errBadField is a lightweight carrier so validation helpers can report a message
// the handler maps to a 400.
type errBadField string

func (e errBadField) Error() string { return string(e) }

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
	// Normalise + validate the optional JD / metadata fields.
	resp := strings.TrimSpace(req.Responsibilities)
	qual := strings.TrimSpace(req.Qualifications)
	benefits := strings.TrimSpace(req.Benefits)
	other := strings.TrimSpace(req.OtherDetails)
	for _, t := range []string{resp, qual, benefits, other} {
		if textTooLong(t) {
			return httpx.Fail(c, fiber.StatusBadRequest, "a job-description field exceeds the maximum length")
		}
	}
	employment := strings.TrimSpace(req.EmploymentType)
	if employment != "" && !ValidEmployment(employment) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid employment_type")
	}
	reason := strings.TrimSpace(req.OpenReason)
	if reason != "" && !ValidReason(reason) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid open_reason")
	}
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = PriorityNormal
	} else if !ValidPriority(priority) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid priority")
	}
	if err := validateSalary(req.SalaryMin, req.SalaryMax); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, err.Error())
	}
	creator, err := uuid.Parse(u.ID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "authenticated user has no internal id")
	}
	// The manager who opens the requisition is its hiring manager (drives the
	// requisition visibility scope). Resolve by email to a real users.id; an
	// unresolved actor leaves the link null rather than failing the create.
	hiringManager := h.resolveHiringManager(c.UserContext(), u.Email)
	req2, err := h.repo.Create(c.UserContext(), CreateInput{
		PositionID:       positionID,
		StoreID:          req.StoreID,
		Headcount:        req.Headcount,
		Responsibilities: resp,
		Qualifications:   qual,
		Benefits:         benefits,
		OtherDetails:     other,
		EmploymentType:   employment,
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		Priority:         priority,
		OpenReason:       reason,
		HiringManager:    hiringManager,
	}, creator)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.Created(c, req2)
}

type updateReq struct {
	PositionID       *string `json:"position_id"`
	StoreID          *int    `json:"store_id"`
	Headcount        *int    `json:"headcount"`
	Responsibilities *string `json:"responsibilities"`
	Qualifications   *string `json:"qualifications"`
	Benefits         *string `json:"benefits"`
	OtherDetails     *string `json:"other_details"`
	EmploymentType   *string `json:"employment_type"`
	SalaryMin        *int    `json:"salary_min"`
	SalaryMax        *int    `json:"salary_max"`
	Priority         *string `json:"priority"`
	OpenReason       *string `json:"open_reason"`
}

// trimmedPtr returns a pointer to the whitespace-trimmed value, preserving nil.
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
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
	in := UpdateInput{
		StoreID:          req.StoreID,
		Headcount:        req.Headcount,
		Responsibilities: trimmedPtr(req.Responsibilities),
		Qualifications:   trimmedPtr(req.Qualifications),
		Benefits:         trimmedPtr(req.Benefits),
		OtherDetails:     trimmedPtr(req.OtherDetails),
		EmploymentType:   trimmedPtr(req.EmploymentType),
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		Priority:         trimmedPtr(req.Priority),
		OpenReason:       trimmedPtr(req.OpenReason),
	}
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
	for _, t := range []*string{in.Responsibilities, in.Qualifications, in.Benefits, in.OtherDetails} {
		if t != nil && textTooLong(*t) {
			return httpx.Fail(c, fiber.StatusBadRequest, "a job-description field exceeds the maximum length")
		}
	}
	if in.EmploymentType != nil && *in.EmploymentType != "" && !ValidEmployment(*in.EmploymentType) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid employment_type")
	}
	if in.OpenReason != nil && *in.OpenReason != "" && !ValidReason(*in.OpenReason) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid open_reason")
	}
	if in.Priority != nil && !ValidPriority(*in.Priority) {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid priority")
	}
	if err := validateSalary(in.SalaryMin, in.SalaryMax); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, err.Error())
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
