//go:build integration

// Integration tests for the pipeline. Require a running stack (Postgres +
// Azurite). Run with: make up && make migrate-up && go test -tags integration ./...
package pipeline

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/branch"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/dedup"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/scoring"
	"github.com/nexto/hr-ats/internal/vacancies"
	"github.com/nexto/hr-ats/pkg/blob"
	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/queue"
)

const azuriteLocal = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"

var errBoom = errors.New("boom")

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- providers we can vary per test ---

type fakeOCR struct {
	conf float64
	err  error
}

func (f fakeOCR) Extract(context.Context, []byte, string) (ai.OCRResult, error) {
	if f.err != nil {
		return ai.OCRResult{}, f.err
	}
	return ai.OCRResult{Text: "# cv\nName: ทดสอบ", Confidence: f.conf}, nil
}

type fakeParser struct {
	profile ai.Profile
	err     error
}

func (f fakeParser) Parse(context.Context, string, string) (ai.Profile, error) {
	return f.profile, f.err
}

// recordingIndexer captures the candidate ids the pipeline asks to index, so a
// test can assert it indexes the canonical (post-dedup) candidate.
type recordingIndexer struct{ ids []uuid.UUID }

func (r *recordingIndexer) Index(_ context.Context, id uuid.UUID) error {
	r.ids = append(r.ids, id)
	return nil
}

type fixture struct {
	pool *pgxpool.Pool
	blob *blob.Client
	cand candidates.Repository
	apps applications.Repository
	pos  positions.Repository
	vac  vacancies.Repository
}

