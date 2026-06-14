package fit

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/interview"
	"github.com/nexto/hr-ats/internal/positions"
)

// maxCatalogue bounds how many Master JD positions are sent to the LLM in one
// prompt — a defensive cap so the catalogue growing (or a misconfig) can't blow
// the token budget. Today's catalogue is ~65; the cap only fires far above that.
const maxCatalogue = 120

// Pre-condition errors surfaced to the handler.
var (
	// ErrNotScored means the application has no CV-screening result yet.
	ErrNotScored = errors.New("fit: application not scored yet")
	// ErrInterviewIncomplete means the AI pre-interview is missing or unfinished.
	ErrInterviewIncomplete = errors.New("fit: ai interview not completed")
)

// Reader interfaces (accept interfaces, return structs). The concrete pgx repos
// from applications/interview/positions/candidates satisfy these.
type appReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*applications.Application, error)
}
type interviewReader interface {
	// FindByApplicationID returns the application's session, or (nil, nil) when none.
	FindByApplicationID(ctx context.Context, applicationID uuid.UUID) (*interview.Session, error)
}
type positionLister interface {
	ListAll(ctx context.Context) ([]positions.Position, error)
}
type candidateReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*candidates.Candidate, error)
}

// Service generates and reads the cross-position fit analysis.
type Service struct {
	repo       Repository
	summarizer Summarizer
	apps       appReader
	interviews interviewReader
	positions  positionLister
	cands      candidateReader
}

// NewService wires the fit service.
func NewService(repo Repository, summarizer Summarizer, apps appReader, interviews interviewReader, pos positionLister, cands candidateReader) *Service {
	return &Service{
		repo:       repo,
		summarizer: summarizer,
		apps:       apps,
		interviews: interviews,
		positions:  pos,
		cands:      cands,
	}
}

// Generate builds the inputs (screening + interview + Master JD catalogue), calls
// the summarizer, persists the result, and returns it. It fails fast when the
// pre-conditions (scored + interview completed) are not met.
func (s *Service) Generate(ctx context.Context, applicationID uuid.UUID, generatedBy *uuid.UUID) (*Analysis, error) {
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if app.AIScore == nil {
		return nil, ErrNotScored
	}

	sess, err := s.interviews.FindByApplicationID(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if sess == nil || sess.Status != interview.StatusCompleted {
		return nil, ErrInterviewIncomplete
	}

	pos, err := s.positions.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(pos) > maxCatalogue {
		log.Warn().Int("total", len(pos)).Int("cap", maxCatalogue).
			Str("application", applicationID.String()).Msg("fit: position catalogue capped for the LLM prompt")
		pos = pos[:maxCatalogue]
	}

	in := Inputs{
		ScreeningScore:     app.AIScore,
		ScreeningSummary:   app.AISummary,
		ScreeningRedFlags:  app.AIRedFlags,
		InterviewScore:     sess.InterviewScore,
		InterviewSummary:   sess.Summary,
		InterviewStrengths: sess.Strengths,
		InterviewConcerns:  sess.Concerns,
		Transcript:         toTurns(sess.Conversation),
		Positions:          toCards(pos),
	}
	if cand, cerr := s.cands.FindByID(ctx, app.CandidateID); cerr == nil {
		in.CandidateName = cand.FullName
	} else {
		log.Warn().Err(cerr).Str("candidate", app.CandidateID.String()).Msg("fit: candidate lookup failed (degraded context)")
	}

	a, err := s.summarizer.Summarize(ctx, in)
	if err != nil {
		return nil, err
	}
	a.ApplicationID = applicationID
	if err := s.repo.Upsert(ctx, a, generatedBy); err != nil {
		return nil, err
	}
	// Re-read so the response carries the authoritative DB timestamp (and the same
	// shape a subsequent GET returns) rather than the in-memory pre-write struct.
	return s.repo.FindByApplicationID(ctx, applicationID)
}

// Get returns the persisted analysis, or ErrNotFound when none exists.
func (s *Service) Get(ctx context.Context, applicationID uuid.UUID) (*Analysis, error) {
	return s.repo.FindByApplicationID(ctx, applicationID)
}

func toTurns(conv []interview.Turn) []Turn {
	out := make([]Turn, 0, len(conv))
	for _, t := range conv {
		out = append(out, Turn{Role: t.Role, Content: t.Content})
	}
	return out
}

func toCards(pos []positions.Position) []PositionCard {
	out := make([]PositionCard, 0, len(pos))
	for _, p := range pos {
		out = append(out, PositionCard{
			ID:               p.ID,
			Title:            p.TitleTH,
			Responsibilities: p.Responsibilities,
			Qualifications:   p.Qualifications,
		})
	}
	return out
}
