//go:build integration

// Integration tests for the pipeline. Require a running stack (Postgres +
// Azurite). Run with: make up && make migrate-up && go test -tags integration ./...
package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/pkg/blob"
	"github.com/nexto/hr-ats/pkg/database"
	"github.com/nexto/hr-ats/pkg/queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

const azuriteLocal = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"

var errBoom = errors.New("boom")

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fakeOCR / fakeParser let us drive confidence and error paths.
type fakeOCR struct {
	conf float64
	err  error
}

func (f fakeOCR) Extract(_ context.Context, _ []byte, _ string) (ai.OCRResult, error) {
	if f.err != nil {
		return ai.OCRResult{}, f.err
	}
	return ai.OCRResult{Text: "# cv\nName: Test", Confidence: f.conf}, nil
}

type fakeParser struct {
	profile ai.Profile
	err     error
}

func (f fakeParser) Parse(_ context.Context, _, _ string) (ai.Profile, error) {
	return f.profile, f.err
}

type fixture struct {
	pool *pgxpool.Pool
	blob *blob.Client
	cand candidates.Repository
	apps applications.Repository
}

func setup(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()

	pool, err := database.Connect(ctx, env("DB_URL", "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"))
	if err != nil {
		t.Fatalf("connect db (is the stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)

	bc, err := blob.Connect(ctx, env("AZURE_BLOB_CONNECTION_STRING", azuriteLocal), "resumes")
	if err != nil {
		t.Fatalf("connect blob: %v", err)
	}

	return fixture{pool: pool, blob: bc, cand: candidates.NewRepository(pool), apps: applications.NewRepository(pool)}
}

// seed creates a position + candidate + pending application and uploads a raw
// file, returning the enqueue payload.
func seed(t *testing.T, f fixture) queue.ProcessApplicationPayload {
	t.Helper()
	ctx := context.Background()

	var posID uuid.UUID
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO positions (title_th, level) VALUES ('ทดสอบ', 'Staff') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	cand, err := f.cand.Create(ctx, candidates.Candidate{FullName: "ผู้สมัคร ทดสอบ", Status: "available"})
	if err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	app, err := f.apps.Create(ctx, applications.Application{
		CandidateID: cand.ID, PositionID: posID, Status: applications.StatusPending, RawFileType: "pdf",
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
		PositionID:    posID.String(),
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

func TestPipeline_HappyPath(t *testing.T) {
	f := setup(t)
	p := seed(t, f)

	proc := NewProcessor(ai.NewMockOCR(), ai.NewMockParser(), f.blob, f.cand, f.apps)
	if err := proc.HandleProcessApplication(context.Background(), task(t, p)); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	id := uuid.MustParse(p.ApplicationID)
	app, err := f.apps.FindByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusParsed {
		t.Errorf("expected status parsed, got %q", app.Status)
	}
	if app.ParsedProfileBlobURL == "" {
		t.Error("expected parsed_profile_blob_url to be set")
	}
	if app.OCRConfidence == nil || *app.OCRConfidence < 0.9 {
		t.Errorf("expected high ocr confidence, got %v", app.OCRConfidence)
	}
	if app.NeedsManualReview {
		t.Error("did not expect manual review flag for high-confidence OCR")
	}
}

func TestPipeline_LowConfidenceFlagsReview(t *testing.T) {
	f := setup(t)
	p := seed(t, f)

	proc := NewProcessor(
		fakeOCR{conf: 0.5},
		fakeParser{profile: ai.Profile{Personal: ai.Personal{Name: "ทดสอบ"}}},
		f.blob, f.cand, f.apps,
	)
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
	if app.Status != applications.StatusParsed {
		t.Errorf("expected status parsed (still parses), got %q", app.Status)
	}
}

func TestPipeline_ParseFailureMarksFailed(t *testing.T) {
	f := setup(t)
	p := seed(t, f)

	proc := NewProcessor(
		fakeOCR{conf: 0.95},
		fakeParser{err: errBoom},
		f.blob, f.cand, f.apps,
	)
	if err := proc.HandleProcessApplication(context.Background(), task(t, p)); err == nil {
		t.Fatal("expected error from parse failure")
	}

	app, err := f.apps.FindByID(context.Background(), uuid.MustParse(p.ApplicationID))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != applications.StatusFailed {
		t.Errorf("expected status failed, got %q", app.Status)
	}
}
