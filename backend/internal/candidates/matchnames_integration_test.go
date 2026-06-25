//go:build integration

package candidates

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestGetAccountMatchNames_Integration is the read half of the resume name-match
// gate: the pipeline asks for the account's Thai + English names to compare the
// parsed CV against. A regression here (wrong column, broken JOIN) silently
// disables the gate, so this asserts both names round-trip and that an accountless
// candidate reports hasAccount=false (so the gate is skipped, never falsely fired).
func TestGetAccountMatchNames_Integration(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to >=045?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE candidates, candidate_accounts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var acctID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts (full_name, name_th, name_en)
		 VALUES ('สมชาย ใจดี', 'สมชาย ใจดี', 'Somchai Jaidee') RETURNING id`).Scan(&acctID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	var candID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, source_channel, status, account_id)
		 VALUES ('สมชาย ใจดี', 'career_portal', 'available', $1) RETURNING id`, acctID).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}

	repo := NewRepository(pool)

	th, en, has, err := repo.GetAccountMatchNames(ctx, candID)
	if err != nil {
		t.Fatalf("GetAccountMatchNames: %v", err)
	}
	if !has {
		t.Fatal("hasAccount = false, want true for an account-linked candidate")
	}
	if th != "สมชาย ใจดี" {
		t.Errorf("name_th = %q, want %q", th, "สมชาย ใจดี")
	}
	if en != "Somchai Jaidee" {
		t.Errorf("name_en = %q, want %q", en, "Somchai Jaidee")
	}

	// Accountless candidate (bulk/webhook intake): no portal account → hasAccount
	// must be false so the pipeline SKIPS the gate rather than flagging a mismatch.
	var orphanID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, source_channel, status)
		 VALUES ('Walk In', 'bulk', 'available') RETURNING id`).Scan(&orphanID); err != nil {
		t.Fatalf("seed accountless candidate: %v", err)
	}
	if _, _, has, err := repo.GetAccountMatchNames(ctx, orphanID); err != nil {
		t.Fatalf("GetAccountMatchNames (accountless): %v", err)
	} else if has {
		t.Error("hasAccount = true for an accountless candidate, want false")
	}
}
