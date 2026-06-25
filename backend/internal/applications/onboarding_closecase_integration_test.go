//go:build integration

package applications

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// closeCaseBlob is a no-op blob: maybeCloseCase only signs URLs to build the
// status, and a signing result is irrelevant to completeness.
type closeCaseBlob struct{}

func (closeCaseBlob) Upload(context.Context, string, []byte, string) (string, error) {
	return "stored://x", nil
}
func (closeCaseBlob) SignedURLForStored(string, time.Duration) (string, error) {
	return "https://signed/x", nil
}

// countingSyncer mirrors the real peoplesoft.Service: it records the call and marks
// the application synced (SetPSSynced), which is what makes the once-only guard work.
type countingSyncer struct {
	calls int
	apps  *pgRepository
}

func (s *countingSyncer) SyncHired(ctx context.Context, appID uuid.UUID) error {
	s.calls++
	return s.apps.SetPSSynced(ctx, appID)
}

func TestOnboarding_DeferredCloseCase(t *testing.T) {
	ctx := context.Background()
	r, _, _, appID := setupOffer(t)
	// Insert one approved required doc so the checklist is complete.
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO onboarding_documents (application_id, doc_type, status, blob_url) VALUES ($1,'id_card','approved','stored://x')`,
		appID); err != nil {
		t.Fatalf("seed approved doc: %v", err)
	}

	syncer := &countingSyncer{apps: r}
	h := &OnboardingHandler{apps: r, blob: closeCaseBlob{}, required: []string{"id_card"}, hired: syncer}

	// First completion → exactly one push, ps_synced_at set.
	h.maybeCloseCase(ctx, appID)
	if syncer.calls != 1 {
		t.Fatalf("first close-case calls = %d, want 1", syncer.calls)
	}
	app, _ := r.FindByID(ctx, appID)
	if app.PSSyncedAt == nil {
		t.Fatal("ps_synced_at not set after close-case")
	}

	// A later approval (re-trigger) must NOT push again (once-only guard).
	h.maybeCloseCase(ctx, appID)
	if syncer.calls != 1 {
		t.Fatalf("re-trigger calls = %d, want 1 (once-only)", syncer.calls)
	}
}

func TestOnboarding_CloseCaseSkippedWhenIncomplete(t *testing.T) {
	ctx := context.Background()
	r, _, _, appID := setupOffer(t)
	// No approved docs → not complete.
	syncer := &countingSyncer{apps: r}
	h := &OnboardingHandler{apps: r, blob: closeCaseBlob{}, required: []string{"id_card"}, hired: syncer}

	h.maybeCloseCase(ctx, appID)
	if syncer.calls != 0 {
		t.Fatalf("incomplete close-case calls = %d, want 0", syncer.calls)
	}
}
