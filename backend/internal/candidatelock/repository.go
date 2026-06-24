package candidatelock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed lock repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Acquire(ctx context.Context, candidateID, byUser uuid.UUID, ttl time.Duration) (Lock, error) {
	// Atomic take-or-refresh: insert, or on conflict update only when the existing
	// lock is expired or already owned by byUser. A live lock held by someone else
	// leaves the row untouched and returns no row (→ ErrLockedByOther below).
	const q = `
		INSERT INTO candidate_locks (candidate_id, locked_by, locked_at, expires_at)
		VALUES ($1, $2, now(), now() + make_interval(secs => $3))
		ON CONFLICT (candidate_id) DO UPDATE
			SET locked_by = EXCLUDED.locked_by, locked_at = now(), expires_at = EXCLUDED.expires_at
			WHERE candidate_locks.expires_at <= now() OR candidate_locks.locked_by = $2
		RETURNING candidate_id`
	var got uuid.UUID
	err := r.pool.QueryRow(ctx, q, candidateID, byUser, int(ttl.Seconds())).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		// A live lock is held by another user — report the current holder.
		if held, gerr := r.Get(ctx, candidateID); gerr == nil && held != nil {
			return *held, ErrLockedByOther
		}
		return Lock{}, ErrLockedByOther
	}
	if err != nil {
		return Lock{}, fmt.Errorf("candidatelock: acquire: %w", err)
	}
	held, err := r.Get(ctx, candidateID)
	if err != nil || held == nil {
		return Lock{}, fmt.Errorf("candidatelock: read after acquire: %w", err)
	}
	return *held, nil
}

func (r *pgRepository) Release(ctx context.Context, candidateID, byUser uuid.UUID, force bool) error {
	const q = `DELETE FROM candidate_locks WHERE candidate_id = $1 AND ($3 OR locked_by = $2)`
	if _, err := r.pool.Exec(ctx, q, candidateID, byUser, force); err != nil {
		return fmt.Errorf("candidatelock: release: %w", err)
	}
	return nil
}

func (r *pgRepository) Get(ctx context.Context, candidateID uuid.UUID) (*Lock, error) {
	const q = `
		SELECT cl.candidate_id, cl.locked_by, COALESCE(u.full_name, ''), cl.locked_at, cl.expires_at
		FROM candidate_locks cl
		LEFT JOIN users u ON u.id = cl.locked_by
		WHERE cl.candidate_id = $1 AND cl.expires_at > now()`
	var l Lock
	err := r.pool.QueryRow(ctx, q, candidateID).Scan(&l.CandidateID, &l.LockedBy, &l.LockedByName, &l.LockedAt, &l.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("candidatelock: get: %w", err)
	}
	return &l, nil
}
