//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestReengageFlow: a rejected applicant with a LINE handle + manual re-engage
// trigger → a reengagement_contacts row (worker processes the async job,
// mock-notify). The candidate needs a line_user_id (or email) to be reachable —
// phone alone is not a valid LINE push recipient (slice 2.3).
func TestReengageFlow(t *testing.T) {
	waitHealthy(t)
	pool := mustPool(t)
	posID := seedPositionStore(t, pool, "E2E-REENGAGE")
	ctx := context.Background()

	var candID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, phone, line_user_id, source_channel, status) VALUES ('รีเอนเกจ E2E','0899999999','U-e2e-reengage','career_portal','available') RETURNING id`).
		Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, talent_pool) VALUES ($1,$2,'rejected',false)`, candID, posID); err != nil {
		t.Fatalf("seed application: %v", err)
	}

	resp := postJSON(t, "/api/v1/positions/"+posID.String()+"/reengage", map[string]any{})
	if resp.StatusCode != 201 && resp.StatusCode != 202 {
		t.Fatalf("reengage expected 201/202, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	pollDB(t, 20*time.Second, func(ctx context.Context) bool {
		var n int
		_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM reengagement_contacts WHERE position_id=$1`, posID).Scan(&n)
		return n >= 1
	})
}

// TestReportsAndSearchFlow: after a hire exists, analytics reflect it, an
// on-demand export is created, and the candidate is findable via search.
func TestReportsAndSearchFlow(t *testing.T) {
	waitHealthy(t)
	pool := mustPool(t)
	posID := seedPositionStore(t, pool, "E2E-REPORTS")
	ctx := context.Background()

	var candID uuid.UUID
	_ = pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, province, source_channel, status) VALUES ('สมหญิง ค้นหา','Bangkok','career_portal','available') RETURNING id`).
		Scan(&candID)
	if _, err := pool.Exec(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, must_have_passed, assigned_store_id) VALUES ($1,$2,'hired',90,true,1)`, candID, posID); err != nil {
		t.Fatalf("seed hired app: %v", err)
	}

	// Funnel reflects the hire.
	var funnel struct {
		Applied int `json:"applied"`
		Hired   int `json:"hired"`
	}
	if code, _ := getEnvelope(t, "/api/v1/reports/funnel", &funnel); code != 200 || funnel.Applied < 1 || funnel.Hired < 1 {
		t.Fatalf("funnel wrong: code=%d %+v", code, funnel)
	}

	// On-demand export persists a row.
	resp := postJSON(t, "/api/v1/reports/exports", map[string]any{})
	if resp.StatusCode != 201 {
		t.Fatalf("export expected 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	var exports int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_exports`).Scan(&exports)
	if exports < 1 {
		t.Fatal("no report_exports row after on-demand export")
	}

	// Search finds the candidate (scoped, super_admin).
	var hits []struct {
		FullName string `json:"full_name"`
	}
	if code, _ := getEnvelope(t, "/api/v1/candidates/search?q=%E0%B8%AA%E0%B8%A1%E0%B8%AB%E0%B8%8D%E0%B8%B4%E0%B8%87", &hits); code != 200 {
		t.Fatalf("search expected 200, got %d", code)
	}
	if len(hits) == 0 {
		t.Fatal("search returned no hits for the hired candidate")
	}
}
