package members

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// validStatuses bounds the status filter so a typo returns 400, not silent zero rows.
var validStatuses = map[string]bool{StatusActive: true, StatusSuspended: true, StatusAnonymized: true}

// maxSearchLen caps the search term (defends ILIKE scan cost).
const maxSearchLen = 200

const actionMemberViewDetail = "member_view_detail"

// memberAdminRoles may use the member-management console. Destructive PDPA
// actions (Phase B) gate on a stricter super_admin-only allowlist.
var memberAdminRoles = map[string]bool{"super_admin": true, "hr_manager": true}

const resumeURLTTL = 10 * time.Minute

const actionMemberViewResume = "member_view_resume"

// memberEraseRoles may run the irreversible PDPA anonymize. Stricter than
// memberAdminRoles: hr_manager can suspend/edit, only super_admin can erase.
var memberEraseRoles = map[string]bool{"super_admin": true}

// ResumeSigner produces a short-lived signed URL for a stored blob URL.
type ResumeSigner interface {
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// blobDeleter erases a member's stored resume on anonymization. Implemented by
// *blob.Client; nil-safe (a nil deleter just skips blob cleanup). The stored
// value is a bare blob key for portal uploads but a full URL for some seeded
// rows, so anonymize picks Delete vs DeleteStored by inspecting the value.
type blobDeleter interface {
	Delete(ctx context.Context, name string) error
	DeleteStored(ctx context.Context, storedURL string) error
}

// activityWriter records an audit entry (satisfied by *activity.Log).
type activityWriter interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
}

// Handler serves the HR member-management endpoints.
type Handler struct {
	repo     Repository
	activity activityWriter
	signer   ResumeSigner
	blob     blobDeleter
}

// NewHandler builds the member-admin handler. blob may be nil (blob cleanup on
// anonymize is then skipped — the DB redaction, the critical part, still runs).
func NewHandler(repo Repository, act activityWriter, signer ResumeSigner, blob blobDeleter) *Handler {
	return &Handler{repo: repo, activity: act, signer: signer, blob: blob}
}

func (h *Handler) authorized(c *fiber.Ctx) bool {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return false // no auth context → treat as unauthenticated, fail closed
	}
	return memberAdminRoles[u.Role]
}

// authorizedErase gates the super_admin-only destructive anonymize action.
func (h *Handler) authorizedErase(c *fiber.Ctx) bool {
	u, ok := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !ok || u.ID == "" {
		return false
	}
	return memberEraseRoles[u.Role]
}

// actorID parses the authenticated user's id as a UUID for the suspended_by /
// anonymized-by column. Mock/dev users may carry a non-UUID id; that maps to NULL
// (the column has no FK to users, so a missing actor id is acceptable).
func actorID(c *fiber.Ctx) *uuid.UUID {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return nil
	}
	return &id
}

// actor returns the authenticated user's email (or id) for audit records.
func actor(c *fiber.Ctx) string {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if u.Email != "" {
		return u.Email
	}
	return u.ID
}

// List handles GET /api/v1/admin/members — paginated, filtered directory.
func (h *Handler) List(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	f, ferr := parseFilter(c)
	if ferr != nil {
		return ferr
	}
	items, total, err := h.repo.List(c.UserContext(), f)
	if err != nil {
		return err
	}
	if items == nil {
		items = []Member{}
	}
	return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Member]{
		Success: true,
		Data:    items,
		Meta:    &httpx.Meta{Total: total, Page: f.Page, Limit: f.Limit},
	})
}

// Stats handles GET /api/v1/admin/members/stats — directory summary.
func (h *Handler) Stats(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	s, err := h.repo.Stats(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, s)
}

// Detail handles GET /api/v1/admin/members/:id — one member.
func (h *Handler) Detail(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	m, err := h.repo.GetByID(c.UserContext(), id)
	if errors.Is(err, ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "member not found")
	}
	if err != nil {
		return err
	}
	h.audit(c, actionMemberViewDetail, id) // PDPA: record who viewed this member's PII
	return httpx.OK(c, m)
}

// audit records an HR action against a member, logging (not failing) on error so a
// broken audit path is visible but never blocks the response.
func (h *Handler) audit(c *fiber.Ctx, action string, id uuid.UUID) {
	if h.activity == nil {
		return
	}
	if err := h.activity.Record(c.UserContext(), action, "member", id, fiber.Map{"by": actor(c)}); err != nil {
		log.Warn().Err(err).Str("member", id.String()).Str("action", action).Msg("members: audit record failed")
	}
}

// Resume handles GET /api/v1/admin/members/:id/resume — signed URL for the
// member's saved resume.
func (h *Handler) Resume(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	url, err := h.repo.GetResumeBlobURL(c.UserContext(), id)
	if errors.Is(err, ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "member not found")
	}
	if err != nil {
		return err
	}
	if url == "" {
		return fiber.NewError(fiber.StatusNotFound, "no resume on file")
	}
	signed, err := h.signer.SignedURLForStored(url, resumeURLTTL)
	if err != nil {
		return err
	}
	h.audit(c, actionMemberViewResume, id) // PDPA: record who accessed the resume
	return httpx.OK(c, fiber.Map{"url": signed, "expires_in_seconds": int(resumeURLTTL.Seconds())})
}

// parseFilter reads + validates the directory query params. Unknown status values
// and malformed dates fail fast (400) rather than silently returning everything.
func parseFilter(c *fiber.Ctx) (ListFilter, error) {
	f := ListFilter{
		Search:   c.Query("search"),
		Provider: c.Query("provider"), // unknown providers are ignored by the repo switch
		Status:   c.Query("status"),
		Page:     atoiDefault(c.Query("page"), 0),
		Limit:    atoiDefault(c.Query("limit"), 0),
	}
	if len(f.Search) > maxSearchLen {
		f.Search = f.Search[:maxSearchLen]
	}
	if f.Status != "" && !validStatuses[f.Status] {
		return ListFilter{}, fiber.NewError(fiber.StatusBadRequest, "invalid status filter")
	}
	if v := c.Query("has_resume"); v != "" {
		b := v == "true" || v == "1"
		f.HasResume = &b
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return ListFilter{}, fiber.NewError(fiber.StatusBadRequest, "invalid 'from' date (want RFC3339)")
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return ListFilter{}, fiber.NewError(fiber.StatusBadRequest, "invalid 'to' date (want RFC3339)")
		}
		f.To = &t
	}
	return f, nil
}

func atoiDefault(s string, dflt int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return dflt
}
