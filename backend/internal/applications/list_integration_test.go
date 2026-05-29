//go:build integration

package applications

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

func setupList(t *testing.T) (*pgRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE applications, candidates, positions, stores, vacancies RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name, subregion) VALUES (1,'A','Upper North'),(2,'B','East')`); err != nil {
		t.Fatalf("seed stores: %v", err)
	}
	var posID, candID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('t') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status) VALUES ('c','career_portal','available') RETURNING id`).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	return &pgRepository{pool: pool}, posID, candID
}

func insertApp(t *testing.T, r *pgRepository, candID, posID uuid.UUID, status string, score float64, store int) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(),
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id) VALUES ($1,$2,$3,$4,$5)`,
		candID, posID, status, score, store); err != nil {
		t.Fatalf("insert app: %v", err)
	}
}

func TestList_FilterRankPaginate(t *testing.T) {
	r, pos, cand := setupList(t)
	insertApp(t, r, cand, pos, "scored", 90, 1)
	insertApp(t, r, cand, pos, "scored", 70, 1)
	insertApp(t, r, cand, pos, "scored", 80, 2)
	insertApp(t, r, cand, pos, "rejected", 95, 1)

	admin := rbac.New("super_admin", nil, "")

	// status filter + ranking
	items, total, err := r.List(context.Background(), ListFilter{Status: "scored", Limit: 10}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("expected 3 scored, got %d", total)
	}
	if len(items) != 3 || *items[0].AIScore != 90 || *items[2].AIScore != 70 {
		t.Errorf("expected rank 90,80,70; got %v", scores(items))
	}

	// pagination
	page1, total, _ := r.List(context.Background(), ListFilter{Status: "scored", Page: 1, Limit: 2}, admin)
	if total != 3 || len(page1) != 2 {
		t.Errorf("expected total 3 / 2 rows, got %d / %d", total, len(page1))
	}
}

func TestList_StoreScope(t *testing.T) {
	r, pos, cand := setupList(t)
	insertApp(t, r, cand, pos, "scored", 90, 1)
	insertApp(t, r, cand, pos, "scored", 80, 2)

	store2 := 2
	items, total, err := r.List(context.Background(), ListFilter{Limit: 10}, rbac.New("hr_staff", &store2, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || *items[0].AssignedStoreID != 2 {
		t.Errorf("store-scoped user should see only store 2, got total %d items %v", total, items)
	}
}

func scores(items []Application) []float64 {
	out := make([]float64, len(items))
	for i, a := range items {
		if a.AIScore != nil {
			out[i] = *a.AIScore
		}
	}
	return out
}
