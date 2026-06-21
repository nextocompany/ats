package members

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// Member-management audit action names (recorded against entity_type "member").
const (
	actionMemberSuspend     = "member_suspend"
	actionMemberReactivate  = "member_reactivate"
	actionMemberForceLogout = "member_force_logout"
	actionMemberProfileEdit = "member_profile_edit"
	actionMemberAnonymize   = "member_anonymize"
)

// settableStatuses are the only statuses the status route may set. 'anonymized' is
// excluded — erasure goes through the dedicated super_admin-only Anonymize route.
var settableStatuses = map[string]bool{StatusActive: true, StatusSuspended: true}

// lifecycleErr maps a repo sentinel to its HTTP status; returns nil otherwise.
func lifecycleErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "member not found")
	case errors.Is(err, ErrAnonymized):
		return fiber.NewError(fiber.StatusConflict, "member is anonymized and cannot be modified")
	case errors.Is(err, ErrEmailTaken):
		return fiber.NewError(fiber.StatusConflict, "email already in use by another member")
	default:
		return nil
	}
}

// auditWith records an HR action with extra context (merged onto {by: actor}).
func (h *Handler) auditWith(c *fiber.Ctx, action string, id uuid.UUID, extra fiber.Map) {
	if h.activity == nil {
		return
	}
	val := fiber.Map{"by": actor(c)}
	for k, v := range extra {
		val[k] = v
	}
	if err := h.activity.Record(c.UserContext(), action, "member", id, val); err != nil {
		log.Warn().Err(err).Str("member", id.String()).Str("action", action).Msg("members: audit record failed")
	}
}

// SetStatus handles PATCH /api/v1/admin/members/:id/status — suspend/reactivate.
// Suspending force-logs-out the member; reactivating restores login.
func (h *Handler) SetStatus(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if !settableStatuses[body.Status] {
		return fiber.NewError(fiber.StatusBadRequest, "status must be 'active' or 'suspended'")
	}
	if err := h.repo.SetStatus(c.UserContext(), id, body.Status, actorID(c)); err != nil {
		if mapped := lifecycleErr(err); mapped != nil {
			return mapped
		}
		return err
	}
	action := actionMemberReactivate
	if body.Status == StatusSuspended {
		action = actionMemberSuspend
	}
	h.auditWith(c, action, id, fiber.Map{"status": body.Status})
	return httpx.OK(c, fiber.Map{"id": id, "status": body.Status})
}

// ForceLogout handles POST /api/v1/admin/members/:id/force-logout — revoke every
// session for the member without changing status.
func (h *Handler) ForceLogout(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	if err := h.repo.ForceLogout(c.UserContext(), id); err != nil {
		if mapped := lifecycleErr(err); mapped != nil {
			return mapped
		}
		return err
	}
	h.auditWith(c, actionMemberForceLogout, id, nil)
	return httpx.OK(c, fiber.Map{"id": id, "logged_out": true})
}

// UpdateProfile handles PATCH /api/v1/admin/members/:id — sparse admin edit.
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	var body struct {
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
		Province string `json:"province"`
		Email    string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	p := ProfileUpdate{
		FullName: strings.TrimSpace(body.FullName),
		Phone:    strings.TrimSpace(body.Phone),
		Province: strings.TrimSpace(body.Province),
		Email:    strings.TrimSpace(strings.ToLower(body.Email)),
	}
	if p.IsEmpty() {
		return fiber.NewError(fiber.StatusBadRequest, "no fields to update")
	}
	if err := h.repo.UpdateProfile(c.UserContext(), id, p); err != nil {
		if mapped := lifecycleErr(err); mapped != nil {
			return mapped
		}
		return err
	}
	m, err := h.repo.GetByID(c.UserContext(), id)
	if errors.Is(err, ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "member not found")
	}
	if err != nil {
		return err
	}
	h.auditWith(c, actionMemberProfileEdit, id, nil)
	return httpx.OK(c, m)
}

// Anonymize handles POST /api/v1/admin/members/:id/anonymize — irreversible PDPA
// erasure. super_admin only. Redacts the account in a transaction, then best-effort
// deletes the resume blob (the DB redaction is the contractual part; an orphaned
// blob is logged, not fatal).
func (h *Handler) Anonymize(c *fiber.Ctx) error {
	if !h.authorizedErase(c) {
		return fiber.NewError(fiber.StatusForbidden, "PDPA erasure requires super_admin")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	resumeURL, err := h.repo.Anonymize(c.UserContext(), id)
	if err != nil {
		if mapped := lifecycleErr(err); mapped != nil {
			return mapped
		}
		return err
	}
	h.deleteResumeBlob(c, id, resumeURL)
	// Cascade: fully erase the applicant data (applications, interviews, onboarding
	// scans, offers, letters, search index) behind this account. The account PII is
	// already redacted; a cascade failure is surfaced (cascade_complete:false) and
	// audited so an operator can retry - re-running anonymize is idempotent.
	cascadeComplete := true
	if h.eraser != nil {
		if err := h.eraser.EraseLinkedCandidates(c.UserContext(), id); err != nil {
			cascadeComplete = false
			log.Warn().Err(err).Str("member", id.String()).Msg("members: candidate erasure cascade failed")
		}
	}
	h.auditWith(c, actionMemberAnonymize, id, fiber.Map{"cascade_complete": cascadeComplete})
	return httpx.OK(c, fiber.Map{"id": id, "status": StatusAnonymized, "cascade_complete": cascadeComplete})
}

// deleteResumeBlob removes the member's stored resume after a successful
// anonymize. Best-effort: failures are logged, never surfaced (the PII row is
// already redacted, so a leftover private blob with no DB pointer is low-risk).
func (h *Handler) deleteResumeBlob(c *fiber.Ctx, id uuid.UUID, resumeURL string) {
	if h.blob == nil || resumeURL == "" {
		return
	}
	var derr error
	if strings.Contains(resumeURL, "://") {
		derr = h.blob.DeleteStored(c.UserContext(), resumeURL) // full URL → derive key
	} else {
		derr = h.blob.Delete(c.UserContext(), resumeURL) // stored value is the blob key
	}
	if derr != nil {
		log.Warn().Err(derr).Str("member", id.String()).Msg("members: anonymize resume blob delete failed (orphaned blob)")
	}
}
