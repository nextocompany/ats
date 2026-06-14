package members

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// truncateRunes caps s to n runes (not bytes) so we never split a multi-byte
// codepoint — a byte-truncated Thai string is invalid UTF-8 and Postgres rejects it.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// CRM audit action names.
const (
	actionMemberNoteAdd   = "member_note_add"
	actionMemberTagAdd    = "member_tag_add"
	actionMemberTagRemove = "member_tag_remove"
)

// normalizeTag trims + lowercases a tag and bounds its length. Returns ("", false)
// when empty so the handler can 400. Lowercasing keeps the tag set canonical
// (so "Retail" and "retail" don't both exist).
func normalizeTag(raw string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(raw))
	if t == "" {
		return "", false
	}
	return truncateRunes(t, maxTagLen), true
}

// ListNotes handles GET /api/v1/admin/members/:id/notes.
func (h *Handler) ListNotes(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	notes, err := h.repo.ListNotes(c.UserContext(), id)
	if err != nil {
		return err
	}
	if notes == nil {
		notes = []Note{}
	}
	return httpx.OK(c, notes)
}

// AddNote handles POST /api/v1/admin/members/:id/notes.
func (h *Handler) AddNote(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	text := strings.TrimSpace(body.Body)
	if text == "" {
		return fiber.NewError(fiber.StatusBadRequest, "note body is required")
	}
	text = truncateRunes(text, maxNoteLen)
	note, err := h.repo.AddNote(c.UserContext(), id, actor(c), text)
	if errors.Is(err, ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "member not found")
	}
	if err != nil {
		return err
	}
	h.auditWith(c, actionMemberNoteAdd, id, nil)
	return c.Status(fiber.StatusCreated).JSON(httpx.Envelope[*Note]{Success: true, Data: note})
}

// ListTags handles GET /api/v1/admin/members/:id/tags.
func (h *Handler) ListTags(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	tags, err := h.repo.ListTags(c.UserContext(), id)
	if err != nil {
		return err
	}
	if tags == nil {
		tags = []string{}
	}
	return httpx.OK(c, tags)
}

// AddTag handles POST /api/v1/admin/members/:id/tags (body {tag}).
func (h *Handler) AddTag(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	var body struct {
		Tag string `json:"tag"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	tag, ok := normalizeTag(body.Tag)
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "tag is required")
	}
	if err := h.repo.AddTag(c.UserContext(), id, tag); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "member not found")
		}
		return err
	}
	h.auditWith(c, actionMemberTagAdd, id, fiber.Map{"tag": tag})
	return httpx.OK(c, fiber.Map{"id": id, "tag": tag})
}

// RemoveTag handles DELETE /api/v1/admin/members/:id/tags?tag=... (idempotent).
func (h *Handler) RemoveTag(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid member id")
	}
	tag, ok := normalizeTag(c.Query("tag"))
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "tag query param is required")
	}
	if err := h.repo.RemoveTag(c.UserContext(), id, tag); err != nil {
		return err
	}
	h.auditWith(c, actionMemberTagRemove, id, fiber.Map{"tag": tag})
	return httpx.OK(c, fiber.Map{"id": id, "tag": tag, "removed": true})
}
