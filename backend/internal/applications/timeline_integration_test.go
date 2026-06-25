//go:build integration

package applications

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTimeline seeds two accounts (A owns the application, B is the attacker),
// one candidate linked to A, a position, and an application with a known public
// token. Returns the repo, account IDs, and the application id. Reuses dsn().
func setupTimeline(t *testing.T) (r *pgRepository, accountA, accountB, appID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to >=46?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE applications, application_status_history, candidates, candidate_accounts, positions, stores RESTART IDENTITY CASCADE`); err != nil {
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
		`INSERT INTO applications (candidate_id, position_id, status, public_token) VALUES ($1,$2,'pending','tok-A') RETURNING id`,
		candID, posID).Scan(&appID); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return &pgRepository{pool: pool}, accountA, accountB, appID
}

// TestPortalTimelineByToken_TriggerRecordsAndScopes is the end-to-end proof that
// the DB trigger records every status change and that the lookup is account-
// scoped (no IDOR).
func TestPortalTimelineByToken_TriggerRecordsAndScopes(t *testing.T) {
	ctx := context.Background()
	r, accountA, accountB, appID := setupTimeline(t)

	// Drive real status transitions through the repository — the trigger fires on
	// each UPDATE. The initial 'pending' (an INSERT) records nothing.
	for _, st := range []string{StatusScored, StatusAIInterview, StatusInterview} {
		if err := r.SetStatus(ctx, appID, st); err != nil {
			t.Fatalf("set status %s: %v", st, err)
		}
	}

	tl, err := r.PortalTimelineByToken(ctx, "tok-A", accountA)
	if err != nil {
		t.Fatalf("owner lookup: %v", err)
	}
	if tl.Status != StatusInterview {
		t.Errorf("current status = %q, want %q", tl.Status, StatusInterview)
	}
	if tl.Position != "แคชเชียร์" {
		t.Errorf("position = %q, want แคชเชียร์", tl.Position)
	}
	if tl.CreatedAt.IsZero() {
		t.Error("created_at is zero")
	}
	// Three UPDATEs → three recorded transitions, in chronological order.
	if len(tl.Events) != 3 {
		t.Fatalf("recorded events = %d, want 3 (%v)", len(tl.Events), tl.Events)
	}
	wantOrder := []string{StatusScored, StatusAIInterview, StatusInterview}
	for i, w := range wantOrder {
		if tl.Events[i].To != w {
			t.Errorf("event[%d] = %q, want %q", i, tl.Events[i].To, w)
		}
		if i > 0 && tl.Events[i].At.Before(tl.Events[i-1].At) {
			t.Errorf("events not chronological at %d", i)
		}
	}

	// IDOR: a different account must not read this application's timeline.
	if _, err := r.PortalTimelineByToken(ctx, "tok-A", accountB); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-account lookup err = %v, want ErrNotFound (no IDOR)", err)
	}

	// Unknown token → ErrNotFound (indistinguishable from unowned).
	if _, err := r.PortalTimelineByToken(ctx, "does-not-exist", accountA); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token err = %v, want ErrNotFound", err)
	}
}
