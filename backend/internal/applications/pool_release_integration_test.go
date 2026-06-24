//go:build integration

package applications

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func poolDSN() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://postgres:test@localhost:5432/atstest?sslmode=disable"
}

// TestReleaseStalePoolCandidates proves the sweep releases ONLY store-specific
// applications that are old and unpicked, leaving picked-up, already-pooled, and
// recent ones untouched.
func TestReleaseStalePoolCandidates(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, poolDSN())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var cand, pos uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name) VALUES ('Pool Sweep') RETURNING id`).Scan(&cand); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th, title_en) VALUES ('ทดสอบ','Test') RETURNING id`).Scan(&pos); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	// A store row for assigned_store_id FK.
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name) VALUES (99001, 'Sweep Store') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	mk := func(label string, storeNo *int, talentPool, pickedUp bool, ageDays int) uuid.UUID {
		var id uuid.UUID
		pickedExpr := map[bool]string{true: "now()", false: "NULL"}[pickedUp]
		q := `INSERT INTO applications (candidate_id, position_id, assigned_store_id, talent_pool, status, created_at, picked_up_at)
		      VALUES ($1,$2,$3,$4,'scored', now() - make_interval(days => $5), ` + pickedExpr + `) RETURNING id`
		if err := pool.QueryRow(ctx, q, cand, pos, storeNo, talentPool, ageDays).Scan(&id); err != nil {
			t.Fatalf("seed app %s: %v", label, err)
		}
		return id
	}
	sn := 99001
	stale := mk("stale-store", &sn, false, false, 5)   // SHOULD be released
	picked := mk("picked-store", &sn, false, true, 5)  // picked up → keep
	recent := mk("recent-store", &sn, false, false, 1) // within grace → keep
	pooled := mk("already-pool", nil, true, false, 5)  // already pool → keep

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM applications WHERE candidate_id = $1`, cand)
		pool.Exec(ctx, `DELETE FROM candidates WHERE id = $1`, cand)
		pool.Exec(ctx, `DELETE FROM positions WHERE id = $1`, pos)
		pool.Exec(ctx, `DELETE FROM stores WHERE store_no = 99001`)
	})

	r := NewRepository(pool).(*pgRepository)
	n, err := r.ReleaseStalePoolCandidates(ctx, 3)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if n != 1 {
		t.Errorf("released count = %d, want 1 (only the stale store-specific app)", n)
	}

	released := func(id uuid.UUID) bool {
		var inPool bool
		pool.QueryRow(ctx, `SELECT talent_pool AND assigned_store_id IS NULL AND released_to_pool_at IS NOT NULL FROM applications WHERE id = $1`, id).Scan(&inPool)
		return inPool
	}
	if !released(stale) {
		t.Error("stale store-specific app should have been released to pool")
	}
	if released(picked) {
		t.Error("picked-up app must NOT be released")
	}
	if released(recent) {
		t.Error("recent app must NOT be released")
	}
	if released(pooled) {
		t.Error("already-pooled app must NOT be re-released (released_to_pool_at would be set)")
	}
}
