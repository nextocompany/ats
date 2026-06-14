//go:build integration

package members

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// m1ID returns the seeded "somchai" member (has resume + 2 apps + 1 session).
func m1ID(t *testing.T, r *pgRepository) uuid.UUID {
	t.Helper()
	items, _, err := r.List(context.Background(), ListFilter{Search: "somchai@example.com"})
	if err != nil || len(items) != 1 {
		t.Fatalf("setup: expected 1 m1, got %d (err %v)", len(items), err)
	}
	return items[0].ID
}

func sessionCount(t *testing.T, r *pgRepository, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM candidate_sessions WHERE account_id=$1`, id).Scan(&n); err != nil {
		t.Fatalf("session count: %v", err)
	}
	return n
}

func TestSetStatus_SuspendClearsSessionsThenReactivate(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if sessionCount(t, r, id) != 1 {
		t.Fatalf("setup: m1 should start with 1 session")
	}
	if err := r.SetStatus(ctx, id, StatusSuspended, nil); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	m, _ := r.GetByID(ctx, id)
	if m.Status != StatusSuspended {
		t.Fatalf("expected suspended, got %q", m.Status)
	}
	if sessionCount(t, r, id) != 0 {
		t.Fatalf("suspend must force-logout (delete sessions), still %d", sessionCount(t, r, id))
	}

	// Reactivate restores active status.
	if err := r.SetStatus(ctx, id, StatusActive, nil); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	m, _ = r.GetByID(ctx, id)
	if m.Status != StatusActive {
		t.Fatalf("expected active after reactivate, got %q", m.Status)
	}
}

func TestSetStatus_MissingMember(t *testing.T) {
	r := setup(t)
	if err := r.SetStatus(context.Background(), uuid.New(), StatusSuspended, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAnonymize_RedactsDeletesSessionsAndIdempotent(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	resumeURL, err := r.Anonymize(ctx, id)
	if err != nil {
		t.Fatalf("anonymize: %v", err)
	}
	if resumeURL != "blob/r1.pdf" {
		t.Fatalf("expected resume url returned for blob cleanup, got %q", resumeURL)
	}

	// Verify redaction directly (the projection hides raw PII, so read the table).
	var fullName, status string
	var emailVerified bool
	var email, resume *string
	if err := r.pool.QueryRow(ctx,
		`SELECT full_name, status, email_verified, email, resume_blob_url FROM candidate_accounts WHERE id=$1`, id,
	).Scan(&fullName, &status, &emailVerified, &email, &resume); err != nil {
		t.Fatalf("read redacted: %v", err)
	}
	if status != StatusAnonymized {
		t.Fatalf("status should be anonymized, got %q", status)
	}
	if fullName != redactedName {
		t.Fatalf("full_name should be redacted, got %q", fullName)
	}
	if email != nil || resume != nil {
		t.Fatalf("email/resume should be NULL, got email=%v resume=%v", email, resume)
	}
	if emailVerified {
		t.Fatalf("email_verified should be cleared on anonymize")
	}
	if sessionCount(t, r, id) != 0 {
		t.Fatalf("anonymize must delete sessions, still %d", sessionCount(t, r, id))
	}

	// Idempotent: a second anonymize is a no-op error.
	if _, err := r.Anonymize(ctx, id); !errors.Is(err, ErrAnonymized) {
		t.Fatalf("second anonymize should ErrAnonymized, got %v", err)
	}
}

func TestAnonymize_ThenLifecycleLocked(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if _, err := r.Anonymize(ctx, id); err != nil {
		t.Fatalf("anonymize: %v", err)
	}
	// Status changes and profile edits are refused on an anonymized account.
	if err := r.SetStatus(ctx, id, StatusActive, nil); !errors.Is(err, ErrAnonymized) {
		t.Fatalf("reactivating anonymized should ErrAnonymized, got %v", err)
	}
	if err := r.UpdateProfile(ctx, id, ProfileUpdate{Phone: "0810000000"}); !errors.Is(err, ErrAnonymized) {
		t.Fatalf("editing anonymized should ErrAnonymized, got %v", err)
	}
}

func TestUpdateProfile_SparseLeavesOthersIntact(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if err := r.UpdateProfile(ctx, id, ProfileUpdate{Phone: "0899999999"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	m, _ := r.GetByID(ctx, id)
	if m.Phone != "0899999999" {
		t.Fatalf("phone not updated, got %q", m.Phone)
	}
	if m.Email != "somchai@example.com" || m.FullName != "สมชาย ใจดี" {
		t.Fatalf("sparse update blanked other fields: email=%q name=%q", m.Email, m.FullName)
	}
}

func TestForceLogout_DeletesSessionsAnd404(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if err := r.ForceLogout(ctx, id); err != nil {
		t.Fatalf("force logout: %v", err)
	}
	if sessionCount(t, r, id) != 0 {
		t.Fatalf("force logout should delete sessions, still %d", sessionCount(t, r, id))
	}
	if err := r.ForceLogout(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing member should ErrNotFound, got %v", err)
	}
}
