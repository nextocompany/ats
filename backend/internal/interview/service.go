package interview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/positions"
)

// Errors surfaced to handlers.
var (
	ErrNotAnswerable = errors.New("interview: session is not open for answers")
	ErrEmptyAnswer   = errors.New("interview: answer must not be empty")
)

// maxAnswerLen bounds a single candidate answer to keep LLM cost predictable.
const maxAnswerLen = 4000

// Reader interfaces (accept interfaces, return structs). The concrete pgx repos
// from applications/positions/candidates satisfy these.
type appReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*applications.Application, error)
	// SetStatus advances the application status as the AI interview progresses
	// (scored → ai_interview on invite, ai_interview → ai_interviewed on completion).
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
}

// ErrNotScreened is returned when an AI interview is requested for an application
// that is not in the "screened" (scored) state — the funnel requires screening first.
var ErrNotScreened = errors.New("interview: AI interview is only available after screening")

type positionReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*positions.Position, error)
}
type candidateReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*candidates.Candidate, error)
}

// Service orchestrates the interview lifecycle: invite, start, and per-turn
// responses. Turns are synchronous LLM calls (no async worker).
type Service struct {
	repo          Repository
	interviewer   Interviewer
	apps          appReader
	positions     positionReader
	cands         candidateReader
	notifier      notify.Notifier
	portalBaseURL string
	maxTurns      int
}

// NewService wires the interview service.
func NewService(repo Repository, interviewer Interviewer, apps appReader, pos positionReader, cands candidateReader, notifier notify.Notifier, portalBaseURL string, maxTurns int) *Service {
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	return &Service{
		repo:          repo,
		interviewer:   interviewer,
		apps:          apps,
		positions:     pos,
		cands:         cands,
		notifier:      notifier,
		portalBaseURL: portalBaseURL,
		maxTurns:      maxTurns,
	}
}

// Invite creates (or returns the existing) interview session for an application
// and sends a best-effort invitation to the candidate. Idempotent: re-inviting
// returns the same session without re-notifying with a new token.
func (s *Service) Invite(ctx context.Context, applicationID uuid.UUID) (*Session, error) {
	existing, err := s.repo.FindByApplicationID(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	// State-machine guard: the AI interview is the only action allowed from the
	// screened (scored) state. Block invites from any other status.
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if app.Status != applications.StatusScored {
		return nil, ErrNotScreened
	}
	token, err := newAccessToken()
	if err != nil {
		return nil, err
	}
	session, err := s.repo.Create(ctx, applicationID, token)
	if errors.Is(err, ErrAlreadyExists) {
		// A concurrent invite won the race (UNIQUE application_id). Re-read and
		// return the winner's session — keeps Invite idempotent under double-click.
		return s.repo.FindByApplicationID(ctx, applicationID)
	}
	if err != nil {
		return nil, err
	}
	// Advance the funnel: screened → ai_interview (best-effort; the session exists).
	if serr := s.apps.SetStatus(ctx, applicationID, applications.StatusAIInterview); serr != nil {
		log.Warn().Err(serr).Str("application", applicationID.String()).Msg("interview invite: set ai_interview status failed (non-fatal)")
	}
	s.notifyInvite(ctx, applicationID, token)
	return session, nil
}

// Get returns the application's session, or (nil, nil) when none exists.
func (s *Service) Get(ctx context.Context, applicationID uuid.UUID) (*Session, error) {
	return s.repo.FindByApplicationID(ctx, applicationID)
}

// Start loads a session by access token and, if it has not begun, seeds the
// first AI question. Used when the candidate opens the interview link.
func (s *Service) Start(ctx context.Context, token string) (*Session, error) {
	session, err := s.repo.FindByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if session.Status == StatusCompleted {
		return session, nil // already done — caller renders the closed state
	}
	if session.Status == StatusExpired || time.Now().After(session.ExpiresAt) {
		s.expire(ctx, session)
		return nil, ErrNotAnswerable
	}
	if len(session.Conversation) > 0 {
		return session, nil // already started — resume where we left off
	}

	expected := session.Version
	ic, err := s.buildContext(ctx, session.ApplicationID)
	if err != nil {
		return nil, err
	}
	reply, _, err := s.interviewer.NextTurn(ctx, ic, session.Conversation)
	if err != nil {
		return nil, err
	}
	session.Conversation = append(session.Conversation, Turn{Role: RoleAssistant, Content: reply, TS: time.Now()})
	started := time.Now()
	if err := s.repo.SaveConversation(ctx, session.ID, session.Conversation, session.userTurns(), StatusInProgress, &started, expected); err != nil {
		if errors.Is(err, ErrConflict) {
			return s.repo.FindByToken(ctx, token) // a concurrent Start seeded it — return that
		}
		return nil, err
	}
	session.Status = StatusInProgress
	return session, nil
}

// Respond records the candidate's answer, asks the next question (or closes and
// evaluates when the interview is complete), and returns the updated session.
func (s *Service) Respond(ctx context.Context, token, answer string) (*Session, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil, ErrEmptyAnswer
	}
	if len(answer) > maxAnswerLen {
		answer = answer[:maxAnswerLen]
	}
	session, err := s.repo.FindByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if session.Status != StatusInProgress {
		return nil, ErrNotAnswerable
	}
	if time.Now().After(session.ExpiresAt) {
		s.expire(ctx, session)
		return nil, ErrNotAnswerable
	}

	expected := session.Version
	session.Conversation = append(session.Conversation, Turn{Role: RoleUser, Content: answer, TS: time.Now()})

	ic, err := s.buildContext(ctx, session.ApplicationID)
	if err != nil {
		return nil, err
	}

	done := session.userTurns() >= s.maxTurns
	if !done {
		reply, llmDone, terr := s.interviewer.NextTurn(ctx, ic, session.Conversation)
		if terr != nil {
			return nil, terr
		}
		session.Conversation = append(session.Conversation, Turn{Role: RoleAssistant, Content: reply, TS: time.Now()})
		done = llmDone
	}

	if !done {
		if err := s.repo.SaveConversation(ctx, session.ID, session.Conversation, session.userTurns(), StatusInProgress, nil, expected); err != nil {
			return nil, err // ErrConflict bubbles → handler maps to 409
		}
		session.Status = StatusInProgress
		return session, nil
	}

	// Completing: evaluate FIRST, then persist conversation + evaluation. The
	// optimistic SaveConversation guards against a concurrent answer (only one
	// finisher wins), and SetEvaluation is the sole writer of the completed status,
	// so a failed/slow LLM evaluation can never leave the session stuck as
	// "completed" with no scores — it stays in_progress and is retryable.
	ev, eerr := s.interviewer.Evaluate(ctx, ic, session.Conversation)
	if eerr != nil {
		return nil, eerr
	}
	if err := s.repo.SaveConversation(ctx, session.ID, session.Conversation, session.userTurns(), StatusInProgress, nil, expected); err != nil {
		return nil, err // ErrConflict → 409: a concurrent request advanced this session
	}
	if err := s.repo.SetEvaluation(ctx, session.ID, ev); err != nil {
		return nil, err
	}
	applyEvaluation(session, ev)
	session.Status = StatusCompleted
	// Advance the funnel: ai_interview → ai_interviewed, opening up Shortlist /
	// Interview / Reject for HR. Best-effort and idempotent: only advance if the
	// application is still in ai_interview (don't clobber a later manual state).
	s.advanceToInterviewed(ctx, session.ApplicationID)
	return session, nil
}

