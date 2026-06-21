//go:build integration

package dsar

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

// TestExport_ScopedToCaller is the critical DSAR security property: an export for
// account A must contain ONLY A's data, never another subject's, even when both
// have candidates/applications/consents in the same tables.
func TestExport_ScopedToCaller(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx,
		`TRUNCATE candidate_accounts, candidates, applications, positions, pdpa_consents RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var posID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	seed := func(name, email string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		var acctID, candID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO candidate_accounts (full_name, email, status) VALUES ($1,$2,'active') RETURNING id`,
			name, email).Scan(&acctID); err != nil {
			t.Fatalf("seed account %s: %v", name, err)
		}
		if err := pool.QueryRow(ctx,
			`INSERT INTO candidates (full_name, email, account_id, source_channel, status)
			 VALUES ($1,$2,$3,'career_portal','available') RETURNING id`,
			name, email, acctID).Scan(&candID); err != nil {
			t.Fatalf("seed candidate %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'scored')`, candID, posID); err != nil {
			t.Fatalf("seed application %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO pdpa_consents (account_id, consent_given, consent_version, source_channel) VALUES ($1,true,'1.0','account')`,
			acctID); err != nil {
			t.Fatalf("seed consent %s: %v", name, err)
		}
		return acctID, candID
	}

	acctA, candA := seed("มานี เอ", "a@example.com")
	_, _ = seed("สมชาย บี", "b@example.com")

	svc := New(pool)
	exp, err := svc.Export(ctx, acctA)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if exp.Account.ID != acctA || exp.Account.Email != "a@example.com" {
		t.Errorf("account mismatch: got %s / %s", exp.Account.ID, exp.Account.Email)
	}
	if len(exp.Candidates) != 1 || exp.Candidates[0].ID != candA {
		t.Fatalf("expected exactly candidate A, got %+v", exp.Candidates)
	}
	if len(exp.Applications) != 1 {
		t.Errorf("expected 1 application for A, got %d", len(exp.Applications))
	}
	if len(exp.Consents) != 1 {
		t.Errorf("expected 1 consent row for A, got %d", len(exp.Consents))
	}
	// Nothing belonging to B may appear.
	for _, c := range exp.Candidates {
		if c.Email == "b@example.com" {
			t.Fatal("LEAK: account B's candidate appeared in A's export")
		}
	}
	for _, ce := range exp.Consents {
		if ce.Version != "1.0" || !ce.Given {
			t.Errorf("unexpected consent row: %+v", ce)
		}
	}
}
