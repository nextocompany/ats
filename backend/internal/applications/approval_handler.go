package applications

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// approvalStore is the narrow slice of the repository the approval handler needs.
type approvalStore interface {
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	CreateApprovalRequest(ctx context.Context, applicationID, createdBy uuid.UUID, slaHours int) (ApprovalRequest, error)
	GetApprovalRequest(ctx context.Context, applicationID uuid.UUID) (*ApprovalRequest, error)
	GetApprovalRequestByID(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error)
	DecideApproval(ctx context.Context, args approvalDecideArgs) (ApprovalRequest, error)
	ListPendingApprovals(ctx context.Context, scope rbac.Scope) ([]ApprovalQueueItem, error)
}

// ApprovalHandler drives the four-level hiring approval chain.
type ApprovalHandler struct {
	lockGuard
	apps     approvalStore
	slaHours int
	hrNotify approvalNotify
}

// approvalNotify bundles the optional best-effort HR notification deps.
type approvalNotify struct {
	notifier         notify.Notifier
	hr               HRDirectory
	dashboardBaseURL string
	teamsEnabled     bool
}

// NewApprovalHandler builds the approval handler. slaHours seeds each active step's
// due_at (used by the SLA escalation sweep).
func NewApprovalHandler(apps approvalStore, slaHours int) *ApprovalHandler {
	return &ApprovalHandler{apps: apps, slaHours: slaHours}
}

// SetNotifier wires best-effort HR notifications. Unset → no notifications.
func (h *ApprovalHandler) SetNotifier(n notify.Notifier, hr HRDirectory, dashboardBaseURL string, teamsEnabled bool) {
	h.hrNotify = approvalNotify{notifier: n, hr: hr, dashboardBaseURL: dashboardBaseURL, teamsEnabled: teamsEnabled}
}

// RegisterApprovalRoutes mounts the approval endpoints. The static collection route
// (/approvals) is registered before any future parameterised /approvals/:id.
func RegisterApprovalRoutes(app *fiber.App, h *ApprovalHandler) {
	app.Get("/api/v1/approvals", h.ListQueue)
	app.Post("/api/v1/applications/:id/approval-request", h.Create)
	app.Get("/api/v1/applications/:id/approval-request", h.GetForApplication)
	app.Post("/api/v1/approval-requests/:id/decide", h.Decide)
}

// Create opens an approval request from the interviewed stage. The caller is the
// Staff-level (level 1) sign-off.
func (h *ApprovalHandler) Create(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !canSubmitApproval(u.Role) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to submit a hiring approval")
	}

	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	// Submitting for approval is an operate action by the candidate's owner — gate
	// it on the lock. (The Decide endpoint is intentionally NOT gated: approvers /
	// hiring managers are read-only and never hold a processing lock.)
	if ok, lerr := h.guardLock(c, app.CandidateID); !ok {
		return lerr
	}
	if !CanRequestApproval(app.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "a hiring approval can only be requested from the interviewed stage")
	}

	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid actor identity")
	}
	req, err := h.apps.CreateApprovalRequest(c.UserContext(), id, uid, h.slaHours)
	if errors.Is(err, ErrApprovalConflict) {
		return fiber.NewError(fiber.StatusConflict, "an approval request already exists or the candidate is no longer at the interviewed stage")
	}
	if err != nil {
		return err
	}
	h.notifyActiveLevel(c.UserContext(), app, req)
	return httpx.Created(c, req)
}

