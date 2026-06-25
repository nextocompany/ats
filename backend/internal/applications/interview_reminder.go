package applications

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/notify"
)

// DueReminder is one appointment due for a "your interview is tomorrow" reminder,
// joined with the candidate contact handles so the sweep needs no second lookup.
type DueReminder struct {
	AppointmentID uuid.UUID
	RoundNo       int
	ScheduledAt   time.Time
	DurationMin   int
	Mode          string
	LocationText  string
	OnlineJoinURL string
	CandidateLine string // line_user_id ("" if none)
	CandidateMail string // email ("" if none)
	CandidateName string
	PublicToken   string
}

// ListAppointmentsDueForReminder returns latest-round appointments whose interview
// is within the next 24h, booked at least 24h in advance (so there is a genuine
// "1 day before" moment — avoids double-notifying same-day bookings), not yet
// reminded, on applications still in the interview stage. The wide 24h window lets
// a missed hourly tick self-heal (fires a few hours late) instead of dropping the
// reminder.
func (r *pgRepository) ListAppointmentsDueForReminder(ctx context.Context) ([]DueReminder, error) {
	const q = `
		SELECT ia.id, ia.round_no, ia.scheduled_at, ia.duration_min, ia.mode,
		       COALESCE(ia.location_text,''), COALESCE(ia.online_join_url,''),
		       COALESCE(c.line_user_id,''), COALESCE(c.email,''), COALESCE(c.full_name,''),
		       COALESCE(a.public_token,'')
		FROM interview_appointments ia
		JOIN applications a ON a.id = ia.application_id
		JOIN candidates c ON c.id = a.candidate_id
		WHERE a.status = $1
		  AND ia.reminder_sent_at IS NULL
		  AND ia.scheduled_at > NOW()
		  AND ia.scheduled_at <= NOW() + INTERVAL '24 hours'
		  AND ia.created_at <= ia.scheduled_at - INTERVAL '24 hours'
		  AND ia.round_no = (
		      SELECT MAX(x.round_no) FROM interview_appointments x
		      WHERE x.application_id = ia.application_id
		  )
		ORDER BY ia.scheduled_at ASC`
	rows, err := r.pool.Query(ctx, q, StatusInterview)
	if err != nil {
		return nil, fmt.Errorf("applications: list reminders due: %w", err)
	}
	defer rows.Close()

	out := make([]DueReminder, 0)
	for rows.Next() {
		var d DueReminder
		if err := rows.Scan(
			&d.AppointmentID, &d.RoundNo, &d.ScheduledAt, &d.DurationMin, &d.Mode,
			&d.LocationText, &d.OnlineJoinURL,
			&d.CandidateLine, &d.CandidateMail, &d.CandidateName, &d.PublicToken,
		); err != nil {
			return nil, fmt.Errorf("applications: scan reminder due: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MarkReminderSent stamps reminder_sent_at so a sweep retry (or the next tick)
// never re-sends. The IS NULL guard keeps it idempotent under any overlap.
func (r *pgRepository) MarkReminderSent(ctx context.Context, appointmentID uuid.UUID) error {
	const q = `UPDATE interview_appointments SET reminder_sent_at = NOW() WHERE id = $1 AND reminder_sent_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, appointmentID); err != nil {
		return fmt.Errorf("applications: mark reminder sent: %w", err)
	}
	return nil
}

// reminderStore is the narrow repository slice the reminder sweep needs.
type reminderStore interface {
	ListAppointmentsDueForReminder(ctx context.Context) ([]DueReminder, error)
	MarkReminderSent(ctx context.Context, appointmentID uuid.UUID) error
}

// InterviewReminderService sends candidates a "your interview is tomorrow"
// reminder ~1 day before a booked human interview. Wired into the worker as the
// handler for queue.TypeInterviewReminderSweep.
type InterviewReminderService struct {
	store         reminderStore
	notifier      notify.Notifier
	portalBaseURL string
}

// NewInterviewReminderService builds the reminder sweep service.
func NewInterviewReminderService(store reminderStore, notifier notify.Notifier, portalBaseURL string) *InterviewReminderService {
	return &InterviewReminderService{store: store, notifier: notifier, portalBaseURL: portalBaseURL}
}

// HandleInterviewReminderSweep finds the due appointments, sends each candidate a
// best-effort LINE + email reminder, and stamps reminder_sent_at per row so the
// reminder is sent at most once. A single row's send/mark failure is logged and
// skipped; only a failure to load the due set returns an error so asynq retries
// the whole sweep without re-sending already-stamped rows.
func (s *InterviewReminderService) HandleInterviewReminderSweep(ctx context.Context, _ *asynq.Task) error {
	due, err := s.store.ListAppointmentsDueForReminder(ctx)
	if err != nil {
		return err
	}
	sent := 0
	for _, d := range due {
		if s.notifier != nil {
			if msg := notify.InterviewReminderMessage(d.CandidateLine, d.CandidateName, d.RoundNo, d.DurationMin, d.ScheduledAt, d.Mode, d.LocationText, d.OnlineJoinURL, s.portalBaseURL, d.PublicToken); msg.Recipient != "" {
				if err := s.notifier.Send(ctx, msg); err != nil {
					log.Warn().Err(err).Str("appointment", d.AppointmentID.String()).Msg("interview reminder: line send failed (non-fatal)")
				}
			}
			if em := notify.InterviewReminderEmailMessage(d.CandidateMail, d.CandidateName, d.RoundNo, d.DurationMin, d.ScheduledAt, d.Mode, d.LocationText, d.OnlineJoinURL, s.portalBaseURL, d.PublicToken); em.Recipient != "" {
				if err := s.notifier.Send(ctx, em); err != nil {
					log.Warn().Err(err).Str("appointment", d.AppointmentID.String()).Msg("interview reminder: email send failed (non-fatal)")
				}
			}
		}
		// Stamp after the attempt regardless of per-channel result (and even when
		// the candidate has no handle) so a contactless row is never re-queried
		// every tick — at-most-once, matching the SLA sweep.
		if err := s.store.MarkReminderSent(ctx, d.AppointmentID); err != nil {
			log.Warn().Err(err).Str("appointment", d.AppointmentID.String()).Msg("interview reminder: mark sent failed (skip)")
			continue
		}
		sent++
	}
	log.Info().Int("due", len(due)).Int("reminded", sent).Msg("interview reminder sweep complete")
	return nil
}
