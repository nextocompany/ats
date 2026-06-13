package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository sentinel errors.
var (
	// ErrNotFound is returned when no session matches the lookup key.
	ErrNotFound = errors.New("interview: session not found")
	// ErrAlreadyExists is returned when a session already exists for the
	// application (UNIQUE application_id violation) — lets the service stay
	// idempotent under a concurrent invite.
	ErrAlreadyExists = errors.New("interview: session already exists")
	// ErrConflict is returned when an optimistic-lock version check fails — another
	// request advanced the session while this one was working (e.g. mid-LLM-call).
	ErrConflict = errors.New("interview: concurrent update conflict")
)

// uniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const uniqueViolation = "23505"

// Repository is the interview-session data-access contract.
type Repository interface {
	// Create inserts a fresh invited session for an application.
	Create(ctx context.Context, applicationID uuid.UUID, token string) (*Session, error)
	FindByToken(ctx context.Context, token string) (*Session, error)
	// FindByApplicationID returns the application's session, or (nil, nil) when none exists.
	FindByApplicationID(ctx context.Context, applicationID uuid.UUID) (*Session, error)
	// SaveConversation persists the conversation, turn count, status and (on first
	// answer) the started_at timestamp. It is an optimistic write: it only applies
	// when the row's version still equals expectedVersion, returning ErrConflict
	// otherwise. This serialises concurrent turns WITHOUT holding a DB lock across
	// the (slow) LLM call.
	SaveConversation(ctx context.Context, id uuid.UUID, conv []Turn, turnCount int, status string, startedAt *time.Time, expectedVersion int) error
	// SetEvaluation writes the final evaluation and marks the session completed.
	// No-op (no error) if the session is already completed.
	SetEvaluation(ctx context.Context, id uuid.UUID, ev Evaluation) error
	// MarkExpired flips a session to the expired status (best-effort, lazy on read).
	MarkExpired(ctx context.Context, id uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed interview repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const sessionColumns = `
	id, application_id, access_token, status,
	COALESCE(conversation, '[]'::jsonb), COALESCE(turn_count, 0), COALESCE(version, 0),
	interview_score, COALESCE(recommendation, ''), strengths, concerns, COALESCE(summary, ''),
	invited_at, started_at, completed_at, expires_at, created_at`

func (r *pgRepository) Create(ctx context.Context, applicationID uuid.UUID, token string) (*Session, error) {
	const q = `
		INSERT INTO interview_sessions (application_id, access_token)
		VALUES ($1, $2)
		RETURNING ` + sessionColumns
	s, err := scanSession(r.pool.QueryRow(ctx, q, applicationID, token))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return nil, ErrAlreadyExists
	}
	if err != nil {
		return nil, fmt.Errorf("interview: create session: %w", err)
	}
	return s, nil
}

func (r *pgRepository) FindByToken(ctx context.Context, token string) (*Session, error) {
	const q = `SELECT ` + sessionColumns + ` FROM interview_sessions WHERE access_token = $1`
	s, err := scanSession(r.pool.QueryRow(ctx, q, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("interview: find by token: %w", err)
	}
	return s, nil
}

func (r *pgRepository) FindByApplicationID(ctx context.Context, applicationID uuid.UUID) (*Session, error) {
	const q = `SELECT ` + sessionColumns + ` FROM interview_sessions WHERE application_id = $1`
	s, err := scanSession(r.pool.QueryRow(ctx, q, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("interview: find by application: %w", err)
	}
	return s, nil
}

func (r *pgRepository) SaveConversation(ctx context.Context, id uuid.UUID, conv []Turn, turnCount int, status string, startedAt *time.Time, expectedVersion int) error {
	if conv == nil {
		conv = []Turn{}
	}
	convJSON, err := json.Marshal(conv)
	if err != nil {
		return fmt.Errorf("interview: marshal conversation: %w", err)
	}
	const q = `
		UPDATE interview_sessions SET
			conversation = $2,
			turn_count   = $3,
			status       = $4,
			started_at   = COALESCE(started_at, $5),
			version      = version + 1,
			updated_at   = NOW()
		WHERE id = $1 AND version = $6`
	tag, err := r.pool.Exec(ctx, q, id, convJSON, turnCount, status, startedAt, expectedVersion)
	if err != nil {
		return fmt.Errorf("interview: save conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict // version moved under us → a concurrent request won
	}
	return nil
}

func (r *pgRepository) MarkExpired(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE interview_sessions SET status = $2, updated_at = NOW() WHERE id = $1 AND status <> $3`
	if _, err := r.pool.Exec(ctx, q, id, StatusExpired, StatusCompleted); err != nil {
		return fmt.Errorf("interview: mark expired: %w", err)
	}
	return nil
}

func (r *pgRepository) SetEvaluation(ctx context.Context, id uuid.UUID, ev Evaluation) error {
	strengths, err := json.Marshal(nonNil(ev.Strengths))
	if err != nil {
		return fmt.Errorf("interview: marshal strengths: %w", err)
	}
	concerns, err := json.Marshal(nonNil(ev.Concerns))
	if err != nil {
		return fmt.Errorf("interview: marshal concerns: %w", err)
	}
	const q = `
		UPDATE interview_sessions SET
			status          = $2,
			interview_score = $3,
			recommendation  = $4,
			strengths       = $5,
			concerns        = $6,
			summary         = $7,
			version         = version + 1,
			completed_at    = NOW(),
			updated_at      = NOW()
		WHERE id = $1 AND status <> $2`
	if _, err := r.pool.Exec(ctx, q, id, StatusCompleted, ev.Score, ev.Recommendation, strengths, concerns, ev.Summary); err != nil {
		return fmt.Errorf("interview: set evaluation: %w", err)
	}
	return nil // idempotent: 0 rows means it was already completed
}

// scanSession reads one row into a Session, decoding the JSONB columns.
func scanSession(row pgx.Row) (*Session, error) {
	var (
		s             Session
		convJSON      []byte
		strengthsJSON []byte
		concernsJSON  []byte
	)
	if err := row.Scan(
		&s.ID, &s.ApplicationID, &s.AccessToken, &s.Status,
		&convJSON, &s.TurnCount, &s.Version,
		&s.InterviewScore, &s.Recommendation, &strengthsJSON, &concernsJSON, &s.Summary,
		&s.InvitedAt, &s.StartedAt, &s.CompletedAt, &s.ExpiresAt, &s.CreatedAt,
	); err != nil {
		return nil, err
	}
	if len(convJSON) > 0 {
		if err := json.Unmarshal(convJSON, &s.Conversation); err != nil {
			return nil, fmt.Errorf("interview: decode conversation: %w", err)
		}
	}
	if len(strengthsJSON) > 0 {
		_ = json.Unmarshal(strengthsJSON, &s.Strengths)
	}
	if len(concernsJSON) > 0 {
		_ = json.Unmarshal(concernsJSON, &s.Concerns)
	}
	if s.Conversation == nil {
		s.Conversation = []Turn{}
	}
	return &s, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
