package candidates

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/rbac"
)

// Repository is the candidate data-access contract. The concrete implementation
// receives the pool via injection (never a global), per the project convention.
type Repository interface {
	Create(ctx context.Context, c Candidate) (Candidate, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error)
	List(ctx context.Context, f Filter, scope rbac.Scope) ([]Candidate, int, error)
	Timeline(ctx context.Context, id uuid.UUID) ([]activity.Entry, error)
	UpdateProfileFields(ctx context.Context, id uuid.UUID, f ProfileFields) error
	// FindDuplicates returns non-duplicate candidates (excluding excludeID) that
	// share an exact id_card, phone, or email with the given values.
	FindDuplicates(ctx context.Context, excludeID uuid.UUID, idCard, phone, email string) ([]Candidate, error)
	// MarkDuplicateOf records dupID as a duplicate of canonicalID.
	MarkDuplicateOf(ctx context.Context, dupID, canonicalID uuid.UUID) error
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

func (r *pgRepository) FindDuplicates(ctx context.Context, excludeID uuid.UUID, idCard, phone, email string) ([]Candidate, error) {
	// Only match on non-empty contact fields; ignore self and existing duplicates.
	const q = `
		SELECT id, full_name, COALESCE(phone,''), COALESCE(email,''), COALESCE(id_card,''),
		       COALESCE(province,''), status, created_at
		FROM candidates
		WHERE id <> $1 AND is_duplicate_of IS NULL
		AND ( (NULLIF($2,'') IS NOT NULL AND id_card = $2)
		   OR (NULLIF($3,'') IS NOT NULL AND phone   = $3)
		   OR (NULLIF($4,'') IS NOT NULL AND email   = $4) )`
	rows, err := r.pool.Query(ctx, q, excludeID, idCard, phone, email)
	if err != nil {
		return nil, fmt.Errorf("candidates: find duplicates: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.FullName, &c.Phone, &c.Email, &c.IDCard, &c.Province, &c.Status, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("candidates: scan duplicate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("candidates: duplicate rows: %w", err)
	}
	return out, nil
}

func (r *pgRepository) MarkDuplicateOf(ctx context.Context, dupID, canonicalID uuid.UUID) error {
	const q = `UPDATE candidates SET is_duplicate_of = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, dupID, canonicalID); err != nil {
		return fmt.Errorf("candidates: mark duplicate: %w", err)
	}
	return nil
}
