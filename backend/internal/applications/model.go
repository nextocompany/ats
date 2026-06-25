// Package applications owns the application lifecycle: intake, the OCR/parse
// result persistence, and (Sprint 1) job status.
package applications

import (
	"time"

	"github.com/google/uuid"
)

// Status values.
//
// Pipeline statuses (set by the system) and the HR funnel statuses form one
// VARCHAR column; the legal FROM→TO progression is enforced by the state machine
// in transitions.go (not the DB). "scored" is the funnel entry — the UI labels it
// "Screened". The funnel terminal action is now "offer" (the Offer Package itself
// is a future feature); StatusHired is retained for backward compatibility.
const (
	StatusPending       = "pending"        // S1: created, awaiting pipeline
	StatusParsed        = "parsed"         // S1: OCR + parse done
	StatusFailed        = "failed"         // pipeline error (transient; asynq may retry)
	StatusInvalidResume = "invalid_resume" // uploaded file is not a resume/CV (recoverable: candidate re-uploads)
	StatusNameMismatch  = "name_mismatch"  // resume name differs from the account holder (recoverable: re-upload own CV)
	StatusScored        = "scored"         // S2: passed gate, scored + assigned == "screened"
	StatusRejected      = "rejected"       // failed must-have gate, or HR reject (with reason)
	StatusHired         = "hired"          // legacy terminal (superseded by StatusOffer)

	// HR funnel statuses (manual transitions, gated by transitions.go).
	StatusAIInterview     = "ai_interview"     // AI pre-interview invited / in progress
	StatusAIInterviewed   = "ai_interviewed"   // AI pre-interview completed (system-set)
	StatusShortlisted     = "shortlisted"      // HR shortlisted
	StatusInterview       = "interview"        // human interview scheduled (carries appointment)
	StatusInterviewed     = "interviewed"      // human interview completed
	StatusPendingApproval = "pending_approval" // hire submitted into the 4-level approval chain
	StatusOffer           = "offer"            // entered Offer Package process (future)
)

// Application maps the applications table (columns used in Sprint 1).
type Application struct {
	ID                   uuid.UUID  `json:"id"`
	CandidateID          uuid.UUID  `json:"candidate_id"`
	PositionID           uuid.UUID  `json:"position_id"`
	Status               string     `json:"status"`
	RejectionReason      string     `json:"rejection_reason,omitempty"` // internal; never sent to the candidate
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
	// PublicToken is the opaque status-page token (set at portal apply; empty for
	// bulk/PeopleSoft/legacy rows). Used to build the candidate deep link
	// /status?token=…; never the application UUID (that page is public).
	PublicToken string `json:"-"`
	// PSSyncedAt is set when the hired candidate was pushed to PeopleSoft (the
	// deferred close-case step). Nil until onboarding is approved-complete and the
	// push succeeds; doubles as the once-only guard. Populated by FindByID.
	PSSyncedAt *time.Time `json:"ps_synced_at,omitempty"`
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

// Appointment is a scheduled human interview for an application (onsite or
// online). For online interviews OnlineJoinURL holds the Teams join link and
// CalendarEventID the Graph event id (for a future cancel/reschedule).
type Appointment struct {
	ID              uuid.UUID  `json:"id"`
	ApplicationID   uuid.UUID  `json:"application_id"`
	RoundNo         int        `json:"round_no"`
	ScheduledAt     time.Time  `json:"scheduled_at"`
	DurationMin     int        `json:"duration_min"`
	Mode            string     `json:"mode"` // "onsite" | "online"
	LocationText    string     `json:"location_text,omitempty"`
	OnlineJoinURL   string     `json:"online_join_url,omitempty"`
	CalendarEventID string     `json:"-"` // internal Graph id, never serialized
	CreatedBy       *uuid.UUID `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Interview modes.
const (
	ModeOnsite = "onsite"
	ModeOnline = "online"
)

// UpcomingInterview is a scheduled interview joined with candidate, position, and
// store — for the HR calendar/agenda view (across applications, role-scoped).
type UpcomingInterview struct {
	ID              uuid.UUID `json:"id"`
	ApplicationID   uuid.UUID `json:"application_id"`
	RoundNo         int       `json:"round_no"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMin     int       `json:"duration_min"`
	Mode            string    `json:"mode"`
	LocationText    string    `json:"location_text,omitempty"`
	OnlineJoinURL   string    `json:"online_join_url,omitempty"`
	CandidateName   string    `json:"candidate_name"`
	PositionTitle   string    `json:"position_title"`
	StoreName       string    `json:"store_name,omitempty"`
	AssignedStoreID *int      `json:"assigned_store_id,omitempty"`
}

// UpcomingFilter parameterizes the HR calendar list: a time window, an optional
// "only mine" filter (by the scheduling user), and paging.
type UpcomingFilter struct {
	From    time.Time
	To      *time.Time
	Mine    bool
	ActorID uuid.UUID // resolved users.id; only used when Mine is true
	Page    int
	Limit   int
}

// PortalApplication is the candidate-facing row of a member's application
// history (GET /api/v1/public/me/applications). Intentionally minimal: no AI
// score / internal fields — only what a candidate may see, plus the opaque
// status token so the portal can deep-link to /status?token=...
type PortalApplication struct {
	StatusToken   string    `json:"status_token"`
	PositionTitle string    `json:"position_title"`
	Status        string    `json:"status"`
	AppliedAt     time.Time `json:"applied_at"`
}

// StatusEvent is one recorded status transition from application_status_history.
// Only to_status + changed_at are exposed — the curation layer (apptimeline)
// must never see from_status, rejection reasons, or actor identity.
type StatusEvent struct {
	To string
	At time.Time
}

// PortalTimeline is the account-scoped input for the candidate status timeline:
// the application's current status, applied-at, position title, and its recorded
// transitions. Returned only when the requesting account owns the application.
type PortalTimeline struct {
	Position  string
	CreatedAt time.Time
	Status    string
	Events    []StatusEvent
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
