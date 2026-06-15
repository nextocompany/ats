package hrauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const uniqueViolation = "23505"

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed hrauth repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// userColumns is the canonical projection shared by every read.
const userColumns = `id, email, COALESCE(full_name, ''), COALESCE(role, ''),
	store_id, COALESCE(subregion, ''), is_active,
	(password_hash IS NOT NULL) AS has_password, last_login_at, created_at`

// scanUser reads a row in userColumns order.
func scanUser(row pgx.Row) (User, error) {
	var u User
	if err := row.Scan(
		&u.ID, &u.Email, &u.FullName, &u.Role,
		&u.StoreID, &u.Subregion, &u.IsActive,
		&u.HasPassword, &u.LastLoginAt, &u.CreatedAt,
	); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *pgRepository) FindCredentialsByEmail(ctx context.Context, email string) (User, string, error) {
	const q = `SELECT ` + userColumns + `, password_hash
		FROM users
		WHERE lower(email) = $1 AND is_active = TRUE AND password_hash IS NOT NULL`
	var u User
	var hash string
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.FullName, &u.Role,
		&u.StoreID, &u.Subregion, &u.IsActive,
		&u.HasPassword, &u.LastLoginAt, &u.CreatedAt, &hash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("hrauth: find credentials: %w", err)
	}
	return u, hash, nil
}

func (r *pgRepository) TouchLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("hrauth: touch last login: %w", err)
	}
	return nil
}

func (r *pgRepository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO hr_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("hrauth: create session: %w", err)
	}
	return nil
}

func (r *pgRepository) FindSessionUser(ctx context.Context, tokenHash string) (User, error) {
	const q = `SELECT u.id, u.email, COALESCE(u.full_name, ''), COALESCE(u.role, ''),
		u.store_id, COALESCE(u.subregion, ''), u.is_active,
		(u.password_hash IS NOT NULL), u.last_login_at, u.created_at
		FROM hr_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		  AND u.is_active = TRUE`
	u, err := scanUser(r.pool.QueryRow(ctx, q, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("hrauth: find session user: %w", err)
	}
	return u, nil
}

func (r *pgRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE hr_sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	if err != nil {
		return fmt.Errorf("hrauth: revoke session: %w", err)
	}
	return nil
}

func (r *pgRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE hr_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID)
	if err != nil {
		return fmt.Errorf("hrauth: revoke all sessions: %w", err)
	}
	return nil
}

func (r *pgRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY email`)
	if err != nil {
		return nil, fmt.Errorf("hrauth: list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("hrauth: scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hrauth: list users: %w", err)
	}
	return out, nil
}

func (r *pgRepository) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("hrauth: get user: %w", err)
	}
	return u, nil
}

func (r *pgRepository) CreateUser(ctx context.Context, email, fullName, role string, storeID *int, subregion, passwordHash string) (User, error) {
	const q = `INSERT INTO users (email, full_name, role, store_id, subregion, is_active, password_hash, password_updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), TRUE, $6, now())
		RETURNING ` + userColumns
	u, err := scanUser(r.pool.QueryRow(ctx, q, email, fullName, role, storeID, subregion, passwordHash))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return User{}, ErrEmailExists
	}
	if err != nil {
		return User{}, fmt.Errorf("hrauth: create user: %w", err)
	}
	return u, nil
}

func (r *pgRepository) UpdateUser(ctx context.Context, id uuid.UUID, in UpdateUserInput, passwordHash *string) (User, error) {
	set := []string{}
	args := []any{}
	add := func(expr string, val any) {
		args = append(args, val)
		set = append(set, fmt.Sprintf("%s = $%d", expr, len(args)))
	}
	if in.FullName != nil {
		add("full_name", *in.FullName)
	}
	if in.Role != nil {
		add("role", *in.Role)
	}
	if in.StoreID != nil {
		add("store_id", *in.StoreID)
	}
	if in.Subregion != nil {
		// Store an empty subregion as NULL to match CreateUser's NULLIF and the
		// COALESCE(subregion, '') reads — "" and NULL must not diverge.
		var v *string
		if *in.Subregion != "" {
			v = in.Subregion
		}
		add("subregion", v)
	}
	if in.IsActive != nil {
		add("is_active", *in.IsActive)
	}
	if passwordHash != nil {
		add("password_hash", *passwordHash)
		set = append(set, "password_updated_at = now()")
	}
	if len(set) == 0 {
		return r.GetUser(ctx, id) // nothing to change
	}
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(set, ", "), len(args), userColumns)
	u, err := scanUser(r.pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("hrauth: update user: %w", err)
	}
	return u, nil
}
