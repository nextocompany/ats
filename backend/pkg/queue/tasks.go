package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// TypeProcessApplication is the asynq task type for the OCR + parse pipeline.
const TypeProcessApplication = "application:process"

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
