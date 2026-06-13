package interview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/positions"
)

// --- in-memory test doubles ---

type memRepo struct {
	byID    map[uuid.UUID]*Session
	byApp   map[uuid.UUID]*Session
	byToken map[string]*Session
}

func newMemRepo() *memRepo {
	return &memRepo{
		byID:    map[uuid.UUID]*Session{},
		byApp:   map[uuid.UUID]*Session{},
		byToken: map[string]*Session{},
	}
}

func (m *memRepo) put(s *Session) {
	m.byID[s.ID] = s
	m.byApp[s.ApplicationID] = s
	m.byToken[s.AccessToken] = s
}

func (m *memRepo) Create(_ context.Context, appID uuid.UUID, token string) (*Session, error) {
	now := time.Now()
	s := &Session{
		ID:            uuid.New(),
		ApplicationID: appID,
		AccessToken:   token,
		Status:        StatusInvited,
		Conversation:  []Turn{},
		InvitedAt:     now,
		ExpiresAt:     now.Add(7 * 24 * time.Hour),
		CreatedAt:     now,
	}
	m.put(s)
	return s, nil
}

func (m *memRepo) FindByToken(_ context.Context, token string) (*Session, error) {
	s, ok := m.byToken[token]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *memRepo) FindByApplicationID(_ context.Context, appID uuid.UUID) (*Session, error) {
	return m.byApp[appID], nil // nil when absent — matches the pg repo contract
}

func (m *memRepo) SaveConversation(_ context.Context, id uuid.UUID, conv []Turn, turnCount int, status string, startedAt *time.Time, expectedVersion int) error {
	s := m.byID[id]
	if s.Version != expectedVersion {
		return ErrConflict // optimistic-lock miss, as the pg repo would report
	}
	s.Conversation = conv
	s.TurnCount = turnCount
	s.Status = status
	s.Version++
	if s.StartedAt == nil && startedAt != nil {
		s.StartedAt = startedAt
	}
	return nil
}

func (m *memRepo) SetEvaluation(_ context.Context, id uuid.UUID, ev Evaluation) error {
	s := m.byID[id]
	if s.Status == StatusCompleted {
		return nil // idempotent, matches the pg repo's WHERE status <> completed
	}
	applyEvaluation(s, ev)
	s.Status = StatusCompleted
	s.Version++
	now := time.Now()
	s.CompletedAt = &now
	return nil
}

func (m *memRepo) MarkExpired(_ context.Context, id uuid.UUID) error {
	s := m.byID[id]
	if s != nil && s.Status != StatusCompleted {
		s.Status = StatusExpired
	}
	return nil
}

type stubApps struct{ app *applications.Application }

func (s stubApps) FindByID(_ context.Context, _ uuid.UUID) (*applications.Application, error) {
	return s.app, nil
}

type stubPositions struct{ pos *positions.Position }

func (s stubPositions) FindByID(_ context.Context, _ uuid.UUID) (*positions.Position, error) {
	return s.pos, nil
}

type stubCands struct{ cand *candidates.Candidate }

func (s stubCands) FindByID(_ context.Context, _ uuid.UUID) (*candidates.Candidate, error) {
	return s.cand, nil
}

type recordingNotifier struct{ sent int }

func (r *recordingNotifier) Send(_ context.Context, _ notify.Message) error {
	r.sent++
	return nil
}

func newTestService(t *testing.T, repo Repository, n notify.Notifier, maxTurns int) (*Service, uuid.UUID) {
	t.Helper()
	appID := uuid.New()
	app := &applications.Application{ID: appID, CandidateID: uuid.New(), PositionID: uuid.New(), AISummary: "ผู้สมัครมีประสบการณ์ค้าปลีก"}
	pos := &positions.Position{TitleTH: "พนักงานขาย", Responsibilities: "ดูแลหน้าร้าน", Qualifications: "สื่อสารดี"}
	cand := &candidates.Candidate{FullName: "สมชาย ใจดี", LineUserID: "U123"}
	svc := NewService(repo, mockInterviewer{}, stubApps{app}, stubPositions{pos}, stubCands{cand}, n, "http://portal", maxTurns)
	return svc, appID
}

// --- tests ---

func TestInvite_Idempotent(t *testing.T) {
	repo := newMemRepo()
	n := &recordingNotifier{}
	svc, appID := newTestService(t, repo, n, 3)
	ctx := context.Background()

	first, err := svc.Invite(ctx, appID)
	if err != nil {
		t.Fatalf("first invite: %v", err)
	}
	second, err := svc.Invite(ctx, appID)
	if err != nil {
		t.Fatalf("second invite: %v", err)
	}
	if first.ID != second.ID || first.AccessToken != second.AccessToken {
		t.Fatalf("invite not idempotent: %v vs %v", first.ID, second.ID)
	}
	if n.sent != 1 {
		t.Fatalf("expected exactly one notification, got %d", n.sent)
	}
}

func TestInvite_NoLineHandle_NoNotify(t *testing.T) {
	repo := newMemRepo()
	n := &recordingNotifier{}
	appID := uuid.New()
	app := &applications.Application{ID: appID, CandidateID: uuid.New(), PositionID: uuid.New()}
	pos := &positions.Position{TitleTH: "พนักงานขาย"}
	cand := &candidates.Candidate{FullName: "ไม่มีไลน์"} // empty LineUserID
	svc := NewService(repo, mockInterviewer{}, stubApps{app}, stubPositions{pos}, stubCands{cand}, n, "http://portal", 3)

	if _, err := svc.Invite(context.Background(), appID); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if n.sent != 0 {
		t.Fatalf("expected no notification for candidate without LINE handle, got %d", n.sent)
	}
}

