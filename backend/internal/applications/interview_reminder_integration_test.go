//go:build integration

package applications

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/notify"
)

// countingNotifier records every Send so a sweep's send count can be asserted.
type countingNotifier struct {
	mu   sync.Mutex
	msgs []notify.Message
}

func (n *countingNotifier) Send(_ context.Context, m notify.Message) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.msgs = append(n.msgs, m)
	return nil
}
func (n *countingNotifier) count() int { n.mu.Lock(); defer n.mu.Unlock(); return len(n.msgs) }

// seedReminderCandidate creates a candidate with both contact handles.
func seedReminderCandidate(t *testing.T, r *pgRepository, line, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := r.pool.QueryRow(context.Background(),
		`INSERT INTO candidates (full_name, source_channel, status, line_user_id, email)
		 VALUES ('สมชาย','career_portal','available',$1,$2) RETURNING id`, line, email).Scan(&id); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	return id
}

// seedInterviewApp inserts an application (status=interview) + an appointment with
// explicit scheduled_at/created_at/round_no so window + lead-time guards can be
// exercised. Returns the appointment id.
func seedInterviewApp(t *testing.T, r *pgRepository, candID, posID uuid.UUID, status string, scheduledAt, createdAt time.Time, roundNo int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var appID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, public_token) VALUES ($1,$2,$3,$4) RETURNING id`,
		candID, posID, status, "tok-"+uuid.NewString()[:8]).Scan(&appID); err != nil {
		t.Fatalf("seed app: %v", err)
	}
	var apptID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO interview_appointments (application_id, scheduled_at, duration_min, mode, location_text, round_no, created_at)
		 VALUES ($1,$2,60,'onsite','สาขา CM',$3,$4) RETURNING id`,
		appID, scheduledAt, roundNo, createdAt).Scan(&apptID); err != nil {
		t.Fatalf("seed appointment: %v", err)
	}
	return apptID
}

func TestListAppointmentsDueForReminder_Filters(t *testing.T) {
	ctx := context.Background()
	r, pos, _ := setupList(t)
	cand := seedReminderCandidate(t, r, "U1", "a@b.com")
	now := time.Now()

	// DUE: interview 12h away, booked 3 days ago.
	due := seedInterviewApp(t, r, cand, pos, StatusInterview, now.Add(12*time.Hour), now.Add(-72*time.Hour), 1)
	// EXCLUDE: >24h away.
	seedInterviewApp(t, r, cand, pos, StatusInterview, now.Add(30*time.Hour), now.Add(-72*time.Hour), 1)
	// EXCLUDE: booked only 2h ago (no genuine "1 day before" moment).
	seedInterviewApp(t, r, cand, pos, StatusInterview, now.Add(12*time.Hour), now.Add(-2*time.Hour), 1)
	// EXCLUDE: application no longer in interview stage.
	seedInterviewApp(t, r, cand, pos, StatusInterviewed, now.Add(12*time.Hour), now.Add(-72*time.Hour), 1)

	items, err := r.ListAppointmentsDueForReminder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("due count = %d, want 1; got %v", len(items), apptIDs(items))
	}
	if items[0].AppointmentID != due {
		t.Errorf("wrong appointment surfaced: %v, want %v", items[0].AppointmentID, due)
	}
	if items[0].CandidateLine != "U1" || items[0].CandidateMail != "a@b.com" {
		t.Errorf("contact not joined: line=%q mail=%q", items[0].CandidateLine, items[0].CandidateMail)
	}
}

func TestListAppointmentsDueForReminder_LatestRoundOnly(t *testing.T) {
	ctx := context.Background()
	r, pos, _ := setupList(t)
	cand := seedReminderCandidate(t, r, "U2", "c@d.com")
	now := time.Now()

	// Rescheduled application: round 1 (superseded) and round 2 (current), both in
	// the next-day window. Only the latest round should be reminded.
	var appID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, public_token) VALUES ($1,$2,$3,'tok-r') RETURNING id`,
		cand, pos, StatusInterview).Scan(&appID); err != nil {
		t.Fatal(err)
	}
	insert := func(sched time.Time, round int) uuid.UUID {
		var id uuid.UUID
		if err := r.pool.QueryRow(ctx,
			`INSERT INTO interview_appointments (application_id, scheduled_at, duration_min, mode, round_no, created_at)
			 VALUES ($1,$2,60,'onsite',$3,$4) RETURNING id`,
			appID, sched, round, now.Add(-72*time.Hour)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert(now.Add(6*time.Hour), 1)
	r2 := insert(now.Add(20*time.Hour), 2)

	items, err := r.ListAppointmentsDueForReminder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AppointmentID != r2 {
		t.Fatalf("want only latest round %v, got %v", r2, apptIDs(items))
	}
}

// TestInterviewReminderSweep_AtMostOnce is the anchor: running the sweep twice
// against the same due appointment sends exactly one reminder (LINE + email = 2
// messages, on the FIRST sweep only).
func TestInterviewReminderSweep_AtMostOnce(t *testing.T) {
	ctx := context.Background()
	r, pos, _ := setupList(t)
	cand := seedReminderCandidate(t, r, "U3", "e@f.com")
	now := time.Now()
	seedInterviewApp(t, r, cand, pos, StatusInterview, now.Add(12*time.Hour), now.Add(-72*time.Hour), 1)
	// A late booking that must NOT be reminded (lead < 24h).
	seedInterviewApp(t, r, cand, pos, StatusInterview, now.Add(10*time.Hour), now.Add(-1*time.Hour), 1)

	notifier := &countingNotifier{}
	svc := NewInterviewReminderService(r, notifier, "https://portal")

	if err := svc.HandleInterviewReminderSweep(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := notifier.count(); got != 2 {
		t.Fatalf("first sweep sends = %d, want 2 (1 line + 1 email for the single due appt)", got)
	}
	if err := svc.HandleInterviewReminderSweep(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := notifier.count(); got != 2 {
		t.Errorf("after second sweep sends = %d, want still 2 (at-most-once)", got)
	}
}

func apptIDs(items []DueReminder) []uuid.UUID {
	out := make([]uuid.UUID, len(items))
	for i, d := range items {
		out[i] = d.AppointmentID
	}
	return out
}
