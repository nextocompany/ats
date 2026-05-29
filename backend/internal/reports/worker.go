package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/queue"
)

// HandleExportReport is the asynq handler for TypeExportReport. The scheduler
// enqueues a static task (no period); a missing period is derived from the
// current ISO week so each run is labelled deterministically.
func (s *ExportService) HandleExportReport(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseExportReportPayload(t.Payload())
	if err != nil {
		return err
	}
	kind := p.Kind
	if kind == "" {
		kind = "weekly"
	}
	period := p.Period
	if period == "" {
		y, w := time.Now().UTC().ISOWeek()
		period = fmt.Sprintf("%d-W%02d", y, w)
	}
	exp, err := s.Export(ctx, kind, period)
	if err != nil {
		return err
	}
	log.Info().Str("kind", kind).Str("period", period).Bool("delivered", exp.Delivered).Msg("reports: export produced")
	return nil
}
