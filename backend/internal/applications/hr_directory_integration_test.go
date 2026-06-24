//go:build integration

package applications

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setupHRDir gives a clean users+stores slate with one store (no 1) and returns a
// pg-backed HRDirectory. Truncating stores CASCADEs users (users.store_id FK), so
// we seed the store first.
func setupHRDir(t *testing.T) (HRDirectory, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE users, stores RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name, subregion) VALUES (1,'A','Upper North'),(2,'B','East')`); err != nil {
		t.Fatalf("seed stores: %v", err)
	}
	return NewHRDirectory(pool), pool
}

func seedUser(t *testing.T, pool *pgxpool.Pool, email, role string, store *int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (email, full_name, role, is_active, store_id) VALUES ($1,$1,$2,TRUE,$3)`,
		email, role, store); err != nil {
		t.Fatalf("seed user %s/%s: %v", email, role, err)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestEmailsForStore_TransitionIncludesLegacyRoles is the split-deploy guard: with
// users still on the PRE-cutover roles (the live prod state before migration 000044),
// store-HR candidate notifications must still reach them. A regression here silently
// drops every store-HR email until cutover — the exact failure the transition lists
// exist to prevent.
func TestEmailsForStore_TransitionIncludesLegacyRoles(t *testing.T) {
	d, pool := setupHRDir(t)
	ctx := context.Background()
	store1 := 1

	seedUser(t, pool, "old-hr-staff@x.test", "hr_staff", &store1)             // pre-cutover
	seedUser(t, pool, "old-sgm@x.test", "sgm", &store1)                       // pre-cutover line manager
	seedUser(t, pool, "new-hr-store@x.test", "hr_store", &store1)             // post-cutover
	seedUser(t, pool, "other-store@x.test", "hr_staff", ptrInt(2))           // different store (must NOT leak)
	seedUser(t, pool, "wrong-role@x.test", "auditor", &store1)               // not a notify role

	got, err := d.EmailsForStore(ctx, &store1)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"old-hr-staff@x.test", "new-hr-store@x.test"} {
		if !contains(got, want) {
			t.Errorf("EmailsForStore missing %s (transition broken); got %v", want, got)
		}
	}
	if contains(got, "other-store@x.test") {
		t.Errorf("LEAK: EmailsForStore returned a different store's HR; got %v", got)
	}
	if contains(got, "wrong-role@x.test") {
		t.Errorf("EmailsForStore returned a non-notify role; got %v", got)
	}
}

// TestLineManagerEmails_TransitionIncludesSGM proves the Top-5 line-manager ping
// still reaches the pre-cutover line manager (sgm) and the post-cutover one.
func TestLineManagerEmails_TransitionIncludesSGM(t *testing.T) {
	d, pool := setupHRDir(t)
	ctx := context.Background()
	store1 := 1

	seedUser(t, pool, "sgm@x.test", "sgm", &store1)
	seedUser(t, pool, "hm-store@x.test", "hiring_manager_store", &store1)
	seedUser(t, pool, "hr@x.test", "hr_staff", &store1) // not a line manager

	got, err := d.LineManagerEmailsForStore(ctx, &store1)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "sgm@x.test") {
		t.Errorf("line-manager emails missing pre-cutover sgm; got %v", got)
	}
	if contains(got, "hr@x.test") {
		t.Errorf("line-manager emails wrongly included plain HR; got %v", got)
	}
}

// TestEmailsForRoleStore_TransitionUnion proves approval-chain approver resolution
// unions the new role with its pre-cutover equivalent, each under its own scope.
func TestEmailsForRoleStore_TransitionUnion(t *testing.T) {
	d, pool := setupHRDir(t)
	ctx := context.Background()
	store1 := 1

	// L1 chain role hr_store (store-scoped) ← legacy hr_staff (store-scoped).
	seedUser(t, pool, "new-l1@x.test", "hr_store", &store1)
	seedUser(t, pool, "old-l1@x.test", "hr_staff", &store1)
	got, err := d.EmailsForRoleStore(ctx, "hr_store", &store1)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "new-l1@x.test") || !contains(got, "old-l1@x.test") {
		t.Errorf("EmailsForRoleStore(hr_store) should union new+legacy; got %v", got)
	}

	// L4 chain role ta (all-scope) ← legacy regional_director (all-scope), store-agnostic.
	seedUser(t, pool, "new-l4@x.test", "ta", nil)
	seedUser(t, pool, "old-l4@x.test", "regional_director", nil)
	got4, err := d.EmailsForRoleStore(ctx, "ta", &store1)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got4, "new-l4@x.test") || !contains(got4, "old-l4@x.test") {
		t.Errorf("EmailsForRoleStore(ta) should union new+legacy store-agnostically; got %v", got4)
	}
}
