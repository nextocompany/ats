//go:build integration

package candidates

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mkAccount inserts a candidate_accounts shell with a unique email and returns its
// id, registering cleanup. Used by the Phase-2 account-provisioning tests.
func mkAccount(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts (email, email_verified) VALUES ($1, FALSE) RETURNING id`,
		email).Scan(&id); err != nil {
		t.Fatalf("insert account %s: %v", email, err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM candidate_accounts WHERE id = $1`, id) })
	return id
}

func TestSetAccountID_OnlyLinksWhenNull(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool)

	accA := mkAccount(t, pool, "set-acct-a@example.com")
	accB := mkAccount(t, pool, "set-acct-b@example.com")

	c, err := repo.Create(ctx, Candidate{FullName: "Provision Me", SourceChannel: "walk_in", Status: "available"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM candidates WHERE id = $1`, c.ID) })

	// First link sticks.
	if err := repo.SetAccountID(ctx, c.ID, accA); err != nil {
		t.Fatalf("set account A: %v", err)
	}
	got, _ := repo.FindByID(ctx, c.ID)
	if got.AccountID == nil || *got.AccountID != accA {
		t.Fatalf("expected account A linked, got %v", got.AccountID)
	}
	// Second link is a no-op (never overwrites an existing account).
	if err := repo.SetAccountID(ctx, c.ID, accB); err != nil {
		t.Fatalf("set account B: %v", err)
	}
	got, _ = repo.FindByID(ctx, c.ID)
	if got.AccountID == nil || *got.AccountID != accA {
		t.Fatalf("account must NOT be overwritten; want A=%v got %v", accA, got.AccountID)
	}
}

func TestMarkDuplicateOf_ReconcilesAccount(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool)

	mk := func(name string, acct *uuid.UUID) Candidate {
		c, err := repo.Create(ctx, Candidate{FullName: name, SourceChannel: "walk_in", Status: "available", AccountID: acct})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM candidates WHERE id = $1`, c.ID) })
		return c
	}

	t.Run("accountless canonical inherits the duplicate's account", func(t *testing.T) {
		accA := mkAccount(t, pool, "dup-donor-a@example.com")
		dup := mk("Member Apply", &accA)      // logged-in member's new row
		canon := mk("Walk-in Canonical", nil) // older accountless canonical
		if err := repo.MarkDuplicateOf(ctx, dup.ID, canon.ID); err != nil {
			t.Fatalf("mark duplicate: %v", err)
		}
		got, _ := repo.FindByID(ctx, canon.ID)
		if got.AccountID == nil || *got.AccountID != accA {
			t.Fatalf("canonical should inherit account A=%v, got %v", accA, got.AccountID)
		}
	})

	t.Run("canonical with its own account is never clobbered", func(t *testing.T) {
		accA := mkAccount(t, pool, "dup-keep-a@example.com")
		accB := mkAccount(t, pool, "dup-keep-b@example.com")
		dup := mk("Dup With A", &accA)
		canon := mk("Canonical With B", &accB)
		if err := repo.MarkDuplicateOf(ctx, dup.ID, canon.ID); err != nil {
			t.Fatalf("mark duplicate: %v", err)
		}
		got, _ := repo.FindByID(ctx, canon.ID)
		if got.AccountID == nil || *got.AccountID != accB {
			t.Fatalf("canonical must keep its own account B=%v, got %v", accB, got.AccountID)
		}
	})
}

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// TestCreate_PersistsLineUserID checks the slice-2.3 column round-trips: a
// candidate created with a LINE `sub` reads it back via FindByID. Self-contained
// and non-destructive (creates then deletes its own row), so it is safe to run
// against a populated dev DB.
func TestCreate_PersistsLineUserID(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	repo := NewRepository(pool)
	c, err := repo.Create(ctx, Candidate{
		FullName:      "LINE Test",
		SourceChannel: "career_portal",
		Status:        "available",
		LineUserID:    "U-test-line-id-2_3",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM candidates WHERE id = $1`, c.ID)
	})

	got, err := repo.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.LineUserID != "U-test-line-id-2_3" {
		t.Errorf("line_user_id = %q, want %q", got.LineUserID, "U-test-line-id-2_3")
	}

	// Empty LINE id must store/read as empty (legacy/demo candidates), not error.
	c2, err := repo.Create(ctx, Candidate{FullName: "No LINE", SourceChannel: "walk_in", Status: "available"})
	if err != nil {
		t.Fatalf("create no-line: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM candidates WHERE id = $1`, c2.ID)
	})
	got2, err := repo.FindByID(ctx, c2.ID)
	if err != nil {
		t.Fatalf("find no-line: %v", err)
	}
	if got2.LineUserID != "" {
		t.Errorf("expected empty line_user_id, got %q", got2.LineUserID)
	}
}
