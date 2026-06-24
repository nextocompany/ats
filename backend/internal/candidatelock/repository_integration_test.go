//go:build integration

package candidatelock

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
	return "postgres://postgres:test@localhost:5432/atstest?sslmode=disable"
}

func setup(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	var cand, userA, userB uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name) VALUES ('Lock Test') RETURNING id`).Scan(&cand); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role) VALUES ('lockA@test.local','Operator A','hr_store') RETURNING id`).Scan(&userA); err != nil {
		t.Fatalf("seed user A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role) VALUES ('lockB@test.local','Operator B','hr_store') RETURNING id`).Scan(&userB); err != nil {
		t.Fatalf("seed user B: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM candidate_locks WHERE candidate_id = $1`, cand)
		pool.Exec(ctx, `DELETE FROM candidates WHERE id = $1`, cand)
		pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, userA, userB)
		pool.Close()
	})
	return pool, cand, userA, userB
}

func TestLockConcurrencySemantics(t *testing.T) {
	ctx := context.Background()
	pool, cand, userA, userB := setup(t)
	r := NewRepository(pool)

	// A acquires.
	if _, err := r.Acquire(ctx, cand, userA, 30*time.Minute); err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	// B is blocked, and sees A as the holder.
	held, err := r.Acquire(ctx, cand, userB, 30*time.Minute)
	if err != ErrLockedByOther {
		t.Fatalf("B should be blocked, got %v", err)
	}
	if held.LockedBy != userA || held.LockedByName != "Operator A" {
		t.Errorf("blocked report wrong holder: %+v", held)
	}
	// A refreshes its own lock (no conflict).
	if _, err := r.Acquire(ctx, cand, userA, 30*time.Minute); err != nil {
		t.Fatalf("A refresh: %v", err)
	}
	// B cannot release A's lock without force; with force it can.
	if err := r.Release(ctx, cand, userB, false); err != nil {
		t.Fatalf("B non-force release (no-op): %v", err)
	}
	if l, _ := r.Get(ctx, cand); l == nil {
		t.Fatal("lock should still be held after B's non-force release")
	}
	if err := r.Release(ctx, cand, userB, true); err != nil {
		t.Fatalf("force release: %v", err)
	}
	if l, _ := r.Get(ctx, cand); l != nil {
		t.Fatalf("lock should be gone after force release, got %+v", l)
	}
	// Now B can acquire the freed candidate.
	if _, err := r.Acquire(ctx, cand, userB, 30*time.Minute); err != nil {
		t.Fatalf("B acquire after release: %v", err)
	}
}

func TestExpiredLockCanBeTakenOver(t *testing.T) {
	ctx := context.Background()
	pool, cand, userA, userB := setup(t)
	r := NewRepository(pool)

	// A takes a normal lock, which we then age past its expiry in the DB.
	if _, err := r.Acquire(ctx, cand, userA, 30*time.Minute); err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candidate_locks SET expires_at = now() - interval '1 hour' WHERE candidate_id = $1`, cand); err != nil {
		t.Fatalf("age lock: %v", err)
	}
	// Get sees nothing live.
	if l, _ := r.Get(ctx, cand); l != nil {
		t.Fatalf("expired lock should not be live, got %+v", l)
	}
	// B takes over the expired lock.
	got, err := r.Acquire(ctx, cand, userB, 30*time.Minute)
	if err != nil {
		t.Fatalf("B take over expired: %v", err)
	}
	if got.LockedBy != userB {
		t.Errorf("B should now hold the lock, got holder %v", got.LockedBy)
	}
}
