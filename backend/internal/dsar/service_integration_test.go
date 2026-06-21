//go:build integration

package dsar

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/pdpa"
)

// fakeBlob satisfies pdpa.BlobDeleter; the test subjects carry no blobs so it is
// never actually invoked, but the eraser requires a non-nil deleter.
type fakeBlob struct{}

func (fakeBlob) Delete(context.Context, string) error       { return nil }
func (fakeBlob) DeleteStored(context.Context, string) error { return nil }

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
		`TRUNCATE candidate_accounts, candidates, applications, positions, pdpa_consents, dsar_requests RESTART IDENTITY CASCADE`); err != nil {
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

// TestRequestErasure_ImmediateAndHeld asserts self-service erasure erases an
// eligible subject immediately, but queues (and does NOT erase) a subject under
// legal hold (a hired application).
func TestRequestErasure_ImmediateAndHeld(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx,
		`TRUNCATE candidate_accounts, candidates, applications, positions, pdpa_consents, dsar_requests RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var posID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	seed := func(name, email, appStatus string) uuid.UUID {
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
			`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,$3)`, candID, posID, appStatus); err != nil {
			t.Fatalf("seed application %s: %v", name, err)
		}
		return acctID
	}

	svc := New(pool).WithEraser(pdpa.NewRetentionService(pool, fakeBlob{}, nil, nil, 365))

	// 1) Eligible subject (rejected application) → erased immediately.
	eligible := seed("ลบ ได้", "erase@example.com", "rejected")
	res, err := svc.RequestErasure(ctx, eligible)
	if err != nil {
		t.Fatalf("erase eligible: %v", err)
	}
	if res != ErasureDone {
		t.Errorf("expected ErasureDone, got %q", res)
	}
	var acctStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM candidate_accounts WHERE id=$1`, eligible).Scan(&acctStatus); err != nil {
		t.Fatalf("read eligible account: %v", err)
	}
	if acctStatus != "anonymized" {
		t.Errorf("expected eligible account anonymized, got %q", acctStatus)
	}
	var eligibleQueued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM dsar_requests WHERE account_id=$1`, eligible).Scan(&eligibleQueued); err != nil {
		t.Fatalf("count eligible queued: %v", err)
	}
	if eligibleQueued != 0 {
		t.Errorf("eligible erase should not queue a request, got %d", eligibleQueued)
	}

	// 2) Held subject (hired application) → queued, NOT erased.
	held := seed("จ้าง แล้ว", "held@example.com", "hired")
	res, err = svc.RequestErasure(ctx, held)
	if err != nil {
		t.Fatalf("erase held: %v", err)
	}
	if res != ErasureHeld {
		t.Errorf("expected ErasureHeld, got %q", res)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM candidate_accounts WHERE id=$1`, held).Scan(&acctStatus); err != nil {
		t.Fatalf("read held account: %v", err)
	}
	if acctStatus == "anonymized" {
		t.Error("held (hired) account must NOT be erased")
	}
	var heldQueued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM dsar_requests WHERE account_id=$1 AND status='pending'`, held).Scan(&heldQueued); err != nil {
		t.Fatalf("count held queued: %v", err)
	}
	if heldQueued != 1 {
		t.Errorf("expected 1 pending dsar_request for held subject, got %d", heldQueued)
	}

	// Idempotent: a second held request does not create a duplicate row.
	if _, err := svc.RequestErasure(ctx, held); err != nil {
		t.Fatalf("erase held (2nd): %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM dsar_requests WHERE account_id=$1 AND status='pending'`, held).Scan(&heldQueued); err != nil {
		t.Fatalf("recount held queued: %v", err)
	}
	if heldQueued != 1 {
		t.Errorf("expected still 1 pending request after retry, got %d", heldQueued)
	}
}
