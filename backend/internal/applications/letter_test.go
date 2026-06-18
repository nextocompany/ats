package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/letters"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

type fakeLetterRepo struct {
	inScope    bool
	gatherErr  error
	gathered   letters.LetterData
	upserted   Letter
	upsertCall bool
	byApp      []Letter
	byID       *Letter
	byAccount  []Letter
}

func (f *fakeLetterRepo) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeLetterRepo) GatherLetterData(_ context.Context, _ uuid.UUID, letterType string) (letters.LetterData, error) {
	if f.gatherErr != nil {
		return letters.LetterData{}, f.gatherErr
	}
	f.gathered.Type = letterType
	return f.gathered, nil
}
func (f *fakeLetterRepo) UpsertLetter(_ context.Context, appID, _ uuid.UUID, letterType, blobURL string) (Letter, error) {
	f.upsertCall = true
	f.upserted = Letter{ID: uuid.New(), ApplicationID: appID, Type: letterType, BlobURL: blobURL, CreatedAt: time.Now()}
	return f.upserted, nil
}
func (f *fakeLetterRepo) GetLettersByApplication(context.Context, uuid.UUID) ([]Letter, error) {
	return f.byApp, nil
}
func (f *fakeLetterRepo) GetLetterByID(context.Context, uuid.UUID) (*Letter, error) {
	return f.byID, nil
}
func (f *fakeLetterRepo) ListLettersByAccount(context.Context, uuid.UUID) ([]Letter, error) {
	return f.byAccount, nil
}

type fakeRenderer struct{ calls int }

func (f *fakeRenderer) Render(letters.LetterData) ([]byte, error) {
	f.calls++
	return []byte("%PDF-1.4 fake"), nil
}

type fakeBlob struct{ uploads int }

func (f *fakeBlob) Upload(context.Context, string, []byte, string) (string, error) {
	f.uploads++
	return "http://blob/resumes/letters/x.pdf", nil
}
func (f *fakeBlob) SignedURLForStored(string, time.Duration) (string, error) {
	return "http://blob/signed?sig=x", nil
}

func letterTestApp(repo letterStore, user middleware.DevUser, rnd letterRenderer, bl letterBlob) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterLetterRoutes(app, NewLetterHandler(repo, rnd, bl))
	return app
}

func postLetter(t *testing.T, app *fiber.App, id string, body any) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(fiber.MethodPost, "/api/v1/applications/"+id+"/letters", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestGenerateLetter_RoleGate(t *testing.T) {
	repo := &fakeLetterRepo{inScope: true}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "auditor"}, &fakeRenderer{}, &fakeBlob{})
	if got := postLetter(t, app, uuid.NewString(), letterReq{Type: "offer"}); got != fiber.StatusForbidden {
		t.Fatalf("auditor generating a letter should be 403, got %d", got)
	}
}

func TestGenerateLetter_BadType(t *testing.T) {
	repo := &fakeLetterRepo{inScope: true}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := postLetter(t, app, uuid.NewString(), letterReq{Type: "bogus"}); got != fiber.StatusBadRequest {
		t.Fatalf("bad letter type should be 400, got %d", got)
	}
}

func TestGenerateLetter_Precondition(t *testing.T) {
	repo := &fakeLetterRepo{inScope: true, gatherErr: ErrLetterPreconditions}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := postLetter(t, app, uuid.NewString(), letterReq{Type: "interview"}); got != fiber.StatusBadRequest {
		t.Fatalf("missing interview should be 400, got %d", got)
	}
}

func TestGenerateLetter_HappyPath(t *testing.T) {
	repo := &fakeLetterRepo{inScope: true}
	rnd := &fakeRenderer{}
	bl := &fakeBlob{}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, rnd, bl)
	if got := postLetter(t, app, uuid.NewString(), letterReq{Type: "offer"}); got != fiber.StatusCreated {
		t.Fatalf("hr_manager generating an offer letter should be 201, got %d", got)
	}
	if rnd.calls != 1 || bl.uploads != 1 || !repo.upsertCall {
		t.Fatalf("expected render+upload+upsert; render=%d upload=%d upsert=%v", rnd.calls, bl.uploads, repo.upsertCall)
	}
}

func TestGenerateLetter_OutOfScope(t *testing.T) {
	repo := &fakeLetterRepo{inScope: false}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := postLetter(t, app, uuid.NewString(), letterReq{Type: "offer"}); got != fiber.StatusNotFound {
		t.Fatalf("out-of-scope application should be 404, got %d", got)
	}
}

func getLetter(t *testing.T, app *fiber.App, appID, letterID string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/applications/"+appID+"/letters/"+letterID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestDownloadLetter_HappyPath(t *testing.T) {
	appID := uuid.New()
	letterID := uuid.New()
	repo := &fakeLetterRepo{inScope: true, byID: &Letter{ID: letterID, ApplicationID: appID, Type: LetterOffer, BlobURL: "http://blob/resumes/letters/a.pdf"}}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := getLetter(t, app, appID.String(), letterID.String()); got != fiber.StatusOK {
		t.Fatalf("downloading own application's letter should be 200, got %d", got)
	}
}

func TestDownloadLetter_CrossApplicationDenied(t *testing.T) {
	// The letter belongs to a DIFFERENT application than the :id in the path.
	repo := &fakeLetterRepo{inScope: true, byID: &Letter{ID: uuid.New(), ApplicationID: uuid.New(), Type: LetterOffer, BlobURL: "x"}}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := getLetter(t, app, uuid.NewString(), uuid.NewString()); got != fiber.StatusNotFound {
		t.Fatalf("downloading a letter from another application should be 404, got %d", got)
	}
}

func TestDownloadLetter_NotFound(t *testing.T) {
	repo := &fakeLetterRepo{inScope: true, byID: nil}
	app := letterTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"}, &fakeRenderer{}, &fakeBlob{})
	if got := getLetter(t, app, uuid.NewString(), uuid.NewString()); got != fiber.StatusNotFound {
		t.Fatalf("missing letter should be 404, got %d", got)
	}
}

// ── Candidate letter handler ────────────────────────────────────────────────

func candidateLetterTestApp(repo letterCandidateStore, bl letterBlob, acct *candidateauth.Account) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		if acct != nil {
			c.Locals("candidate_account", acct)
		}
		return c.Next()
	})
	passthrough := func(c *fiber.Ctx) error { return c.Next() }
	RegisterCandidateLetterRoutes(app, NewLetterCandidateHandler(repo, bl), passthrough)
	return app
}

func getMineLetters(t *testing.T, app *fiber.App) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/public/auth/letters", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

func TestCandidateLetters_Unauthed(t *testing.T) {
	app := candidateLetterTestApp(&fakeLetterRepo{}, &fakeBlob{}, nil)
	if code, _ := getMineLetters(t, app); code != fiber.StatusUnauthorized {
		t.Fatalf("no session should be 401, got %d", code)
	}
}

func TestCandidateLetters_ListsSignedUrls(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	repo := &fakeLetterRepo{byAccount: []Letter{
		{ID: uuid.New(), Type: LetterOffer, BlobURL: "http://blob/resumes/letters/a.pdf", CreatedAt: time.Now()},
	}}
	app := candidateLetterTestApp(repo, &fakeBlob{}, acct)
	code, body := getMineLetters(t, app)
	if code != fiber.StatusOK {
		t.Fatalf("listing my letters should be 200, got %d", code)
	}
	var env struct {
		Data []LetterView `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].URL == "" {
		t.Fatalf("expected 1 letter with a signed URL, got %+v", env.Data)
	}
}
