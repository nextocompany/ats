//go:build integration

package fit

import (
	"context"
	"errors"
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

// setup truncates the relevant tables and seeds a position, candidate, and a
// scored application, returning the repo and the application id.
func setup(t *testing.T) (*pgRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to v15?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE application_fit_analyses, applications, candidates, positions, stores, vacancies RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	var posID, candID, appID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('พนักงานขาย') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status) VALUES ('สมชาย','career_portal','available') RETURNING id`).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, ai_score) VALUES ($1,$2,'scored',81) RETURNING id`,
		candID, posID).Scan(&appID); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return &pgRepository{pool: pool}, appID, posID
}

func TestRepository_UpsertAndRead(t *testing.T) {
	r, appID, posID := setup(t)
	ctx := context.Background()

	first := Analysis{
		ApplicationID: appID,
		OverallFit:    OverallModerate,
		Summary:       "เหมาะกับบางตำแหน่ง",
		Strengths:     []string{"ขยัน"},
		Concerns:      []string{"ประสบการณ์น้อย"},
		Recommended:   []RecommendedPosition{{PositionID: posID, Title: "พนักงานขาย", FitScore: 80, Reasons: []string{"ตรงสาย"}}},
		Model:         "mock",
	}
	if err := r.Upsert(ctx, first, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := r.FindByApplicationID(ctx, appID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.OverallFit != OverallModerate || got.Summary != "เหมาะกับบางตำแหน่ง" {
		t.Errorf("read back mismatch: %+v", got)
	}
	if len(got.Recommended) != 1 || got.Recommended[0].FitScore != 80 {
		t.Errorf("recommended mismatch: %+v", got.Recommended)
	}

	// Regenerate → single row reflects the latest (no duplicate PK).
	second := first
	second.OverallFit = OverallNone
	second.Summary = "ไม่เหมาะกับตำแหน่งใดเลย"
	second.NoMatchReason = "ประสบการณ์ไม่ตรง"
	second.Recommended = nil
	if err := r.Upsert(ctx, second, nil); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got2, err := r.FindByApplicationID(ctx, appID)
	if err != nil {
		t.Fatalf("find2: %v", err)
	}
	if got2.OverallFit != OverallNone || got2.NoMatchReason != "ประสบการณ์ไม่ตรง" {
		t.Errorf("regenerate not reflected: %+v", got2)
	}
	if len(got2.Recommended) != 0 {
		t.Errorf("recommended should be empty after regenerate, got %d", len(got2.Recommended))
	}
}

func TestRepository_NotFound(t *testing.T) {
	r, _, _ := setup(t)
	if _, err := r.FindByApplicationID(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
