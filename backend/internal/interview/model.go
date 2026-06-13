// Package interview owns the AI pre-interview: an adaptive, text-based screening
// conversation the candidate completes after the resume scoring stage. HR invites
// a candidate from the dashboard; the candidate chats with an AI HR interviewer
// (grounded in the position JD) via an opaque access token; the AI produces a
// transcript plus a structured evaluation HR reviews before deciding. Turns are
// synchronous API calls (no async worker). The interviewer LLM lives behind a
// provider seam (mock default, Azure OpenAI when configured), mirroring ai/ and
// scoring/.
package interview

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// Session status values.
const (
	StatusInvited    = "invited"     // created, candidate not yet started
	StatusInProgress = "in_progress" // candidate started; awaiting more answers
	StatusCompleted  = "completed"   // conversation done + evaluated
	StatusExpired    = "expired"     // past expires_at, no longer answerable
)

// Conversation turn roles. These map directly onto chat-completion roles.
const (
	RoleAssistant = "assistant" // the AI interviewer
	RoleUser      = "user"      // the candidate
)

// defaultMaxTurns is the fallback question cap when none is configured.
const defaultMaxTurns = 6

// Recommendation values the evaluator may return.
const (
	RecStrong   = "strong_recommend"
	RecPositive = "recommend"
	RecNeutral  = "neutral"
	RecCaution  = "caution"
)

// Turn is one message in the interview conversation, persisted as JSONB.
type Turn struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	TS      time.Time `json:"ts"`
}

// Evaluation is the AI's structured assessment of the completed conversation.
type Evaluation struct {
	Score          float64  `json:"interview_score"`
	Recommendation string   `json:"recommendation"`
	Strengths      []string `json:"strengths"`
	Concerns       []string `json:"concerns"`
	Summary        string   `json:"summary"`
}

// Session maps the interview_sessions table. Evaluation fields are nil/zero until
// the conversation completes.
type Session struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	AccessToken   string    `json:"access_token"`
	Status        string    `json:"status"`
	Conversation  []Turn    `json:"conversation"`
	TurnCount     int       `json:"turn_count"`
	Version       int       `json:"-"` // optimistic-lock counter; not exposed to clients
	// Evaluation (populated once Status == completed).
	InterviewScore *float64   `json:"interview_score"`
	Recommendation string     `json:"recommendation"`
	Strengths      []string   `json:"strengths"`
	Concerns       []string   `json:"concerns"`
	Summary        string     `json:"summary"`
	InvitedAt      time.Time  `json:"invited_at"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// userTurns counts how many answers the candidate has given so far.
func (s *Session) userTurns() int {
	n := 0
	for _, t := range s.Conversation {
		if t.Role == RoleUser {
			n++
		}
	}
	return n
}

// newAccessToken returns a URL-safe opaque token (never the session UUID),
// mirroring public.newPublicToken.
func newAccessToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
