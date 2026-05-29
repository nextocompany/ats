package applications

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/httpx"
)

// multipartBody builds a multipart request body with an optional file part.
func multipartBody(t *testing.T, fields map[string]string, fileField, fileName, contentType string, fileBytes []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if fileField != "" {
		h := map[string][]string{
			"Content-Disposition": {`form-data; name="` + fileField + `"; filename="` + fileName + `"`},
			"Content-Type":        {contentType},
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		_, _ = part.Write(fileBytes)
	}
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

func newTestApp(h *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, h)
	return app
}

// These cases all fail validation before the service/repos are touched, so a
// handler with nil collaborators is sufficient.

func TestIntake_MissingFile(t *testing.T) {
	app := newTestApp(NewHandler(nil, nil, nil))
	body, ct := multipartBody(t, map[string]string{"full_name": "A", "position_id": uuid.NewString()}, "", "", "", nil)
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications", body)
	req.Header.Set("Content-Type", ct)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntake_UnsupportedType(t *testing.T) {
	app := newTestApp(NewHandler(nil, nil, nil))
	body, ct := multipartBody(t,
		map[string]string{"full_name": "A", "position_id": uuid.NewString()},
		"resume", "x.exe", "application/x-msdownload", []byte("MZ"))
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications", body)
	req.Header.Set("Content-Type", ct)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", resp.StatusCode)
	}
}

func TestIntake_BadPositionID(t *testing.T) {
	app := newTestApp(NewHandler(nil, nil, nil))
	body, ct := multipartBody(t,
		map[string]string{"full_name": "A", "position_id": "not-a-uuid"},
		"resume", "cv.pdf", "application/pdf", []byte("%PDF-1.4"))
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications", body)
	req.Header.Set("Content-Type", ct)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
