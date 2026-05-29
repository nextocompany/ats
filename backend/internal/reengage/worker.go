package reengage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/queue"
)

// HandleReengageVacancy is the asynq handler for TypeReengageVacancy. It decodes
// the payload and runs re-engagement for the position.
func (s *Service) HandleReengageVacancy(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseReengageVacancyPayload(t.Payload())
	if err != nil {
		return err
	}
	positionID, err := uuid.Parse(p.PositionID)
	if err != nil {
		return fmt.Errorf("reengage: invalid position_id %q: %w", p.PositionID, err)
	}
	sent, err := s.Reengage(ctx, positionID)
	if err != nil {
		return err
	}
	log.Info().Str("position_id", positionID.String()).Int("sent", sent).Msg("reengage: vacancy processed")
	return nil
}
