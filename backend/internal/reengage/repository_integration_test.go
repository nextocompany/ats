//go:build integration

package reengage

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

func setup(t *testing.T) (*pgRepo, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE reengagement_contacts, applications, candidates, positions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	var posID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return &pgRepo{pool: pool}, posID
}

func seedCandidateApp(t *testing.T, r *pgRepo, posID uuid.UUID, status string, talentPool bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var candID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, source_channel, status) VALUES ('ทดสอบ','0812345678','career_portal','available') RETURNING id`,
	).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, talent_pool) VALUES ($1,$2,$3,$4)`,
		candID, posID, status, talentPool); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return candID
}

func TestMatchingCandidates_TalentPoolAndRejected(t *testing.T) {
	r, pos := setup(t)
	seedCandidateApp(t, r, pos, "scored", true)   // talent pool → match
	seedCandidateApp(t, r, pos, "rejected", false) // rejected → match
	seedCandidateApp(t, r, pos, "scored", false)   // active, not talent pool → no match

	targets, err := r.MatchingCandidates(context.Background(), pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matching candidates, got %d", len(targets))
	}
}

func TestRecordContact_SuppressesSecondTime(t *testing.T) {
	r, pos := setup(t)
	cand := seedCandidateApp(t, r, pos, "rejected", false)
	ctx := context.Background()

	first, err := r.RecordContact(ctx, cand, pos, "line")
	if err != nil || !first {
		t.Fatalf("expected first contact recorded, got inserted=%v err=%v", first, err)
	}
	second, err := r.RecordContact(ctx, cand, pos, "line")
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Fatal("expected second contact suppressed (no insert)")
	}

	// A contacted candidate drops out of the matching set.
	targets, err := r.MatchingCandidates(ctx, pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected contacted candidate excluded, got %d targets", len(targets))
	}
}
