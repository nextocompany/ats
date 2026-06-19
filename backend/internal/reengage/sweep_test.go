package reengage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTriggerType(t *testing.T) {
	if got := TriggerType(6); got != "time_6mo" {
		t.Errorf("TriggerType(6) = %q, want time_6mo", got)
	}
	if got := TriggerType(12); got != "time_12mo" {
		t.Errorf("TriggerType(12) = %q, want time_12mo", got)
	}
}

func TestSweepTimeBased_NudgesDormant(t *testing.T) {
	repo := &fakeRepo{dormant: []Target{target("a@x.com"), target("b@x.com")}}
	n := &fakeNotifier{}
	svc := NewService(repo, n, &fakeAudit{}, "http://portal")

	sent, err := svc.SweepTimeBased(context.Background(), 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 2 || n.sent != 2 {
		t.Fatalf("expected 2 nudged, got sent=%d notifier=%d", sent, n.sent)
	}
	if repo.dormantMonths != 6 || repo.dormantTrigger != "time_6mo" {
		t.Fatalf("expected repo queried months=6 trigger=time_6mo, got months=%d trigger=%q", repo.dormantMonths, repo.dormantTrigger)
	}
}

func TestSweepTimeBased_SuppressesAlreadyNudged(t *testing.T) {
	dup := target("dup@x.com")
	repo := &fakeRepo{
		dormant:       []Target{dup},
		timeContacted: map[string]bool{dup.CandidateID.String() + "|time_12mo": true},
	}
	n := &fakeNotifier{}
	svc := NewService(repo, n, &fakeAudit{}, "http://portal")

	sent, err := svc.SweepTimeBased(context.Background(), 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || n.sent != 0 {
		t.Fatalf("expected suppression (0 nudged), got sent=%d notifier=%d", sent, n.sent)
	}
}

func TestSweepTimeBased_NotifyFailureDoesNotFailRun(t *testing.T) {
	repo := &fakeRepo{dormant: []Target{target("a@x.com")}}
	n := &fakeNotifier{err: errors.New("line down")}
	audit := &fakeAudit{}
	svc := NewService(repo, n, audit, "http://portal")

	sent, err := svc.SweepTimeBased(context.Background(), 6)
	if err != nil {
		t.Fatalf("notify failure must not fail sweep, got %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected sent=0 on notify failure, got %d", sent)
	}
	if audit.calls != 1 {
		t.Fatalf("expected audit recorded once, got %d", audit.calls)
	}
}

func TestSweepTimeBased_SkipsNoChannel(t *testing.T) {
	noChannel := Target{CandidateID: uuid.New(), FullName: "ไร้ช่องทาง"} // no email, no LINE
	repo := &fakeRepo{dormant: []Target{noChannel}}
	n := &fakeNotifier{}
	svc := NewService(repo, n, &fakeAudit{}, "http://portal")

	sent, err := svc.SweepTimeBased(context.Background(), 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 || n.sent != 0 {
		t.Fatalf("expected skip (0), got sent=%d notifier=%d", sent, n.sent)
	}
}
