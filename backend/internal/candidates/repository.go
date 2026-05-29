package candidates

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the candidate data-access contract. The concrete implementation
// receives the pool via injection (never a global), per the project convention.
type Repository interface {
	Create(ctx context.Context, c Candidate) (Candidate, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error)
	UpdateProfileFields(ctx context.Context, id uuid.UUID, f ProfileFields) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed candidate repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// nullable returns nil for empty strings so unique columns (id_card) store NULL
// rather than colliding on the empty string.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *pgRepository) Create(ctx context.Context, c Candidate) (Candidate, error) {
	const q = `
		INSERT INTO candidates (full_name, phone, email, id_card, address, province, subregion, date_of_birth, source_channel, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10,''), 'available'))
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, q,
		c.FullName, nullable(c.Phone), nullable(c.Email), nullable(c.IDCard),
		nullable(c.Address), nullable(c.Province), nullable(c.Subregion),
		c.DateOfBirth, nullable(c.SourceChannel), c.Status,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return Candidate{}, fmt.Errorf("candidates: create: %w", err)
	}
	return c, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error) {
	const q = `
		SELECT id, full_name, COALESCE(phone,''), COALESCE(email,''), COALESCE(id_card,''),
		       COALESCE(address,''), COALESCE(province,''), COALESCE(subregion,''),
		       date_of_birth, COALESCE(source_channel,''), status, created_at
		FROM candidates WHERE id = $1`
	var c Candidate
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.FullName, &c.Phone, &c.Email, &c.IDCard,
		&c.Address, &c.Province, &c.Subregion, &c.DateOfBirth,
		&c.SourceChannel, &c.Status, &c.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("candidates: find by id: %w", err)
	}
	return &c, nil
}

// UpdateProfileFields applies parsed values, overwriting only non-empty inputs
// so a sparse parse never blanks existing data.
func (r *pgRepository) UpdateProfileFields(ctx context.Context, id uuid.UUID, f ProfileFields) error {
	const q = `
		UPDATE candidates SET
			full_name     = COALESCE(NULLIF($2,''), full_name),
			phone         = COALESCE(NULLIF($3,''), phone),
			email         = COALESCE(NULLIF($4,''), email),
			address       = COALESCE(NULLIF($5,''), address),
			province      = COALESCE(NULLIF($6,''), province),
			date_of_birth = COALESCE($7, date_of_birth),
			updated_at    = NOW()
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, f.FullName, f.Phone, f.Email, f.Address, f.Province, f.DateOfBirth); err != nil {
		return fmt.Errorf("candidates: update profile fields: %w", err)
	}
	return nil
}