func setup(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, env("DB_URL", "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"))
	if err != nil {
		t.Fatalf("connect db (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	// Isolate each test.
	if _, err := pool.Exec(ctx, `TRUNCATE applications, candidates, vacancies, stores, positions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	bc, err := blob.Connect(ctx, env("AZURE_BLOB_CONNECTION_STRING", azuriteLocal), "resumes")
	if err != nil {
		t.Fatalf("connect blob: %v", err)
	}

	return fixture{
		pool: pool, blob: bc,
		cand: candidates.NewRepository(pool),
		apps: applications.NewRepository(pool),
		pos:  positions.NewRepository(pool),
		vac:  vacancies.NewRepository(pool),
	}
}

func (f fixture) processor(ocr ai.OCR, parser ai.Parser) *Processor {
	scorer := scoring.NewScorer(&config.Config{AIProvider: "mock"})
	return NewProcessor(ocr, parser, f.blob, f.cand, f.apps,
		dedup.NewService(f.cand), scorer, branch.NewAssigner(f.vac), f.pos)
}

func seedPosition(t *testing.T, f fixture, minEdu, minExp int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	mh := `{"min_education_level":` + strconv.Itoa(minEdu) + `,"min_experience_months":` + strconv.Itoa(minExp) + `}`
	err := f.pool.QueryRow(context.Background(),
		`INSERT INTO positions (title_th, level, must_have_criteria, keywords, format_types)
		 VALUES ('ทดสอบ','Staff',$1::jsonb, $2, '{}') RETURNING id`,
		mh, []string{"cashier", "POS"},
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return id
}

func seedStoreVacancy(t *testing.T, f fixture, positionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO stores (store_no, store_name, format_type, subregion, province, latitude, longitude)
		 VALUES (1,'CM','A','Upper North','เชียงใหม่',18.79,98.99)`); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO vacancies (ps_vacancy_id, store_id, position_id, status, opened_at)
		 VALUES ('V-T-1', 1, $1, 'open', NOW())`, positionID); err != nil {
		t.Fatalf("seed vacancy: %v", err)
	}
}

func seedCandidateApp(t *testing.T, f fixture, positionID uuid.UUID) queue.ProcessApplicationPayload {
	t.Helper()
	ctx := context.Background()
	cand, err := f.cand.Create(ctx, candidates.Candidate{FullName: "ผู้สมัคร", Province: "เชียงใหม่", Status: "available"})
	if err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	app, err := f.apps.Create(ctx, applications.Application{
		CandidateID: cand.ID, PositionID: positionID, Status: applications.StatusPending, RawFileType: "pdf",
	})
	if err != nil {
		t.Fatalf("seed application: %v", err)
	}
	blobName := "resumes/" + app.ID.String() + "/cv.pdf"
	if _, err := f.blob.Upload(ctx, blobName, []byte("%PDF-1.4 dummy"), "application/pdf"); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	return queue.ProcessApplicationPayload{
		ApplicationID: app.ID.String(),
		CandidateID:   cand.ID.String(),
		BlobName:      blobName,
		FileType:      "pdf",
		PositionID:    positionID.String(),
	}
}

func task(t *testing.T, p queue.ProcessApplicationPayload) *asynq.Task {
	t.Helper()
	tk, err := queue.NewProcessApplicationTask(p)
	if err != nil {
		t.Fatalf("build task: %v", err)
	}
	return tk
}

func TestPipeline_ScoredAndAssigned(t *testing.T) {
	f := setup(t)
	pos := seedPosition(t, f, 1, 6) // mock profile (diploma, 24mo) passes
	seedStoreVacancy(t, f, pos)
	p := seedCandidateApp(t, f, pos)

	if err := f.processor(ai.NewMockOCR(), ai.NewMockParser()).HandleProcessApplication(context.Background(), task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusScored {
		t.Errorf("expected scored, got %q", app.Status)
	}
	if app.AIScore == nil || *app.AIScore <= 0 {
		t.Errorf("expected positive ai_score, got %v", app.AIScore)
	}
	if app.AssignedStoreID == nil || *app.AssignedStoreID != 1 {
		t.Errorf("expected assigned store 1, got %v", app.AssignedStoreID)
	}
}

func TestPipeline_GateFailRejected(t *testing.T) {
	f := setup(t)
	pos := seedPosition(t, f, 4, 0) // require master's → mock diploma fails
	p := seedCandidateApp(t, f, pos)

	if err := f.processor(ai.NewMockOCR(), ai.NewMockParser()).HandleProcessApplication(context.Background(), task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusRejected {
		t.Errorf("expected rejected, got %q", app.Status)
	}
	if app.MustHavePassed == nil || *app.MustHavePassed {
		t.Errorf("expected must_have_passed false, got %v", app.MustHavePassed)
	}
}

func TestPipeline_DuplicateRepointed(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pos := seedPosition(t, f, 1, 6)
	seedStoreVacancy(t, f, pos)

	// Pre-existing canonical with the same phone the mock parser will produce.
	canonical, err := f.cand.Create(ctx, candidates.Candidate{
		FullName: "สมชาย ใจดี", Phone: "0812345678", Status: "available",
	})
	if err != nil {
		t.Fatal(err)
	}

	p := seedCandidateApp(t, f, pos)
	if err := f.processor(ai.NewMockOCR(), ai.NewMockParser()).HandleProcessApplication(ctx, task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	app, err := f.apps.FindByID(ctx, uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.CandidateID != canonical.ID {
		t.Errorf("expected application repointed to canonical %v, got %v", canonical.ID, app.CandidateID)
	}
	if app.DedupState != dedup.StateAutoMerged {
		t.Errorf("expected dedup_state auto_merged, got %q", app.DedupState)
	}
}

// TestPipeline_IndexesCanonicalAfterDedup is the regression guard for the
// index-staleness bug: when an upload is deduped onto an existing canonical, the
// pipeline must index the CANONICAL candidate id, not the just-created (now
// is_duplicate_of) one. Indexing the deduped id no-ops against the index
// projection (which excludes duplicates), leaving the canonical's search doc
// stale (missing this better-scoring application).
func TestPipeline_IndexesCanonicalAfterDedup(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pos := seedPosition(t, f, 1, 6)
	seedStoreVacancy(t, f, pos)

	canonical, err := f.cand.Create(ctx, candidates.Candidate{
		FullName: "สมชาย ใจดี", Phone: "0812345678", Status: "available",
	})
	if err != nil {
		t.Fatal(err)
	}

	p := seedCandidateApp(t, f, pos)
	dedupedID := uuid.MustParse(p.CandidateID) // the new candidate that will be merged away

	rec := &recordingIndexer{}
	proc := f.processor(ai.NewMockOCR(), ai.NewMockParser())
	proc.SetIndexer(rec)
	if err := proc.HandleProcessApplication(ctx, task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	if len(rec.ids) != 1 {
		t.Fatalf("expected exactly 1 index call, got %d", len(rec.ids))
	}
	if rec.ids[0] == dedupedID {
		t.Fatal("indexed the deduped candidate id - no-ops against the projection, canonical doc goes stale")
	}
	if rec.ids[0] != canonical.ID {
		t.Errorf("indexed %v, want canonical %v", rec.ids[0], canonical.ID)
	}
}

func TestPipeline_LowConfidenceFlagsReview(t *testing.T) {
	f := setup(t)
	pos := seedPosition(t, f, 0, 0)
	p := seedCandidateApp(t, f, pos)

	proc := f.processor(fakeOCR{conf: 0.5}, fakeParser{profile: ai.Profile{Personal: ai.Personal{Name: "ทดสอบ"}}})
	if err := proc.HandleProcessApplication(context.Background(), task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if !app.NeedsManualReview {
		t.Error("expected needs_manual_review=true for low confidence")
	}
}

func TestPipeline_ParseFailureMarksFailed(t *testing.T) {
	f := setup(t)
	pos := seedPosition(t, f, 0, 0)
	p := seedCandidateApp(t, f, pos)

	proc := f.processor(fakeOCR{conf: 0.95}, fakeParser{err: errBoom})
	if err := proc.HandleProcessApplication(context.Background(), task(t, p)); err == nil {
		t.Fatal("expected error from parse failure")
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusFailed {
		t.Errorf("expected failed, got %q", app.Status)
	}
}

// A non-resume upload (parser reports is_resume=false) is a recoverable terminal
// outcome: status invalid_resume, and the task returns NIL (no asynq retry) — unlike
// a transient parse failure which errors + marks failed (test above).
func TestPipeline_NonResumeFlagsInvalidResume(t *testing.T) {
	f := setup(t)
	pos := seedPosition(t, f, 0, 0)
	p := seedCandidateApp(t, f, pos)

	proc := f.processor(fakeOCR{conf: 0.95}, fakeParser{profile: ai.Profile{IsResume: false}})
	if err := proc.HandleProcessApplication(context.Background(), task(t, p)); err != nil {
		t.Fatalf("non-resume must not return an error (no retry), got %v", err)
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusInvalidResume {
		t.Errorf("expected invalid_resume, got %q", app.Status)
	}
}
