package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// TypeProcessApplication is the asynq task type for the OCR + parse pipeline.
const TypeProcessApplication = "application:process"

// TypeReengageVacancy is the asynq task type for re-engaging talent-pool / prior
// candidates when a vacancy opens (Sprint 5a).
const TypeReengageVacancy = "vacancy:reengage"

// TypeExportReport is the asynq task type for generating + delivering a recurring
// or on-demand report export (Sprint 5b).
const TypeExportReport = "report:export"

const (
	taskMaxRetry  = 3
	taskTimeout   = 90 * time.Second
	taskRetention = 24 * time.Hour // keep completed tasks queryable for status polling
)

// ProcessApplicationPayload is the job body enqueued on intake.
type ProcessApplicationPayload struct {
	ApplicationID string `json:"application_id"`
	CandidateID   string `json:"candidate_id"`
	BlobName      string `json:"blob_name"`
	FileType      string `json:"file_type"`
	PositionID    string `json:"position_id"`
}

// NewProcessApplicationTask builds the task with retry/timeout policy.
func NewProcessApplicationTask(p ProcessApplicationPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal payload: %w", err)
	}
	return asynq.NewTask(
		TypeProcessApplication,
		body,
		asynq.MaxRetry(taskMaxRetry),
		asynq.Timeout(taskTimeout),
		asynq.Retention(taskRetention),
	), nil
}

// ParseProcessApplicationPayload decodes a task body.
func ParseProcessApplicationPayload(body []byte) (ProcessApplicationPayload, error) {
	var p ProcessApplicationPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("queue: unmarshal payload: %w", err)
	}
	return p, nil
}

// ReengageVacancyPayload is the job body enqueued when a vacancy opens.
type ReengageVacancyPayload struct {
	PositionID string `json:"position_id"`
}

// NewReengageVacancyTask builds the re-engagement task with retry/timeout policy.
func NewReengageVacancyTask(p ReengageVacancyPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal payload: %w", err)
	}
	return asynq.NewTask(
		TypeReengageVacancy,
		body,
		asynq.MaxRetry(taskMaxRetry),
		asynq.Timeout(taskTimeout),
		asynq.Retention(taskRetention),
	), nil
}

// ParseReengageVacancyPayload decodes a re-engagement task body.
func ParseReengageVacancyPayload(body []byte) (ReengageVacancyPayload, error) {
	var p ReengageVacancyPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("queue: unmarshal payload: %w", err)
	}
	return p, nil
}

// ExportReportPayload is the job body for a report export.
type ExportReportPayload struct {
	Kind   string `json:"kind"`   // "weekly" | "ondemand"
	Period string `json:"period"` // e.g. "2026-W22" or an RFC3339 date
}

// exportUniqueTTL dedups identical export enqueues (same kind+period payload)
// within a short window, so a brief multi-scheduler overlap during a rolling
// deploy cannot double-enqueue. Distinct periods never collide.
const exportUniqueTTL = 1 * time.Hour

// NewExportReportTask builds the report-export task with retry/timeout policy and
// payload-scoped uniqueness.
func NewExportReportTask(p ExportReportPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal payload: %w", err)
	}
	return asynq.NewTask(
		TypeExportReport,
		body,
		asynq.MaxRetry(taskMaxRetry),
		asynq.Timeout(taskTimeout),
		asynq.Retention(taskRetention),
		asynq.Unique(exportUniqueTTL),
	), nil
}

// ParseExportReportPayload decodes a report-export task body.
func ParseExportReportPayload(body []byte) (ExportReportPayload, error) {
	var p ExportReportPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("queue: unmarshal payload: %w", err)
	}
	return p, nil
}

// TypeRetentionSweep is the asynq task type for the daily PDPA retention sweep
// (Sprint 7): anonymize candidates whose retention window has elapsed.
const TypeRetentionSweep = "retention:sweep"

// RetentionSweepPayload is the job body for a retention sweep. The field is
// optional; the handler derives the default batch from config when zero.
type RetentionSweepPayload struct {
	Batch int `json:"batch"` // max candidates per run; 0 → config default
}

// retentionUniqueTTL dedups overlapping sweep enqueues during a rolling deploy,
// mirroring exportUniqueTTL.
const retentionUniqueTTL = 1 * time.Hour

// NewRetentionSweepTask builds the retention-sweep task with retry/timeout policy
// and enqueue-scoped uniqueness.
func NewRetentionSweepTask(p RetentionSweepPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal payload: %w", err)
	}
	return asynq.NewTask(
		TypeRetentionSweep,
		body,
		asynq.MaxRetry(taskMaxRetry),
		asynq.Timeout(taskTimeout),
		asynq.Retention(taskRetention),
		asynq.Unique(retentionUniqueTTL),
	), nil
}

// ParseRetentionSweepPayload decodes a retention-sweep task body.
func ParseRetentionSweepPayload(body []byte) (RetentionSweepPayload, error) {
	var p RetentionSweepPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("queue: unmarshal payload: %w", err)
	}
	return p, nil
}

// TypeAuthCleanup is the asynq task type for the candidate-membership auth
// cleanup: delete expired/consumed email_otps and expired/revoked
// candidate_sessions so those tables don't grow unbounded.
const TypeAuthCleanup = "auth:cleanup"

// AuthCleanupPayload is the job body for an auth cleanup. Batch is optional; the
// handler derives the default from config when zero.
type AuthCleanupPayload struct {
	Batch int `json:"batch"` // max rows per delete batch; 0 → config default
}

// authCleanupUniqueTTL dedups overlapping cleanup enqueues during a rolling deploy.
const authCleanupUniqueTTL = 1 * time.Hour

// NewAuthCleanupTask builds the auth-cleanup task with retry/timeout policy and
// enqueue-scoped uniqueness.
func NewAuthCleanupTask(p AuthCleanupPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal payload: %w", err)
	}
	return asynq.NewTask(
		TypeAuthCleanup,
		body,
		asynq.MaxRetry(taskMaxRetry),
		asynq.Timeout(taskTimeout),
		asynq.Retention(taskRetention),
		asynq.Unique(authCleanupUniqueTTL),
	), nil
}

// ParseAuthCleanupPayload decodes an auth-cleanup task body.
func ParseAuthCleanupPayload(body []byte) (AuthCleanupPayload, error) {
	var p AuthCleanupPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("queue: unmarshal payload: %w", err)
	}
	return p, nil
}
