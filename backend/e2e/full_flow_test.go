//go:build e2e

package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestFullFlow drives the whole cross-system funnel: PeopleSoft opens a vacancy →
// the portal lists it → a candidate applies → the async pipeline scores/assigns →
// the dashboard inbox shows it → the candidate goes through the AI interview, the
// human-interview + approval funnel, and finally accepts an offer, which sets the
// application to hired and fires the mock PeopleSoft sync.
//
// The hire is offer-based: "hired" is reached only by the candidate accepting their
// offer (see internal/applications/transitions.go), not a direct status PATCH.
func TestFullFlow(t *testing.T) {
	waitHealthy(t)
	pool := mustPool(t)
	posID := seedPositionStore(t, pool, "E2E-CASHIER")
	// AFTER seedPositionStore: its TRUNCATE … stores CASCADE wipes users (FK
	// users.store_id → stores). Approval/offer stamp created_by under a FK to users.
	seedDevSuperAdmin(t, pool)

	// 1) PeopleSoft opens a vacancy mapped to the seeded position.
	resp := postJSON(t, "/api/v1/ps/vacancy-opened", map[string]any{
		"ps_vacancy_id": "E2E-V1", "store_id": 1, "position_code": "E2E-CASHIER", "headcount": 2,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("vacancy-opened expected 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 2) The portal lists the open position.
	var positions []struct {
		ID string `json:"id"`
	}
	if code, _ := getEnvelope(t, "/api/v1/public/positions", &positions); code != 200 {
		t.Fatalf("public positions expected 200, got %d", code)
	}
	found := false
	for _, p := range positions {
		if p.ID == posID.String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("opened position %s not in public list", posID)
	}

	// 3) Candidate applies (deterministic mock resume).
	token := applyMultipart(t, posID, "สมชาย ใจดี")

	// 4) Poll the DB until the async pipeline finishes (status leaves pending/parsed).
	var appID uuid.UUID
	var status string
	pollDB(t, 40*time.Second, func(ctx context.Context) bool {
		err := pool.QueryRow(ctx,
			`SELECT id, status FROM applications WHERE position_id=$1 ORDER BY created_at DESC LIMIT 1`, posID).
			Scan(&appID, &status)
		return err == nil && status != "pending" && status != "parsed"
	})
	if status != "scored" {
		t.Fatalf("expected pipeline to score the lenient position, got status=%q", status)
	}

	// 5) Public status endpoint resolves the token (single call — rate-limit-safe).
	var st struct {
		Status string `json:"status"`
	}
	if code, _ := getEnvelope(t, "/api/v1/public/status/"+token, &st); code != 200 {
		t.Fatalf("status lookup expected 200, got %d", code)
	}
	if st.Status == "" {
		t.Fatal("status lookup returned empty status")
	}

	// 6) Dashboard inbox (super_admin via MockJWT) shows the application.
	var apps []struct {
		ID string `json:"id"`
	}
	if code, _ := getEnvelope(t, "/api/v1/applications?limit=50", &apps); code != 200 {
		t.Fatalf("dashboard list expected 200, got %d", code)
	}
	if len(apps) == 0 {
		t.Fatal("dashboard inbox empty after apply")
	}

	appPath := "/api/v1/applications/" + appID.String()

	// 7) Send the AI pre-interview (scored → ai_interview); capture the chat token.
	var invite struct {
		AccessToken string `json:"access_token"`
	}
	if code := postEnvelope(t, appPath+"/interview", nil, &invite); code != 200 {
		t.Fatalf("send AI interview expected 200, got %d", code)
	}
	if invite.AccessToken == "" {
		t.Fatal("AI interview returned no access_token")
	}

	// 8) Candidate completes the AI interview. Drive the chat until done (the mock
	//    interviewer ends the script), then confirm via DB — the ai_interviewed
	//    advance is best-effort, so trust the row, not the HTTP turn.
	chatPath := "/api/v1/public/interview/" + invite.AccessToken
	var chat struct {
		Done bool `json:"done"`
	}
	if code, _ := getEnvelope(t, chatPath, &chat); code != 200 {
		t.Fatalf("interview start expected 200, got %d", code)
	}
	for i := 0; i < 12 && !chat.Done; i++ {
		if code := postEnvelope(t, chatPath+"/message", map[string]string{"content": "ตอบคำถามข้อนี้ครับ"}, &chat); code != 200 {
			t.Fatalf("interview message expected 200, got %d", code)
		}
	}
	if !chat.Done {
		t.Fatal("AI interview did not complete within 12 turns")
	}
	pollDB(t, 20*time.Second, func(ctx context.Context) bool {
		var s string
		_ = pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, appID).Scan(&s)
		return s == "ai_interviewed"
	})

	// 9) Shortlist (ai_interviewed → shortlisted).
	if r := patchJSON(t, appPath+"/status", map[string]string{"status": "shortlisted"}); r.StatusCode != 200 {
		t.Fatalf("shortlist expected 200, got %d", r.StatusCode)
	} else {
		_ = r.Body.Close()
	}

	// 10) Schedule a human interview (shortlisted → interview). Onsite avoids the
	//     calendar/Teams provider + candidate-email requirement of an online invite.
	sched := map[string]any{
		"mode": "onsite", "scheduled_at": "2027-01-04T03:00:00Z", "duration_min": 60, "location_text": "E2E Store",
	}
	if r := postJSON(t, appPath+"/interview-schedule", sched); r.StatusCode != 200 {
		t.Fatalf("schedule interview expected 200, got %d", r.StatusCode)
	} else {
		_ = r.Body.Close()
	}

	// 11) Mark the interview done (interview → interviewed).
	if r := patchJSON(t, appPath+"/status", map[string]string{"status": "interviewed"}); r.StatusCode != 200 {
		t.Fatalf("mark interviewed expected 200, got %d", r.StatusCode)
	} else {
		_ = r.Body.Close()
	}

	// 12) Submit for hiring approval (interviewed → pending_approval).
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if code := postEnvelope(t, appPath+"/approval-request", nil, &req); code != 201 {
		t.Fatalf("approval request expected 201, got %d", code)
	}
	if req.ID == "" {
		t.Fatal("approval request returned no id")
	}

	// 13) Decide the approval chain to completion. super_admin holds every
	//     approval.decide.lN, so it advances each level; loop until approved (app → offer).
	for i := 0; i < 6 && req.Status == "pending"; i++ {
		if code := postEnvelope(t, "/api/v1/approval-requests/"+req.ID+"/decide",
			map[string]string{"decision": "approve"}, &req); code != 200 {
			t.Fatalf("approval decide expected 200, got %d", code)
		}
	}
	if req.Status != "approved" {
		t.Fatalf("approval chain not approved, got status=%q", req.Status)
	}
	pollDB(t, 10*time.Second, func(ctx context.Context) bool {
		var s string
		_ = pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, appID).Scan(&s)
		return s == "offer"
	})

	// 14) Compose the offer (salary + start_date are required to send).
	var offer struct {
		ID string `json:"id"`
	}
	offerBody := map[string]any{
		"salary": 25000, "start_date": "2027-02-01T00:00:00Z", "terms": "E2E offer", "expires_at": "2027-01-15T00:00:00Z",
	}
	if code := postEnvelope(t, appPath+"/offer", offerBody, &offer); code != 201 {
		t.Fatalf("create offer expected 201, got %d", code)
	}
	if offer.ID == "" {
		t.Fatal("create offer returned no id")
	}

	// 15) Send the offer to the candidate.
	if r := postJSON(t, appPath+"/offer/send", nil); r.StatusCode != 200 {
		t.Fatalf("send offer expected 200, got %d", r.StatusCode)
	} else {
		_ = r.Body.Close()
	}

	// 16) The candidate accepts the offer → hired + mock PeopleSoft sync. The respond
	//     endpoint is candidate-session-authed (not MockJWT) and offer-ownership is
	//     account-scoped, so seed an active account, link this candidate to it, and
	//     mint a session cookie.
	sessionToken := seedCandidateSessionForApp(t, pool, appID)
	if r := postJSONCookie(t, "/api/v1/public/auth/offers/"+offer.ID+"/respond",
		map[string]string{"decision": "accept"}, sessionToken); r.StatusCode != 200 {
		t.Fatalf("accept offer expected 200, got %d", r.StatusCode)
	} else {
		_ = r.Body.Close()
	}

	// 17) The application is now hired.
	var hired string
	pollDB(t, 10*time.Second, func(ctx context.Context) bool {
		_ = pool.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1`, appID).Scan(&hired)
		return hired == "hired"
	})
	if hired != "hired" {
		t.Fatalf("expected status hired, got %q", hired)
	}
}

// seedDevSuperAdmin inserts the fixed users row that MockJWT impersonates in
// ENV=development. HR mutations in the funnel (approval-request, offer) stamp
// created_by with this id under a FK to users, so the row must exist.
func seedDevSuperAdmin(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const devID = "00000000-0000-0000-0000-000000000001" // mock_jwt.go DevUser.ID
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, full_name, role, is_active, source)
		 VALUES ($1, 'dev.superadmin@local.test', 'Dev Super Admin', 'super_admin', TRUE, 'local')
		 ON CONFLICT (id) DO NOTHING`, devID); err != nil {
		t.Fatalf("seed dev super_admin: %v", err)
	}
}

// seedCandidateSessionForApp provisions an active portal account, links the
// application's candidate to it (offer-acceptance is account-scoped), and mints a
// session whose plaintext token is returned for the cp_session cookie. Mirrors the
// candidateauth session storage: token_hash = hex(sha256(token)).
func seedCandidateSessionForApp(t *testing.T, pool *pgxpool.Pool, appID uuid.UUID) string {
	t.Helper()
	ctx := context.Background()

	var candID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT candidate_id FROM applications WHERE id=$1`, appID).Scan(&candID); err != nil {
		t.Fatalf("find candidate for app: %v", err)
	}

	email := "e2e-accept-" + uuid.NewString() + "@example.com"
	var acctID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidate_accounts (email, email_verified, status) VALUES ($1, TRUE, 'active') RETURNING id`,
		email).Scan(&acctID); err != nil {
		t.Fatalf("seed candidate account: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candidates SET account_id=$1 WHERE id=$2`, acctID, candID); err != nil {
		t.Fatalf("link candidate to account: %v", err)
	}

	sessionToken := uuid.NewString()
	sum := sha256.Sum256([]byte(sessionToken))
	if _, err := pool.Exec(ctx,
		`INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour')`,
		acctID, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("seed candidate session: %v", err)
	}
	return sessionToken
}
