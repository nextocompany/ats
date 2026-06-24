//go:build integration

package applications

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// insertAppReturning inserts an application and returns its id (setupList's
// insertApp does not return the id, which the appointment FK needs).
func insertAppReturning(t *testing.T, r *pgRepository, candID, posID uuid.UUID, store int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := r.pool.QueryRow(context.Background(),
		`INSERT INTO applications (candidate_id, position_id, status, assigned_store_id)
		 VALUES ($1,$2,'interview',$3) RETURNING id`,
		candID, posID, store).Scan(&id); err != nil {
		t.Fatalf("insert app: %v", err)
	}
	return id
}

func insertAppt(t *testing.T, r *pgRepository, appID uuid.UUID, at time.Time, createdBy *uuid.UUID) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(),
		`INSERT INTO interview_appointments (application_id, scheduled_at, duration_min, mode, created_by)
		 VALUES ($1,$2,60,'online',$3)`,
		appID, at, createdBy); err != nil {
		t.Fatalf("insert appointment: %v", err)
	}
}

// TestListUpcomingInterviews_ScopeBoundary is the privacy guard: a store-scoped HR
// must see ONLY their own store's interviews, never another store's. A leak here
// is silent (no error), so this test is the real boundary check.
func TestListUpcomingInterviews_ScopeBoundary(t *testing.T) {
	r, pos, cand := setupList(t)
	ctx := context.Background()
	soon := time.Now().Add(24 * time.Hour)

	app1 := insertAppReturning(t, r, cand, pos, 1) // store A
	app2 := insertAppReturning(t, r, cand, pos, 2) // store B
	insertAppt(t, r, app1, soon, nil)
	insertAppt(t, r, app2, soon, nil)

	// super_admin (KindAll) sees both.
	all, total, err := r.ListUpcomingInterviews(ctx, UpcomingFilter{From: time.Now()}, rbac.New("super_admin", nil, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("admin should see 2 interviews, got total=%d len=%d", total, len(all))
	}

	// Store-1 HR (KindStore) sees ONLY store 1's interview.
	store1 := 1
	scoped, total, err := r.ListUpcomingInterviews(ctx, UpcomingFilter{From: time.Now()}, rbac.New("hr_staff", &store1, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(scoped) != 1 {
		t.Fatalf("store-1 HR should see exactly 1 interview, got total=%d len=%d", total, len(scoped))
	}
	if scoped[0].ApplicationID != app1 {
		t.Errorf("LEAK: store-1 HR saw application %s, want %s (store 1)", scoped[0].ApplicationID, app1)
	}
	if scoped[0].AssignedStoreID == nil || *scoped[0].AssignedStoreID != 1 {
		t.Errorf("scoped interview store = %v, want 1", scoped[0].AssignedStoreID)
	}

	// Subregion HR (KindSubregion) sees only their subregion's interviews. Store 1 is
	// 'Upper North', store 2 'East' (setupList) — an Upper North director sees app1 only.
	sub, total, err := r.ListUpcomingInterviews(ctx, UpcomingFilter{From: time.Now()}, rbac.New("operation_director", nil, "Upper North"))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(sub) != 1 || sub[0].ApplicationID != app1 {
		t.Fatalf("LEAK: Upper North director should see exactly app1, got total=%d", total)
	}
}

// TestListUpcomingInterviews_MineAndFrom proves the mine (created_by) filter and
// the From window both narrow the result.
func TestListUpcomingInterviews_MineAndFrom(t *testing.T) {
	r, pos, cand := setupList(t)
	ctx := context.Background()

	// Seed a user to own one appointment.
	var uid uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, full_name, role, is_active) VALUES ('hr@x.test','HR','hr_staff',TRUE) RETURNING id`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	app := insertAppReturning(t, r, cand, pos, 1)
	other := insertAppReturning(t, r, cand, pos, 1)
	past := insertAppReturning(t, r, cand, pos, 1)
	insertAppt(t, r, app, time.Now().Add(48*time.Hour), &uid)   // mine, future
	insertAppt(t, r, other, time.Now().Add(48*time.Hour), nil)  // not mine
	insertAppt(t, r, past, time.Now().Add(-48*time.Hour), &uid) // mine but past

	admin := rbac.New("super_admin", nil, "")

	// From now: the past appointment is excluded (2 future remain).
	_, total, err := r.ListUpcomingInterviews(ctx, UpcomingFilter{From: time.Now()}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("From-now should exclude the past appointment, got total=%d (want 2)", total)
	}

	// Mine + from now: only the future appointment owned by uid.
	mine, total, err := r.ListUpcomingInterviews(ctx, UpcomingFilter{From: time.Now(), Mine: true, ActorID: uid}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(mine) != 1 || mine[0].ApplicationID != app {
		t.Errorf("mine+from should be exactly the one future owned appointment, got total=%d", total)
	}
}
