package applications

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
)

// statusNotifyDeps bundles the best-effort candidate-notification dependencies
// shared by the intake and dashboard handlers. All fields are optional: when
// unset (tests, or NOTIFY disabled) notifyStatusChange is a no-op.
type statusNotifyDeps struct {
	notifier      notify.Notifier
	cands         candidates.Repository
	portalBaseURL string
}

// notifyStatusChange sends a best-effort LINE message to the candidate after a
// status transition. It never returns an error — a notify failure must not affect
// the HR action. No-op when deps are unset, the status isn't candidate-notifiable,
// or the candidate has no LINE handle.
func (d statusNotifyDeps) notifyStatusChange(ctx context.Context, apps Repository, appID uuid.UUID, status string) {
	if d.notifier == nil || d.cands == nil {
		return
	}
	app, err := apps.FindByID(ctx, appID)
	if err != nil {
		log.Warn().Err(err).Str("application", appID.String()).Msg("status notify: load application failed")
		return
	}
	cand, err := d.cands.FindByID(ctx, app.CandidateID)
	if err != nil {
		log.Warn().Err(err).Str("candidate", app.CandidateID.String()).Msg("status notify: load candidate failed")
		return
	}
	msg := notify.StatusMessage(cand.LineUserID, cand.FullName, status, d.portalBaseURL)
	if msg.Recipient == "" {
		return // not notifiable, or candidate has no LINE handle
	}
	if err := d.notifier.Send(ctx, msg); err != nil {
		log.Warn().Err(err).Str("application", appID.String()).Msg("status notify: send failed (non-fatal)")
	}
}
