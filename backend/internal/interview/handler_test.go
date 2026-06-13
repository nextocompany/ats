package interview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// allowScoper / denyScoper are ScopeChecker test doubles.
type allowScoper struct{}

func (allowScoper) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return true, nil
}

type denyScoper struct{}

func (denyScoper) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return false, nil
}

// testApp wires a Fiber app with the interview handler over in-memory doubles, so
// the HTTP layer (routing, mapError, projections) is exercised without a DB.
func testApp(t *testing.T) (*fiber.App, *Service, uuid.UUID) {
	t.Helper()
	repo := newMemRepo()
	appID := uuid.New()
	app := &applications.Application{ID: appID, CandidateID: uuid.New(), PositionID: uuid.New(), AISummary: "สรุป"}
	pos := &positions.Position{TitleTH: "พนักงานขาย", Responsibilities: "ดูแลร้าน", Qualifications: "สื่อสารดี"}
	cand := &candidates.Candidate{FullName: "สมหญิง"}
	svc := NewService(repo, mockInterviewer{}, stubApps{app}, stubPositions{pos}, stubCands{cand}, &recordingNotifier{}, "http://portal", 3)
	h := NewHandler(svc, allowScoper{}, "http://portal")

	fa := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterPublicRoutes(fa, h)
	RegisterDashboardRoutes(fa, h)
	return fa, svc, appID
}

func doJSON(t *testing.T, fa *fiber.App, method, path, body string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := fa.Test(req, -1)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var env map[string]any
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env
}

func TestHandler_BadToken404(t *testing.T) {
	fa, _, _ := testApp(t)
	code, env := doJSON(t, fa, http.MethodGet, "/api/v1/public/interview/nope", "")
	if code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", code)
	}
	if env["success"] != false {
		t.Fatalf("want success=false envelope, got %v", env)
	}
}

func TestHandler_InviteThenStartThenGet(t *testing.T) {
	fa, _, appID := testApp(t)

	// Invite.
	code, env := doJSON(t, fa, http.MethodPost, "/api/v1/applications/"+appID.String()+"/interview", "")
	if code != http.StatusOK {
		t.Fatalf("invite want 200, got %d (%v)", code, env)
	}
	data := env["data"].(map[string]any)
	token, _ := data["access_token"].(string)
	if token == "" {
		t.Fatalf("invite did not return an access_token")
	}

	// Candidate Start seeds the first question.
	code, env = doJSON(t, fa, http.MethodGet, "/api/v1/public/interview/"+token, "")
	if code != http.StatusOK {
		t.Fatalf("start want 200, got %d", code)
	}
	turns := env["data"].(map[string]any)["turns"].([]any)
	if len(turns) != 1 {
		t.Fatalf("want 1 seeded turn, got %d", len(turns))
	}

	// HR Get must NOT echo the raw access_token in the session body (H2).
	code, env = doJSON(t, fa, http.MethodGet, "/api/v1/applications/"+appID.String()+"/interview", "")
	if code != http.StatusOK {
		t.Fatalf("get want 200, got %d", code)
	}
	session := env["data"].(map[string]any)["session"].(map[string]any)
	if at, _ := session["access_token"].(string); at != "" {
		t.Fatalf("HR Get leaked access_token: %q", at)
	}
	if url, _ := env["data"].(map[string]any)["interview_url"].(string); !strings.Contains(url, token) {
		t.Fatalf("interview_url should carry the token, got %q", url)
	}
}

func TestHandler_RespondEmptyContent400(t *testing.T) {
	fa, _, appID := testApp(t)
	code, env := doJSON(t, fa, http.MethodPost, "/api/v1/applications/"+appID.String()+"/interview", "")
	token := env["data"].(map[string]any)["access_token"].(string)
	if _, _ = doJSON(t, fa, http.MethodGet, "/api/v1/public/interview/"+token, ""); code != http.StatusOK {
		t.Fatalf("start failed")
	}
	code, _ = doJSON(t, fa, http.MethodPost, "/api/v1/public/interview/"+token+"/message", `{"content":"   "}`)
	if code != http.StatusBadRequest {
		t.Fatalf("empty content want 400, got %d", code)
	}
}

func TestHandler_GetNoInterview404(t *testing.T) {
	fa, _, _ := testApp(t)
	code, _ := doJSON(t, fa, http.MethodGet, "/api/v1/applications/"+uuid.New().String()+"/interview", "")
	if code != http.StatusNotFound {
		t.Fatalf("want 404 for application without an interview, got %d", code)
	}
}

func TestHandler_InviteOutOfScope404(t *testing.T) {
	repo := newMemRepo()
	appID := uuid.New()
	app := &applications.Application{ID: appID, CandidateID: uuid.New(), PositionID: uuid.New()}
	svc := NewService(repo, mockInterviewer{}, stubApps{app}, stubPositions{&positions.Position{}}, stubCands{&candidates.Candidate{}}, &recordingNotifier{}, "http://portal", 3)
	h := NewHandler(svc, denyScoper{}, "http://portal") // caller cannot see this application
	fa := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterDashboardRoutes(fa, h)

	code, _ := doJSON(t, fa, http.MethodPost, "/api/v1/applications/"+appID.String()+"/interview", "")
	if code != http.StatusNotFound {
		t.Fatalf("out-of-scope invite want 404, got %d", code)
	}
}

func TestHandler_InvalidApplicationID400(t *testing.T) {
	fa, _, _ := testApp(t)
	code, _ := doJSON(t, fa, http.MethodPost, "/api/v1/applications/not-a-uuid/interview", "")
	if code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid id, got %d", code)
	}
}
