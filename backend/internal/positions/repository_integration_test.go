//go:build integration

package positions

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
