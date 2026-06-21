//go:build integration

package pdpa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// connectV2 opens a pool to the migrated test DB (shares the dsn() helper from
// retention_integration_test.go).
func connectV2(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestCurrentVersion_FromRegistry asserts the seeded v1.0 is reported current and
// that promoting a newer version flips it.
func TestCurrentVersion_FromRegistry(t *testing.T) {
	ctx := context.Background()
	pool := connectV2(t)
	r := New(pool)

	v, err := r.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("current version: %v", err)
	}
	if v != "1.0" {
		t.Fatalf("expected seeded current version 1.0, got %q", v)
	}

	// Promote a v2.0 (th); 1.0 stays valid history but is no longer current.
	if _, err := pool.Exec(ctx, `UPDATE consent_documents SET is_current = FALSE WHERE locale = 'th'`); err != nil {
		t.Fatalf("demote: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO consent_documents (version, locale, title, body, is_current)
		 VALUES ('2.0','th','ใหม่','ข้อความใหม่', TRUE)
		 ON CONFLICT (version, locale) DO UPDATE SET is_current = TRUE`); err != nil {
		t.Fatalf("promote: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM consent_documents WHERE version = '2.0'`)
		_, _ = pool.Exec(context.Background(), `UPDATE consent_documents SET is_current = TRUE WHERE version = '1.0'`)
	})

	v, err = r.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("current version after promote: %v", err)
	}
	if v != "2.0" {
		t.Errorf("expected current 2.0 after promote, got %q", v)
	}

	doc, err := r.CurrentDocuments(ctx, "en")
	if err != nil {
		t.Fatalf("current doc en: %v", err)
	}
	// en has no v2.0 current row, so it falls back to th's current (v2.0).
	if doc.Version != "2.0" {
		t.Errorf("expected en fallback to current th doc 2.0, got %q (%s)", doc.Version, doc.Locale)
	}
}

// TestConsentLedgerUnified_AccountRow asserts an account-keyed consent row can be
// written to the unified ledger (the column added in migration 000031).
func TestConsentLedgerUnified_AccountRow(t *testing.T) {
	ctx := context.Background()
	pool := connectV2(t)
	r := New(pool)

	var acctID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts (full_name, email, status) VALUES ('เทส','ledger@example.com','active') RETURNING id`,
	).Scan(&acctID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pdpa_consents WHERE account_id = $1`, acctID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM candidate_accounts WHERE id = $1`, acctID)
	})

	ver, err := r.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("current version: %v", err)
	}
	if err := r.Record(ctx, Consent{AccountID: &acctID, ConsentGiven: true, Version: ver, SourceChannel: "account"}, ""); err != nil {
		// Record updates candidates by candidate_id; with only account_id the
		// candidate UPDATE affects 0 rows, which is fine. Assert the ledger row.
		t.Fatalf("record account consent: %v", err)
	}

	var given bool
	var source string
	if err := pool.QueryRow(ctx,
		`SELECT consent_given, source_channel FROM pdpa_consents WHERE account_id = $1 ORDER BY created_at DESC LIMIT 1`, acctID,
	).Scan(&given, &source); err != nil {
		t.Fatalf("read ledger row: %v", err)
	}
	if !given || source != "account" {
		t.Errorf("expected given=true source=account, got given=%v source=%q", given, source)
	}
}