func TestStart_SeedsFirstQuestion(t *testing.T) {
	repo := newMemRepo()
	svc, appID := newTestService(t, repo, &recordingNotifier{}, 3)
	ctx := context.Background()

	invited, _ := svc.Invite(ctx, appID)
	started, err := svc.Start(ctx, invited.AccessToken)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.Status != StatusInProgress {
		t.Fatalf("expected in_progress, got %q", started.Status)
	}
	if len(started.Conversation) != 1 || started.Conversation[0].Role != RoleAssistant {
		t.Fatalf("expected one assistant turn, got %+v", started.Conversation)
	}
	if started.StartedAt == nil {
		t.Fatalf("expected started_at set")
	}
}

func TestRespond_RunsToCompletionAndEvaluates(t *testing.T) {
	repo := newMemRepo()
	const maxTurns = 3
	svc, appID := newTestService(t, repo, &recordingNotifier{}, maxTurns)
	ctx := context.Background()

	invited, _ := svc.Invite(ctx, appID)
	if _, err := svc.Start(ctx, invited.AccessToken); err != nil {
		t.Fatalf("start: %v", err)
	}

	var last *Session
	for i := 0; i < maxTurns+2; i++ {
		s, err := svc.Respond(ctx, invited.AccessToken, "นี่คือคำตอบของผม")
		if err != nil {
			t.Fatalf("respond %d: %v", i, err)
		}
		last = s
		if s.Status == StatusCompleted {
			break
		}
	}
	if last == nil || last.Status != StatusCompleted {
		t.Fatalf("interview did not complete within max turns: %+v", last)
	}
	if last.InterviewScore == nil {
		t.Fatalf("expected an evaluation score after completion")
	}
	if last.userTurns() > maxTurns {
		t.Fatalf("exceeded max turns: %d > %d", last.userTurns(), maxTurns)
	}
}

func TestRespond_EmptyAnswerRejected(t *testing.T) {
	repo := newMemRepo()
	svc, appID := newTestService(t, repo, &recordingNotifier{}, 3)
	ctx := context.Background()
	invited, _ := svc.Invite(ctx, appID)
	_, _ = svc.Start(ctx, invited.AccessToken)

	if _, err := svc.Respond(ctx, invited.AccessToken, "   "); !errors.Is(err, ErrEmptyAnswer) {
		t.Fatalf("expected ErrEmptyAnswer, got %v", err)
	}
}

func TestRespond_NotStartedRejected(t *testing.T) {
	repo := newMemRepo()
	svc, appID := newTestService(t, repo, &recordingNotifier{}, 3)
	ctx := context.Background()
	invited, _ := svc.Invite(ctx, appID) // invited, not started → not in_progress

	if _, err := svc.Respond(ctx, invited.AccessToken, "คำตอบ"); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("expected ErrNotAnswerable, got %v", err)
	}
}

func TestSaveConversation_StaleVersionConflicts(t *testing.T) {
	repo := newMemRepo()
	s, _ := repo.Create(context.Background(), uuid.New(), "tok") // version 0
	// A save at the current version succeeds and bumps the version to 1.
	if err := repo.SaveConversation(context.Background(), s.ID, []Turn{}, 0, StatusInProgress, nil, 0); err != nil {
		t.Fatalf("save at v0: %v", err)
	}
	// A second save still expecting version 0 (a stale snapshot, as a concurrent
	// request would hold) must conflict rather than overwrite.
	if err := repo.SaveConversation(context.Background(), s.ID, []Turn{}, 0, StatusInProgress, nil, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on stale version, got %v", err)
	}
}

func TestStart_ExpiredMarksExpired(t *testing.T) {
	repo := newMemRepo()
	svc, appID := newTestService(t, repo, &recordingNotifier{}, 3)
	ctx := context.Background()
	invited, _ := svc.Invite(ctx, appID)
	repo.byToken[invited.AccessToken].ExpiresAt = time.Now().Add(-time.Hour)

	if _, err := svc.Start(ctx, invited.AccessToken); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("expected ErrNotAnswerable, got %v", err)
	}
	if got := repo.byToken[invited.AccessToken].Status; got != StatusExpired {
		t.Fatalf("expected status persisted as expired, got %q", got)
	}
}

func TestStart_ExpiredRejected(t *testing.T) {
	repo := newMemRepo()
	svc, appID := newTestService(t, repo, &recordingNotifier{}, 3)
	ctx := context.Background()
	invited, _ := svc.Invite(ctx, appID)
	// force-expire
	repo.byToken[invited.AccessToken].ExpiresAt = time.Now().Add(-time.Hour)

	if _, err := svc.Start(ctx, invited.AccessToken); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("expected ErrNotAnswerable for expired session, got %v", err)
	}
}

func TestStart_UnknownTokenNotFound(t *testing.T) {
	repo := newMemRepo()
	svc, _ := newTestService(t, repo, &recordingNotifier{}, 3)
	if _, err := svc.Start(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
