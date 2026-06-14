package members

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// exportRowCap bounds a CSV export so a pathological filter can't materialise the
// whole table in memory. If the cap is hit we log it (no silent truncation).
const exportRowCap = 50000

const actionMemberExport = "member_export"

var exportHeader = []string{"name", "email", "phone", "province", "providers", "status", "applications", "joined"}

// Export handles GET /api/v1/admin/members/export.csv — a synchronous CSV download
// of the directory honouring the same filters as the list (search/provider/status/
// tag/has_resume/date). Unlike the reports export (blob + email), this streams the
// file straight back as an attachment.
func (h *Handler) Export(c *fiber.Ctx) error {
	if !h.authorized(c) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role for member management")
	}
	f, ferr := parseFilter(c)
	if ferr != nil {
		return ferr
	}
	rows, err := h.repo.ListForExport(c.UserContext(), f, exportRowCap)
	if err != nil {
		return err
	}
	if len(rows) == exportRowCap {
		log.Warn().Int("cap", exportRowCap).Msg("members: CSV export hit the row cap — output truncated")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(exportHeader); err != nil {
		return err
	}
	for _, m := range rows {
		rec := []string{
			csvSafe(m.FullName),
			csvSafe(m.Email),
			csvSafe(m.Phone),
			csvSafe(m.Province),
			providerList(m), // fixed vocabulary (line|google|email)
			m.Status,        // enum
			strconv.Itoa(m.AppsCount),
			m.CreatedAt.Format("2006-01-02"),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	h.auditExport(c, len(rows))
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="members.csv"`)
	return c.Send(buf.Bytes())
}

// csvSafe defuses CSV formula injection: a cell beginning with =, +, -, @ (or a
// control char) is interpreted as a formula by Excel/Sheets. Thai phone numbers
// legitimately start with '+', so user-controlled fields are prefixed with a
// single quote to force text. encoding/csv already handles comma/quote escaping.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// providerList renders a member's linked providers as a pipe-joined cell.
func providerList(m Member) string {
	var ps []string
	if m.LineLinked {
		ps = append(ps, "line")
	}
	if m.GoogleLinked {
		ps = append(ps, "google")
	}
	if m.EmailLinked {
		ps = append(ps, "email")
	}
	return strings.Join(ps, "|")
}

// auditExport records who exported and how many rows (entity_type "member", a nil
// entity id since an export spans many members).
func (h *Handler) auditExport(c *fiber.Ctx, count int) {
	if h.activity == nil {
		return
	}
	if err := h.activity.Record(c.UserContext(), actionMemberExport, "member", uuid.Nil, fiber.Map{"by": actor(c), "rows": count}); err != nil {
		log.Warn().Err(err).Msg("members: export audit record failed")
	}
}
