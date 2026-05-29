//go:build integration

package search

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

func setup(t *testing.T) (*pgSearcher, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE applications, candidates, positions, stores RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name, subregion) VALUES (1,'A','East'),(2,'B','West')`); err != nil {
		t.Fatalf("seed stores: %v", err)
	}
	var pos uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&pos); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return &pgSearcher{pool: pool}, pos
}

func seed(t *testing.T, s *pgSearcher, name, province string, pos uuid.UUID, store int, score float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var cand uuid.UUID
	if err := s.pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, province, subregion, source_channel, status)
		 VALUES ($1,$2,(SELECT subregion FROM stores WHERE store_no=$3),'career_portal','available') RETURNING id`,
		name, province, store).Scan(&cand); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id) VALUES ($1,$2,'scored',$3,$4)`,
		cand, pos, score, store); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return cand
}

func TestSearch_ByNameRanked(t *testing.T) {
	s, pos := setup(t)
	seed(t, s, "สมชาย ใจดี", "กรุงเทพ", pos, 1, 70)
	seed(t, s, "สมชาย เก่งมาก", "กรุงเทพ", pos, 1, 90)
	seed(t, s, "วิชัย อื่น", "เชียงใหม่", pos, 2, 95)

	admin := rbac.New("super_admin", nil, "")
	hits, total, err := s.Search(context.Background(), Query{Text: "สมชาย", Limit: 10}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(hits) != 2 {
		t.Fatalf("expected 2 สมชาย hits, got total=%d len=%d", total, len(hits))
	}
	if hits[0].AIScore == nil || *hits[0].AIScore != 90 {
		t.Errorf("expected top hit score 90, got %v", hits[0].AIScore)
	}
}

func TestSearch_OneHitPerCandidate(t *testing.T) {
	s, pos := setup(t)
	cand := seed(t, s, "หลายใบ สมัคร", "กรุงเทพ", pos, 1, 60)
	// second application for the SAME candidate, higher score
	if _, err := s.pool.Exec(context.Background(),
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id) VALUES ($1,$2,'scored',88,1)`,
		cand, pos); err != nil {
		t.Fatal(err)
	}
	hits, total, err := s.Search(context.Background(), Query{Text: "หลายใบ", Limit: 10}, rbac.New("super_admin", nil, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(hits) != 1 {
		t.Fatalf("expected single deduped hit, got total=%d len=%d", total, len(hits))
	}
	if *hits[0].AIScore != 88 {
		t.Errorf("expected best score 88, got %v", *hits[0].AIScore)
	}
}

func TestSearch_StoreScopeNoCrossStoreLeak(t *testing.T) {
	s, pos := setup(t)
	// One candidate with applications at BOTH stores; the store-2 app scores higher.
	cand := seed(t, s, "ข้ามร้าน ทดสอบ", "กรุงเทพ", pos, 1, 60) // store-1 app, score 60
	if _, err := s.pool.Exec(context.Background(),
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id) VALUES ($1,$2,'scored',95,2)`,
		cand, pos); err != nil { // store-2 app, score 95
		t.Fatal(err)
	}

	store1 := 1
	hits, total, err := s.Search(context.Background(), Query{Text: "ข้ามร้าน", Limit: 10}, rbac.New("hr_staff", &store1, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(hits) != 1 {
		t.Fatalf("expected the candidate once, got total=%d len=%d", total, len(hits))
	}
	// The store-1 user must see the store-1 application's score (60), NOT the
	// out-of-scope store-2 score (95).
	if hits[0].AIScore == nil || *hits[0].AIScore != 60 {
		t.Fatalf("cross-store leak: expected in-scope score 60, got %v", hits[0].AIScore)
	}
}

func TestSearch_StoreScopeFiltersOut(t *testing.T) {
	s, pos := setup(t)
	seed(t, s, "อยู่ร้านหนึ่ง", "กรุงเทพ", pos, 1, 80) // store 1
	seed(t, s, "อยู่ร้านสอง", "เชียงใหม่", pos, 2, 85)  // store 2

	store1 := 1
	scoped := rbac.New("hr_staff", &store1, "")
	hits, total, err := s.Search(context.Background(), Query{Text: "อยู่ร้าน", Limit: 10}, scoped)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(hits) != 1 {
		t.Fatalf("store-scoped user should see only own store, got total=%d", total)
	}
	if hits[0].FullName != "อยู่ร้านหนึ่ง" {
		t.Errorf("wrong candidate leaked: %s", hits[0].FullName)
	}
}
