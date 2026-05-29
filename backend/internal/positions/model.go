// Package positions exposes Master JD data needed for scoring.
package positions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MustHave is the deterministic gate criteria stored in must_have_criteria.
// Sprint 2 seed uses an object form: {"min_education_level":2,"min_experience_months":12}.
type MustHave struct {
	MinEducationLevel   int `json:"min_education_level"`
	MinExperienceMonths int `json:"min_experience_months"`
}

// Position maps the positions table (fields used in scoring).
type Position struct {
	ID          uuid.UUID `json:"id"`
	TitleTH     string    `json:"title_th"`
	TitleEN     string    `json:"title_en"`
	Level       string    `json:"level"`
	MustHave    MustHave  `json:"must_have"`
	Keywords    []string  `json:"keywords"`
	FormatTypes []string  `json:"format_types"`
}

// Repository is the position data-access contract.
type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Position, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed position repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Position, error) {
	const q = `
		SELECT id, title_th, COALESCE(title_en,''), COALESCE(level,''),
		       COALESCE(must_have_criteria, '{}'::jsonb),
		       COALESCE(keywords, '{}'), COALESCE(format_types, '{}')
		FROM positions WHERE id = $1`
	var (
		p           Position
		mustHaveRaw []byte
	)
	if err := r.pool.QueryRow(ctx, q, id).Scan(
		&p.ID, &p.TitleTH, &p.TitleEN, &p.Level, &mustHaveRaw, &p.Keywords, &p.FormatTypes,
	); err != nil {
		return nil, fmt.Errorf("positions: find by id: %w", err)
	}
	if len(mustHaveRaw) > 0 {
		if err := json.Unmarshal(mustHaveRaw, &p.MustHave); err != nil {
			return nil, fmt.Errorf("positions: parse must_have_criteria: %w", err)
		}
	}
	return &p, nil
}
