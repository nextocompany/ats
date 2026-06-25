//go:build integration

package applications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupOffer seeds an account, a candidate linked to it, a position, and an
// application at the 'offer' stage. The offer row itself is created per-test so
// each case controls its starting status. Returns the repo + key ids. Reuses dsn().
func setupOffer(t *testing.T) (r *pgRepository, accountA, accountB, appID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to >=48?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE offers, applications, application_status_history, candidates, candidate_accounts, positions, stores, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidate_accounts (full_name) VALUES ('Owner A') RETURNING id`).Scan(&accountA); err != nil {
		t.Fatalf("seed account A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidate_accounts (full_name) VALUES ('Attacker B') RETURNING id`).Scan(&accountB); err != nil {
		t.Fatalf("seed account B: %v", err)
	}
	var posID, candID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, source_channel, status, account_id) VALUES ('c','career_portal','available',$1) RETURNING id`,
		accountA).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'offer') RETURNING id`,
		candID, posID).Scan(&appID); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return &pgRepository{pool: pool}, accountA, accountB, appID
}

// seedSentOffer inserts a sent offer for appID with the given round, returning its id.
func seedSentOffer(t *testing.T, r *pgRepository, appID uuid.UUID, round int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	start := time.Now().Add(7 * 24 * time.Hour)
	if err := r.pool.QueryRow(context.Background(),
		`INSERT INTO offers (application_id, status, salary, start_date, sent_at, negotiation_round)
		 VALUES ($1,'sent',20000,$2,NOW(),$3) RETURNING id`, appID, start, round).Scan(&id); err != nil {
		t.Fatalf("seed sent offer: %v", err)
	}
	return id
}

func TestOffer_BenefitsRoundTrip(t *testing.T) {
	ctx := context.Background()
	r, _, _, appID := setupOffer(t)
	var userID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, full_name, role, source) VALUES ('hr@example.com','HR','hr_staff','local') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	in := OfferInput{
		Salary:   ptrF(20000),
		Benefits: []Benefit{{Label: "ประกันสังคม", Value: "ตามกฎหมาย"}, {Label: "โบนัส", Value: "2 เดือน/ปี"}},
	}
	if _, err := r.CreateOffer(ctx, appID, userID, in); err != nil {
		t.Fatalf("create offer: %v", err)
	}
	got, err := r.GetOfferByApplication(ctx, appID)
	if err != nil || got == nil {
		t.Fatalf("get offer: %v", err)
	}
	if len(got.Benefits) != 2 || got.Benefits[0].Label != "ประกันสังคม" || got.Benefits[1].Value != "2 เดือน/ปี" {
		t.Fatalf("benefits round-trip = %+v", got.Benefits)
	}
}

func TestOffer_NegotiateFromSent(t *testing.T) {
	ctx := context.Background()
	r, accountA, _, appID := setupOffer(t)
	offerID := seedSentOffer(t, r, appID, 0)

	o, err := r.NegotiateOffer(ctx, offerID, accountA, ptrF(25000), "ขอเพิ่มค่าเดินทาง", 3)
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if o.Status != OfferNegotiating || o.NegotiationRound != 1 {
		t.Fatalf("after negotiate status=%q round=%d, want negotiating/1", o.Status, o.NegotiationRound)
	}
	if o.CounterSalary == nil || *o.CounterSalary != 25000 {
		t.Fatalf("counter_salary = %v, want 25000", o.CounterSalary)
	}
	// Application must stay at 'offer' (negotiation does not move the funnel).
	app, _ := r.FindByID(ctx, appID)
	if app.Status != StatusOffer {
		t.Fatalf("application status = %q, want offer", app.Status)
	}
	// A second negotiate without an HR re-send (status now 'negotiating') conflicts.
	if _, err := r.NegotiateOffer(ctx, offerID, accountA, ptrF(26000), "", 3); !errors.Is(err, ErrOfferConflict) {
		t.Fatalf("re-negotiate err = %v, want ErrOfferConflict", err)
	}
}

