package members

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// maxBulkIDs caps a single bulk request (mirrors internal/applications).
const maxBulkIDs = 100

const actionMemberBulk = "member_bulk"

// Bulk handles POST /api/v1/admin/members/bulk — apply one action to many members.
// Supported actions: "tag" (value=tag), "suspend", "reactivate". Each id is applied
// independently (a failure on one is counted, not fatal); irreversible erasure is
// deliberately NOT a bulk action (single, super_admin-only, confirmed).
func (h *Handler) Bulk(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	var req struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
		Value  string   `json:"value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.IDs) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "ids is required")
	}
	if len(req.IDs) > maxBulkIDs {
		return fiber.NewError(fiber.StatusBadRequest, "too many ids (max 100)")
	}

	// Resolve the per-id operation up front so an unsupported action fails fast.
	apply, label, aerr := h.bulkOp(c, req.Action, req.Value)
	if aerr != nil {
		return aerr
	}

	var updated, failed, skipped int
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			failed++
			continue
		}
		if err := apply(id); err != nil {
			// Anonymized accounts are terminal — count them separately from real
			// failures so the response/audit reflects that they were intentionally
			// not modified (a suspend/reactivate can't apply to an erased member).
			if errors.Is(err, ErrAnonymized) {
				skipped++
			} else {
				failed++
			}
			continue
		}
		h.auditWith(c, actionMemberBulk, id, fiber.Map{"action": req.Action, "value": label})
		updated++
	}
	return httpx.OK(c, fiber.Map{"updated": updated, "failed": failed, "skipped": skipped, "action": req.Action})
}

// bulkOp returns the per-id function for a bulk action, plus a label for the audit
// record. Returns a 400 error for an unsupported action or missing value.
func (h *Handler) bulkOp(c *fiber.Ctx, action, value string) (func(uuid.UUID) error, string, error) {
	ctx := c.UserContext()
	switch action {
	case "tag":
		tag, ok := normalizeTag(value)
		if !ok {
			return nil, "", fiber.NewError(fiber.StatusBadRequest, "tag value is required")
		}
		return func(id uuid.UUID) error { return h.repo.AddTag(ctx, id, tag) }, tag, nil
	case "suspend":
		by := actorID(c)
		return func(id uuid.UUID) error { return h.repo.SetStatus(ctx, id, StatusSuspended, by) }, StatusSuspended, nil
	case "reactivate":
		by := actorID(c)
		return func(id uuid.UUID) error { return h.repo.SetStatus(ctx, id, StatusActive, by) }, StatusActive, nil
	default:
		return nil, "", fiber.NewError(fiber.StatusBadRequest, "unsupported action")
	}
}
