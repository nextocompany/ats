//go:build integration

package candidateauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func freshOTPDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to 000013?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE email_otps RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func TestConsumeOTP_LocksAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool := freshOTPDB(t)
	repo := NewRepository(pool)

	const code = "123456"
	if err := repo.CreateOTP(ctx, "brute@x.com", hashSecret(code), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// maxOTPAttempts wrong guesses are allowed (each returns ErrOTPInvalid).
	for i := 0; i < maxOTPAttempts; i++ {
		if err := repo.ConsumeOTP(ctx, "brute@x.com", hashSecret("000000")); !errors.Is(err, ErrOTPInvalid) {
			t.Fatalf("attempt %d: expected ErrOTPInvalid, got %v", i+1, err)
		}
	}
	// The correct code is now rejected — the challenge is locked.
	if err := repo.ConsumeOTP(ctx, "brute@x.com", hashSecret(code)); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("expected lockout (ErrOTPInvalid) after %d failures, got %v", maxOTPAttempts, err)
	}
	// And it's consumed (no live challenge remains).
	if n := count(t, pool, `SELECT count(*) FROM email_otps WHERE consumed_at IS NULL`); n != 0 {
		t.Fatalf("expected the locked challenge consumed, %d still live", n)
	}
}

func TestConsumeOTP_SucceedsWithinAttempts(t *testing.T) {
	ctx := context.Background()
	pool := freshOTPDB(t)
	repo := NewRepository(pool)

	const code = "424242"
	if err := repo.CreateOTP(ctx, "ok@x.com", hashSecret(code), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Two wrong, then correct → success.
	_ = repo.ConsumeOTP(ctx, "ok@x.com", hashSecret("999999"))
	_ = repo.ConsumeOTP(ctx, "ok@x.com", hashSecret("888888"))
	if err := repo.ConsumeOTP(ctx, "ok@x.com", hashSecret(code)); err != nil {
		t.Fatalf("expected success within attempt budget, got %v", err)
	}
	// Reuse now fails (consumed).
	if err := repo.ConsumeOTP(ctx, "ok@x.com", hashSecret(code)); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("expected ErrOTPInvalid on reuse, got %v", err)
	}
}