// advanceToInterviewed moves the application to ai_interviewed on AI-interview
// completion, but only from ai_interview (so a re-completed session or an
// already-moved application is never clobbered). Best-effort — a failure here
// must not fail the candidate's final answer.
func (s *Service) advanceToInterviewed(ctx context.Context, applicationID uuid.UUID) {
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		log.Warn().Err(err).Str("application", applicationID.String()).Msg("interview complete: load application failed (non-fatal)")
		return
	}
	if app.Status != applications.StatusAIInterview {
		return
	}
	if serr := s.apps.SetStatus(ctx, applicationID, applications.StatusAIInterviewed); serr != nil {
		log.Warn().Err(serr).Str("application", applicationID.String()).Msg("interview complete: set ai_interviewed status failed (non-fatal)")
	}
}

// expire flips an out-of-date session to the expired status, best-effort, so the
// DB reflects reality (the status is otherwise derived from expires_at at read).
func (s *Service) expire(ctx context.Context, session *Session) {
	if session.Status == StatusExpired || session.Status == StatusCompleted {
		return
	}
	if err := s.repo.MarkExpired(ctx, session.ID); err != nil {
		log.Warn().Err(err).Str("session", session.ID.String()).Msg("interview: mark expired failed (non-fatal)")
	}
}

// buildContext assembles the LLM grounding from the position JD + candidate +
// resume summary. Position/candidate fetch failures degrade gracefully rather
// than failing the interview.
func (s *Service) buildContext(ctx context.Context, applicationID uuid.UUID) (InterviewContext, error) {
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		return InterviewContext{}, err
	}
	ic := InterviewContext{ProfileSummary: app.AISummary, MaxTurns: s.maxTurns}
	if pos, perr := s.positions.FindByID(ctx, app.PositionID); perr == nil {
		ic.PositionTitle = pos.TitleTH
		ic.Responsibilities = pos.Responsibilities
		ic.Qualifications = pos.Qualifications
	} else {
		log.Warn().Err(perr).Str("application", applicationID.String()).Msg("interview: position lookup failed (degraded context)")
	}
	if cand, cerr := s.cands.FindByID(ctx, app.CandidateID); cerr == nil {
		ic.CandidateName = cand.FullName
	}
	return ic, nil
}

// notifyInvite sends a best-effort LINE invitation. Never returns an error — a
// notify failure must not fail the HR action (mirrors applications.notifyStatusChange).
func (s *Service) notifyInvite(ctx context.Context, applicationID uuid.UUID, token string) {
	if s.notifier == nil || s.cands == nil || s.apps == nil {
		return
	}
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		log.Warn().Err(err).Str("application", applicationID.String()).Msg("interview invite: load application failed")
		return
	}
	cand, err := s.cands.FindByID(ctx, app.CandidateID)
	if err != nil {
		log.Warn().Err(err).Str("candidate", app.CandidateID.String()).Msg("interview invite: load candidate failed")
		return
	}
	msg := notify.InterviewInviteMessage(cand.LineUserID, cand.FullName, s.portalBaseURL, token)
	if msg.Recipient == "" {
		return // candidate has no LINE handle — HR shares the link manually
	}
	if err := s.notifier.Send(ctx, msg); err != nil {
		log.Warn().Err(err).Str("application", applicationID.String()).Msg("interview invite: send failed (non-fatal)")
	}
}

// applyEvaluation copies a freshly computed Evaluation onto the in-memory session
// so the response reflects the completed state without a re-read.
func applyEvaluation(s *Session, ev Evaluation) {
	score := ev.Score
	s.InterviewScore = &score
	s.Recommendation = ev.Recommendation
	s.Strengths = ev.Strengths
	s.Concerns = ev.Concerns
	s.Summary = ev.Summary
}
