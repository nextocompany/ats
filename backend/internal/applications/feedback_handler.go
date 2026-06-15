package applications

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// feedbackRecordRoles may record interview feedback. Reads are open to anyone with
// RBAC visibility of the application; only the hiring decision-makers may write.
// sgm (store GM) is the closest role to the "line manager" who runs the interview.
var feedbackRecordRoles = map[string]bool{
	"super_admin": true,
	"hr_manager":  true,
	"sgm":         true,
}

// feedbackStore is the narrow slice of the repository this handler needs (accept
// interfaces, return structs). The concrete pgRepository satisfies it.
type feedbackStore interface {
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	FindAppointment(ctx context.Context, applicationID uuid.UUID) (*Appointment, error)
	CreateFeedback(ctx context.Context, f InterviewFeedback) (InterviewFeedback, error)
	ListFeedback(ctx context.Context, applicationID uuid.UUID) ([]InterviewFeedback, error)
}

// FeedbackHandler records and lists structured interview feedback.
type FeedbackHandler struct {
	apps feedbackStore
}

// NewFeedbackHandler builds the interview-feedback handler.
func NewFeedbackHandler(apps feedbackStore) *FeedbackHandler {
	return &FeedbackHandler{apps: apps}
}

// RegisterFeedbackRoutes mounts the interview-feedback endpoints.
func RegisterFeedbackRoutes(app *fiber.App, h *FeedbackHandler) {
	app.Get("/api/v1/applications/:id/interview-feedback", h.List)
	app.Post("/api/v1/applications/:id/interview-feedback", h.Create)
}

type feedbackReq struct {
	OverallRating  int                   `json:"overall_rating"`
	Recommendation string                `json:"recommendation"`
	Competencies   InterviewCompetencies `json:"competencies"`
	Strengths      string                `json:"strengths"`
	Concerns       string                `json:"concerns"`
	Notes          string                `json:"notes"`
}

// List handles GET /api/v1/applications/:id/interview-feedback. Visible to any user
// whose RBAC scope includes the application (same gate as viewing it).
func (h *FeedbackHandler) List(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	list, err := h.apps.ListFeedback(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, list)
}

// Create handles POST /api/v1/applications/:id/interview-feedback. Restricted to
// the decision-maker roles; allowed only while the application is in the human
// interview stage (interview/interviewed). Recorded independently of the
// "mark interviewed" transition, so a panel can log notes any number of times.
func (h *FeedbackHandler) Create(c *fiber.Ctx) error {
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
	if !feedbackRecordRoles[u.Role] {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to record interview feedback")
	}

	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	if !CanRecordFeedback(app.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "interview feedback can only be recorded during the interview stage")
	}

	var req feedbackReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}

	fb := InterviewFeedback{
		ApplicationID:  id,
		OverallRating:  req.OverallRating,
		Recommendation: strings.TrimSpace(req.Recommendation),
		Competencies:   req.Competencies,
		Strengths:      strings.TrimSpace(req.Strengths),
		Concerns:       strings.TrimSpace(req.Concerns),
		Notes:          strings.TrimSpace(req.Notes),
	}
	if err := ValidateFeedback(fb); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	// Stamp the authenticated interviewer (best-effort: a non-UUID dev id is left
	// null rather than failing the write).
	if uid, perr := uuid.Parse(u.ID); perr == nil {
		fb.InterviewerID = &uid
	}
	// Link to the scheduled appointment when one exists (for later reporting).
	if appt, aerr := h.apps.FindAppointment(c.UserContext(), id); aerr == nil && appt != nil {
		fb.AppointmentID = &appt.ID
	}

	saved, err := h.apps.CreateFeedback(c.UserContext(), fb)
	if err != nil {
		return err
	}
	// The saved row has no joined name (we just inserted it); surface the actor.
	if saved.InterviewerName == "" {
		saved.InterviewerName = u.Email
	}
	return httpx.Created(c, saved)
}
