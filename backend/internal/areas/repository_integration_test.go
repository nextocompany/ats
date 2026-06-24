//go:build integration

package areas

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
	return "postgres://postgres:test@localhost:5432/atstest?sslmode=disable"
}

func TestAreaCRUDAndMembership(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Seed two stores and a user for membership.
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name) VALUES (98001,'A'),(98002,'B') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed stores: %v", err)
	}
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role) VALUES ('area.hr@test.local','Area HR','area_hr') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	r := NewRepository(pool)
	a, err := r.Create(ctx, "Upper North Cluster")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM areas WHERE id = $1`, a.ID)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		pool.Exec(ctx, `DELETE FROM stores WHERE store_no IN (98001,98002)`)
	})

	// Set stores + members.
	if err := r.SetStores(ctx, a.ID, []int{98001, 98002}); err != nil {
		t.Fatalf("set stores: %v", err)
	}
	if err := r.SetMembers(ctx, a.ID, []uuid.UUID{userID}); err != nil {
		t.Fatalf("set members: %v", err)
	}

	got, err := r.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.StoreCount != 2 || len(got.StoreNos) != 2 {
		t.Errorf("store membership wrong: %+v", got)
	}
	if len(got.MemberIDs) != 1 || got.MemberIDs[0] != userID.String() {
		t.Errorf("member membership wrong: %+v", got.MemberIDs)
	}

	// Replace stores with a single one (proves the transactional clear+insert).
	if err := r.SetStores(ctx, a.ID, []int{98001}); err != nil {
		t.Fatalf("replace stores: %v", err)
	}
	got, _ = r.Get(ctx, a.ID)
	if got.StoreCount != 1 || got.StoreNos[0] != 98001 {
		t.Errorf("store replace wrong: %+v", got.StoreNos)
	}

	// List shows the area with its store count.
	list, err := r.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, x := range list {
		if x.ID == a.ID {
			found = true
			if x.StoreCount != 1 {
				t.Errorf("list store_count = %d, want 1", x.StoreCount)
			}
		}
	}
	if !found {
		t.Error("created area not in list")
	}

	// The area scope SQL resolves this user → store 98001 (proves end-to-end wiring).
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM area_stores WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $1)`,
		userID).Scan(&n); err != nil {
		t.Fatalf("scope resolve: %v", err)
	}
	if n != 1 {
		t.Errorf("area scope resolved %d stores for the user, want 1", n)
	}

	// User-side area coverage (the user-admin picker).
	if err := r.SetUserAreas(ctx, userID, []uuid.UUID{a.ID}); err != nil {
		t.Fatalf("set user areas: %v", err)
	}
	uareas, err := r.AreaIDsForUser(ctx, userID)
	if err != nil {
		t.Fatalf("area ids for user: %v", err)
	}
	if len(uareas) != 1 || uareas[0] != a.ID.String() {
		t.Errorf("user areas wrong: %v", uareas)
	}
	if err := r.SetUserAreas(ctx, userID, []uuid.UUID{}); err != nil {
		t.Fatalf("clear user areas: %v", err)
	}
	if got, _ := r.AreaIDsForUser(ctx, userID); len(got) != 0 {
		t.Errorf("user areas should be empty after clear, got %v", got)
	}

	// Update + delete.
	active := false
	if _, err := r.Update(ctx, a.ID, nil, &active); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := r.Delete(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.Get(ctx, a.ID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
