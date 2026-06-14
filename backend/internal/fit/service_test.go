package fit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/interview"
	"github.com/nexto/hr-ats/internal/positions"
)

// --- stubs ---

type stubApps struct{ app *applications.Application }

func (s stubApps) FindByID(context.Context, uuid.UUID) (*applications.Application, error) {
	return s.app, nil
}

type stubInterviews struct{ sess *interview.Session }

func (s stubInterviews) FindByApplicationID(context.Context, uuid.UUID) (*interview.Session, error) {
	return s.sess, nil
}

type stubPositions struct{ pos []positions.Position }

func (s stubPositions) ListAll(context.Context) ([]positions.Position, error) { return s.pos, nil }

type stubCands struct{ cand *candidates.Candidate }

func (s stubCands) FindByID(context.Context, uuid.UUID) (*candidates.Candidate, error) {
	return s.cand, nil
}

type stubRepo struct{ saved *Analysis }

func (s *stubRepo) Upsert(_ context.Context, a Analysis, _ *uuid.UUID) error {
	cp := a
	cp.GeneratedAt = time.Now() // simulate the DB-stamped updated_at read back by Get
	s.saved = &cp
	return nil
}
func (s *stubRepo) FindByApplicationID(context.Context, uuid.UUID) (*Analysis, error) {
	if s.saved == nil {
		return nil, ErrNotFound
	}
	return s.saved, nil
}

func score(f float64) *float64 { return &f }

func completedApp() *applications.Application {
	return &applications.Application{ID: uuid.New(), CandidateID: uuid.New(), AIScore: score(81)}
}

func completedSession() *interview.Session {
	return &interview.Session{Status: interview.StatusCompleted, InterviewScore: score(78), Summary: "ดี"}
}

func newSvc(app *applications.Application, sess *interview.Session, pos []positions.Position) (*Service, *stubRepo) {
	repo := &stubRepo{}
	svc := NewService(repo, mockSummarizer{}, stubApps{app}, stubInterviews{sess}, stubPositions{pos},
		stubCands{&candidates.Candidate{FullName: "สมชาย ใจดี"}})
	return svc, repo
}

func TestGenerate_NotScored(t *testing.T) {
	app := completedApp()
	app.AIScore = nil
	svc, _ := newSvc(app, completedSession(), nil)
	if _, err := svc.Generate(context.Background(), app.ID, nil); !errors.Is(err, ErrNotScored) {
		t.Fatalf("err = %v, want ErrNotScored", err)
	}
}

func TestGenerate_InterviewMissing(t *testing.T) {
	app := completedApp()
	svc, _ := newSvc(app, nil, nil)
	if _, err := svc.Generate(context.Background(), app.ID, nil); !errors.Is(err, ErrInterviewIncomplete) {
		t.Fatalf("err = %v, want ErrInterviewIncomplete", err)
	}
}

func TestGenerate_InterviewNotCompleted(t *testing.T) {
	app := completedApp()
	sess := completedSession()
	sess.Status = interview.StatusInProgress
	svc, _ := newSvc(app, sess, nil)
	if _, err := svc.Generate(context.Background(), app.ID, nil); !errors.Is(err, ErrInterviewIncomplete) {
		t.Fatalf("err = %v, want ErrInterviewIncomplete", err)
	}
}

func TestGenerate_HappyPathPersists(t *testing.T) {
	app := completedApp()
	pos := []positions.Position{{ID: uuid.New(), TitleTH: "พนักงานขาย"}}
	svc, repo := newSvc(app, completedSession(), pos)

	a, err := svc.Generate(context.Background(), app.ID, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if a.ApplicationID != app.ID {
		t.Errorf("application_id = %v, want %v", a.ApplicationID, app.ID)
	}
	if a.GeneratedAt.IsZero() {
		t.Error("generated_at should be set")
	}
	if repo.saved == nil {
		t.Fatal("analysis was not persisted")
	}
	if len(repo.saved.Recommended) == 0 {
		t.Error("expected at least one recommendation persisted")
	}
}
