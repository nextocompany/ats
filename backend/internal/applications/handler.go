package applications

import (
	"context"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/httpx"
)

const (
	maxResumeBytes = 10 * 1024 * 1024 // 10MB (NFR §16)
	defaultQueue   = "default"
)

// JobInspector is the subset of asynq.Inspector used for status polling.
type JobInspector interface {
	GetTaskInfo(queue, id string) (*asynq.TaskInfo, error)
}

// HiredSyncer pushes a hired application to PeopleSoft. Injected as an interface
// so this package does not import peoplesoft (which imports this package).
type HiredSyncer interface {
	SyncHired(ctx context.Context, applicationID uuid.UUID) error
}

// Handler serves the intake and status endpoints.
type Handler struct {
	svc        *Service
	apps       Repository
	inspector  JobInspector
	hired      HiredSyncer
	notifyDeps statusNotifyDeps
	activity   activity.Writer // optional; records status changes onto the candidate journey
}

// SetNotifier wires best-effort candidate notifications on status changes. Unset
// → no notifications (CI/local/tests). Mirrors DashboardHandler.SetIndexer.
func (h *Handler) SetNotifier(n notify.Notifier, cands candidates.Repository, portalBaseURL string) {
	h.notifyDeps = statusNotifyDeps{notifier: n, cands: cands, portalBaseURL: portalBaseURL}
}

// SetActivity wires the audit/journey writer so single status changes are recorded
// onto the candidate timeline (mirrors the bulk handler). Unset → not recorded.
func (h *Handler) SetActivity(w activity.Writer) { h.activity = w }

// NewHandler builds the applications handler.
func NewHandler(svc *Service, apps Repository, inspector JobInspector, hired HiredSyncer) *Handler {
	return &Handler{svc: svc, apps: apps, inspector: inspector, hired: hired}
}

// contentTypeToFileType maps an allowlisted content type to our file_type tag.
var contentTypeToFileType = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

// Intake handles POST /api/v1/applications (multipart).
func (h *Handler) Intake(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("resume")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "resume file is required")
	}
	if fileHeader.Size > maxResumeBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "resume exceeds 10MB limit")
	}

	contentType := fileHeader.Header.Get("Content-Type")
	fileType, ok := contentTypeToFileType[contentType]
	if !ok {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "unsupported file type (allowed: pdf, docx, jpeg, png)")
	}

	positionID, err := uuid.Parse(c.FormValue("position_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}

	name := c.FormValue("full_name")
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "full_name is required")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not read uploaded file")
	}

	result, err := h.svc.Intake(c.UserContext(), IntakeInput{
		CandidateName: name,
		Phone:         c.FormValue("phone"),
		Email:         c.FormValue("email"),
		IDCard:        c.FormValue("id_card"),
		Province:      c.FormValue("province"),
		SourceChannel: c.FormValue("source_channel"),
		PositionID:    positionID,
		FileName:      fileHeader.Filename,
		FileType:      fileType,
		ContentType:   contentType,
		FileBytes:     data,
	})
	if err != nil {
		return err // central error handler logs + masks 5xx
	}
	return httpx.Created(c, result)
}

// Get handles GET /api/v1/applications/:id.
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	return httpx.OK(c, app)
}

// JobStatus handles GET /api/v1/ai/jobs/:job_id.
func (h *Handler) JobStatus(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	info, err := h.inspector.GetTaskInfo(defaultQueue, jobID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "job not found")
	}
	return httpx.OK(c, fiber.Map{
		"job_id":  info.ID,
		"state":   info.State.String(),
		"queue":   info.Queue,
		"retried": info.Retried,
	})
}

type updateStatusReq struct {
	Status string `json:"status"`
	Reason string `json:"reason"` // required when status=rejected
}

// UpdateStatus handles PATCH /api/v1/applications/:id/status. It enforces the
// candidate state machine (transitions.go): the move is only allowed if
// CanTransition(current, target). "interview" is reachable only via the schedule
// endpoint (it needs a date/time + mode); "rejected" requires a reason (stored,
// never sent to the candidate); "offer" is the hire action (entering the Offer
// Package process — PeopleSoft sync is deferred to a future offer-accepted step).
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	var req updateStatusReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	// Per-record authorization: a store/subregion-scoped user may only act on
	// applications within their visibility (consistent with the list scoping).
	if ok, serr := h.apps.ExistsInScope(c.UserContext(), id, scopeFrom(c)); serr != nil {
		return serr
	} else if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	// "interview" carries a schedule — force the dedicated endpoint.
	if RequiresSchedule(req.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "use the interview-schedule endpoint to set an interview")
	}
	app, err := h.apps.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	if !CanTransition(app.Status, req.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "transition not allowed from "+app.Status)
	}

	// Reject: mandatory reason, stored internally; the candidate is NOT notified.
	if req.Status == StatusRejected {
		if strings.TrimSpace(req.Reason) == "" {
			return fiber.NewError(fiber.StatusBadRequest, "a rejection reason is required")
		}
		if err := h.apps.SetRejection(c.UserContext(), id, strings.TrimSpace(req.Reason)); err != nil {
			return err
		}
		h.recordStatusChange(c.UserContext(), id, app.Status, StatusRejected)
		return httpx.OK(c, fiber.Map{"id": id, "status": StatusRejected})
	}

	if err := h.apps.SetStatus(c.UserContext(), id, req.Status); err != nil {
		return err
	}
	h.recordStatusChange(c.UserContext(), id, app.Status, req.Status)
	// Best-effort candidate notification (only shortlisted produces a message today;
	// offer/interviewed have none defined — the seam is a no-op for those).
	h.notifyDeps.notifyStatusChange(c.UserContext(), h.apps, id, req.Status)
	return httpx.OK(c, fiber.Map{"id": id, "status": req.Status})
}

// recordStatusChange best-effort logs a status transition onto the candidate
// journey. No-op when the activity writer is unset; a failure is swallowed (the
// status change already succeeded — the audit entry is non-critical).
func (h *Handler) recordStatusChange(ctx context.Context, id uuid.UUID, from, to string) {
	if h.activity == nil {
		return
	}
	_ = h.activity.Record(ctx, activity.ActionStatusChange, "application", id, fiber.Map{"from": from, "to": to})
}
