package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// fakeApprovalStore is a minimal approvalStore for handler tests.
type fakeApprovalStore struct {
	inScope      bool
	app          *Application
	req          *ApprovalRequest // returned by GetApprovalRequest{,ByID}
	created      ApprovalRequest
	createErr    error
	decideResult ApprovalRequest
	decideErr    error
	decideArgs   approvalDecideArgs
	queue        []ApprovalQueueItem
}

func (f *fakeApprovalStore) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return f.inScope, nil
}
func (f *fakeApprovalStore) FindByID(context.Context, uuid.UUID) (*Application, error) {
	return f.app, nil
}
func (f *fakeApprovalStore) CreateApprovalRequest(_ context.Context, appID, _ uuid.UUID, _ int) (ApprovalRequest, error) {
	if f.createErr != nil {
		return ApprovalRequest{}, f.createErr
	}
	f.created.ApplicationID = appID
	return f.created, nil
}
func (f *fakeApprovalStore) GetApprovalRequest(context.Context, uuid.UUID) (*ApprovalRequest, error) {
	return f.req, nil
}
func (f *fakeApprovalStore) GetApprovalRequestByID(context.Context, uuid.UUID) (*ApprovalRequest, error) {
	return f.req, nil
}
func (f *fakeApprovalStore) DecideApproval(_ context.Context, a approvalDecideArgs) (ApprovalRequest, error) {
	f.decideArgs = a
	if f.decideErr != nil {
		return ApprovalRequest{}, f.decideErr
	}
	return f.decideResult, nil
}
func (f *fakeApprovalStore) ListPendingApprovals(context.Context, rbac.Scope) ([]ApprovalQueueItem, error) {
	return f.queue, nil
}

func approvalTestApp(store approvalStore, user middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, user)
		return c.Next()
	})
	RegisterApprovalRoutes(app, NewApprovalHandler(store, 48))
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any) (int, []byte) {
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
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

// --- Create -----------------------------------------------------------------

func TestCreateApproval_HappyPath(t *testing.T) {
	store := &fakeApprovalStore{
		inScope: true,
		app:     &Application{Status: StatusInterviewed},
		created: ApprovalRequest{ID: uuid.New(), Status: ApprovalPending, CurrentLevel: 2},
	}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/approval-request", nil)
	if got != fiber.StatusCreated {
		t.Fatalf("hr_staff submit from interviewed should be 201, got %d", got)
	}
}

func TestCreateApproval_WrongStatus(t *testing.T) {
	store := &fakeApprovalStore{inScope: true, app: &Application{Status: StatusShortlisted}}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/approval-request", nil)
	if got != fiber.StatusBadRequest {
		t.Fatalf("submit from shortlisted should be 400, got %d", got)
	}
}

func TestCreateApproval_WrongRole(t *testing.T) {
	store := &fakeApprovalStore{inScope: true, app: &Application{Status: StatusInterviewed}}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/approval-request", nil)
	if got != fiber.StatusForbidden {
		t.Fatalf("hr_manager submit should be 403 (only hr_staff/super_admin), got %d", got)
	}
}

func TestCreateApproval_OutOfScope(t *testing.T) {
	store := &fakeApprovalStore{inScope: false, app: &Application{Status: StatusInterviewed}}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/approval-request", nil)
	if got != fiber.StatusNotFound {
		t.Fatalf("out-of-scope application should be 404, got %d", got)
	}
}

// --- Decide -----------------------------------------------------------------

func pendingReq(level int) *ApprovalRequest {
	return &ApprovalRequest{ID: uuid.New(), ApplicationID: uuid.New(), Status: ApprovalPending, CurrentLevel: level}
}

func TestDecideApproval_WrongLevelRole(t *testing.T) {
	// Active level is 2 (HR Manager); an sgm (level 3) must not decide it.
	store := &fakeApprovalStore{inScope: true, app: &Application{}, req: pendingReq(2)}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "sgm"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusForbidden {
		t.Fatalf("sgm deciding an L2 step should be 403, got %d", got)
	}
}

func TestDecideApproval_RejectNoReason(t *testing.T) {
	store := &fakeApprovalStore{inScope: true, app: &Application{}, req: pendingReq(2)}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionReject})
	if got != fiber.StatusBadRequest {
		t.Fatalf("reject without a reason should be 400, got %d", got)
	}
}

func TestDecideApproval_AlreadyDecided(t *testing.T) {
	decided := pendingReq(2)
	decided.Status = ApprovalApproved
	store := &fakeApprovalStore{inScope: true, app: &Application{}, req: decided}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusConflict {
		t.Fatalf("deciding an already-decided request should be 409, got %d", got)
	}
}

