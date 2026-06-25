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
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ── HR offer handler ────────────────────────────────────────────────────────

// fakeOfferRepo embeds the Repository interface (nil) and overrides only the
// methods the HR offer handler calls; any other call would panic (intentionally).
type fakeOfferRepo struct {
	Repository
	inScope   bool
	app       *Application
	existing  *Offer
	created   Offer
	createErr error
	updated   Offer
	sent      Offer
	sendErr   error
}

func (f *fakeOfferRepo) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeOfferRepo) FindByID(context.Context, uuid.UUID) (*Application, error) { return f.app, nil }
func (f *fakeOfferRepo) CreateOffer(_ context.Context, _, _ uuid.UUID, _ OfferInput) (Offer, error) {
	return f.created, f.createErr
}
func (f *fakeOfferRepo) UpdateOffer(context.Context, uuid.UUID, OfferInput) (Offer, error) {
	return f.updated, nil
}
func (f *fakeOfferRepo) GetOfferByApplication(context.Context, uuid.UUID) (*Offer, error) {
	return f.existing, nil
}
func (f *fakeOfferRepo) SendOffer(context.Context, uuid.UUID) (Offer, error) {
	return f.sent, f.sendErr
}

func offerTestApp(repo Repository, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterOfferRoutes(app, NewOfferHandler(repo))
	return app
}

func doOffer(t *testing.T, app *fiber.App, method, path string, body any) int {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestCreateOffer_RoleGate(t *testing.T) {
	repo := &fakeOfferRepo{inScope: true, app: &Application{Status: StatusOffer}}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer", OfferInput{}); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff composing an offer should be 403, got %d", got)
	}
}

func TestCreateOffer_WrongStatus(t *testing.T) {
	repo := &fakeOfferRepo{inScope: true, app: &Application{Status: StatusInterviewed}}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer", OfferInput{}); got != fiber.StatusBadRequest {
		t.Fatalf("composing before the offer stage should be 400, got %d", got)
	}
}

func TestCreateOffer_HappyPath(t *testing.T) {
	repo := &fakeOfferRepo{inScope: true, app: &Application{Status: StatusOffer}, created: Offer{ID: uuid.New(), Status: OfferDraft}}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer", OfferInput{}); got != fiber.StatusCreated {
		t.Fatalf("hr_manager composing from offer stage should be 201, got %d", got)
	}
}

func TestCreateOffer_DuplicateIs409(t *testing.T) {
	repo := &fakeOfferRepo{inScope: true, app: &Application{Status: StatusOffer}, createErr: ErrOfferExists}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer", OfferInput{}); got != fiber.StatusConflict {
		t.Fatalf("duplicate offer should be 409, got %d", got)
	}
}

func TestUpdateOffer_RoleGate(t *testing.T) {
	repo := &fakeOfferRepo{inScope: true}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	if got := doOffer(t, app, fiber.MethodPatch, "/api/v1/applications/"+uuid.NewString()+"/offer", OfferInput{}); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff editing an offer should be 403, got %d", got)
	}
}

func TestSendOffer_Incomplete(t *testing.T) {
	// Offer with no salary/start date cannot be sent.
	repo := &fakeOfferRepo{inScope: true, existing: &Offer{Status: OfferDraft}}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer/send", nil); got != fiber.StatusBadRequest {
		t.Fatalf("sending an incomplete offer should be 400, got %d", got)
	}
}

func TestSendOffer_HappyPath(t *testing.T) {
	sal := 25000.0
	day := time.Now().AddDate(0, 0, 14)
	repo := &fakeOfferRepo{
		inScope:  true,
		existing: &Offer{Status: OfferDraft, Salary: &sal, StartDate: &day},
		sent:     Offer{Status: OfferSent, Salary: &sal, StartDate: &day},
	}
	app := offerTestApp(repo, middleware.DevUser{ID: uuid.NewString(), Role: "super_admin"})
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/offer/send", nil); got != fiber.StatusOK {
		t.Fatalf("sending a complete offer should be 200, got %d", got)
	}
}

// ── Candidate offer handler ─────────────────────────────────────────────────

type fakeOfferCandStore struct {
	list         []OfferView
	respond      Offer
	respondErr   error
	negotiate    Offer
	negotiateErr error
	app          *Application
}

