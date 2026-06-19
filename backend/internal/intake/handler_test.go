package intake

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/positions"
)

type fakeIntake struct {
	called bool
	last   applications.IntakeInput
	result applications.IntakeResult
}

func (f *fakeIntake) Intake(_ context.Context, in applications.IntakeInput) (applications.IntakeResult, error) {
	f.called = true
	f.last = in
	if f.result.ApplicationID == uuid.Nil {
		f.result = applications.IntakeResult{ApplicationID: uuid.New(), CandidateID: uuid.New(), JobID: "job-1"}
	}
	return f.result, nil
}

type fakePos struct {
	byID   *positions.Position
	byCode *positions.Position
}

func (f *fakePos) FindByID(_ context.Context, _ uuid.UUID) (*positions.Position, error) {
	return f.byID, nil
}
func (f *fakePos) FindByPSCode(_ context.Context, _ string) (*positions.Position, error) {
	return f.byCode, nil
}

func newTestApp(h *Handler) *fiber.App {
	app := fiber.New()
	// No HMAC in the unit tests — exercise the handler directly.
	app.Post("/api/v1/intake/:source", h.Submit)
	return app
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func post(t *testing.T, app *fiber.App, source string, body any) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/intake/"+source, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func validBody(positionID string) map[string]any {
	return map[string]any{
		"position_id": positionID,
		"full_name":   "สมชาย ใจดี",
		"email":       "somchai@example.com",
		"resume":      map[string]any{"filename": "cv.pdf", "content_type": "application/pdf", "base64": b64("%PDF-1.4 fake")},
	}
}

func TestSubmit_HappyPath(t *testing.T) {
	pos := &positions.Position{ID: uuid.New()}
	fi := &fakeIntake{}
	app := newTestApp(NewHandler(fi, &fakePos{byID: pos}))

	if code := post(t, app, "seek", validBody(pos.ID.String())); code != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}
	if !fi.called {
		t.Fatal("expected Intake to be called")
	}
	if fi.last.SourceChannel != "seek" {
		t.Errorf("source_channel = %q, want seek", fi.last.SourceChannel)
	}
	if fi.last.PositionID != pos.ID {
		t.Errorf("position id not resolved")
	}
	if fi.last.FileType != "pdf" {
		t.Errorf("file type = %q, want pdf", fi.last.FileType)
	}
}

func TestSubmit_UnknownSource(t *testing.T) {
	app := newTestApp(NewHandler(&fakeIntake{}, &fakePos{byID: &positions.Position{ID: uuid.New()}}))
	if code := post(t, app, "linkedin", validBody(uuid.New().String())); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 for unknown source, got %d", code)
	}
}

func TestSubmit_ResolvesByPositionRef(t *testing.T) {
	pos := &positions.Position{ID: uuid.New()}
	fi := &fakeIntake{}
	app := newTestApp(NewHandler(fi, &fakePos{byCode: pos}))
	body := map[string]any{
		"position_ref": "PS-1234",
		"full_name":    "Jane",
		"resume":       map[string]any{"filename": "r.pdf", "content_type": "application/pdf", "base64": b64("%PDF")},
	}
	if code := post(t, app, "ms_forms", body); code != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}
	if fi.last.PositionID != pos.ID {
		t.Error("expected position resolved via ps code")
	}
}

func TestSubmit_PositionNotFound(t *testing.T) {
	app := newTestApp(NewHandler(&fakeIntake{}, &fakePos{byID: nil}))
	if code := post(t, app, "seek", validBody(uuid.New().String())); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 for missing position, got %d", code)
	}
}

// errPos mirrors the real positions repo, which returns a wrapped pgx.ErrNoRows
// (not a nil pointer) when the row is absent — must still surface as 404, not 500.
type errPos struct{ err error }

func (e *errPos) FindByID(_ context.Context, _ uuid.UUID) (*positions.Position, error) {
	return nil, e.err
}
func (e *errPos) FindByPSCode(_ context.Context, _ string) (*positions.Position, error) {
	return nil, e.err
}

func TestSubmit_PositionNoRows404(t *testing.T) {
	app := newTestApp(NewHandler(&fakeIntake{}, &errPos{err: fmt.Errorf("positions: find: %w", pgx.ErrNoRows)}))
	if code := post(t, app, "seek", validBody(uuid.New().String())); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 for pgx.ErrNoRows, got %d", code)
	}
}

func TestSubmit_Validation(t *testing.T) {
	pos := &positions.Position{ID: uuid.New()}
	app := newTestApp(NewHandler(&fakeIntake{}, &fakePos{byID: pos}))

	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing name", map[string]any{"position_id": pos.ID.String(), "resume": map[string]any{"content_type": "application/pdf", "base64": b64("x")}}, fiber.StatusBadRequest},
		{"no position", map[string]any{"full_name": "X", "resume": map[string]any{"content_type": "application/pdf", "base64": b64("x")}}, fiber.StatusBadRequest},
		{"bad content type", map[string]any{"position_id": pos.ID.String(), "full_name": "X", "resume": map[string]any{"content_type": "text/plain", "base64": b64("x")}}, fiber.StatusUnsupportedMediaType},
		{"bad base64", map[string]any{"position_id": pos.ID.String(), "full_name": "X", "resume": map[string]any{"content_type": "application/pdf", "base64": "!!!not base64!!!"}}, fiber.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code := post(t, app, "seek", tc.body); code != tc.want {
				t.Errorf("got %d, want %d", code, tc.want)
			}
		})
	}
}

func TestSafeFilename(t *testing.T) {
	cases := []struct{ in, ft, want string }{
		{"../../etc/passwd", "pdf", "resume.pdf"},     // path stripped to "passwd"; no extension → default
		{"../../etc/passwd.pdf", "pdf", "passwd.pdf"}, // path stripped, extension kept
		{"resume final.docx", "docx", "resume_final.docx"},
		{"", "pdf", "resume.pdf"},
		{"weird*name?.pdf", "pdf", "weirdname.pdf"},
	}
	for _, c := range cases {
		got := safeFilename(c.in, c.ft)
		if got != c.want {
			t.Errorf("safeFilename(%q,%q) = %q, want %q", c.in, c.ft, got, c.want)
		}
	}
}
