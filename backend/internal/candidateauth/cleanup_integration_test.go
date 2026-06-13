//go:build integration

package candidateauth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// TestCleanExpired verifies the sweep deletes only dead auth rows (expired/
// consumed OTPs, expired/revoked sessions) and keeps live ones.
func TestCleanExpired(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to 000013?): %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE candidate_sessions, email_otps, candidate_accounts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// An account to anchor sessions (FK).
	var acctID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts (email, email_verified) VALUES ('cleanup@example.com', true) RETURNING id`).Scan(&acctID); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	now := time.Now()
	// OTPs: 1 expired, 1 consumed, 1 live.
	mustExec(t, pool, `INSERT INTO email_otps (email, code_hash, expires_at) VALUES ('a@x.com','h1',$1)`, now.Add(-time.Hour))                     // expired
	mustExec(t, pool, `INSERT INTO email_otps (email, code_hash, expires_at, consumed_at) VALUES ('a@x.com','h2',$1,$2)`, now.Add(time.Hour), now) // consumed
	mustExec(t, pool, `INSERT INTO email_otps (email, code_hash, expires_at) VALUES ('a@x.com','h3',$1)`, now.Add(time.Hour))                      // live

	// Sessions: 1 expired, 1 revoked, 1 live.
	mustExec(t, pool, `INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1,'t1',$2)`, acctID, now.Add(-time.Hour))                    // expired
	mustExec(t, pool, `INSERT INTO candidate_sessions (account_id, token_hash, expires_at, revoked_at) VALUES ($1,'t2',$2,$3)`, acctID, now.Add(time.Hour), now) // revoked
	mustExec(t, pool, `INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1,'t3',$2)`, acctID, now.Add(time.Hour))                     // live

	svc := NewCleanupService(pool)
	otps, sessions, err := svc.CleanExpired(ctx, 500)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if otps != 2 {
		t.Errorf("expected 2 OTPs deleted, got %d", otps)
	}
	if sessions != 2 {
		t.Errorf("expected 2 sessions deleted, got %d", sessions)
	}

	// The live rows survive.
	if got := count(t, pool, `SELECT count(*) FROM email_otps`); got != 1 {
		t.Errorf("expected 1 OTP remaining, got %d", got)
	}
	if got := count(t, pool, `SELECT count(*) FROM candidate_sessions`); got != 1 {
		t.Errorf("expected 1 session remaining, got %d", got)
	}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
