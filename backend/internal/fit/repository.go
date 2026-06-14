package fit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when no analysis exists for an application.
var ErrNotFound = errors.New("fit: analysis not found")

// Repository is the fit-analysis data-access contract.
type Repository interface {
	// Upsert writes (or overwrites) the current analysis for an application.
	Upsert(ctx context.Context, a Analysis, generatedBy *uuid.UUID) error
	// FindByApplicationID returns the application's analysis, or ErrNotFound.
	FindByApplicationID(ctx context.Context, applicationID uuid.UUID) (*Analysis, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed fit-analysis repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Upsert(ctx context.Context, a Analysis, generatedBy *uuid.UUID) error {
	strengths, err := json.Marshal(nonNilStr(a.Strengths))
	if err != nil {
		return fmt.Errorf("fit: marshal strengths: %w", err)
	}
	concerns, err := json.Marshal(nonNilStr(a.Concerns))
	if err != nil {
		return fmt.Errorf("fit: marshal concerns: %w", err)
	}
	recommended, err := json.Marshal(nonNilRec(a.Recommended))
	if err != nil {
		return fmt.Errorf("fit: marshal recommended: %w", err)
	}
	const q = `
		INSERT INTO application_fit_analyses
			(application_id, overall_fit, summary, strengths, concerns, recommended, no_match_reason, model, generated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (application_id) DO UPDATE SET
			overall_fit     = EXCLUDED.overall_fit,
			summary         = EXCLUDED.summary,
			strengths       = EXCLUDED.strengths,
			concerns        = EXCLUDED.concerns,
			recommended     = EXCLUDED.recommended,
			no_match_reason = EXCLUDED.no_match_reason,
			model           = EXCLUDED.model,
			generated_by    = EXCLUDED.generated_by,
			updated_at      = NOW()`
	if _, err := r.pool.Exec(ctx, q, a.ApplicationID, a.OverallFit, a.Summary,
		strengths, concerns, recommended, a.NoMatchReason, a.Model, generatedBy); err != nil {
		return fmt.Errorf("fit: upsert: %w", err)
	}
	return nil
}

func (r *pgRepository) FindByApplicationID(ctx context.Context, applicationID uuid.UUID) (*Analysis, error) {
	const q = `
		SELECT application_id, overall_fit, COALESCE(summary,''),
		       COALESCE(strengths,'[]'::jsonb), COALESCE(concerns,'[]'::jsonb),
		       COALESCE(recommended,'[]'::jsonb), COALESCE(no_match_reason,''), updated_at
		FROM application_fit_analyses WHERE application_id = $1`
	var (
		a               Analysis
		strengthsJSON   []byte
		concernsJSON    []byte
		recommendedJSON []byte
	)
	err := r.pool.QueryRow(ctx, q, applicationID).Scan(
		&a.ApplicationID, &a.OverallFit, &a.Summary,
		&strengthsJSON, &concernsJSON, &recommendedJSON, &a.NoMatchReason, &a.GeneratedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fit: find by application: %w", err)
	}
	if len(strengthsJSON) > 0 {
		_ = json.Unmarshal(strengthsJSON, &a.Strengths)
	}
	if len(concernsJSON) > 0 {
		_ = json.Unmarshal(concernsJSON, &a.Concerns)
	}
	if len(recommendedJSON) > 0 {
		_ = json.Unmarshal(recommendedJSON, &a.Recommended)
	}
	if a.Recommended == nil {
		a.Recommended = []RecommendedPosition{}
	}
	return &a, nil
}

func nonNilStr(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilRec(r []RecommendedPosition) []RecommendedPosition {
	if r == nil {
		return []RecommendedPosition{}
	}
	return r
}
