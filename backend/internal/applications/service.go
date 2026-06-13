package applications

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/pkg/queue"
)

// Enqueuer is the subset of asynq.Client the service needs (enables mocking).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// BlobUploader is the subset of blob.Client the service needs.
type BlobUploader interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
}

// IntakeInput is a single resume submission.
type IntakeInput struct {
	CandidateName string
	Phone         string
	Email         string
	IDCard        string
	Province      string
	SourceChannel string
	LineUserID    string     // verified LINE `sub` from the LIFF id-token (empty under mock)
	AccountID     *uuid.UUID // career-portal member account (nil for guest/legacy)
	PositionID    uuid.UUID
	FileName      string
	FileType      string // pdf | docx | image
	ContentType   string
	FileBytes     []byte
}

// IntakeResult is returned to the caller after a successful submission.
type IntakeResult struct {
	ApplicationID uuid.UUID `json:"application_id"`
	CandidateID   uuid.UUID `json:"candidate_id"`
	JobID         string    `json:"job_id"`
}

// Service coordinates intake: persist candidate + application, store the file,
// and enqueue the processing job.
type Service struct {
	candidates candidates.Repository
	apps       Repository
	blob       BlobUploader
	queue      Enqueuer
}

// NewService wires the intake service.
func NewService(c candidates.Repository, a Repository, b BlobUploader, q Enqueuer) *Service {
	return &Service{candidates: c, apps: a, blob: b, queue: q}
}

// Intake validates and processes a submission, returning the new ids.
func (s *Service) Intake(ctx context.Context, in IntakeInput) (IntakeResult, error) {
	if in.CandidateName == "" {
		return IntakeResult{}, fmt.Errorf("intake: candidate name is required")
	}
	if in.PositionID == uuid.Nil {
		return IntakeResult{}, fmt.Errorf("intake: position id is required")
	}
	if len(in.FileBytes) == 0 {
		return IntakeResult{}, fmt.Errorf("intake: resume file is required")
	}

	cand, err := s.candidates.Create(ctx, candidates.Candidate{
		FullName:      in.CandidateName,
		Phone:         in.Phone,
		Email:         in.Email,
		IDCard:        in.IDCard,
		Province:      in.Province,
		SourceChannel: in.SourceChannel,
		LineUserID:    in.LineUserID,
		AccountID:     in.AccountID,
		Status:        "available",
	})
	if err != nil {
		return IntakeResult{}, err
	}

	app, err := s.apps.Create(ctx, Application{
		CandidateID: cand.ID,
		PositionID:  in.PositionID,
		Status:      StatusPending,
		RawFileType: in.FileType,
	})
	if err != nil {
		return IntakeResult{}, err
	}

	// Key inside the "resumes" container; no "resumes/" prefix (that would
	// duplicate the container name in the blob path).
	blobName := fmt.Sprintf("%s/%s", app.ID, in.FileName)
	blobURL, err := s.blob.Upload(ctx, blobName, in.FileBytes, in.ContentType)
	if err != nil {
		_ = s.apps.SetStatus(ctx, app.ID, StatusFailed)
		return IntakeResult{}, err
	}
	if err := s.apps.SetRawFile(ctx, app.ID, blobURL); err != nil {
		return IntakeResult{}, err
	}

	task, err := queue.NewProcessApplicationTask(queue.ProcessApplicationPayload{
		ApplicationID: app.ID.String(),
		CandidateID:   cand.ID.String(),
		BlobName:      blobName,
		FileType:      in.FileType,
		PositionID:    in.PositionID.String(),
	})
	if err != nil {
		_ = s.apps.SetStatus(ctx, app.ID, StatusFailed)
		return IntakeResult{}, err
	}

	info, err := s.queue.Enqueue(task)
	if err != nil {
		_ = s.apps.SetStatus(ctx, app.ID, StatusFailed)
		return IntakeResult{}, fmt.Errorf("intake: enqueue: %w", err)
	}
	if err := s.apps.SetQueueTaskID(ctx, app.ID, info.ID); err != nil {
		log.Warn().Err(err).Str("application_id", app.ID.String()).Msg("failed to persist queue task id")
	}

	log.Info().
		Str("application_id", app.ID.String()).
		Str("candidate_id", cand.ID.String()).
		Str("job_id", info.ID).
		Msg("application intake enqueued")

	return IntakeResult{ApplicationID: app.ID, CandidateID: cand.ID, JobID: info.ID}, nil
}
