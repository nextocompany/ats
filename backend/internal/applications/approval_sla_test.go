package applications

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/notify"
)

type fakeSLAStore struct {
	overdue   []OverdueApprovalStep
	listErr   error
	escalated []uuid.UUID
}

func (f *fakeSLAStore) ListOverdueApprovalSteps(context.Context) ([]OverdueApprovalStep, error) {
	return f.overdue, f.listErr
}
func (f *fakeSLAStore) MarkApprovalStepEscalated(_ context.Context, id uuid.UUID) error {
	f.escalated = append(f.escalated, id)
	return nil
}

type capturingNotifier struct{ sent []notify.Message }

func (c *capturingNotifier) Send(_ context.Context, m notify.Message) error {
	c.sent = append(c.sent, m)
	return nil
}

func TestApprovalSLASweep_EscalatesOverdue(t *testing.T) {
	s1, s2 := uuid.New(), uuid.New()
	store := &fakeSLAStore{overdue: []OverdueApprovalStep{
		{StepID: s1, Role: "hr_manager", CandidateName: "Somchai"},
		{StepID: s2, Role: "regional_director", CandidateName: "Suda"},
	}}
	notifier := &capturingNotifier{}
	svc := NewApprovalSLAService(store, notifier, fakeHRDir{emails: []string{"a@x.co"}}, "http://dash", false)

	if err := svc.HandleApprovalSLASweep(context.Background(), nil); err != nil {
		t.Fatalf("sweep error: %v", err)
	}
	if len(store.escalated) != 2 {
		t.Fatalf("expected 2 steps marked escalated, got %d", len(store.escalated))
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("expected 2 escalation emails sent, got %d", len(notifier.sent))
	}
}

func TestApprovalSLASweep_NoOverdue(t *testing.T) {
	store := &fakeSLAStore{}
	notifier := &capturingNotifier{}
	svc := NewApprovalSLAService(store, notifier, fakeHRDir{}, "http://dash", false)
	if err := svc.HandleApprovalSLASweep(context.Background(), nil); err != nil {
		t.Fatalf("sweep error: %v", err)
	}
	if len(store.escalated) != 0 || len(notifier.sent) != 0 {
		t.Fatalf("no overdue steps → nothing escalated/sent, got %d/%d", len(store.escalated), len(notifier.sent))
	}
}
