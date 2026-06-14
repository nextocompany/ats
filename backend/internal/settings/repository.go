// Package settings stores runtime, admin-managed configuration flags in a small
// key/value table (system_settings). The first flag is the "allow all Entra
// tenants" sign-in toggle, read by the auth middleware on the hot path.
package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the persistence seam for system settings.
type Repository interface {
	// GetBool returns the boolean value for key. A missing key (or NULL value)
	// returns false — callers treat absence as the safe default.
	GetBool(ctx context.Context, key string) (bool, error)
	// SetBool upserts the boolean value for key and records who changed it.
	SetBool(ctx context.Context, key string, val bool, updatedBy string) error
}

type pgRepository struct{ pool *pgxpool.Pool }

// NewRepository builds the Postgres-backed settings repository.
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }

func (r *pgRepository) GetBool(ctx context.Context, key string) (bool, error) {
	const q = `SELECT value_bool FROM system_settings WHERE key = $1`
	var val *bool
	if err := r.pool.QueryRow(ctx, q, key).Scan(&val); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("settings: get %q: %w", key, err)
	}
	if val == nil {
		return false, nil
	}
	return *val, nil
}

func (r *pgRepository) SetBool(ctx context.Context, key string, val bool, updatedBy string) error {
	const q = `
		INSERT INTO system_settings (key, value_bool, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value_bool = EXCLUDED.value_bool,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = NOW()`
	if _, err := r.pool.Exec(ctx, q, key, val, updatedBy); err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	return nil
}
