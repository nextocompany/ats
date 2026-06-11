package applications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Repository is the application data-access contract.
type Repository interface {
	Create(ctx context.Context, a Application) (Application, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Application, int, error)
	ListByCandidate(ctx context.Context, candidateID uuid.UUID) ([]Application, error)
	SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error
	SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	SetParseResults(ctx context.Context, id uuid.UUID, r ParseResult) error
	// Sprint 2:
	SetCanonicalCandidate(ctx context.Context, id, candidateID uuid.UUID) error
	SetDedupState(ctx context.Context, id uuid.UUID, state string, confidence float64) error
	SetScore(ctx context.Context, id uuid.UUID, s Score) error
	SetAssignment(ctx context.Context, id uuid.UUID, storeNo *int, talentPool bool) error
	// Sprint 3:
	SetHired(ctx context.Context, id uuid.UUID) error
	SetPSSynced(ctx context.Context, id uuid.UUID) error
	SetPublicToken(ctx context.Context, id uuid.UUID, token string) error
	FindByPublicToken(ctx context.Context, token string) (*Application, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed application repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Create(ctx context.Context, a Application) (Application, error) {
	const q = `
		INSERT INTO applications (candidate_id, position_id, status, raw_file_type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, q, a.CandidateID, a.PositionID, a.Status, a.RawFileType).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return Application{}, fmt.Errorf("applications: create: %w", err)
	}
	return a, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Application, error) {
	const q = `
		SELECT id, candidate_id, position_id, status,
		       COALESCE(raw_file_blob_url,''), COALESCE(raw_file_type,''),
		       COALESCE(ocr_text_blob_url,''), COALESCE(parsed_profile_blob_url,''),
		       ocr_confidence, COALESCE(needs_manual_review,false),
		       COALESCE(queue_task_id,''), parsed_at,
		       ai_score, must_have_passed, assigned_store_id,
		       COALESCE(talent_pool,false), COALESCE(dedup_state,''), created_at,
		       ai_score_breakdown, COALESCE(ai_summary,''), COALESCE(ai_red_flags,''),
		       ai_suggested_positions
		FROM applications WHERE id = $1`
	var a Application
	var breakdownRaw, suggestedRaw []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
		&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
		&breakdownRaw, &a.AISummary, &a.AIRedFlags, &suggestedRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("applications: find by id: %w", err)
	}
	// Explainability JSONB columns are NULL until the application is scored;
	// unmarshal only when present so an unscored record stays clean.
	if len(breakdownRaw) > 0 {
		var bd ScoreBreakdown
		if jsonErr := json.Unmarshal(breakdownRaw, &bd); jsonErr == nil {
			a.AIScoreBreakdown = &bd
		}
	}
	if len(suggestedRaw) > 0 {
		_ = json.Unmarshal(suggestedRaw, &a.AISuggestedPositions)
	}
	return &a, nil
}

func (r *pgRepository) SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error {
	const q = `UPDATE applications SET raw_file_blob_url = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, blobURL); err != nil {
		return fmt.Errorf("applications: set raw file: %w", err)
	}
	return nil
}

func (r *pgRepository) SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error {
	const q = `UPDATE applications SET queue_task_id = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, taskID); err != nil {
		return fmt.Errorf("applications: set queue task id: %w", err)
	}
	return nil
}

func (r *pgRepository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE applications SET status = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, status); err != nil {
		return fmt.Errorf("applications: set status: %w", err)
	}
	return nil
}

func (r *pgRepository) SetParseResults(ctx context.Context, id uuid.UUID, res ParseResult) error {
	const q = `
		UPDATE applications SET
			status                  = $2,
			ocr_text_blob_url       = $3,
			parsed_profile_blob_url = $4,
			ocr_confidence          = $5,
			needs_manual_review     = $6,
			parsed_at               = NOW(),
			updated_at              = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, StatusParsed,
		res.OCRTextBlobURL, res.ParsedProfileBlobURL, res.OCRConfidence, res.NeedsManualReview)
	if err != nil {
		return fmt.Errorf("applications: set parse results: %w", err)
	}
	return nil
}

func (r *pgRepository) SetCanonicalCandidate(ctx context.Context, id, candidateID uuid.UUID) error {
	const q = `UPDATE applications SET candidate_id = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, candidateID); err != nil {
		return fmt.Errorf("applications: set canonical candidate: %w", err)
	}
	return nil
}

func (r *pgRepository) SetDedupState(ctx context.Context, id uuid.UUID, state string, confidence float64) error {
	const q = `UPDATE applications SET dedup_state = $2, dedup_confidence = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, state, confidence); err != nil {
		return fmt.Errorf("applications: set dedup state: %w", err)
	}
	return nil
}

func (r *pgRepository) SetScore(ctx context.Context, id uuid.UUID, s Score) error {
	const q = `
		UPDATE applications SET
			status                 = $2,
			must_have_passed       = $3,
			ai_score               = $4,
			ai_score_breakdown     = $5,
			ai_summary             = $6,
			ai_red_flags           = $7,
			ai_suggested_positions = $8,
			updated_at             = NOW()
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, s.Status, s.MustHavePassed, s.Total,
		s.BreakdownJSON, s.Summary, s.RedFlags, s.SuggestedJSON); err != nil {
		return fmt.Errorf("applications: set score: %w", err)
	}
	return nil
}

func (r *pgRepository) SetAssignment(ctx context.Context, id uuid.UUID, storeNo *int, talentPool bool) error {
	const q = `UPDATE applications SET assigned_store_id = $2, talent_pool = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, storeNo, talentPool); err != nil {
		return fmt.Errorf("applications: set assignment: %w", err)
	}
	return nil
}

func (r *pgRepository) SetHired(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET status = $2, hired_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, StatusHired); err != nil {
		return fmt.Errorf("applications: set hired: %w", err)
	}
	return nil
}

func (r *pgRepository) SetPSSynced(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET ps_synced_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("applications: set ps synced: %w", err)
	}
	return nil
}

func (r *pgRepository) SetPublicToken(ctx context.Context, id uuid.UUID, token string) error {
	const q = `UPDATE applications SET public_token = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, token); err != nil {
		return fmt.Errorf("applications: set public token: %w", err)
	}
	return nil
}

func (r *pgRepository) FindByPublicToken(ctx context.Context, token string) (*Application, error) {
	const q = `
		SELECT id, candidate_id, position_id, status,
		       COALESCE(raw_file_blob_url,''), COALESCE(raw_file_type,''),
		       COALESCE(ocr_text_blob_url,''), COALESCE(parsed_profile_blob_url,''),
		       ocr_confidence, COALESCE(needs_manual_review,false),
		       COALESCE(queue_task_id,''), parsed_at,
		       ai_score, must_have_passed, assigned_store_id,
		       COALESCE(talent_pool,false), COALESCE(dedup_state,''), created_at
		FROM applications WHERE public_token = $1`
	var a Application
	err := r.pool.QueryRow(ctx, q, token).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
		&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("applications: find by public token: %w", err)
	}
	return &a, nil
}
