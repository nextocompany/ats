package candidates

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/rbac"
)

// Repository is the candidate data-access contract. The concrete implementation
// receives the pool via injection (never a global), per the project convention.
type Repository interface {
	Create(ctx context.Context, c Candidate) (Candidate, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error)
	// GetAccountMatchNames returns the candidate account's Thai + English names,
	// the two fields matched against the parsed resume name (a CV is in one
	// language, so the gate accepts a match against either). hasAccount is false
	// for accountless intakes (bulk/webhook), which have no registered name.
	GetAccountMatchNames(ctx context.Context, candidateID uuid.UUID) (nameTH, nameEN string, hasAccount bool, err error)
	List(ctx context.Context, f Filter, scope rbac.Scope) ([]Candidate, int, error)
	Timeline(ctx context.Context, id uuid.UUID) ([]activity.Entry, error)
	UpdateProfileFields(ctx context.Context, id uuid.UUID, f ProfileFields) error
	// FindDuplicates returns non-duplicate candidates (excluding excludeID) that
	// share an exact id_card, phone, or email with the given values.
	FindDuplicates(ctx context.Context, excludeID uuid.UUID, idCard, phone, email string) ([]Candidate, error)
	// MarkDuplicateOf records dupID as a duplicate of canonicalID and reconciles
	// the owning account (a member-linked dup donates its account to an accountless
	// canonical).
	MarkDuplicateOf(ctx context.Context, dupID, canonicalID uuid.UUID) error
	// SetAccountID links a candidate to an owning portal account, only when it has
	// no link yet (never overwrites an existing account). Used by silent at-intake
	// account provisioning (Phase 2).
	SetAccountID(ctx context.Context, id, accountID uuid.UUID) error
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
		INSERT INTO candidates (full_name, phone, email, id_card, address, province, subregion, date_of_birth, source_channel, status, line_user_id, account_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10,''), 'available'), $11, $12)
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, q,
		c.FullName, nullable(c.Phone), nullable(c.Email), nullable(c.IDCard),
		nullable(c.Address), nullable(c.Province), nullable(c.Subregion),
		c.DateOfBirth, nullable(c.SourceChannel), c.Status, nullable(c.LineUserID), c.AccountID,
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
		       date_of_birth, COALESCE(source_channel,''), status, COALESCE(line_user_id,''), created_at, account_id
		FROM candidates WHERE id = $1`
	var c Candidate
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.FullName, &c.Phone, &c.Email, &c.IDCard,
		&c.Address, &c.Province, &c.Subregion, &c.DateOfBirth,
		&c.SourceChannel, &c.Status, &c.LineUserID, &c.CreatedAt, &c.AccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("candidates: find by id: %w", err)
	}
	return &c, nil
}

func (r *pgRepository) GetAccountMatchNames(ctx context.Context, candidateID uuid.UUID) (string, string, bool, error) {
	const q = `SELECT COALESCE(a.name_th,''), COALESCE(a.name_en,'')
		FROM candidates c JOIN candidate_accounts a ON a.id = c.account_id
		WHERE c.id = $1`
	var nameTH, nameEN string
	err := r.pool.QueryRow(ctx, q, candidateID).Scan(&nameTH, &nameEN)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil // accountless candidate (no portal account)
	}
	if err != nil {
		return "", "", false, fmt.Errorf("candidates: get account match names: %w", err)
	}
	return nameTH, nameEN, true, nil
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
	// One transaction: flag the duplicate, then donate its account to the canonical
	// only when the canonical has no account yet. This covers a logged-in member
	// applying (their new row carries account_id) that auto-merges into an older
	// accountless walk-in canonical — the canonical inherits the member account so
	// the unified list attributes apps correctly. COALESCE + WHERE account_id IS
	// NULL guarantees a canonical that already has a real account is never clobbered.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("candidates: mark duplicate begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE candidates SET is_duplicate_of = $2, updated_at = NOW() WHERE id = $1`,
		dupID, canonicalID); err != nil {
		return fmt.Errorf("candidates: mark duplicate: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE candidates
		   SET account_id = (SELECT account_id FROM candidates WHERE id = $1), updated_at = NOW()
		 WHERE id = $2 AND account_id IS NULL`,
		dupID, canonicalID); err != nil {
		return fmt.Errorf("candidates: reconcile duplicate account: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("candidates: mark duplicate commit: %w", err)
	}
	return nil
}

func (r *pgRepository) SetAccountID(ctx context.Context, id, accountID uuid.UUID) error {
	const q = `UPDATE candidates SET account_id = $2, updated_at = NOW() WHERE id = $1 AND account_id IS NULL`
	if _, err := r.pool.Exec(ctx, q, id, accountID); err != nil {
		return fmt.Errorf("candidates: set account id: %w", err)
	}
	return nil
}
