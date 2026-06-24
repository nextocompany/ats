//go:build integration

package members

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// setup truncates and seeds 4 members with varied providers/status/apps.
func setup(t *testing.T) *pgRepository {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to v16?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE candidate_accounts, candidate_sessions, applications, candidates, positions, stores, vacancies RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// m1: verified email member, has resume, active, 2 applications
	var m1 uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO candidate_accounts (full_name, email, email_verified, phone, resume_blob_url, resume_file_type, status, pdpa_consent, pdpa_version)
		VALUES ('สมชาย ใจดี','somchai@example.com',TRUE,'0810000001','blob/r1.pdf','pdf','active',TRUE,'1.0') RETURNING id`).Scan(&m1); err != nil {
		t.Fatalf("seed m1: %v", err)
	}
	// m2: LINE member, active, no apps
	if _, err := pool.Exec(ctx, `INSERT INTO candidate_accounts (full_name, line_user_id, status) VALUES ('สุดา LINE','U_line_1','active')`); err != nil {
		t.Fatalf("seed m2: %v", err)
	}
	// m3: Google member, suspended
	if _, err := pool.Exec(ctx, `INSERT INTO candidate_accounts (full_name, google_sub, status) VALUES ('Google Guy','g_sub_1','suspended')`); err != nil {
		t.Fatalf("seed m3: %v", err)
	}
	// m4: verified email member, active, older than a week
	if _, err := pool.Exec(ctx, `INSERT INTO candidate_accounts (full_name, email, email_verified, status, created_at)
		VALUES ('Old Member','old@example.com',TRUE,'active', NOW() - INTERVAL '30 days')`); err != nil {
		t.Fatalf("seed m4: %v", err)
	}

	// 2 applications for m1 (via a candidate row linked by account_id)
	var posID, candID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('พนักงานขาย') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status, account_id) VALUES ('สมชาย ใจดี','career_portal','available',$1) RETURNING id`, m1).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'scored')`, candID, posID); err != nil {
			t.Fatalf("seed application: %v", err)
		}
	}
	// an active session for m1 (for active_sessions / last_seen)
	if _, err := pool.Exec(ctx, `INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1,'hash1', NOW() + INTERVAL '1 day')`, m1); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return &pgRepository{pool: pool}
}

func TestList_NoFilter(t *testing.T) {
	r := setup(t)
	items, total, err := r.List(context.Background(), ListFilter{}, rbac.AllScope())
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(items) != 4 {
		t.Fatalf("expected 4 members, got total=%d len=%d", total, len(items))
	}
}

func TestList_SearchProviderStatusResume(t *testing.T) {
	r := setup(t)
	ctx := context.Background()

	if _, total, _ := r.List(ctx, ListFilter{Search: "สมชาย"}, rbac.AllScope()); total != 1 {
		t.Errorf("search สมชาย: want 1, got %d", total)
	}
	if _, total, _ := r.List(ctx, ListFilter{Provider: "line"}, rbac.AllScope()); total != 1 {
		t.Errorf("provider line: want 1, got %d", total)
	}
	if _, total, _ := r.List(ctx, ListFilter{Provider: "google"}, rbac.AllScope()); total != 1 {
		t.Errorf("provider google: want 1, got %d", total)
	}
	if _, total, _ := r.List(ctx, ListFilter{Status: "suspended"}, rbac.AllScope()); total != 1 {
		t.Errorf("status suspended: want 1, got %d", total)
	}
	yes := true
	if _, total, _ := r.List(ctx, ListFilter{HasResume: &yes}, rbac.AllScope()); total != 1 {
		t.Errorf("has_resume: want 1, got %d", total)
	}
}

func TestList_Paginate(t *testing.T) {
	r := setup(t)
	items, total, err := r.List(context.Background(), ListFilter{Page: 1, Limit: 2}, rbac.AllScope())
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(items) != 2 {
		t.Fatalf("expected total 4 / 2 rows, got %d / %d", total, len(items))
	}
}

func TestGetByID_DerivedFields(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	items, _, _ := r.List(ctx, ListFilter{Search: "somchai@example.com"}, rbac.AllScope())
	if len(items) != 1 {
		t.Fatalf("setup: expected 1 m1, got %d", len(items))
	}
	m, err := r.GetByID(ctx, items[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if m.AppsCount != 2 {
		t.Errorf("apps_count = %d, want 2", m.AppsCount)
	}
	if !m.EmailLinked || m.LineLinked || m.GoogleLinked {
		t.Errorf("provider flags wrong: %+v", m)
	}
	if !m.HasResume || m.ResumeType != "pdf" {
		t.Errorf("resume wrong: has=%v type=%q", m.HasResume, m.ResumeType)
	}
	if m.ActiveSessions != 1 || m.LastLoginAt == nil {
		t.Errorf("session derived fields wrong: active=%d lastLogin=%v", m.ActiveSessions, m.LastLoginAt)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	r := setup(t)
	if _, err := r.GetByID(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStats(t *testing.T) {
	r := setup(t)
	s, err := r.Stats(context.Background(), rbac.AllScope())
	if err != nil {
		t.Fatal(err)
	}
	if s.Total != 4 {
		t.Errorf("total = %d, want 4", s.Total)
	}
	if s.Suspended != 1 {
		t.Errorf("suspended = %d, want 1", s.Suspended)
	}
	if s.WithApplications != 1 {
		t.Errorf("with_applications = %d, want 1", s.WithApplications)
	}
	if s.ByProvider["line"] != 1 || s.ByProvider["google"] != 1 || s.ByProvider["email"] != 2 {
		t.Errorf("by_provider wrong: %+v", s.ByProvider)
	}
	if s.NewThisWeek != 3 { // m1,m2,m3 created now; m4 is 30 days old
		t.Errorf("new_this_week = %d, want 3", s.NewThisWeek)
	}
}
