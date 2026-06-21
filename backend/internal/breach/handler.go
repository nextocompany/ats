package breach

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const maxAffectedSubjects = 100_000_000 // sanity bound on the affected-count field

// clockSkew tolerates a small amount of caller/server clock drift when rejecting
// a future discovered_at.
const clockSkew = time.Minute

// Handler serves /api/v1/breaches, gated entirely to breach.manage. The register
// is company-wide (no RBAC data-scope clause). The DPO contact block is injected
// for the generated PDPC notification (wired from config; Phase 5.4 fills the DPO
// fields, until then the generator shows placeholders).
type Handler struct {
	repo Repository
	dpo  DPOContact
	now  func() time.Time
}

// NewHandler builds the breach handler.
func NewHandler(repo Repository, dpo DPOContact) *Handler {
	return &Handler{repo: repo, dpo: dpo, now: time.Now}
}

// RegisterRoutes mounts the breach endpoints. Static action segments are declared
// before the parameterised :id reads so Fiber does not capture them.
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/breaches")
	g.Get("/", h.List)
	g.Post("/", h.Create)
	g.Get("/:id", h.Get)
	g.Patch("/:id", h.Update)
	g.Get("/:id/notification", h.Notification)
	g.Post("/:id/notify-pdpc", h.NotifyPDPC)
	g.Post("/:id/notify-subjects", h.NotifySubjects)
	g.Post("/:id/resolve", h.Resolve)
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

// authz gates every endpoint on breach.manage and returns the caller.
func authz(c *fiber.Ctx) (middleware.DevUser, bool) {
	u, ok := user(c)
	if !ok || !rbac.Can(u.Role, rbac.PermBreachManage) {
		return middleware.DevUser{}, false
	}
	return u, true
}

func forbidden(c *fiber.Ctx) error {
	return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage the breach register")
}

func (h *Handler) List(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	f := ListFilter{Status: c.Query("status"), Severity: c.Query("severity")}
	if f.Status != "" && !validStatus(f.Status) {
		return httpx.Fail(c, fiber.StatusBadRequest, "status filter must be one of open|contained|resolved")
	}
	if f.Severity != "" && !validSeverity(f.Severity) {
		return httpx.Fail(c, fiber.StatusBadRequest, "severity filter must be one of low|medium|high|critical")
	}
	if v := c.QueryInt("page"); v > 0 {
		f.Page = v
	}
	if v := c.QueryInt("limit"); v > 0 {
		f.Limit = v
	}
	items, total, err := h.repo.List(c.UserContext(), f)
	if err != nil {
		log.Error().Err(err).Msg("breach: list failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list breaches")
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Breach]{
		Success: true,
		Data:    items,
		Meta:    &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

type createReq struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Severity         string  `json:"severity"`
	AffectedSubjects int     `json:"affected_subjects"`
	DataCategories   string  `json:"data_categories"`
	DiscoveredAt     string  `json:"discovered_at"` // RFC3339; defaults to now if empty
	OccurredAt       *string `json:"occurred_at"`   // RFC3339, optional
	HighRisk         bool    `json:"high_risk"`
	Remediation      string  `json:"remediation"`
}

func parseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(s))
}

func (h *Handler) Create(c *fiber.Ctx) error {
	u, ok := authz(c)
	if !ok {
		return forbidden(c)
	}
	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Title) == "" {
		return httpx.Fail(c, fiber.StatusBadRequest, "title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return httpx.Fail(c, fiber.StatusBadRequest, "description is required")
	}
	if req.Severity == "" {
		req.Severity = SeverityMedium
	}
	if !validSeverity(req.Severity) {
		return httpx.Fail(c, fiber.StatusBadRequest, "severity must be one of low|medium|high|critical")
	}
	if req.AffectedSubjects < 0 || req.AffectedSubjects > maxAffectedSubjects {
		return httpx.Fail(c, fiber.StatusBadRequest, "affected_subjects out of range")
	}
	discovered := h.now()
	if strings.TrimSpace(req.DiscoveredAt) != "" {
		t, perr := parseRFC3339(req.DiscoveredAt)
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "discovered_at must be an RFC3339 timestamp")
		}
		// A future discovery date would start a 72h countdown that never elapses,
		// masking the breach from the deadline surfacing. Backdating is allowed.
		if t.After(h.now().Add(clockSkew)) {
			return httpx.Fail(c, fiber.StatusBadRequest, "discovered_at cannot be in the future")
		}
		discovered = t
	}
	in := CreateInput{
		Title:            strings.TrimSpace(req.Title),
		Description:      strings.TrimSpace(req.Description),
		Severity:         req.Severity,
		AffectedSubjects: req.AffectedSubjects,
		DataCategories:   strings.TrimSpace(req.DataCategories),
		DiscoveredAt:     discovered,
		HighRisk:         req.HighRisk,
		Remediation:      strings.TrimSpace(req.Remediation),
	}
	if req.OccurredAt != nil && strings.TrimSpace(*req.OccurredAt) != "" {
		t, perr := parseRFC3339(*req.OccurredAt)
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "occurred_at must be an RFC3339 timestamp")
		}
		in.OccurredAt = &t
	}
	creator, err := uuid.Parse(u.ID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "authenticated user has no internal id")
	}
	b, err := h.repo.Create(c.UserContext(), in, creator)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.Created(c, b)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	b, err := h.repo.GetByID(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, b)
}

