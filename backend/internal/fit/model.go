// Package fit produces an AI cross-position fit analysis for a candidate's
// application: it combines the CV-screening result and the AI pre-interview
// evaluation, then recommends which Master JD position(s) the candidate fits —
// with Thai reasons — or states plainly that none fit.
package fit

import (
	"time"

	"github.com/google/uuid"
)

// Overall-fit verdicts. Unknown values from the LLM normalize to OverallWeak.
const (
	OverallStrong   = "strong"
	OverallModerate = "moderate"
	OverallWeak     = "weak"
	OverallNone     = "none"
)

// RecommendedPosition is one Master JD position the candidate is judged a fit for.
type RecommendedPosition struct {
	PositionID uuid.UUID `json:"position_id"`
	Title      string    `json:"title"`
	FitScore   int       `json:"fit_score"` // 0-100
	Reasons    []string  `json:"reasons"`   // Thai bullet points
}

// Analysis is the persisted, HR-facing fit verdict for an application.
type Analysis struct {
	ApplicationID uuid.UUID             `json:"application_id"`
	OverallFit    string                `json:"overall_fit"` // strong | moderate | weak | none
	Summary       string                `json:"summary"`     // 2-3 Thai sentences
	Strengths     []string              `json:"strengths"`   // Thai bullets
	Concerns      []string              `json:"concerns"`    // Thai bullets
	Recommended   []RecommendedPosition `json:"recommended"`
	NoMatchReason string                `json:"no_match_reason,omitempty"` // set when OverallFit == none
	Model         string                `json:"-"`                         // provider/deployment, audit only
	GeneratedAt   time.Time             `json:"generated_at"`
}

// Turn is a candidate-safe projection of one interview message fed to the LLM.
type Turn struct {
	Role    string
	Content string
}

// PositionCard is the slimmed Master JD entry handed to the LLM catalogue.
type PositionCard struct {
	ID               uuid.UUID
	Title            string
	Responsibilities string
	Qualifications   string
}

// Inputs is everything the service gathers and the Summarizer consumes. It is
// provider-agnostic so the mock and Azure implementations share one contract.
type Inputs struct {
	CandidateName string

	ScreeningScore    *float64
	ScreeningSummary  string // app.AISummary (จุดแข็ง joined)
	ScreeningRedFlags string // app.AIRedFlags

	InterviewScore     *float64
	InterviewSummary   string
	InterviewStrengths []string
	InterviewConcerns  []string
	Transcript         []Turn

	Positions []PositionCard
}
