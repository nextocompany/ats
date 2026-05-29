//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestFullFlow: PS vacancy-opened → portal sees position → apply → pipeline
// scores/assigns → dashboard inbox shows it → hire → PS sync (mock).
func TestFullFlow(t *testing.T) {
	waitHealthy(t)
	pool := mustPool(t)
	posID := seedPositionStore(t, pool, "E2E-CASHIER")

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
		ID      string `json:"id"`
		TitleTH string `json:"title_th"`
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
		Status   string `json:"status"`
		Position string `json:"position"`
	}
	if code, _ := getEnvelope(t, "/api/v1/public/status/"+token, &st); code != 200 {
		t.Fatalf("status lookup expected 200, got %d", code)
	}
	if st.Status == "" {
		t.Fatal("status lookup returned empty status")
	}

	// 6) Dashboard inbox (super_admin via MockJWT) shows the application.
	var apps []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if code, _ := getEnvelope(t, "/api/v1/applications?limit=50", &apps); code != 200 {
		t.Fatalf("dashboard list expected 200, got %d", code)
	}
	if len(apps) == 0 {
		t.Fatal("dashboard inbox empty after apply")
	}

	// 7) Hire → triggers mock PeopleSoft sync (never fails the hire).
	hr := patchJSON(t, "/api/v1/applications/"+appID.String()+"/status", map[string]string{"status": "hired"})
	if hr.StatusCode != 200 {
		t.Fatalf("hire expected 200, got %d", hr.StatusCode)
	}
	_ = hr.Body.Close()

	var hired string
	_ = pool.QueryRow(context.Background(), `SELECT status FROM applications WHERE id=$1`, appID).Scan(&hired)
	if hired != "hired" {
		t.Fatalf("expected status hired, got %q", hired)
	}
}