func TestDecideApproval_ApproveAdvances(t *testing.T) {
	store := &fakeApprovalStore{
		inScope:      true,
		app:          &Application{},
		req:          pendingReq(2),
		decideResult: ApprovalRequest{Status: ApprovalPending, CurrentLevel: 3},
	}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusOK {
		t.Fatalf("hr_manager approving L2 should be 200, got %d", got)
	}
	if !store.decideArgs.Approve || store.decideArgs.Level != 2 {
		t.Fatalf("decide args: approve=%v level=%d, want approve=true level=2", store.decideArgs.Approve, store.decideArgs.Level)
	}
}

func TestDecideApproval_FinalApprove(t *testing.T) {
	store := &fakeApprovalStore{
		inScope:      true,
		app:          &Application{},
		req:          pendingReq(4),
		decideResult: ApprovalRequest{Status: ApprovalApproved, CurrentLevel: 4},
	}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "regional_director"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusOK {
		t.Fatalf("regional_director approving L4 should be 200, got %d", got)
	}
}

func TestDecideApproval_Reject(t *testing.T) {
	store := &fakeApprovalStore{
		inScope:      true,
		app:          &Application{},
		req:          pendingReq(3),
		decideResult: ApprovalRequest{Status: ApprovalRejected, CurrentLevel: 3},
	}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "sgm"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionReject, Reason: "headcount frozen"})
	if got != fiber.StatusOK {
		t.Fatalf("sgm rejecting L3 with reason should be 200, got %d", got)
	}
	if store.decideArgs.Approve || store.decideArgs.Reason != "headcount frozen" {
		t.Fatalf("decide args: approve=%v reason=%q, want approve=false reason set", store.decideArgs.Approve, store.decideArgs.Reason)
	}
}

func TestDecideApproval_SuperAdminAnyLevel(t *testing.T) {
	store := &fakeApprovalStore{
		inScope:      true,
		app:          &Application{},
		req:          pendingReq(3),
		decideResult: ApprovalRequest{Status: ApprovalPending, CurrentLevel: 4},
	}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "super_admin"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusOK {
		t.Fatalf("super_admin should decide any level, got %d", got)
	}
}

func TestCreateApproval_ConflictIs409(t *testing.T) {
	// A lost race (SQL guard fired → ErrApprovalConflict) must surface as 409.
	store := &fakeApprovalStore{inScope: true, app: &Application{Status: StatusInterviewed}, createErr: ErrApprovalConflict}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_staff"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/applications/"+uuid.NewString()+"/approval-request", nil)
	if got != fiber.StatusConflict {
		t.Fatalf("create conflict should be 409, got %d", got)
	}
}

func TestDecideApproval_ConflictIs409(t *testing.T) {
	// Level advanced by a concurrent decide between read and tx → ErrApprovalConflict → 409.
	store := &fakeApprovalStore{inScope: true, app: &Application{}, req: pendingReq(2), decideErr: ErrApprovalConflict}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	got, _ := doJSON(t, app, fiber.MethodPost, "/api/v1/approval-requests/"+uuid.NewString()+"/decide",
		ApprovalDecisionInput{Decision: DecisionApprove})
	if got != fiber.StatusConflict {
		t.Fatalf("decide conflict should be 409, got %d", got)
	}
}

// --- Queue ------------------------------------------------------------------

func TestListQueue_LevelFilter(t *testing.T) {
	store := &fakeApprovalStore{queue: []ApprovalQueueItem{
		{RequestID: uuid.New(), ActiveLevel: 2, ActiveRole: "hr_manager"},
		{RequestID: uuid.New(), ActiveLevel: 3, ActiveRole: "sgm"},
		{RequestID: uuid.New(), ActiveLevel: 2, ActiveRole: "hr_manager"},
	}}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "hr_manager"})
	code, body := doJSON(t, app, fiber.MethodGet, "/api/v1/approvals", nil)
	if code != fiber.StatusOK {
		t.Fatalf("queue should be 200, got %d", code)
	}
	var env struct {
		Data []ApprovalQueueItem `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("hr_manager should see only the 2 L2-active items, got %d", len(env.Data))
	}
}

func TestListQueue_SuperAdminSeesAll(t *testing.T) {
	store := &fakeApprovalStore{queue: []ApprovalQueueItem{
		{RequestID: uuid.New(), ActiveLevel: 2},
		{RequestID: uuid.New(), ActiveLevel: 3},
		{RequestID: uuid.New(), ActiveLevel: 4},
	}}
	app := approvalTestApp(store, middleware.DevUser{ID: uuid.NewString(), Role: "super_admin"})
	code, body := doJSON(t, app, fiber.MethodGet, "/api/v1/approvals", nil)
	if code != fiber.StatusOK {
		t.Fatalf("queue should be 200, got %d", code)
	}
	var env struct {
		Data []ApprovalQueueItem `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("super_admin should see all 3 items, got %d", len(env.Data))
	}
}
