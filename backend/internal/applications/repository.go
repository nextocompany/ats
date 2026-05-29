package applications

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the application data-access contract.
type Repository interface {
	Create(ctx context.Context, a Application) (Application, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error
	SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	SetParseResults(ctx context.Context, id uuid.UUID, r ParseResult) error
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
		       COALESCE(queue_task_id,''), parsed_at, created_at
		FROM applications WHERE id = $1`
	var a Application
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("applications: find by id: %w", err)
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
