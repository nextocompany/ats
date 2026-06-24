//go:build integration

package hrauth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func emailDSN() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// TestFindByEmail_ResolvesSSOUser covers the linchpin SQL: FindByEmail must resolve
// an active SSO-provisioned account that has NO password (unlike
// FindCredentialsByEmail), case-insensitively. This is the production path that
// stamps interview created_by + names the HR attendee — its silent failure mode is
// a permanently-empty "mine" + email-as-name, so it needs real-PG coverage.
func TestFindByEmail_ResolvesSSOUser(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, emailDSN())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE email = 'sso.find@x.test'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, full_name, role, source, azure_ad_oid, is_active)
		 VALUES ('sso.find@x.test','SSO Finder','hr_staff','sso','oid-find-1',TRUE) RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed sso user (no password): %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = 'sso.find@x.test'`) })

	repo := NewRepository(pool)

	// Exact + case-insensitive both resolve the SSO user.
	for _, in := range []string{"sso.find@x.test", "SSO.Find@X.TEST"} {
		u, err := repo.FindByEmail(ctx, in)
		if err != nil {
			t.Fatalf("FindByEmail(%q): %v (SSO user with no password must still resolve)", in, err)
		}
		if u.ID.String() != id || u.FullName != "SSO Finder" {
			t.Errorf("FindByEmail(%q) = id %s name %q, want %s SSO Finder", in, u.ID, u.FullName, id)
		}
	}

	// Unknown email → ErrNotFound (not a different user, not a nil error).
	if _, err := repo.FindByEmail(ctx, "nobody@x.test"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByEmail(unknown) err = %v, want ErrNotFound", err)
	}
}
