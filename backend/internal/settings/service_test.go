package settings

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRepo is an in-memory Repository whose reads can be forced to fail.
type fakeRepo struct {
	val      bool
	getErr   error
	getCalls int
	setCalls int
}

func (f *fakeRepo) GetBool(_ context.Context, _ string) (bool, error) {
	f.getCalls++
	if f.getErr != nil {
		return false, f.getErr
	}
	return f.val, nil
}

func (f *fakeRepo) SetBool(_ context.Context, _ string, val bool, _ string) error {
	f.setCalls++
	f.val = val
	return nil
}

func TestService_AllowAllTenants_CachesWithinTTL(t *testing.T) {
	repo := &fakeRepo{val: true}
	svc := NewService(repo)
	now := time.Unix(1_700_000_000, 0)
	svc.now = func() time.Time { return now }

	if !svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected true on first read")
	}
	// Second read within the TTL must not hit the repo again.
	if !svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected cached true")
	}
	if repo.getCalls != 1 {
		t.Fatalf("expected 1 repo read within TTL, got %d", repo.getCalls)
	}

	// After the TTL elapses, it refreshes.
	now = now.Add(cacheTTL + time.Second)
	repo.val = false
	if svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected refreshed false after TTL")
	}
	if repo.getCalls != 2 {
		t.Fatalf("expected 2 repo reads after TTL, got %d", repo.getCalls)
	}
}

func TestService_AllowAllTenants_FailsClosed(t *testing.T) {
	// No prior good value + repo error ⇒ false (closed).
	repo := &fakeRepo{getErr: errors.New("db down")}
	svc := NewService(repo)
	if svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected false when repo errors and no cache")
	}
}

func TestService_AllowAllTenants_KeepsLastGoodOnError(t *testing.T) {
	repo := &fakeRepo{val: true}
	svc := NewService(repo)
	now := time.Unix(1_700_000_000, 0)
	svc.now = func() time.Time { return now }

	if !svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected initial true")
	}
	// TTL expires and the repo now errors — keep the last known value.
	now = now.Add(cacheTTL + time.Second)
	repo.getErr = errors.New("transient")
	if !svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected last-good true to be retained on error")
	}
}

func TestService_SetAllowAll_BustsCache(t *testing.T) {
	repo := &fakeRepo{val: false}
	svc := NewService(repo)
	now := time.Unix(1_700_000_000, 0)
	svc.now = func() time.Time { return now }

	if svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected initial false")
	}
	if err := svc.SetAllowAll(context.Background(), true, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if repo.setCalls != 1 {
		t.Fatalf("expected 1 set call, got %d", repo.setCalls)
	}
	// Immediately reflects the new value from the busted cache (no repo read).
	before := repo.getCalls
	if !svc.AllowAllTenants(context.Background()) {
		t.Fatal("expected true immediately after set")
	}
	if repo.getCalls != before {
		t.Fatalf("expected no repo read after set bust, got %d extra", repo.getCalls-before)
	}
}
