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
	ID               uuid.UUID `json:"id"`
	TitleTH          string    `json:"title_th"`
	TitleEN          string    `json:"title_en"`
	Level            string    `json:"level"`
	MustHave         MustHave  `json:"must_have"`
	Keywords         []string  `json:"keywords"`
	FormatTypes      []string  `json:"format_types"`
	Responsibilities string    `json:"responsibilities"`
	Qualifications   string    `json:"qualifications"`
	Benefits         string    `json:"benefits"`
}

// PublicPosition is the safe projection exposed on the public Career API.
type PublicPosition struct {
	ID        uuid.UUID `json:"id"`
	TitleTH   string    `json:"title_th"`
	TitleEN   string    `json:"title_en"`
	Level     string    `json:"level"`
	OpenCount int       `json:"open_count"`
}

// Repository is the position data-access contract.
type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Position, error)
	// FindByPSCode maps a PeopleSoft position code to an internal position.
	FindByPSCode(ctx context.Context, code string) (*Position, error)
	// ListPublic returns active positions that have at least one open vacancy.
	ListPublic(ctx context.Context) ([]PublicPosition, error)
	// ListAll returns every active position with its full Master JD text — used by
	// the cross-position fit analysis to match a candidate against the whole catalogue.
	ListAll(ctx context.Context) ([]Position, error)
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
		       COALESCE(keywords, '{}'), COALESCE(format_types, '{}'),
		       COALESCE(responsibilities, ''), COALESCE(qualifications, ''), COALESCE(benefits, '')
		FROM positions WHERE id = $1`
	var (
		p           Position
		mustHaveRaw []byte
	)
	if err := r.pool.QueryRow(ctx, q, id).Scan(
		&p.ID, &p.TitleTH, &p.TitleEN, &p.Level, &mustHaveRaw, &p.Keywords, &p.FormatTypes,
		&p.Responsibilities, &p.Qualifications, &p.Benefits,
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

func (r *pgRepository) FindByPSCode(ctx context.Context, code string) (*Position, error) {
	const q = `
		SELECT id, title_th, COALESCE(title_en,''), COALESCE(level,''),
		       COALESCE(must_have_criteria, '{}'::jsonb),
		       COALESCE(keywords, '{}'), COALESCE(format_types, '{}'),
		       COALESCE(responsibilities, ''), COALESCE(qualifications, ''), COALESCE(benefits, '')
		FROM positions WHERE ps_position_code = $1`
	var (
		p           Position
		mustHaveRaw []byte
	)
	if err := r.pool.QueryRow(ctx, q, code).Scan(
		&p.ID, &p.TitleTH, &p.TitleEN, &p.Level, &mustHaveRaw, &p.Keywords, &p.FormatTypes,
		&p.Responsibilities, &p.Qualifications, &p.Benefits,
	); err != nil {
		return nil, fmt.Errorf("positions: find by ps code: %w", err)
	}
	if len(mustHaveRaw) > 0 {
		if err := json.Unmarshal(mustHaveRaw, &p.MustHave); err != nil {
			return nil, fmt.Errorf("positions: parse must_have_criteria: %w", err)
		}
	}
	return &p, nil
}

func (r *pgRepository) ListAll(ctx context.Context) ([]Position, error) {
	const q = `
		SELECT id, title_th, COALESCE(title_en,''), COALESCE(level,''),
		       COALESCE(must_have_criteria, '{}'::jsonb),
		       COALESCE(keywords, '{}'), COALESCE(format_types, '{}'),
		       COALESCE(responsibilities, ''), COALESCE(qualifications, ''), COALESCE(benefits, '')
		FROM positions WHERE is_active = TRUE ORDER BY title_th`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("positions: list all: %w", err)
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		var (
			p           Position
			mustHaveRaw []byte
		)
		if err := rows.Scan(
			&p.ID, &p.TitleTH, &p.TitleEN, &p.Level, &mustHaveRaw, &p.Keywords, &p.FormatTypes,
			&p.Responsibilities, &p.Qualifications, &p.Benefits,
		); err != nil {
			return nil, fmt.Errorf("positions: scan all: %w", err)
		}
		if len(mustHaveRaw) > 0 {
			if err := json.Unmarshal(mustHaveRaw, &p.MustHave); err != nil {
				return nil, fmt.Errorf("positions: parse must_have_criteria: %w", err)
			}
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("positions: all rows: %w", err)
	}
	return out, nil
}

func (r *pgRepository) ListPublic(ctx context.Context) ([]PublicPosition, error) {
	const q = `
		SELECT p.id, p.title_th, COALESCE(p.title_en,''), COALESCE(p.level,''), COUNT(v.id) AS open_count
		FROM positions p
		JOIN vacancies v ON v.position_id = p.id AND v.status = 'open'
		WHERE p.is_active = TRUE
		GROUP BY p.id, p.title_th, p.title_en, p.level
		ORDER BY p.title_th`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("positions: list public: %w", err)
	}
	defer rows.Close()

	var out []PublicPosition
	for rows.Next() {
		var pp PublicPosition
		if err := rows.Scan(&pp.ID, &pp.TitleTH, &pp.TitleEN, &pp.Level, &pp.OpenCount); err != nil {
			return nil, fmt.Errorf("positions: scan public: %w", err)
		}
		out = append(out, pp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("positions: public rows: %w", err)
	}
	return out, nil
}
