//go:build integration

package candidates

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