func TestOffer_NegotiateRoundCap(t *testing.T) {
	ctx := context.Background()
	r, accountA, _, appID := setupOffer(t)
	offerID := seedSentOffer(t, r, appID, 3) // already at the cap

	if _, err := r.NegotiateOffer(ctx, offerID, accountA, ptrF(25000), "", 3); !errors.Is(err, ErrNegotiationClosed) {
		t.Fatalf("at-cap negotiate err = %v, want ErrNegotiationClosed", err)
	}
}

func TestOffer_NegotiateCrossAccount(t *testing.T) {
	ctx := context.Background()
	r, _, accountB, appID := setupOffer(t)
	offerID := seedSentOffer(t, r, appID, 0)

	if _, err := r.NegotiateOffer(ctx, offerID, accountB, ptrF(25000), "", 3); !errors.Is(err, ErrOfferNotFound) {
		t.Fatalf("cross-account negotiate err = %v, want ErrOfferNotFound (no IDOR)", err)
	}
}

func TestOffer_ReopenClearsExpiryAndAllowsResend(t *testing.T) {
	ctx := context.Background()
	r, accountA, _, appID := setupOffer(t)
	offerID := seedSentOffer(t, r, appID, 0)
	if _, err := r.NegotiateOffer(ctx, offerID, accountA, ptrF(25000), "", 3); err != nil {
		t.Fatalf("negotiate: %v", err)
	}

	reopened, err := r.ReopenOffer(ctx, appID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Status != OfferDraft || reopened.ExpiresAt != nil || reopened.SentAt != nil {
		t.Fatalf("reopened = status:%q expires:%v sent:%v, want draft/nil/nil", reopened.Status, reopened.ExpiresAt, reopened.SentAt)
	}
	// HR revises (round is preserved) then re-sends through the existing path.
	if _, err := r.UpdateOffer(ctx, appID, OfferInput{Salary: ptrF(23000), StartDate: ptrT(time.Now().Add(7 * 24 * time.Hour))}); err != nil {
		t.Fatalf("update after reopen: %v", err)
	}
	sent, err := r.SendOffer(ctx, appID)
	if err != nil {
		t.Fatalf("re-send: %v", err)
	}
	if sent.Status != OfferSent || sent.SentAt == nil {
		t.Fatalf("re-sent = status:%q sent_at:%v, want sent/non-nil", sent.Status, sent.SentAt)
	}
	if sent.NegotiationRound != 1 {
		t.Fatalf("negotiation_round after reopen+resend = %d, want 1 (preserved)", sent.NegotiationRound)
	}
}

func TestOffer_WithdrawReconcilesOfferAndApplication(t *testing.T) {
	ctx := context.Background()
	r, accountA, _, appID := setupOffer(t)
	offerID := seedSentOffer(t, r, appID, 0)
	if _, err := r.NegotiateOffer(ctx, offerID, accountA, ptrF(25000), "", 3); err != nil {
		t.Fatalf("negotiate: %v", err)
	}

	o, err := r.WithdrawOffer(ctx, appID, "budget not approved")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if o.Status != OfferDeclined {
		t.Fatalf("withdrawn offer status = %q, want declined", o.Status)
	}
	app, _ := r.FindByID(ctx, appID)
	if app.Status != StatusRejected {
		t.Fatalf("application status after withdraw = %q, want rejected", app.Status)
	}
}

func TestOffer_WithdrawNoActiveOfferConflicts(t *testing.T) {
	ctx := context.Background()
	r, _, _, appID := setupOffer(t)
	// Offer-stage application but no offer row composed yet → WithdrawOffer reports
	// ErrOfferConflict, which the UpdateStatus handler treats as "fall back to a
	// plain rejection".
	if _, err := r.WithdrawOffer(ctx, appID, "no offer yet"); !errors.Is(err, ErrOfferConflict) {
		t.Fatalf("withdraw with no active offer err = %v, want ErrOfferConflict", err)
	}
}

func ptrF(f float64) *float64     { return &f }
func ptrT(t time.Time) *time.Time { return &t }
