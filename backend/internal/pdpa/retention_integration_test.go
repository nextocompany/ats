//go:build integration

package pdpa

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

// fakeBlobDeleter records the URLs it was asked to delete.
type fakeBlobDeleter struct{ deleted []string }

func (f *fakeBlobDeleter) DeleteStored(_ context.Context, storedURL string) error {
	f.deleted = append(f.deleted, storedURL)
	return nil
}

// fakeAudit records retention audit entries.
type fakeAudit struct{ records int }

func (f *fakeAudit) Record(_ context.Context, _ string, _ string, _ uuid.UUID, _ any) error {
	f.records++
	return nil
}

// seededIDs holds the candidate IDs created by setupRetention.
type seededIDs struct {
	expiredTerminal uuid.UUID // expired + only terminal app → eligible
	expiredActive   uuid.UUID // expired + active app → skipped
	recent          uuid.UUID // within window, no apps → skipped
}

func setupRetention(t *testing.T) (*pgxpool.Pool, seededIDs) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx,
		`TRUNCATE pdpa_consents, applications, candidates, positions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var posID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	var ids seededIDs

	// 1) Expired (400 days) + a terminal (rejected) application carrying a resume blob.
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, email, id_card, source_channel, status, created_at)
		 VALUES ('สมชาย ใจดี','0800000001','a@example.com','1100000000001','career_portal','available', NOW() - INTERVAL '400 days')
		 RETURNING id`).Scan(&ids.expiredTerminal); err != nil {
		t.Fatalf("seed expiredTerminal: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications
			(candidate_id, position_id, status, resume_blob_url, resume_original_filename,
			 raw_file_blob_url, ocr_text_blob_url, parsed_profile_blob_url, ai_summary, ai_red_flags)
		 VALUES ($1,$2,'rejected',
			'http://blob/resumes/r1.pdf','r1.pdf',
			'http://blob/resumes/raw1.pdf','http://blob/resumes/ocr1.txt','http://blob/resumes/parsed1.json',
			'summary pii','flags pii')`,
		ids.expiredTerminal, posID); err != nil {
		t.Fatalf("seed expiredTerminal app: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO pdpa_consents (candidate_id, consent_given, ip_address) VALUES ($1,true,'203.0.113.5'::inet)`,
		ids.expiredTerminal); err != nil {
		t.Fatalf("seed consent: %v", err)
	}

	// 2) Expired (400 days) but with an ACTIVE (pending) application → must be skipped.
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, source_channel, status, created_at)
		 VALUES ('สมหญิง รอผล','0800000002','career_portal','available', NOW() - INTERVAL '400 days')
		 RETURNING id`).Scan(&ids.expiredActive); err != nil {
		t.Fatalf("seed expiredActive: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'pending')`,
		ids.expiredActive, posID); err != nil {
		t.Fatalf("seed expiredActive app: %v", err)
	}

	// 3) Recent candidate (within window) with no applications → must be skipped.
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, source_channel, status)
		 VALUES ('ใหม่ เพิ่งสมัคร','0800000003','career_portal','available')
		 RETURNING id`).Scan(&ids.recent); err != nil {
		t.Fatalf("seed recent: %v", err)
	}

	return pool, ids
}

func TestSweep_AnonymizesExpiredTerminal(t *testing.T) {
	ctx := context.Background()
	pool, ids := setupRetention(t)
	blob := &fakeBlobDeleter{}
	audit := &fakeAudit{}
	svc := NewRetentionService(pool, blob, audit, 365)

	n, err := svc.Sweep(ctx, 500)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 anonymized, got %d", n)
	}

	// Candidate 1 redacted.
	var name string
	var phone, email, idCard *string
	var anonAt *string
	if err := pool.QueryRow(ctx,
		`SELECT full_name, phone, email, id_card, pdpa_anonymized_at::text FROM candidates WHERE id=$1`,
		ids.expiredTerminal).Scan(&name, &phone, &email, &idCard, &anonAt); err != nil {
		t.Fatalf("read candidate 1: %v", err)
	}
	if name != redactedName {
		t.Errorf("expected name %q, got %q", redactedName, name)
	}
	if phone != nil || email != nil || idCard != nil {
		t.Errorf("expected phone/email/id_card NULL, got %v/%v/%v", phone, email, idCard)
	}
	if anonAt == nil {
		t.Error("expected pdpa_anonymized_at set")
	}

	// Its application redacted — every resume-derived blob pointer + free-text PII.
	var resumeURL, resumeName, rawURL, ocrURL, parsedURL, aiSummary, aiFlags *string
	if err := pool.QueryRow(ctx,
		`SELECT resume_blob_url, resume_original_filename, raw_file_blob_url, ocr_text_blob_url,
		        parsed_profile_blob_url, ai_summary, ai_red_flags
		 FROM applications WHERE candidate_id=$1`,
		ids.expiredTerminal).Scan(&resumeURL, &resumeName, &rawURL, &ocrURL, &parsedURL, &aiSummary, &aiFlags); err != nil {
		t.Fatalf("read app 1: %v", err)
	}
	if resumeURL != nil || resumeName != nil || rawURL != nil || ocrURL != nil ||
		parsedURL != nil || aiSummary != nil || aiFlags != nil {
		t.Errorf("expected all application PII NULL, got resume=%v raw=%v ocr=%v parsed=%v summary=%v flags=%v",
			resumeURL, rawURL, ocrURL, parsedURL, aiSummary, aiFlags)
	}

	// Consent IP nulled, consent row kept.
	var ip *string
	if err := pool.QueryRow(ctx,
		`SELECT ip_address::text FROM pdpa_consents WHERE candidate_id=$1`, ids.expiredTerminal).Scan(&ip); err != nil {
		t.Fatalf("read consent: %v", err)
	}
	if ip != nil {
		t.Errorf("expected consent ip NULL, got %v", *ip)
	}

	// Side effects: all four resume-derived blobs deleted from storage.
	if len(blob.deleted) != 4 {
		t.Errorf("expected 4 blobs deleted, got %d: %v", len(blob.deleted), blob.deleted)
	}
	for _, want := range []string{
		"http://blob/resumes/r1.pdf", "http://blob/resumes/raw1.pdf",
		"http://blob/resumes/ocr1.txt", "http://blob/resumes/parsed1.json",
	} {
		found := false
		for _, got := range blob.deleted {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected blob %q deleted, deletes were %v", want, blob.deleted)
		}
	}
	if audit.records != 1 {
		t.Errorf("expected 1 audit record, got %d", audit.records)
	}

	// Candidates 2 & 3 untouched.
	for _, id := range []uuid.UUID{ids.expiredActive, ids.recent} {
		var anon *string
		if err := pool.QueryRow(ctx,
			`SELECT pdpa_anonymized_at::text FROM candidates WHERE id=$1`, id).Scan(&anon); err != nil {
			t.Fatalf("read skipped candidate %s: %v", id, err)
		}
		if anon != nil {
			t.Errorf("candidate %s should NOT be anonymized", id)
		}
	}
}

func TestSweep_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool, _ := setupRetention(t)
	blob := &fakeBlobDeleter{}
	audit := &fakeAudit{}
	svc := NewRetentionService(pool, blob, audit, 365)

	if _, err := svc.Sweep(ctx, 500); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	n, err := svc.Sweep(ctx, 500)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 on second sweep, got %d", n)
	}
	if len(blob.deleted) != 4 {
		t.Errorf("expected blob deletes not repeated (4 from first sweep only), total deletes = %d", len(blob.deleted))
	}
}
