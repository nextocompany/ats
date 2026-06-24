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

// fakeBlobDeleter records the URLs/keys it was asked to delete (via either form).
type fakeBlobDeleter struct{ deleted []string }

func (f *fakeBlobDeleter) DeleteStored(_ context.Context, storedURL string) error {
	f.deleted = append(f.deleted, storedURL)
	return nil
}

func (f *fakeBlobDeleter) Delete(_ context.Context, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

// fakeIndexer records the candidate IDs it was asked to delete from the index.
type fakeIndexer struct{ deleted []string }

func (f *fakeIndexer) Delete(_ context.Context, candidateIDs []string) error {
	f.deleted = append(f.deleted, candidateIDs...)
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
	expiredTerminal uuid.UUID // expired + only terminal (rejected) app → eligible
	expiredActive   uuid.UUID // expired + active (pending) app → skipped
	expiredHired    uuid.UUID // expired + hired app → skipped (retained for HR/PS)
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

	// 3) Expired (400 days) but HIRED → must be skipped (retained for HR/PS).
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, source_channel, status, created_at)
		 VALUES ('ผ่าน ได้งาน','0800000004','career_portal','available', NOW() - INTERVAL '400 days')
		 RETURNING id`).Scan(&ids.expiredHired); err != nil {
		t.Fatalf("seed expiredHired: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'hired')`,
		ids.expiredHired, posID); err != nil {
		t.Fatalf("seed expiredHired app: %v", err)
	}

	// 4) Recent candidate (within window) with no applications → must be skipped.
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
	idx := &fakeIndexer{}
	audit := &fakeAudit{}
	svc := NewRetentionService(pool, blob, idx, audit, 365)

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

	// Candidates 2, 3 (hired) & 4 untouched.
	for _, id := range []uuid.UUID{ids.expiredActive, ids.expiredHired, ids.recent} {
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
	idx := &fakeIndexer{}
	audit := &fakeAudit{}
	svc := NewRetentionService(pool, blob, idx, audit, 365)

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

// TestEraseSubject_Completeness is the PDPA acceptance test: one erasure must
// clear EVERY personal-data store in the inventory (DB + blobs + search index).
// It seeds one expired candidate fully linked to a portal account with a row in
// every PII-bearing table, runs the sweep, and asserts nothing personal remains.
func TestEraseSubject_Completeness(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx,
		`TRUNCATE candidate_accounts, candidates, applications, positions, pdpa_consents, notifications, email_otps
		 RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var posID, acctID, candID, appID uuid.UUID

	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	// Portal account with a bare-key resume blob (portal-upload form).
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts
			(full_name, email, phone, line_user_id, google_sub, province, resume_blob_url, resume_file_type, status)
		 VALUES ('มานี ทดสอบ','manee@example.com','0810000000','LINEUSER1','GOOGLESUB1','กรุงเทพ',
			'portal/acct-resume.pdf','pdf','active')
		 RETURNING id`).Scan(&acctID); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	// Expired candidate linked to the account (with a LINE OAuth subject id).
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates
			(full_name, phone, email, id_card, address, date_of_birth, line_user_id, account_id, source_channel, status, created_at)
		 VALUES ('มานี ทดสอบ','0810000000','manee@example.com','1100000000099','99/9 กรุงเทพ','1990-01-01','LINEUSERCAND',
			$1,'career_portal','available', NOW() - INTERVAL '400 days')
		 RETURNING id`, acctID).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO applications
			(candidate_id, position_id, status, resume_blob_url, resume_original_filename,
			 raw_file_blob_url, ocr_text_blob_url, parsed_profile_blob_url, ai_summary, ai_red_flags)
		 VALUES ($1,$2,'rejected',
			'http://blob/resumes/r1.pdf','r1.pdf',
			'http://blob/resumes/raw1.pdf','http://blob/resumes/ocr1.txt','http://blob/resumes/parsed1.json',
			'summary pii','flags pii')
		 RETURNING id`, candID, posID).Scan(&appID); err != nil {
		t.Fatalf("seed application: %v", err)
	}

	exec := func(label, q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("seed %s: %v", label, err)
		}
	}
	exec("consent", `INSERT INTO pdpa_consents (candidate_id, consent_given, ip_address) VALUES ($1,true,'203.0.113.9'::inet)`, candID)
	exec("interview_session", `INSERT INTO interview_sessions (application_id, access_token, conversation, summary, strengths, concerns)
		VALUES ($1,'tok-erase-1','[{"role":"user","content":"my id is 1234"}]'::jsonb,'eval pii','["s"]'::jsonb,'["c"]'::jsonb)`, appID)
	exec("fit_analysis", `INSERT INTO application_fit_analyses (application_id, summary, strengths, concerns, no_match_reason)
		VALUES ($1,'fit pii','["fs"]'::jsonb,'["fc"]'::jsonb,'reason pii')`, appID)
	exec("interview_feedback", `INSERT INTO interview_feedback (application_id, overall_rating, recommendation, strengths, concerns, notes)
		VALUES ($1,4,'pass','strong pii','concern pii','notes pii')`, appID)
	exec("offer", `INSERT INTO offers (application_id, status, terms, decline_reason, salary, start_date) VALUES ($1,'declined','terms pii','declined pii',45000.00,'2026-07-01')`, appID)
	exec("appointment", `INSERT INTO interview_appointments (application_id, scheduled_at, mode, location_text, online_join_url, calendar_event_id)
		VALUES ($1, NOW(), 'online','สาขาพระราม9','https://teams/join/pii','evt-pii')`, appID)
	var reqID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO approval_requests (application_id, decision_reason) VALUES ($1,'reject reason pii') RETURNING id`, appID).Scan(&reqID); err != nil {
		t.Fatalf("seed approval_request: %v", err)
	}
	exec("approval_step", `INSERT INTO approval_steps (request_id, level, role, comment) VALUES ($1,1,'hr_manager','approver comment pii')`, reqID)
	exec("reengage_contact", `INSERT INTO reengagement_contacts (candidate_id, position_id, channel) VALUES ($1,$2,'line')`, candID, posID)
	exec("reengage_log", `INSERT INTO reengagement_logs (candidate_id, trigger_type, response) VALUES ($1,'vacancy_open','interested')`, candID)
	exec("email_otp", `INSERT INTO email_otps (email, code_hash, expires_at) VALUES ('manee@example.com','hash-otp', NOW() + INTERVAL '10 min')`)
	exec("onboarding_doc", `INSERT INTO onboarding_documents (application_id, doc_type, blob_url, uploaded_by) VALUES ($1,'id_card','onboarding/doc1.pdf',$2)`, appID, acctID)
	exec("letter", `INSERT INTO letters (application_id, type, blob_url) VALUES ($1,'offer','letters/offer1.pdf')`, appID)
	exec("notification", `INSERT INTO notifications (candidate_id, channel, template, payload) VALUES ($1,'line','offer','{"name":"มานี"}'::jsonb)`, candID)
	exec("member_note", `INSERT INTO member_notes (account_id, author_email, body) VALUES ($1,'hr@x.com','sensitive note')`, acctID)
	exec("member_tag", `INSERT INTO member_tags (account_id, tag) VALUES ($1,'vip')`, acctID)
	exec("session", `INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1,'hash-erase-1', NOW() + INTERVAL '1 day')`, acctID)
	// Resume library: a second CV the candidate kept on the portal (its blob must
	// also be erased, and the row deleted).
	exec("account_resume_library", `INSERT INTO candidate_account_resumes (account_id, blob_key, file_type) VALUES ($1,'portal/lib-resume.pdf','pdf')`, acctID)

	blob := &fakeBlobDeleter{}
	idx := &fakeIndexer{}
	audit := &fakeAudit{}
	svc := NewRetentionService(pool, blob, idx, audit, 365)

	n, err := svc.Sweep(ctx, 500)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 erased, got %d", n)
	}

	// --- assert every store is cleared ---
	assertEmpty := func(label, q string, args ...any) {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, q, args...).Scan(&count); err != nil {
			t.Fatalf("check %s: %v", label, err)
		}
		if count != 0 {
			t.Errorf("%s: expected 0 personal rows remaining, got %d", label, count)
		}
	}

	assertEmpty("candidate identifiers",
		`SELECT count(*) FROM candidates WHERE id=$1 AND (phone IS NOT NULL OR email IS NOT NULL OR id_card IS NOT NULL OR address IS NOT NULL OR date_of_birth IS NOT NULL OR line_user_id IS NOT NULL OR pdpa_anonymized_at IS NULL OR full_name <> $2)`, candID, redactedName)
	assertEmpty("application PII",
		`SELECT count(*) FROM applications WHERE candidate_id=$1 AND (resume_blob_url IS NOT NULL OR raw_file_blob_url IS NOT NULL OR ocr_text_blob_url IS NOT NULL OR parsed_profile_blob_url IS NOT NULL OR ai_summary IS NOT NULL OR ai_red_flags IS NOT NULL)`, candID)
	assertEmpty("consent ip", `SELECT count(*) FROM pdpa_consents WHERE candidate_id=$1 AND ip_address IS NOT NULL`, candID)
	assertEmpty("interview session free-text",
		`SELECT count(*) FROM interview_sessions WHERE application_id=$1 AND (conversation <> '[]'::jsonb OR summary IS NOT NULL OR strengths IS NOT NULL OR concerns IS NOT NULL)`, appID)
	assertEmpty("fit analysis free-text",
		`SELECT count(*) FROM application_fit_analyses WHERE application_id=$1 AND (summary <> '' OR strengths <> '[]'::jsonb OR concerns <> '[]'::jsonb OR no_match_reason <> '')`, appID)
	assertEmpty("interview feedback free-text",
		`SELECT count(*) FROM interview_feedback WHERE application_id=$1 AND (strengths IS NOT NULL OR concerns IS NOT NULL OR notes IS NOT NULL)`, appID)
	assertEmpty("offer free-text + financial",
		`SELECT count(*) FROM offers WHERE application_id=$1 AND (terms IS NOT NULL OR decline_reason IS NOT NULL OR salary IS NOT NULL OR start_date IS NOT NULL)`, appID)
	assertEmpty("interview session token still live",
		`SELECT count(*) FROM interview_sessions WHERE application_id=$1 AND access_token NOT LIKE 'erased:%'`, appID)
	assertEmpty("approval step comment",
		`SELECT count(*) FROM approval_steps WHERE request_id=$1 AND comment IS NOT NULL`, reqID)
	assertEmpty("approval request reason",
		`SELECT count(*) FROM approval_requests WHERE application_id=$1 AND decision_reason IS NOT NULL`, appID)
	assertEmpty("reengagement contacts", `SELECT count(*) FROM reengagement_contacts WHERE candidate_id=$1`, candID)
	assertEmpty("reengagement logs", `SELECT count(*) FROM reengagement_logs WHERE candidate_id=$1`, candID)
	assertEmpty("email otps", `SELECT count(*) FROM email_otps WHERE email='manee@example.com'`)
	assertEmpty("interview appointment links",
		`SELECT count(*) FROM interview_appointments WHERE application_id=$1 AND (location_text IS NOT NULL OR online_join_url IS NOT NULL OR calendar_event_id IS NOT NULL)`, appID)
	assertEmpty("onboarding documents", `SELECT count(*) FROM onboarding_documents WHERE application_id=$1`, appID)
	assertEmpty("letters", `SELECT count(*) FROM letters WHERE application_id=$1`, appID)
	assertEmpty("notification payload", `SELECT count(*) FROM notifications WHERE candidate_id=$1 AND payload IS NOT NULL`, candID)
	assertEmpty("account identifiers",
		`SELECT count(*) FROM candidate_accounts WHERE id=$1 AND (email IS NOT NULL OR phone IS NOT NULL OR line_user_id IS NOT NULL OR google_sub IS NOT NULL OR province IS NOT NULL OR resume_blob_url IS NOT NULL OR status <> 'anonymized' OR full_name <> $2)`, acctID, redactedName)
	assertEmpty("member notes", `SELECT count(*) FROM member_notes WHERE account_id=$1`, acctID)
	assertEmpty("member tags", `SELECT count(*) FROM member_tags WHERE account_id=$1`, acctID)
	assertEmpty("candidate sessions", `SELECT count(*) FROM candidate_sessions WHERE account_id=$1`, acctID)
	assertEmpty("account resume library", `SELECT count(*) FROM candidate_account_resumes WHERE account_id=$1`, acctID)

	// --- assert external stores erased ---
	wantBlobs := map[string]bool{
		"http://blob/resumes/r1.pdf": true, "http://blob/resumes/raw1.pdf": true,
		"http://blob/resumes/ocr1.txt": true, "http://blob/resumes/parsed1.json": true,
		"onboarding/doc1.pdf": true, "letters/offer1.pdf": true, "portal/acct-resume.pdf": true,
		"portal/lib-resume.pdf": true,
	}
	if len(blob.deleted) != len(wantBlobs) {
		t.Errorf("expected %d blob deletes, got %d: %v", len(wantBlobs), len(blob.deleted), blob.deleted)
	}
	for _, got := range blob.deleted {
		if !wantBlobs[got] {
			t.Errorf("unexpected blob delete: %s", got)
		}
		delete(wantBlobs, got)
	}
	for missing := range wantBlobs {
		t.Errorf("expected blob %q deleted but it was not", missing)
	}

	if len(idx.deleted) != 1 || idx.deleted[0] != candID.String() {
		t.Errorf("expected search index delete for %s, got %v", candID, idx.deleted)
	}
}
