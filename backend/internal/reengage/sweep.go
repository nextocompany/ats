package reengage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/pkg/queue"
)

// TriggerType derives the suppression trigger key for a months threshold, e.g.
// 6 → "time_6mo". Kept here so the scheduler, handler, and repo agree on the key.
func TriggerType(months int) string { return fmt.Sprintf("time_%dmo", months) }

// SweepTimeBased nudges dormant candidates whose most recent application is older
// than `months` months, inviting them back to browse current openings. Mirrors
// Reengage: a single notify failure is logged and skipped; a DB failure aborts so
// asynq retries. The contact is recorded before sending (at-most-once per trigger),
// so a retry never double-sends. Returns how many were nudged.
func (s *Service) SweepTimeBased(ctx context.Context, months int) (int, error) {
	trigger := TriggerType(months)
	targets, err := s.repo.DormantCandidates(ctx, months, trigger)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, t := range targets {
		channel, recipient := pickChannel(t)
		if recipient == "" {
			continue // no usable channel (query already filters, but stay defensive)
		}

		inserted, err := s.repo.RecordTimeContact(ctx, t.CandidateID, trigger)
		if err != nil {
			return sent, fmt.Errorf("reengage: record time contact for candidate %s: %w", t.CandidateID, err)
		}
		if !inserted {
			continue // already nudged for this trigger
		}

		msg := notify.Message{
			Channel:   channel,
			Recipient: recipient,
			Subject:   "CP Axtra ยังเปิดรับสมัครงานอยู่",
			Body: fmt.Sprintf(
				"สวัสดีคุณ%s เรายังมีตำแหน่งงานใหม่ ๆ ที่เปิดรับอยู่ กลับมาดูและสมัครได้ที่ %s/jobs",
				t.FullName, s.portalBaseURL,
			),
		}
		if err := s.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("candidate_id", t.CandidateID.String()).Msg("reengage: time-based notify failed")
			continue
		}
		sent++
	}

	if err := s.audit.Record(ctx, activity.ActionReengage, "candidate_sweep", uuid.Nil, map[string]any{
		"trigger": trigger, "matched": len(targets), "sent": sent,
	}); err != nil {
		log.Warn().Err(err).Str("trigger", trigger).Msg("reengage: sweep audit record failed")
	}
	return sent, nil
}

// HandleReengageSweep is the asynq handler for TypeReengageSweep. It decodes the
// months threshold and runs the time-based sweep for it.
func (s *Service) HandleReengageSweep(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseReengageSweepPayload(t.Payload())
	if err != nil {
		return err
	}
	if p.MonthsSince <= 0 {
		return fmt.Errorf("reengage: invalid months_since %d", p.MonthsSince)
	}
	sent, err := s.SweepTimeBased(ctx, p.MonthsSince)
	if err != nil {
		return err
	}
	log.Info().Int("months", p.MonthsSince).Int("sent", sent).Msg("reengage: time-based sweep processed")
	return nil
}
