package pdpa

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/queue"
)

// HandleRetentionSweep is the asynq handler for TypeRetentionSweep. The scheduler
// enqueues a static task (no batch); a zero batch falls back to the service
// default. The enable-gate lives at the scheduler, so a disabled environment
// never enqueues this in the first place.
func (s *RetentionService) HandleRetentionSweep(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseRetentionSweepPayload(t.Payload())
	if err != nil {
		return err
	}
	n, err := s.Sweep(ctx, p.Batch)
	if err != nil {
		return err
	}
	log.Info().Int("anonymized", n).Msg("pdpa: retention sweep complete")
	return nil
}
