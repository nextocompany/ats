package applications

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/queue"
)

// poolReleaseStore is the narrow repository slice the pool-release sweep needs.
type poolReleaseStore interface {
	ReleaseStalePoolCandidates(ctx context.Context, graceDays int) (int, error)
}

// PoolReleaseService releases store-specific candidates that no store HR acted on
// within the grace window back into the shared central pool. Wired into the worker
// as the handler for queue.TypePoolReleaseSweep.
//
// SAFETY: this depends on picked_up_at being stamped on the first store-HR action.
// Until that stamping is wired, EVERY store-specific application older than the
// grace window would be released — so the scheduler keeps this sweep disabled by
// default (POOL_RELEASE_ENABLED=false).
type PoolReleaseService struct {
	store        poolReleaseStore
	defaultGrace int
}

// NewPoolReleaseService builds the sweep service. A non-positive defaultGrace
// falls back to 3 days.
func NewPoolReleaseService(store poolReleaseStore, defaultGrace int) *PoolReleaseService {
	if defaultGrace <= 0 {
		defaultGrace = 3
	}
	return &PoolReleaseService{store: store, defaultGrace: defaultGrace}
}

// HandlePoolReleaseSweep releases stale store-specific candidates to the pool. The
// payload grace window overrides the default when positive.
func (s *PoolReleaseService) HandlePoolReleaseSweep(ctx context.Context, t *asynq.Task) error {
	grace := s.defaultGrace
	if t != nil {
		if p, err := queue.ParsePoolReleaseSweepPayload(t.Payload()); err == nil && p.GraceDays > 0 {
			grace = p.GraceDays
		}
	}
	n, err := s.store.ReleaseStalePoolCandidates(ctx, grace)
	if err != nil {
		return err // let asynq retry the whole sweep
	}
	log.Info().Int("released", n).Int("grace_days", grace).Msg("pool release sweep: store-specific candidates returned to central pool")
	return nil
}
