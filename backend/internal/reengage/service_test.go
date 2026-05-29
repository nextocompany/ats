package reengage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/notify"
)

type fakeRepo struct {
	targets   []Target
	contacted map[uuid.UUID]bool // candidates already contacted
	recorded  []uuid.UUID
}

func (f *fakeRepo) MatchingCandidates(_ context.Context, _ uuid.UUID) ([]Target, error) {
	return f.targets, nil
}

func (f *fakeRepo) RecordContact(_ context.Context, candidateID, _ uuid.UUID, _ string) (bool, error) {
	if f.contacted == nil {
		f.contacted = map[uuid.UUID]bool{}
	}
	if f.contacted[candidateID] {
		return false, nil
	}
	f.contacted[candidateID] = true
	f.recorded = append(f.recorded, candidateID)
	return true, nil
}

type fakeNotifier struct {
	sent int
	err  error
}

func (f *fakeNotifier) Send(_ context.Context, _ notify.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent++
	return nil
}

type fakeAudit struct{ calls int }

func (f *fakeAudit) Record(_ context.Context, _, _ string, _ uuid.UUID, _ any) error {
	f.calls++
	return nil
}

func target(email string) Target {
	return Target{CandidateID: uuid.New(), FullName: "ทดสอบ", Email: email, Phone: "0812345678"}
}

func TestReengage_SendsToFreshTargets(t *testing.T) {
	repo := &fakeRepo{targets: []Target{target("a@x.com"), target("b@x.com")}}
	n := &fakeNotifier{}
	svc := NewService(repo, n, &fakeAudit{}, "http://portal")

	sent, err := svc.Reengage(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 2 || n.sent != 2 {
		t.Fatalf("expected 2 sent, got sent=%d notifier=%d", sent, n.sent)
	}
}

func TestReengage_SuppressesAlreadyContacted(t *testing.T) {
	dup := target("dup@x.com")
	repo := &fakeRepo{
		targets:   []Target{dup},
		contacted: map[uuid.UUID]bool{dup.CandidateID: true},
	}
	n := &fakeNotifier{}
	svc := NewService(repo, n, &fakeAudit{}, "http://portal")

	sent, err := svc.Reengage(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || n.sent != 0 {
		t.Fatalf("expected suppression (0 sent), got sent=%d notifier=%d", sent, n.sent)
	}
}

func TestReengage_NotifyFailureDoesNotFailRun(t *testing.T) {
	repo := &fakeRepo{targets: []Target{target("a@x.com")}}
	n := &fakeNotifier{err: errors.New("line down")}
	audit := &fakeAudit{}
	svc := NewService(repo, n, audit, "http://portal")

	sent, err := svc.Reengage(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("notify failure must not fail Reengage, got %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected sent=0 on notify failure, got %d", sent)
	}
	if audit.calls != 1 {
		t.Fatalf("expected audit recorded once, got %d", audit.calls)
	}
}
