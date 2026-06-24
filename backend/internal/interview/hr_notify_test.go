package interview

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/positions"
)

// hrPassSubject mirrors notify.AIInterviewPassedHR's subject so tests can pick the
// HR-pass messages out of the captured stream (which also holds the LINE invite).
const hrPassSubject = "ผู้สมัครผ่านการสัมภาษณ์ AI เบื้องต้น"

// capturingNotifier records every message so a test can assert on channel/subject.
type capturingNotifier struct{ msgs []notify.Message }

func (c *capturingNotifier) Send(_ context.Context, m notify.Message) error {
	c.msgs = append(c.msgs, m)
	return nil
}

func (c *capturingNotifier) hrPass() []notify.Message {
	out := make([]notify.Message, 0)
	for _, m := range c.msgs {
		if m.Subject == hrPassSubject {
			out = append(out, m)
		}
	}
	return out
}

// stubHR is an in-memory HRDirectory. EmailsForStore returns the canned emails for
// a non-nil store (and respects err); the other methods are unused here.
type stubHR struct {
	emails []string
	err    error
}

func (s stubHR) EmailsForStore(_ context.Context, storeID *int) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	if storeID == nil {
		return nil, nil
	}
	return s.emails, nil
}

func (s stubHR) LineManagerEmailsForStore(_ context.Context, _ *int) ([]string, error) {
	return nil, nil
}

func (s stubHR) EmailsForRoleStore(_ context.Context, _ string, _ *int) ([]string, error) {
	return nil, nil
}

func (s stubHR) HiringManagerForVacancy(_ context.Context, _ uuid.UUID) (string, string, error) {
	return "", "", nil
}

// runInterviewToCompletion invites, starts, and answers until the session
// completes (maxTurns=1 finishes in a single Respond). Fails the test on any error.
func runInterviewToCompletion(t *testing.T, svc *Service, appID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	invited, err := svc.Invite(ctx, appID)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := svc.Start(ctx, invited.AccessToken); err != nil {
		t.Fatalf("start: %v", err)
	}
	for i := 0; i < 8; i++ {
		s, err := svc.Respond(ctx, invited.AccessToken, "นี่คือคำตอบของผม")
		if err != nil {
			t.Fatalf("respond %d: %v", i, err)
		}
		if s.Status == StatusCompleted {
			return
		}
	}
	t.Fatal("interview did not complete")
}

// newHRTestService builds a service whose candidate has a LINE handle (so invite
// notifies) and whose application carries the given store assignment. maxTurns=1 so
// a single Respond completes the interview. The mock evaluator scores 75.
func newHRTestService(repo Repository, n notify.Notifier, storeID *int) (*Service, uuid.UUID) {
	appID := uuid.New()
	app := &applications.Application{
		ID: appID, CandidateID: uuid.New(), PositionID: uuid.New(),
		Status: applications.StatusScored, AssignedStoreID: storeID,
	}
	pos := &positions.Position{TitleTH: "พนักงานขาย"}
	cand := &candidates.Candidate{FullName: "สมชาย ใจดี", LineUserID: "U123"}
	svc := NewService(repo, mockInterviewer{}, stubApps{app}, stubPositions{pos}, stubCands{cand}, n, "http://portal", 1)
	return svc, appID
}

func storePtr(n int) *int { return &n }

func TestNotifyHRPassed_AtThresholdNotifies(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, storePtr(101))
	// Threshold 75 == mock score 75 → boundary should notify.
	svc.SetHRNotifier(stubHR{emails: []string{"hr@store.example"}}, "http://dash", false, 75)

	runInterviewToCompletion(t, svc, appID)

	hr := n.hrPass()
	if len(hr) != 1 {
		t.Fatalf("expected one HR-pass email at threshold, got %d", len(hr))
	}
	if hr[0].Channel != notify.ChannelEmail || hr[0].Recipient != "hr@store.example" {
		t.Fatalf("unexpected HR message: %+v", hr[0])
	}
}

func TestNotifyHRPassed_BelowThresholdSkips(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, storePtr(101))
	// Threshold 76 > mock score 75 → below the bar, no HR alert.
	svc.SetHRNotifier(stubHR{emails: []string{"hr@store.example"}}, "http://dash", false, 76)

	runInterviewToCompletion(t, svc, appID)

	if got := len(n.hrPass()); got != 0 {
		t.Fatalf("expected no HR-pass message below threshold, got %d", got)
	}
}

func TestNotifyHRPassed_UnassignedNoStoreNoNotify(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, nil) // talent pool / unassigned
	svc.SetHRNotifier(stubHR{emails: []string{"hr@store.example"}}, "http://dash", false, 75)

	runInterviewToCompletion(t, svc, appID)

	if got := len(n.hrPass()); got != 0 {
		t.Fatalf("expected no HR-pass message for unassigned candidate, got %d", got)
	}
}

func TestNotifyHRPassed_NoHRNotifierConfigured(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, storePtr(101))
	// SetHRNotifier intentionally not called → HR notify is skipped entirely.

	runInterviewToCompletion(t, svc, appID)

	if got := len(n.hrPass()); got != 0 {
		t.Fatalf("expected no HR-pass message without SetHRNotifier, got %d", got)
	}
}

func TestNotifyHRPassed_TeamsOnlyWhenNoEmails(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, storePtr(101))
	// No store-HR emails but Teams enabled → exactly one Teams message.
	svc.SetHRNotifier(stubHR{emails: nil}, "http://dash", true, 75)

	runInterviewToCompletion(t, svc, appID)

	hr := n.hrPass()
	if len(hr) != 1 || hr[0].Channel != notify.ChannelTeams {
		t.Fatalf("expected a single Teams HR-pass message, got %+v", hr)
	}
}

func TestNotifyHRPassed_DirectoryErrorIsNonFatal(t *testing.T) {
	n := &capturingNotifier{}
	svc, appID := newHRTestService(newMemRepo(), n, storePtr(101))
	svc.SetHRNotifier(stubHR{err: errors.New("db down")}, "http://dash", true, 75)

	// Must not fail the candidate's final answer despite the directory error.
	runInterviewToCompletion(t, svc, appID)

	if got := len(n.hrPass()); got != 0 {
		t.Fatalf("expected no HR-pass message when directory errors, got %d", got)
	}
}
