package reengage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/notify"
)

// Service finds matching candidates for a position and notifies them about the
// new opening, recording each contact for suppression and an audit entry.
type Service struct {
	repo          Repo
	notifier      notify.Notifier
	audit         activity.Writer
	portalBaseURL string
}

// NewService builds the re-engagement service.
func NewService(repo Repo, notifier notify.Notifier, audit activity.Writer, portalBaseURL string) *Service {
	return &Service{repo: repo, notifier: notifier, audit: audit, portalBaseURL: portalBaseURL}
}

// Reengage notifies eligible candidates about a (re-)opened position and returns
// how many were contacted. A single notify failure never fails the run — it is
// logged and skipped (mirrors peoplesoft.Service: external failure ≠ core
// failure). Contact is recorded before sending so retries cannot double-send.
func (s *Service) Reengage(ctx context.Context, positionID uuid.UUID) (int, error) {
	targets, err := s.repo.MatchingCandidates(ctx, positionID)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, t := range targets {
		channel, recipient := pickChannel(t)
		if recipient == "" {
			log.Warn().Str("candidate_id", t.CandidateID.String()).Msg("reengage: no contact channel — skipping")
			continue
		}

		inserted, err := s.repo.RecordContact(ctx, t.CandidateID, positionID, channel)
		if err != nil {
			// A DB failure is a real failure — let the job retry. Suppression makes
			// the retry safe (already-contacted candidates are skipped).
			return sent, fmt.Errorf("reengage: record contact for candidate %s: %w", t.CandidateID, err)
		}
		if !inserted {
			continue // already contacted for this position
		}

		msg := notify.Message{
			Channel:   channel,
			Recipient: recipient,
			Subject:   "มีตำแหน่งงานใหม่ที่เหมาะกับคุณ",
			Body: fmt.Sprintf(
				"สวัสดีคุณ%s มีตำแหน่งงานใหม่ที่เปิดรับ สมัครได้ที่ %s/jobs/%s",
				t.FullName, s.portalBaseURL, positionID,
			),
		}
		if err := s.notifier.Send(ctx, msg); err != nil {
			// Contact row already recorded (at-most-once); log and move on.
			log.Warn().Err(err).Str("candidate_id", t.CandidateID.String()).Msg("reengage: notify failed")
			continue
		}
		sent++
	}

	if err := s.audit.Record(ctx, activity.ActionReengage, "position", positionID, map[string]int{
		"matched": len(targets), "sent": sent,
	}); err != nil {
		log.Warn().Err(err).Str("position_id", positionID.String()).Msg("reengage: audit record failed")
	}
	return sent, nil
}

// pickChannel prefers a real LINE push (requires the stored LINE user id), then
// falls back to email. Phone is NOT a valid LINE recipient, so it is no longer
// used as a handle. Returns ("", "") when the candidate has no usable channel.
func pickChannel(t Target) (channel, recipient string) {
	if t.LineUserID != "" {
		return notify.ChannelLINE, t.LineUserID
	}
	if t.Email != "" {
		return notify.ChannelEmail, t.Email
	}
	return "", ""
}
