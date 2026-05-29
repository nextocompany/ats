// Package pdpa records and checks candidate PDPA consent (F13).
package pdpa

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Consent is a recorded consent decision.
type Consent struct {
	CandidateID   uuid.UUID  `json:"candidate_id"`
	ConsentGiven  bool       `json:"consent_given"`
	Version       string     `json:"consent_version"`
	SourceChannel string     `json:"source_channel"`
	RecordedAt    *time.Time `json:"recorded_at,omitempty"`
}

// Repo persists/reads consent.
type Repo struct{ pool *pgxpool.Pool }

// New builds the PDPA repository.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Record inserts a consent row and updates the candidate's consent snapshot.
func (r *Repo) Record(ctx context.Context, c Consent, ip string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pdpa: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO pdpa_consents (candidate_id, consent_given, consent_version, source_channel, ip_address)
		 VALUES ($1,$2,$3,$4,NULLIF($5,'')::inet)`,
		c.CandidateID, c.ConsentGiven, c.Version, c.SourceChannel, ip); err != nil {
		return fmt.Errorf("pdpa: insert consent: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE candidates SET pdpa_consent=$2, pdpa_consent_at=NOW(), pdpa_version=$3, updated_at=NOW() WHERE id=$1`,
		c.CandidateID, c.ConsentGiven, c.Version); err != nil {
		return fmt.Errorf("pdpa: update candidate: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pdpa: commit: %w", err)
	}
	return nil
}

// Latest returns the most recent consent for a candidate.
func (r *Repo) Latest(ctx context.Context, candidateID uuid.UUID) (Consent, error) {
	const q = `
		SELECT candidate_id, consent_given, COALESCE(consent_version,''), COALESCE(source_channel,''), created_at
		FROM pdpa_consents WHERE candidate_id = $1 ORDER BY created_at DESC LIMIT 1`
	var c Consent
	var at time.Time
	if err := r.pool.QueryRow(ctx, q, candidateID).Scan(&c.CandidateID, &c.ConsentGiven, &c.Version, &c.SourceChannel, &at); err != nil {
		return Consent{}, fmt.Errorf("pdpa: latest: %w", err)
	}
	c.RecordedAt = &at
	return c, nil
}