func (f *fakeOfferCandStore) ListOffersByAccount(context.Context, uuid.UUID) ([]OfferView, error) {
	return f.list, nil
}
func (f *fakeOfferCandStore) GetOfferByID(context.Context, uuid.UUID) (*Offer, error) {
	return nil, nil
}
func (f *fakeOfferCandStore) RespondOffer(context.Context, uuid.UUID, uuid.UUID, bool, string) (Offer, error) {
	return f.respond, f.respondErr
}
func (f *fakeOfferCandStore) NegotiateOffer(context.Context, uuid.UUID, uuid.UUID, *float64, string, int) (Offer, error) {
	return f.negotiate, f.negotiateErr
}
func (f *fakeOfferCandStore) FindByID(context.Context, uuid.UUID) (*Application, error) {
	return f.app, nil
}

func candidateOfferTestApp(store offerCandidateStore, acct *candidateauth.Account) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		if acct != nil {
			// candidateauth.candidateLocalsKey is the literal "candidate_account".
			c.Locals("candidate_account", acct)
		}
		return c.Next()
	})
	passthrough := func(c *fiber.Ctx) error { return c.Next() }
	RegisterCandidateOfferRoutes(app, NewOfferCandidateHandler(store, 3), passthrough)
	return app
}

func TestRespondOffer_Unauthed(t *testing.T) {
	app := candidateOfferTestApp(&fakeOfferCandStore{}, nil)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "accept"}); got != fiber.StatusUnauthorized {
		t.Fatalf("no session should be 401, got %d", got)
	}
}

func TestRespondOffer_DeclineNoReason(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	app := candidateOfferTestApp(&fakeOfferCandStore{}, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "decline"}); got != fiber.StatusBadRequest {
		t.Fatalf("decline without reason should be 400, got %d", got)
	}
}

func TestRespondOffer_NotFound(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOfferCandStore{respondErr: ErrOfferNotFound}
	app := candidateOfferTestApp(store, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "accept"}); got != fiber.StatusNotFound {
		t.Fatalf("offer not owned by account should be 404, got %d", got)
	}
}

func TestRespondOffer_Conflict(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOfferCandStore{respondErr: ErrOfferConflict}
	app := candidateOfferTestApp(store, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "accept"}); got != fiber.StatusConflict {
		t.Fatalf("expired/decided offer should be 409, got %d", got)
	}
}

func TestRespondOffer_AcceptOK(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOfferCandStore{respond: Offer{ID: uuid.New(), ApplicationID: uuid.New(), Status: OfferAccepted}}
	app := candidateOfferTestApp(store, acct)
	// Accept returns 200; the PeopleSoft push is deferred to onboarding
	// approve-complete and is NOT fired by this handler.
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "accept"}); got != fiber.StatusOK {
		t.Fatalf("accept should be 200, got %d", got)
	}
}

func TestRespondOffer_DeclineOK(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	store := &fakeOfferCandStore{respond: Offer{ID: uuid.New(), ApplicationID: uuid.New(), Status: OfferDeclined}}
	app := candidateOfferTestApp(store, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "decline", Reason: "took another role"}); got != fiber.StatusOK {
		t.Fatalf("decline should be 200, got %d", got)
	}
}

func TestRespondOffer_NegotiateNoCounter(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	app := candidateOfferTestApp(&fakeOfferCandStore{}, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "negotiate"}); got != fiber.StatusBadRequest {
		t.Fatalf("negotiate without a counter amount should be 400, got %d", got)
	}
}

func TestRespondOffer_NegotiateOK(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	counter := 25000.0
	store := &fakeOfferCandStore{negotiate: Offer{ID: uuid.New(), ApplicationID: uuid.New(), Status: OfferNegotiating, CounterSalary: &counter}}
	app := candidateOfferTestApp(store, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "negotiate", CounterSalary: &counter, Note: "ขอเพิ่มค่าเดินทาง"}); got != fiber.StatusOK {
		t.Fatalf("valid negotiate should be 200, got %d", got)
	}
}

func TestRespondOffer_NegotiateRoundsExhausted(t *testing.T) {
	acct := &candidateauth.Account{ID: uuid.New()}
	counter := 25000.0
	store := &fakeOfferCandStore{negotiateErr: ErrNegotiationClosed}
	app := candidateOfferTestApp(store, acct)
	if got := doOffer(t, app, fiber.MethodPost, "/api/v1/public/auth/offers/"+uuid.NewString()+"/respond", OfferResponseInput{Decision: "negotiate", CounterSalary: &counter}); got != fiber.StatusConflict {
		t.Fatalf("exhausted negotiation should be 409, got %d", got)
	}
}
