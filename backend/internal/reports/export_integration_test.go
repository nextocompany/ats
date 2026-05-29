//go:build integration

package reports

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/notify"
)

// dsn() is defined in reports_integration_test.go (same build tag/package).

type fakeBlob struct{ uploads int }

func (f *fakeBlob) Upload(_ context.Context, name string, _ []byte, _ string) (string, error) {
	f.uploads++
	return "http://blob/resumes/" + name, nil
}
func (f *fakeBlob) SignedURLForStored(stored string, _ time.Duration) (string, error) {
	return stored + "?sig=test", nil
}

type fakeNotifier struct{ sent int }

func (f *fakeNotifier) Send(_ context.Context, _ notify.Message) error { f.sent++; return nil }

func setup(t *testing.T) *Repo {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE report_exports, applications, candidates, positions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	var posID, candID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('แคชเชียร์') RETURNING id`).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status) VALUES ('c','career_portal','available') RETURNING id`).Scan(&candID); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO applications (candidate_id, position_id, status, must_have_passed) VALUES ($1,$2,'hired',true)`, candID, posID); err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return New(pool)
}

func TestExport_StoresAndDelivers(t *testing.T) {
	repo := setup(t)
	blob := &fakeBlob{}
	notifier := &fakeNotifier{}
	svc := NewExportService(repo, blob, notifier, []string{"hr@example.com"})

	exp, err := svc.Export(context.Background(), "ondemand", "2026-W22")
	if err != nil {
		t.Fatal(err)
	}
	if blob.uploads != 2 {
		t.Errorf("expected 2 blob uploads (csv+json), got %d", blob.uploads)
	}
	if notifier.sent != 1 {
		t.Errorf("expected 1 delivery, got %d", notifier.sent)
	}
	if !exp.Delivered {
		t.Error("expected delivered=true with a recipient + successful send")
	}
	if exp.ID == uuid.Nil || exp.CSVBlob == "" || exp.JSONBlob == "" {
		t.Errorf("expected persisted export with blob urls, got %+v", exp)
	}

	list, err := repo.ListExports(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 export listed, got %d", len(list))
	}
}

func TestExport_NoRecipientsNotDelivered(t *testing.T) {
	repo := setup(t)
	notifier := &fakeNotifier{}
	svc := NewExportService(repo, &fakeBlob{}, notifier, nil)

	exp, err := svc.Export(context.Background(), "weekly", "2026-W23")
	if err != nil {
		t.Fatal(err)
	}
	if notifier.sent != 0 {
		t.Errorf("expected no delivery with no recipients, got %d", notifier.sent)
	}
	if exp.Delivered {
		t.Error("expected delivered=false with no recipients")
	}
}
