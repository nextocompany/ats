// Package applications owns the application lifecycle: intake, the OCR/parse
// result persistence, and (Sprint 1) job status.
package applications

import (
	"time"

	"github.com/google/uuid"
)

// Status values.
const (
	StatusPending  = "pending"  // S1: created, awaiting pipeline
	StatusParsed   = "parsed"   // S1: OCR + parse done
	StatusFailed   = "failed"   // pipeline error
	StatusScored   = "scored"   // S2: passed gate, scored + assigned
	StatusRejected = "rejected" // S2: failed must-have gate
	StatusHired    = "hired"    // S3: HR hired → pushed to PeopleSoft
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
	// Sprint 2: scoring + assignment + dedup.
	AIScore         *float64  `json:"ai_score"`
	MustHavePassed  *bool     `json:"must_have_passed"`
	AssignedStoreID *int      `json:"assigned_store_id"`
	TalentPool      bool      `json:"talent_pool"`
	DedupState      string    `json:"dedup_state"`
	CreatedAt       time.Time `json:"created_at"`
	// Score explainability — the per-dimension breakdown and the LLM's
	// qualitative output, surfaced on the candidate detail view so HR can see
	// where a score came from. Populated by FindByID; omitempty keeps inbox list
	// rows (which never select these) and unscored applications lean.
	AIScoreBreakdown     *ScoreBreakdown `json:"ai_score_breakdown,omitempty"`
	AISummary            string          `json:"ai_summary,omitempty"`
	AIRedFlags           string          `json:"ai_red_flags,omitempty"`
	AISuggestedPositions []string        `json:"ai_suggested_positions,omitempty"`
	// Display fields — human-readable joins populated by the inbox List query so
	// the UI can lead with a person (name + role + store) instead of a UUID.
	// omitempty keeps single-record responses (Get/Intake) unchanged.
	CandidateName     string `json:"candidate_name,omitempty"`
	CandidateProvince string `json:"candidate_province,omitempty"`
	SourceChannel     string `json:"source_channel,omitempty"`
	PositionTitle     string `json:"position_title,omitempty"`
	StoreName         string `json:"store_name,omitempty"`
}

// ScoreBreakdown is the per-dimension score read back for the detail view. The
// JSON keys mirror what scoring.Breakdown writes into ai_score_breakdown, so the
// stored JSONB unmarshals straight into this struct. Max points: experience 30,
// skills 20 (LLM), education 10, language 10, location 20.
type ScoreBreakdown struct {
	Experience int `json:"experience"`
	Skills     int `json:"skills"`
	Education  int `json:"education"`
	Language   int `json:"language"`
	Location   int `json:"location"`
}

// Score carries scoring results in a repository-friendly (pre-serialized) form,
// so this package does not depend on the scoring package.
type Score struct {
	Status         string
	MustHavePassed bool
	Total          float64
	BreakdownJSON  []byte
	Summary        string
	RedFlags       string
	SuggestedJSON  []byte
}

// ParseResult is what the pipeline writes back after OCR + parse.
type ParseResult struct {
	OCRTextBlobURL       string
	ParsedProfileBlobURL string
	OCRConfidence        float64
	NeedsManualReview    bool
}
