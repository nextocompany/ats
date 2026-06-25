//go:build integration

package applications

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// seedCompareApp inserts an application and (when sessionStatus != "") a matching
// interview_sessions row, so CompareByPosition's INNER JOIN can find it. Returns
// the application id. Reuses the package dsn()/setupList helpers.
func seedCompareApp(t *testing.T, r *pgRepository, candID, posID uuid.UUID, status string, ai, intv float64, store int, breakdown, sessionStatus string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var bd any
	if breakdown != "" {
		bd = breakdown
	}
	var appID uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status, ai_score, assigned_store_id, ai_score_breakdown)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		candID, posID, status, ai, store, bd).Scan(&appID); err != nil {
		t.Fatalf("insert app: %v", err)
	}
	if sessionStatus != "" {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO interview_sessions (application_id, access_token, status, interview_score, recommendation, summary)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			appID, "tok-"+appID.String()[:12], sessionStatus, intv, "recommend", "ok"); err != nil {
			t.Fatalf("insert interview session: %v", err)
		}
	}
	return appID
}

func newPosition(t *testing.T, r *pgRepository, title string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := r.pool.QueryRow(context.Background(),
		`INSERT INTO positions (title_th) VALUES ($1) RETURNING id`, title).Scan(&id); err != nil {
		t.Fatalf("insert position: %v", err)
	}
	return id
}

func TestCompareByPosition_EligibilityAndRanking(t *testing.T) {
	ctx := context.Background()
	r, pos1, cand := setupList(t) // seeds stores 1,2 + pos1 + cand
	pos2 := newPosition(t, r, "other")
	admin := rbac.New("super_admin", nil, "")

	// pos1 eligible pool — composite = 0.5*ai + 0.5*intv:
	//   A: ai90,intv80 -> 85.0 ; B: ai70,intv99 -> 84.5
	seedCompareApp(t, r, cand, pos1, StatusAIInterviewed, 90, 80, 1, "", "completed") // A
	seedCompareApp(t, r, cand, pos1, StatusShortlisted, 70, 99, 1, "", "completed")   // B
	// EXCLUDED: scored (no interview session at all)
	seedCompareApp(t, r, cand, pos1, StatusScored, 95, 0, 1, "", "")
	// EXCLUDED: ai_interview invited but NOT completed
	seedCompareApp(t, r, cand, pos1, StatusAIInterview, 96, 0, 1, "", "invited")
	// EXCLUDED: belongs to a different position
	seedCompareApp(t, r, cand, pos2, StatusAIInterviewed, 99, 99, 1, "", "completed")

	items, err := r.CompareByPosition(ctx, pos1, admin, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("eligible count = %d, want 2 (A,B only); got %v", len(items), statuses(items))
	}
	// ranking: 85.0 (A) before 84.5 (B)
	if items[0].Composite != 85.0 || items[1].Composite != 84.5 {
		t.Errorf("composite order = [%g,%g], want [85,84.5]", items[0].Composite, items[1].Composite)
	}
	if items[0].Status != StatusAIInterviewed {
		t.Errorf("top item status = %q, want %q", items[0].Status, StatusAIInterviewed)
	}
}

func TestCompareByPosition_StoreScope(t *testing.T) {
	ctx := context.Background()
	r, pos1, cand := setupList(t)
	seedCompareApp(t, r, cand, pos1, StatusAIInterviewed, 88, 80, 1, "", "completed")
	seedCompareApp(t, r, cand, pos1, StatusAIInterviewed, 88, 80, 2, "", "completed")

	store2 := 2
	items, err := r.CompareByPosition(ctx, pos1, rbac.New("hr_staff", &store2, ""), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("store-2 scope count = %d, want 1", len(items))
	}
	if items[0].AssignedStoreID == nil || *items[0].AssignedStoreID != 2 {
		t.Errorf("scoped item store = %v, want 2 (no cross-store leak)", items[0].AssignedStoreID)
	}
}

func TestCompareByPosition_BreakdownUnmarshal(t *testing.T) {
	ctx := context.Background()
	r, pos1, cand := setupList(t)
	seedCompareApp(t, r, cand, pos1, StatusInterview, 84, 90, 1,
		`{"experience":28,"skills":18,"education":9,"language":9,"location":18}`, "completed")

	items, err := r.CompareByPosition(ctx, pos1, rbac.New("super_admin", nil, ""), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("count = %d, want 1", len(items))
	}
	bd := items[0].Breakdown
	if bd == nil {
		t.Fatal("breakdown nil, want unmarshalled")
	}
	if bd.Experience != 28 || bd.Skills != 18 || bd.Location != 18 {
		t.Errorf("breakdown = %+v, want experience28/skills18/location18", *bd)
	}
	if items[0].InterviewScore == nil || *items[0].InterviewScore != 90 {
		t.Errorf("interview_score = %v, want 90", items[0].InterviewScore)
	}
}

func TestCompareByPosition_ExcludesDuplicateCandidates(t *testing.T) {
	ctx := context.Background()
	r, pos1, cand := setupList(t)
	// canonical candidate: eligible, included.
	seedCompareApp(t, r, cand, pos1, StatusAIInterviewed, 88, 80, 1, "", "completed")
	// a duplicate candidate row (is_duplicate_of -> canonical) with its own
	// eligible application must NOT surface (mirrors the /candidates roster fix).
	var dupCand uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO candidates (full_name, source_channel, status, is_duplicate_of)
		 VALUES ('dup','career_portal','available',$1) RETURNING id`, cand).Scan(&dupCand); err != nil {
		t.Fatalf("seed dup candidate: %v", err)
	}
	seedCompareApp(t, r, dupCand, pos1, StatusAIInterviewed, 99, 99, 1, "", "completed")

	items, err := r.CompareByPosition(ctx, pos1, rbac.New("super_admin", nil, ""), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("count = %d, want 1 (duplicate candidate excluded)", len(items))
	}
	if items[0].CandidateName != "c" {
		t.Errorf("surfaced %q, want canonical 'c'", items[0].CandidateName)
	}
}

func statuses(items []CompareItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Status
	}
	return out
}
