package candidateauth

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/queue"
)

// defaultCleanupBatch is the per-DELETE row cap when the payload/config omits one.
const defaultCleanupBatch = 500

// maxCleanupIterations bounds a single run so a large backlog drains over a few
// batches without an unbounded loop (maxIterations × batch rows per table).
const maxCleanupIterations = 1000

// CleanupService deletes expired candidate auth artifacts (email OTPs and
// sessions). It holds the pool directly and runs batched DELETEs, mirroring the
// PDPA retention service. Wired onto the scheduler→worker cadence.
type CleanupService struct {
	pool *pgxpool.Pool
}

// NewCleanupService builds the auth-cleanup service.
func NewCleanupService(pool *pgxpool.Pool) *CleanupService {
	return &CleanupService{pool: pool}
}

const (
	deleteExpiredOTPsSQL = `
		DELETE FROM email_otps
		WHERE id IN (
			SELECT id FROM email_otps
			WHERE consumed_at IS NOT NULL OR expires_at < NOW()
			LIMIT $1
		)`
	deleteExpiredSessionsSQL = `
		DELETE FROM candidate_sessions
		WHERE id IN (
			SELECT id FROM candidate_sessions
			WHERE revoked_at IS NOT NULL OR expires_at < NOW()
			LIMIT $1
		)`
)

// CleanExpired deletes expired/consumed OTPs and expired/revoked sessions in
// batches. Returns the per-table counts deleted. A failure on one table is
// returned (the run is retried by asynq); partial progress is durable.
func (s *CleanupService) CleanExpired(ctx context.Context, batch int) (otps, sessions int, err error) {
	if batch <= 0 {
		batch = defaultCleanupBatch
	}
	otps, err = s.deleteBatched(ctx, deleteExpiredOTPsSQL, batch)
	if err != nil {
		return otps, 0, err
	}
	sessions, err = s.deleteBatched(ctx, deleteExpiredSessionsSQL, batch)
	return otps, sessions, err
}

// deleteBatched runs the DELETE repeatedly (LIMIT batch) until a run removes
// fewer than batch rows (drained) or the iteration cap is hit.
func (s *CleanupService) deleteBatched(ctx context.Context, sql string, batch int) (int, error) {
	total := 0
	for i := 0; i < maxCleanupIterations; i++ {
		tag, err := s.pool.Exec(ctx, sql, batch)
		if err != nil {
			return total, fmt.Errorf("candidateauth: cleanup delete: %w", err)
		}
		n := int(tag.RowsAffected())
		total += n
		if n < batch {
			break
		}
	}
	return total, nil
}

// HandleAuthCleanup is the asynq handler for TypeAuthCleanup.
func (s *CleanupService) HandleAuthCleanup(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseAuthCleanupPayload(t.Payload())
	if err != nil {
		return err
	}
	otps, sessions, err := s.CleanExpired(ctx, p.Batch)
	if err != nil {
		return err
	}
	log.Info().Int("otps", otps).Int("sessions", sessions).Msg("candidateauth: auth cleanup complete")
	return nil
}
