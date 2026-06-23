//go:build integration

package positions

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/scoring"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// TestFindByID_RoundTripsBenefits proves the FindByID projection + scan include the
// new benefits column (added in migration 000040) alongside responsibilities and
// qualifications. This is the position-level JD the public portal surfaces (Phase 2).
func TestFindByID_RoundTripsBenefits(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to 000040?): %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool)

	var posID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th, title_en, responsibilities, qualifications, benefits)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		"ตำแหน่งทดสอบ", "Test Position", "หน้าที่กลาง", "คุณสมบัติกลาง", "สวัสดิการกลาง").Scan(&posID); err != nil {
		t.Fatalf("insert position: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM positions WHERE id = $1`, posID) })

	got, err := repo.FindByID(ctx, posID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.Responsibilities != "หน้าที่กลาง" || got.Qualifications != "คุณสมบัติกลาง" {
		t.Fatalf("resp/qual mismatch: %+v", got)
	}
	if got.Benefits != "สวัสดิการกลาง" {
		t.Fatalf("benefits round-trip failed: got %q", got.Benefits)
	}
}

// TestUpdateScoreWeights_RoundTrip proves the score_weights column persists and
// reads back, and that an unset position reports nil (scorer then uses defaults).
func TestUpdateScoreWeights_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to 000041?): %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool)

	var posID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th) VALUES ($1) RETURNING id`, "ตำแหน่งถ่วงน้ำหนัก").Scan(&posID); err != nil {
		t.Fatalf("insert position: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM positions WHERE id = $1`, posID) })

	// Unset -> nil.
	got, err := repo.FindByID(ctx, posID)
	if err != nil {
		t.Fatalf("find (pre): %v", err)
	}
	if got.ScoreWeights != nil {
		t.Fatalf("expected nil weights before set, got %+v", got.ScoreWeights)
	}

	w := scoring.Weights{Experience: 40, Skills: 20, Education: 10, Language: 10, Location: 20}
	if err := repo.UpdateScoreWeights(ctx, posID, w); err != nil {
		t.Fatalf("update weights: %v", err)
	}
	got, err = repo.FindByID(ctx, posID)
	if err != nil {
		t.Fatalf("find (post): %v", err)
	}
	if got.ScoreWeights == nil || *got.ScoreWeights != w {
		t.Fatalf("weights round-trip failed: got %+v want %+v", got.ScoreWeights, w)
	}

	// Updating a missing id -> ErrNotFound.
	if err := repo.UpdateScoreWeights(ctx, uuid.New(), w); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing id, got %v", err)
	}
}