type updateReq struct {
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	Severity         *string `json:"severity"`
	Status           *string `json:"status"`
	AffectedSubjects *int    `json:"affected_subjects"`
	DataCategories   *string `json:"data_categories"`
	OccurredAt       *string `json:"occurred_at"`
	HighRisk         *bool   `json:"high_risk"`
	Remediation      *string `json:"remediation"`
}

func (h *Handler) Update(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	var req updateReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	// Validate everything before building the input, so no partially-validated
	// value is ever assembled.
	if req.AffectedSubjects != nil && (*req.AffectedSubjects < 0 || *req.AffectedSubjects > maxAffectedSubjects) {
		return httpx.Fail(c, fiber.StatusBadRequest, "affected_subjects out of range")
	}
	in := UpdateInput{
		AffectedSubjects: req.AffectedSubjects,
		DataCategories:   req.DataCategories,
		HighRisk:         req.HighRisk,
		Remediation:      req.Remediation,
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return httpx.Fail(c, fiber.StatusBadRequest, "title cannot be blank")
		}
		in.Title = &t
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if d == "" {
			return httpx.Fail(c, fiber.StatusBadRequest, "description cannot be blank")
		}
		in.Description = &d
	}
	if req.Severity != nil {
		if !validSeverity(*req.Severity) {
			return httpx.Fail(c, fiber.StatusBadRequest, "severity must be one of low|medium|high|critical")
		}
		in.Severity = req.Severity
	}
	if req.Status != nil {
		if !validStatus(*req.Status) {
			return httpx.Fail(c, fiber.StatusBadRequest, "status must be one of open|contained|resolved")
		}
		// Resolving must go through POST /:id/resolve so the resolver and time are
		// recorded; a bare PATCH would set status=resolved with no audit trail.
		if *req.Status == StatusResolved {
			return httpx.Fail(c, fiber.StatusBadRequest, "use POST /:id/resolve to resolve a breach")
		}
		in.Status = req.Status
	}
	if req.OccurredAt != nil && strings.TrimSpace(*req.OccurredAt) != "" {
		t, perr := parseRFC3339(*req.OccurredAt)
		if perr != nil {
			return httpx.Fail(c, fiber.StatusBadRequest, "occurred_at must be an RFC3339 timestamp")
		}
		in.OccurredAt = &t
	}
	b, err := h.repo.Update(c.UserContext(), id, in)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, b)
}

func (h *Handler) Notification(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	b, err := h.repo.GetByID(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, GenerateNotification(b, h.dpo, h.now()))
}

func (h *Handler) NotifyPDPC(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	b, err := h.repo.MarkPDPCNotified(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, b)
}

func (h *Handler) NotifySubjects(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	// The s.37(4) obligation to notify data subjects applies only to high-risk
	// breaches; stamping it on a low-risk one would pollute the audit record.
	current, err := h.repo.GetByID(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	if !current.HighRisk {
		return httpx.Fail(c, fiber.StatusConflict, "subject notification applies only to high-risk breaches")
	}
	b, err := h.repo.MarkSubjectsNotified(c.UserContext(), id)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, b)
}

func (h *Handler) Resolve(c *fiber.Ctx) error {
	u, ok := authz(c)
	if !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	resolver, err := uuid.Parse(u.ID)
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "authenticated user has no internal id")
	}
	b, err := h.repo.Resolve(c.UserContext(), id, resolver)
	if err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, b)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	if _, ok := authz(c); !ok {
		return forbidden(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid breach id")
	}
	if err := h.repo.Delete(c.UserContext(), id); err != nil {
		return h.writeErr(c, err)
	}
	return httpx.OK(c, fiber.Map{"deleted": id.String()})
}

func (h *Handler) writeErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return httpx.Fail(c, fiber.StatusNotFound, "breach not found")
	case errors.Is(err, ErrBadState):
		return httpx.Fail(c, fiber.StatusConflict, "breach is not in a state that allows this action")
	default:
		log.Error().Err(err).Msg("breach: write failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	}
}
