package applications

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
)

// recNotifier records sends (and can simulate failure).
type recNotifier struct {
	sent []notify.Message
	err  error
}

func (r *recNotifier) Send(_ context.Context, m notify.Message) error {
	r.sent = append(r.sent, m)
	return r.err
}

// stubApps/stubCands embed the interfaces (nil) and override only FindByID, so
// they satisfy the full contract while exercising one method.
type stubApps struct {
	Repository
	app *Application
}

func (s stubApps) FindByID(context.Context, uuid.UUID) (*Application, error) { return s.app, nil }

type stubCands struct {
	candidates.Repository
	cand *candidates.Candidate
}

func (s stubCands) FindByID(context.Context, uuid.UUID) (*candidates.Candidate, error) {
	return s.cand, nil
}

func deps(n notify.Notifier, cand *candidates.Candidate) statusNotifyDeps {
	return statusNotifyDeps{notifier: n, cands: stubCands{cand: cand}, portalBaseURL: "https://x"}
}

func TestNotifyStatusChange_SendsOnNotifiableWithLineID(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{FullName: "ก", LineUserID: "U-1"})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusHired)

	if len(rn.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(rn.sent))
	}
	if rn.sent[0].Channel != notify.ChannelLINE || rn.sent[0].Recipient != "U-1" {
		t.Errorf("unexpected message: %+v", rn.sent[0])
	}
}

func TestNotifyStatusChange_BestEffortOnSendError(t *testing.T) {
	rn := &recNotifier{err: errors.New("line push 4xx (not a friend)")}
	d := deps(rn, &candidates.Candidate{FullName: "ก", LineUserID: "U-1"})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	// Must not panic / propagate — best-effort.
	d.notifyStatusChange(context.Background(), apps, uuid.New(), "shortlisted")

	if len(rn.sent) != 1 {
		t.Errorf("expected the send to be attempted once, got %d", len(rn.sent))
	}
}

func TestNotifyStatusChange_SkipsWithoutLineID(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{FullName: "ก", LineUserID: ""})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusHired)

	if len(rn.sent) != 0 {
		t.Errorf("no LINE id → no send, got %d", len(rn.sent))
	}
}

func TestNotifyStatusChange_SkipsNonNotifiableStatus(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{LineUserID: "U-1"})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	// 'parsed' is an internal pipeline step with no candidate-facing copy.
	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusParsed)

	if len(rn.sent) != 0 {
		t.Errorf("parsed is not candidate-notifiable, got %d sends", len(rn.sent))
	}
}

// 'scored' became candidate-notifiable (item 3: notify on every meaningful status)
// — the auto-screening outcome now reaches the candidate.
func TestNotifyStatusChange_SendsForScored(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{LineUserID: "U-1"})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusScored)

	if len(rn.sent) == 0 {
		t.Error("scored should now notify the candidate, got 0 sends")
	}
}

func TestNotifyStatusChange_SendsEmailWhenEmailPresent(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{FullName: "ก", Email: "a@b.com"}) // no LINE id
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusHired)

	if len(rn.sent) != 1 || rn.sent[0].Channel != notify.ChannelEmail || rn.sent[0].Recipient != "a@b.com" {
		t.Fatalf("expected 1 email send, got %+v", rn.sent)
	}
}

func TestNotifyStatusChange_SendsBothChannels(t *testing.T) {
	rn := &recNotifier{}
	d := deps(rn, &candidates.Candidate{FullName: "ก", LineUserID: "U-1", Email: "a@b.com"})
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}

	d.notifyStatusChange(context.Background(), apps, uuid.New(), "interview")

	if len(rn.sent) != 2 {
		t.Fatalf("expected LINE + email (2 sends), got %d: %+v", len(rn.sent), rn.sent)
	}
	channels := map[string]bool{rn.sent[0].Channel: true, rn.sent[1].Channel: true}
	if !channels[notify.ChannelLINE] || !channels[notify.ChannelEmail] {
		t.Fatalf("expected both line + email channels, got %+v", rn.sent)
	}
}

func TestNotifyStatusChange_NilDepsNoop(t *testing.T) {
	var d statusNotifyDeps // zero value: nil notifier + nil cands
	apps := stubApps{app: &Application{CandidateID: uuid.New()}}
	// Must not panic.
	d.notifyStatusChange(context.Background(), apps, uuid.New(), StatusHired)
}