// Decide records an approve/reject on the active level. Approving the final level
// advances the application to offer; rejecting (with a mandatory reason) rejects it.
func (h *ApprovalHandler) Decide(c *fiber.Ctx) error {
	reqID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid approval request id")
	}
	req, err := h.apps.GetApprovalRequestByID(c.UserContext(), reqID)
	if err != nil {
		return err
	}
	if req == nil {
		return fiber.NewError(fiber.StatusNotFound, "approval request not found")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), req.ApplicationID, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "approval request not found")
	}
	if req.Status != ApprovalPending {
		return fiber.NewError(fiber.StatusConflict, "approval request already decided")
	}
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	level := req.CurrentLevel
	if !canDecideLevel(u.Role, level) {
		return fiber.NewError(fiber.StatusForbidden, "not your approval level")
	}

	var in ApprovalDecisionInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	decision := strings.TrimSpace(in.Decision)
	if !validDecision(decision) {
		return fiber.NewError(fiber.StatusBadRequest, "decision must be approve or reject")
	}
	reason := strings.TrimSpace(in.Reason)
	if decision == DecisionReject && reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "a rejection reason is required")
	}
	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid actor identity")
	}

	updated, err := h.apps.DecideApproval(c.UserContext(), approvalDecideArgs{
		RequestID:  reqID,
		Level:      level,
		Approve:    decision == DecisionApprove,
		ApproverID: uid,
		Comment:    strings.TrimSpace(in.Comment),
		Reason:     reason,
		SLAHours:   h.slaHours,
	})
	if errors.Is(err, ErrApprovalConflict) {
		return fiber.NewError(fiber.StatusConflict, "this approval level was already decided")
	}
	if err != nil {
		return err
	}
	h.notifyAfterDecision(c.UserContext(), updated)
	return httpx.OK(c, updated)
}

// GetForApplication returns the approval request for an application, or null when
// none has been opened (the frontend treats null as "not yet submitted").
func (h *ApprovalHandler) GetForApplication(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	req, err := h.apps.GetApprovalRequest(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, req)
}

// ListQueue returns the in-flight approvals awaiting the caller's decision level
// (within their RBAC scope). super_admin sees every active level.
func (h *ApprovalHandler) ListQueue(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	items, err := h.apps.ListPendingApprovals(c.UserContext(), scopeFrom(c))
	if err != nil {
		return err
	}
	mine := make([]ApprovalQueueItem, 0, len(items))
	for _, it := range items {
		if canDecideLevel(u.Role, it.ActiveLevel) {
			mine = append(mine, it)
		}
	}
	return httpx.OK(c, mine)
}

// notifyActiveLevel best-effort emails the approvers responsible for the request's
// current active level.
func (h *ApprovalHandler) notifyActiveLevel(ctx context.Context, app *Application, req ApprovalRequest) {
	d := h.hrNotify
	if d.notifier == nil || d.hr == nil {
		return
	}
	role := roleForLevel(req.CurrentLevel)
	if role == "" {
		return
	}
	emails, err := d.hr.EmailsForRoleStore(ctx, role, app.AssignedStoreID)
	if err != nil {
		log.Warn().Err(err).Str("role", role).Msg("approval notify: resolve approvers failed (non-fatal)")
		return
	}
	if len(emails) == 0 && !d.teamsEnabled {
		return
	}
	dashURL := d.dashboardBaseURL + "/approvals"
	msgs := notify.ApprovalPendingHR(emails, d.teamsEnabled, app.CandidateName, levelLabel(req.CurrentLevel), dashURL)
	dispatchHR(ctx, d.notifier, msgs)
}

// notifyAfterDecision routes the right notification after a decision: the next
// level's approvers when the chain advances, or store HR on a terminal outcome.
func (h *ApprovalHandler) notifyAfterDecision(ctx context.Context, req ApprovalRequest) {
	d := h.hrNotify
	if d.notifier == nil || d.hr == nil {
		return
	}
	app, err := h.apps.FindByID(ctx, req.ApplicationID)
	if err != nil {
		log.Warn().Err(err).Str("application", req.ApplicationID.String()).Msg("approval notify: load application failed (non-fatal)")
		return
	}
	switch req.Status {
	case ApprovalPending:
		h.notifyActiveLevel(ctx, app, req)
	case ApprovalApproved, ApprovalRejected:
		emails, err := d.hr.EmailsForStore(ctx, app.AssignedStoreID)
		if err != nil {
			log.Warn().Err(err).Str("application", app.ID.String()).Msg("approval notify: resolve HR emails failed (non-fatal)")
			return
		}
		if len(emails) == 0 && !d.teamsEnabled {
			return
		}
		dashURL := d.dashboardBaseURL + "/applications/" + app.ID.String()
		msgs := notify.ApprovalDecidedHR(emails, d.teamsEnabled, app.CandidateName, req.Status == ApprovalApproved, dashURL)
		dispatchHR(ctx, d.notifier, msgs)
	}
}
