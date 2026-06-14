//go:build integration

package candidateauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func freshAuthDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to v16?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE candidate_accounts, candidate_sessions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

// TestSuspendedSession_StopsResolving proves an EXISTING cookie dies the moment
// the account leaves 'active' — the session row is still live, but the resolve
// query filters on status so it no longer returns the account.
func TestSuspendedSession_StopsResolving(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	acct, err := repo.FindOrCreateByEmail(ctx, "sus@example.com")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	const token = "live-token"
	if err := repo.CreateSession(ctx, acct.ID, hashSecret(token), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Sanity: the session resolves while active.
	if _, err := repo.FindAccountBySessionHash(ctx, hashSecret(token)); err != nil {
		t.Fatalf("active session should resolve, got %v", err)
	}

	// Suspend the account (what members.SetStatus does for the status column).
	if _, err := pool.Exec(ctx, `UPDATE candidate_accounts SET status='suspended' WHERE id=$1`, acct.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	// The same live cookie now resolves to nothing → middleware treats as logged-out.
	if _, err := repo.FindAccountBySessionHash(ctx, hashSecret(token)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("suspended account session must not resolve; got %v", err)
	}
}

// TestSuspendedAccount_FreshLoginRefused proves a fresh provider login can't
// re-establish a session for a suspended account (suspend isn't cosmetic).
func TestSuspendedAccount_FreshLoginRefused(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)
	svc := NewService(repo, nil, nil, time.Minute, time.Hour)

	const sub = "U_suspended_line"
	acct, err := repo.FindOrCreateByLineSub(ctx, sub, "ผู้ใช้", "")
	if err != nil {
		t.Fatalf("create line account: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candidate_accounts SET status='suspended' WHERE id=$1`, acct.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	// A fresh LINE login resolves to the same suspended account → refused.
	if _, _, err := svc.LoginWithLine(ctx, sub, "ผู้ใช้", ""); !errors.Is(err, ErrAccountSuspended) {
		t.Fatalf("fresh login on suspended account must be refused; got %v", err)
	}

	// Reactivating restores the ability to log in.
	if _, err := pool.Exec(ctx, `UPDATE candidate_accounts SET status='active' WHERE id=$1`, acct.ID); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if _, _, err := svc.LoginWithLine(ctx, sub, "ผู้ใช้", ""); err != nil {
		t.Fatalf("reactivated account should log in, got %v", err)
	}
}
