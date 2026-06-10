//go:build integration

package search

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestFetchDoc_BestApplication verifies the projection picks a candidate's
// highest-scoring application and carries subregion + assigned_store_id.
func TestFetchDoc_BestApplication(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name, subregion) VALUES (901,'IdxStore','Upper North') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	var pos uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('ทดสอบ-idx') RETURNING id`).Scan(&pos); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	var cand uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, province, subregion, source_channel, status) VALUES ('สมหญิง ดัชนี','เชียงใหม่','Upper North','career_portal','available') RETURNING id`,
	).Scan(&cand); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	// Two applications — the higher-scored (90, store 901) must win.
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id) VALUES ($1,$2,'scored',55,NULL),($1,$2,'scored',90,901)`, cand, pos); err != nil {
		t.Fatalf("seed apps: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM applications WHERE candidate_id=$1`, cand)
		_, _ = pool.Exec(ctx, `DELETE FROM candidates WHERE id=$1`, cand)
		_, _ = pool.Exec(ctx, `DELETE FROM positions WHERE id=$1`, pos)
	})

	doc, ok, err := FetchDoc(ctx, pool, cand)
	if err != nil {
		t.Fatalf("FetchDoc: %v", err)
	}
	if !ok {
		t.Fatal("expected a doc, got none")
	}
	if doc.FullName != "สมหญิง ดัชนี" {
		t.Errorf("full_name = %q", doc.FullName)
	}
	if doc.AIScore == nil || *doc.AIScore != 90 {
		t.Errorf("ai_score = %v, want 90 (best app)", doc.AIScore)
	}
	if doc.AssignedStoreID == nil || *doc.AssignedStoreID != 901 {
		t.Errorf("assigned_store_id = %v, want 901", doc.AssignedStoreID)
	}
	if doc.Subregion != "Upper North" {
		t.Errorf("subregion = %q, want Upper North", doc.Subregion)
	}

	// FetchAllDocs includes this candidate.
	all, err := FetchAllDocs(ctx, pool, 0, 1000)
	if err != nil {
		t.Fatalf("FetchAllDocs: %v", err)
	}
	found := false
	for _, d := range all {
		if d.CandidateID == cand.String() {
			found = true
		}
	}
	if !found {
		t.Error("FetchAllDocs did not include the seeded candidate")
	}
}
