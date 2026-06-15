package applications

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/calendar"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const (
	minDurationMin     = 15
	maxDurationMin     = 480
	defaultDurationMin = 60
)

// candidateReader / positionReader are the narrow reads the scheduler needs
// (accept interfaces, return structs — the concrete pgx repos satisfy them).
type candidateReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*candidates.Candidate, error)
}
type positionReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*positions.Position, error)
}

// ScheduleHandler books a human interview: it sets status=interview, persists the
// appointment, and for an online interview creates a Teams meeting + calendar
// invite via the calendar provider.
type ScheduleHandler struct {
	apps       Repository
	cal        calendar.Provider
	cands      candidateReader
	pos        positionReader
	notifyDeps statusNotifyDeps
}

// NewScheduleHandler builds the interview-schedule handler.
func NewScheduleHandler(apps Repository, cal calendar.Provider, cands candidateReader, pos positionReader) *ScheduleHandler {
	return &ScheduleHandler{apps: apps, cal: cal, cands: cands, pos: pos}
}

// SetNotifier wires best-effort candidate notifications (mirrors the other handlers).
func (h *ScheduleHandler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notifyDeps = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}

// RegisterScheduleRoutes mounts the interview-schedule endpoint.
func RegisterScheduleRoutes(app *fiber.App, h *ScheduleHandler) {
	app.Post("/api/v1/applications/:id/interview-schedule", h.Schedule)
}

type scheduleReq struct {
	ScheduledAt  string `json:"scheduled_at"` // RFC3339
	DurationMin  int    `json:"duration_min"`
	Mode         string `json:"mode"` // onsite | online
	LocationText string `json:"location_text"`
}

// Schedule handles POST /api/v1/applications/:id/interview-schedule. Allowed only
// from ai_interviewed/shortlisted (the state machine); sets status=interview.
func (h *ScheduleHandler) Schedule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	if !CanTransition(app.Status, StatusInterview) {
		return fiber.NewError(fiber.StatusBadRequest, "cannot schedule an interview from "+app.Status)
	}

	var req scheduleReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if req.Mode != ModeOnsite && req.Mode != ModeOnline {
		return fiber.NewError(fiber.StatusBadRequest, "mode must be 'onsite' or 'online'")
	}
	when, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "scheduled_at must be RFC3339")
	}
	if when.Before(time.Now()) {
		return fiber.NewError(fiber.StatusBadRequest, "scheduled_at must be in the future")
	}
	dur := req.DurationMin
	if dur == 0 {
		dur = defaultDurationMin
	}
	if dur < minDurationMin || dur > maxDurationMin {
		return fiber.NewError(fiber.StatusBadRequest, "duration_min out of range")
	}

	cand, err := h.cands.FindByID(c.UserContext(), app.CandidateID)
	if err != nil {
		return err
	}
	if req.Mode == ModeOnline && strings.TrimSpace(cand.Email) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "candidate has no email for an online invite")
	}

	appt := Appointment{
		ApplicationID: id,
		ScheduledAt:   when,
		DurationMin:   dur,
		Mode:          req.Mode,
		LocationText:  strings.TrimSpace(req.LocationText),
	}

	// Online: book the Teams meeting + calendar invite (best-effort — a Graph
	// failure must not lose the schedule; the join link just stays empty).
	if req.Mode == ModeOnline {
		res, calErr := h.cal.CreateInterview(c.UserContext(), calendar.Appointment{
			Subject:       h.subject(c.UserContext(), app, cand),
			BodyHTML:      "<p>นัดสัมภาษณ์งาน CP Axtra</p>",
			Start:         when,
			End:           when.Add(time.Duration(dur) * time.Minute),
			Mode:          req.Mode,
			LocationText:  appt.LocationText,
			AttendeeEmail: cand.Email,
			AttendeeName:  cand.FullName,
		})
		if calErr != nil {
			log.Warn().Err(calErr).Str("application", id.String()).Msg("schedule: calendar invite failed (non-fatal)")
		}
		appt.OnlineJoinURL = res.JoinURL
		appt.CalendarEventID = res.EventID
	}

	saved, err := h.apps.CreateAppointment(c.UserContext(), appt)
	if err != nil {
		return err
	}
	if err := h.apps.SetStatus(c.UserContext(), id, StatusInterview); err != nil {
		return err
	}
	// Best-effort candidate notification ("interview" status message).
	h.notifyDeps.notifyStatusChange(c.UserContext(), h.apps, id, StatusInterview)
	return httpx.OK(c, saved)
}

// subject builds the calendar event title, including the position when available.
func (h *ScheduleHandler) subject(ctx context.Context, app *Application, cand *candidates.Candidate) string {
	title := "สัมภาษณ์งาน"
	if h.pos != nil {
		if p, err := h.pos.FindByID(ctx, app.PositionID); err == nil && p != nil && p.TitleTH != "" {
			title = "สัมภาษณ์งาน: " + p.TitleTH
		}
	}
	if cand != nil && cand.FullName != "" {
		title += " — " + cand.FullName
	}
	return title
}
