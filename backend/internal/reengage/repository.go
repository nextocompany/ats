// Package reengage re-contacts talent-pool / prior candidates when a matching
// vacancy opens, feeding them back into the apply→pipeline. Sends ride the
// notify seam; a contact log enforces at-most-once outreach per (candidate, role).
package reengage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Target is a candidate eligible for re-engagement on a position.
type Target struct {
	CandidateID uuid.UUID
	FullName    string
	Phone       string
	Email       string
	Province    string
	LineUserID  string // verified LINE `sub` — the only valid LINE push recipient
}

// Repo is the re-engagement data-access contract (accept-interface in the service).
type Repo interface {
	// MatchingCandidates returns talent-pool / previously-rejected candidates for
	// the position who have not already been contacted about it.
	MatchingCandidates(ctx context.Context, positionID uuid.UUID) ([]Target, error)
	// RecordContact inserts a suppression row; reports whether it created one
	// (false means the candidate was already contacted for this position).
	RecordContact(ctx context.Context, candidateID, positionID uuid.UUID, channel string) (bool, error)
	// DormantCandidates returns contactable, never-hired candidates whose most
	// recent application is older than `months` months and who have not yet been
	// nudged under triggerType (e.g. "time_6mo"). Drives the time-based sweep.
	DormantCandidates(ctx context.Context, months int, triggerType string) ([]Target, error)
	// RecordTimeContact logs a time-based nudge; reports whether it created the row
	// (false ⇒ the candidate was already nudged for this trigger). At-most-once per
	// (candidate, trigger_type) via the partial unique index.
	RecordTimeContact(ctx context.Context, candidateID uuid.UUID, triggerType string) (bool, error)
}

type pgRepo struct{ pool *pgxpool.Pool }

// NewRepository builds a Postgres-backed re-engagement repository.
func NewRepository(pool *pgxpool.Pool) Repo { return &pgRepo{pool: pool} }

func (r *pgRepo) MatchingCandidates(ctx context.Context, positionID uuid.UUID) ([]Target, error) {
	// Talent-pool or rejected applicants for this position, excluding merged
	// duplicates and anyone already re-engaged for it.
	const q = `
		SELECT DISTINCT c.id, c.full_name, COALESCE(c.phone,''), COALESCE(c.email,''), COALESCE(c.province,''), COALESCE(c.line_user_id,'')
		FROM candidates c
		JOIN applications a ON a.candidate_id = c.id
		WHERE a.position_id = $1
		  AND (a.talent_pool IS TRUE OR a.status = 'rejected')
		  AND c.is_duplicate_of IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM reengagement_contacts rc
		      WHERE rc.candidate_id = c.id AND rc.position_id = $1
		  )`
	rows, err := r.pool.Query(ctx, q, positionID)
	if err != nil {
		return nil, fmt.Errorf("reengage: matching candidates: %w", err)
	}
	defer rows.Close()

	var out []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.CandidateID, &t.FullName, &t.Phone, &t.Email, &t.Province, &t.LineUserID); err != nil {
			return nil, fmt.Errorf("reengage: scan target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *pgRepo) RecordContact(ctx context.Context, candidateID, positionID uuid.UUID, channel string) (bool, error) {
	const q = `
		INSERT INTO reengagement_contacts (candidate_id, position_id, channel)
		VALUES ($1, $2, $3)
		ON CONFLICT (candidate_id, position_id) DO NOTHING
		RETURNING id`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, candidateID, positionID, channel).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already contacted for this position
	}
	if err != nil {
		return false, fmt.Errorf("reengage: record contact: %w", err)
	}
	return true, nil
}

func (r *pgRepo) DormantCandidates(ctx context.Context, months int, triggerType string) ([]Target, error) {
	// Contactable, never-hired candidates whose most recent application is older
	// than `months` months and who have not yet been nudged under this trigger.
	// Grouped so the age test is on the latest application; excludes merged dupes.
	const q = `
		SELECT c.id, c.full_name, COALESCE(c.phone,''), COALESCE(c.email,''), COALESCE(c.province,''), COALESCE(c.line_user_id,'')
		FROM candidates c
		JOIN applications a ON a.candidate_id = c.id
		WHERE c.is_duplicate_of IS NULL
		  AND (COALESCE(c.line_user_id,'') <> '' OR COALESCE(c.email,'') <> '')
		  AND NOT EXISTS (
		      SELECT 1 FROM applications h WHERE h.candidate_id = c.id AND h.status = 'hired'
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM reengagement_logs rl
		      WHERE rl.candidate_id = c.id AND rl.trigger_type = $2
		  )
		GROUP BY c.id, c.full_name, c.phone, c.email, c.province, c.line_user_id
		HAVING MAX(a.created_at) < now() - make_interval(months => $1)`
	rows, err := r.pool.Query(ctx, q, months, triggerType)
	if err != nil {
		return nil, fmt.Errorf("reengage: dormant candidates: %w", err)
	}
	defer rows.Close()

	var out []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.CandidateID, &t.FullName, &t.Phone, &t.Email, &t.Province, &t.LineUserID); err != nil {
			return nil, fmt.Errorf("reengage: scan dormant target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *pgRepo) RecordTimeContact(ctx context.Context, candidateID uuid.UUID, triggerType string) (bool, error) {
	const q = `
		INSERT INTO reengagement_logs (candidate_id, trigger_type)
		VALUES ($1, $2)
		ON CONFLICT (candidate_id, trigger_type) WHERE trigger_type IS NOT NULL DO NOTHING
		RETURNING id`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, candidateID, triggerType).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already nudged for this trigger
	}
	if err != nil {
		return false, fmt.Errorf("reengage: record time contact: %w", err)
	}
	return true, nil
}
