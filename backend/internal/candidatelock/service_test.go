package candidatelock

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepo struct {
	gotTTL time.Duration
	lock   Lock
	err    error
}

func (f *fakeRepo) Acquire(_ context.Context, candidateID, byUser uuid.UUID, ttl time.Duration) (Lock, error) {
	f.gotTTL = ttl
	if f.err != nil {
		return Lock{}, f.err
	}
	return Lock{CandidateID: candidateID, LockedBy: byUser, ExpiresAt: time.Now().Add(ttl)}, nil
}
func (f *fakeRepo) Release(context.Context, uuid.UUID, uuid.UUID, bool) error { return f.err }
func (f *fakeRepo) Get(context.Context, uuid.UUID) (*Lock, error)             { return &f.lock, f.err }

func TestServiceAppliesDefaultTTL(t *testing.T) {
	f := &fakeRepo{}
	s := NewService(f, 0) // 0 → DefaultTTL
	if _, err := s.Acquire(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if f.gotTTL != DefaultTTL {
		t.Errorf("ttl = %v, want default %v", f.gotTTL, DefaultTTL)
	}
}

func TestServiceUsesConfiguredTTL(t *testing.T) {
	f := &fakeRepo{}
	s := NewService(f, 10*time.Minute)
	if _, err := s.Acquire(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if f.gotTTL != 10*time.Minute {
		t.Errorf("ttl = %v, want 10m", f.gotTTL)
	}
}

func TestServicePropagatesLockedByOther(t *testing.T) {
	f := &fakeRepo{err: ErrLockedByOther}
	s := NewService(f, 0)
	if _, err := s.Acquire(context.Background(), uuid.New(), uuid.New()); err != ErrLockedByOther {
		t.Errorf("want ErrLockedByOther, got %v", err)
	}
}
