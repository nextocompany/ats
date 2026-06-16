package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// fakeIntaker records each Intake call and returns a fresh application id.
type fakeIntaker struct {
	calls []IntakeInput
}

func (f *fakeIntaker) Intake(_ context.Context, in IntakeInput) (IntakeResult, error) {
	f.calls = append(f.calls, in)
	return IntakeResult{ApplicationID: uuid.New(), CandidateID: uuid.New(), JobID: "job"}, nil
}

// bulkBody builds a multipart body: a position_id field + N resume files. Each
// file is (filename, contentType, bytes).
func bulkBody(t *testing.T, positionID string, files []struct {
	name, ct string
	data     []byte
}) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if positionID != "" {
		_ = w.WriteField("position_id", positionID)
	}
	for _, f := range files {
		h := map[string][]string{
			"Content-Disposition": {`form-data; name="resumes"; filename="` + f.name + `"`},
			"Content-Type":        {f.ct},
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		_, _ = part.Write(f.data)
	}
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

func bulkTestApp(svc bulkIntaker, role string) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
		return c.Next()
	})
	RegisterBulkRoutes(app, NewBulkHandler(svc))
	return app
}

func postBulk(t *testing.T, app *fiber.App, body *bytes.Buffer, ct string) (*fiber.App, int, bulkResult) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications/bulk-intake", body)
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data bulkResult `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return app, resp.StatusCode, env.Data
}

func TestBulkIntake_RoleGate(t *testing.T) {
	app := bulkTestApp(&fakeIntaker{}, "auditor")
	body, ct := bulkBody(t, uuid.NewString(), []struct {
		name, ct string
		data     []byte
	}{{"a.pdf", "application/pdf", []byte("%PDF")}})
	_, code, _ := postBulk(t, app, body, ct)
	if code != fiber.StatusForbidden {
		t.Fatalf("auditor should be 403, got %d", code)
	}
}

func TestBulkIntake_MissingPosition(t *testing.T) {
	app := bulkTestApp(&fakeIntaker{}, "hr_manager")
	body, ct := bulkBody(t, "", []struct {
		name, ct string
		data     []byte
	}{{"a.pdf", "application/pdf", []byte("%PDF")}})
	_, code, _ := postBulk(t, app, body, ct)
	if code != fiber.StatusBadRequest {
		t.Fatalf("missing position should be 400, got %d", code)
	}
}

func TestBulkIntake_EmptyFiles(t *testing.T) {
	app := bulkTestApp(&fakeIntaker{}, "sgm")
	body, ct := bulkBody(t, uuid.NewString(), nil)
	_, code, _ := postBulk(t, app, body, ct)
	if code != fiber.StatusBadRequest {
		t.Fatalf("no files should be 400, got %d", code)
	}
}

func TestBulkIntake_MixedValidInvalid(t *testing.T) {
	fi := &fakeIntaker{}
	app := bulkTestApp(fi, "hr_staff")
	body, ct := bulkBody(t, uuid.NewString(), []struct {
		name, ct string
		data     []byte
	}{
		{"ok1.pdf", "application/pdf", []byte("%PDF")},
		{"bad.exe", "application/x-msdownload", []byte("MZ")},
		{"ok2.png", "image/png", []byte("\x89PNG")},
	})
	_, code, res := postBulk(t, app, body, ct)
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res.Total != 3 || res.Succeeded != 2 || res.FailedCount != 1 {
		t.Fatalf("expected 3/2/1, got %+v", res)
	}
	if len(fi.calls) != 2 {
		t.Fatalf("intake should be called for the 2 valid files, got %d", len(fi.calls))
	}
	// Placeholder name = filename sans extension.
	if fi.calls[0].CandidateName != "ok1" {
		t.Fatalf("placeholder name = %q, want ok1", fi.calls[0].CandidateName)
	}
}
