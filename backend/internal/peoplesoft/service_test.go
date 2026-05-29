package peoplesoft

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
)

type fakeApps struct {
	app     *applications.Application
	synced  bool
	syncErr error
}

func (f *fakeApps) FindByID(context.Context, uuid.UUID) (*applications.Application, error) {
	return f.app, nil
}
func (f *fakeApps) SetPSSynced(context.Context, uuid.UUID) error {
	f.synced = true
	return f.syncErr
}

type fakeCands struct{ cand *candidates.Candidate }

func (f *fakeCands) FindByID(context.Context, uuid.UUID) (*candidates.Candidate, error) {
	return f.cand, nil
}

type fakeBlob struct{ uploaded bool }

func (f *fakeBlob) Upload(context.Context, string, []byte, string) (string, error) {
	f.uploaded = true
	return "blob://x", nil
}

type failClient struct{}

func (failClient) SyncHired(context.Context, Applicant) error { return errors.New("PS down") }

func newFixture() (*fakeApps, *fakeCands, *fakeBlob) {
	score := 82.0
	return &fakeApps{app: &applications.Application{ID: uuid.New(), CandidateID: uuid.New(), AIScore: &score}},
		&fakeCands{cand: &candidates.Candidate{FullName: "สมชาย", SourceChannel: "career_portal"}},
		&fakeBlob{}
}

func TestSyncHired_Success(t *testing.T) {
	apps, cands, blob := newFixture()
	svc := NewService(mockClient{}, apps, cands, blob, "ps-export")

	if err := svc.SyncHired(context.Background(), apps.app.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !apps.synced {
		t.Error("expected ps_synced_at to be set on success")
	}
	if blob.uploaded {
		t.Error("did not expect CSV fallback on success")
	}
}

func TestSyncHired_FallbackOnPSFailure(t *testing.T) {
	apps, cands, blob := newFixture()
	svc := NewService(failClient{}, apps, cands, blob, "ps-export")

	if err := svc.SyncHired(context.Background(), apps.app.ID); err != nil {
		t.Fatalf("hire must not fail on PS error, got: %v", err)
	}
	if apps.synced {
		t.Error("ps_synced_at must NOT be set when PS failed")
	}
	if !blob.uploaded {
		t.Error("expected CSV fallback to be written on PS failure")
	}
}
