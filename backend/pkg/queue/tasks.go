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
