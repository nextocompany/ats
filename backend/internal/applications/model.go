// Package applications owns the application lifecycle: intake, the OCR/parse
// result persistence, and (Sprint 1) job status.
package applications

import (
	"time"

	"github.com/google/uuid"
)

// Status values used in Sprint 1.
const (
	StatusPending = "pending"
	StatusParsed  = "parsed"
	StatusFailed  = "failed"
)

// Application maps the applications table (columns used in Sprint 1).
type Application struct {
	ID                   uuid.UUID  `json:"id"`
	CandidateID          uuid.UUID  `json:"candidate_id"`
	PositionID           uuid.UUID  `json:"position_id"`
	Status               string     `json:"status"`
	RawFileBlobURL       string     `json:"raw_file_blob_url"`
	RawFileType          string     `json:"raw_file_type"`
	OCRTextBlobURL       string     `json:"ocr_text_blob_url"`
	ParsedProfileBlobURL string     `json:"parsed_profile_blob_url"`
	OCRConfidence        *float64   `json:"ocr_confidence"`
	NeedsManualReview    bool       `json:"needs_manual_review"`
	QueueTaskID          string     `json:"queue_task_id"`
	ParsedAt             *time.Time `json:"parsed_at"`
	CreatedAt            time.Time  `json:"created_at"`
}

// ParseResult is what the pipeline writes back after OCR + parse.
type ParseResult struct {
	OCRTextBlobURL       string
	ParsedProfileBlobURL string
	OCRConfidence        float64
	NeedsManualReview    bool
}
