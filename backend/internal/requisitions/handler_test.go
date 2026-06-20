package requisitions

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
)

// fakeReader seeds an authorizer that grants the requisition permissions to the
// roles the migration would, so rbac.Can works in-process without a DB.
type fakeReader struct{}

func (fakeReader) ListRoles(context.Context) ([]rbac.Role, error) {
	return []rbac.Role{
		{Key: "super_admin", ScopeKind: rbac.KindAll, IsBuiltin: true},
		{Key: "regional_director", ScopeKind: rbac.KindAll, Permissions: []string{rbac.PermRequisitionManage, rbac.PermRequisitionApprove}},
		{Key: "hr_manager", ScopeKind: rbac.KindStore, Permissions: []string{rbac.PermRequisitionManage}},
		{Key: "hr_staff", ScopeKind: rbac.KindStore},
	}, nil
}

func init() {
	a := rbac.NewAuthorizer(fakeReader{}, 0)
	_ = a.Reload(context.Background())
	rbac.SetDefault(a)
}

// stubRepo is a configurable in-memory Repository double.
type stubRepo struct {
	existsInScope bool
	approveErr    error
	created       int
	approved      int
}

func (s *stubRepo) List(context.Context, ListFilter, rbac.Scope) ([]Requisition, int, error) {
	return []Requisition{}, 0, nil
}
func (s *stubRepo) Create(_ context.Context, in CreateInput, _ uuid.UUID) (Requisition, error) {
	s.created++
	return Requisition{ID: uuid.New(), Status: StatusPendingApproval, Headcount: in.Headcount}, nil
}
func (s *stubRepo) Update(_ context.Context, id uuid.UUID, _ UpdateInput) (Requisition, error) {
	return Requisition{ID: id}, nil
}
func (s *stubRepo) Approve(_ context.Context, id uuid.UUID, _ uuid.UUID) (Requisition, error) {
	if s.approveErr != nil {
		return Requisition{}, s.approveErr
	}
	s.approved++
	return Requisition{ID: id, Status: StatusOpen}, nil
}
func (s *stubRepo) Close(_ context.Context, id uuid.UUID) (Requisition, error) {
	return Requisition{ID: id, Status: StatusClosed}, nil
}
func (s *stubRepo) Delete(context.Context, uuid.UUID) error { return nil }
func (s *stubRepo) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return s.existsInScope, nil
}

func appWithRole(role string, repo Repository) *fiber.App {
	fa := fiber.New()
	fa.Use(func(c *fiber.Ctx) error {
		store := 1
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role, StoreID: &store})
		return c.Next()
	})
	RegisterRoutes(fa, NewHandler(repo))
	return fa
}

func do(t *testing.T, fa *fiber.App, method, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := fa.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

func TestList_Gating(t *testing.T) {
	cases := []struct {
		role string
		want int
	}{
		{"hr_staff", fiber.StatusForbidden},
		{"hr_manager", fiber.StatusOK},
		{"regional_director", fiber.StatusOK},
		{"super_admin", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			fa := appWithRole(tc.role, &stubRepo{})
			if got := do(t, fa, fiber.MethodGet, "/api/v1/requisitions/", ""); got != tc.want {
				t.Fatalf("%s list: got %d want %d", tc.role, got, tc.want)
			}
		})
	}
}

func TestCreate_Validation(t *testing.T) {
	pid := uuid.NewString()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"valid", `{"position_id":"` + pid + `","store_id":1,"headcount":2}`, fiber.StatusCreated},
		{"missing position", `{"store_id":1,"headcount":2}`, fiber.StatusBadRequest},
		{"zero headcount", `{"position_id":"` + pid + `","store_id":1,"headcount":0}`, fiber.StatusBadRequest},
		{"no store", `{"position_id":"` + pid + `","store_id":0,"headcount":1}`, fiber.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := appWithRole("hr_manager", &stubRepo{})
			if got := do(t, fa, fiber.MethodPost, "/api/v1/requisitions/", tc.body); got != tc.want {
				t.Fatalf("create %s: got %d want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestCreate_ForbiddenForStaff(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	body := `{"position_id":"` + uuid.NewString() + `","store_id":1,"headcount":1}`
	if got := do(t, fa, fiber.MethodPost, "/api/v1/requisitions/", body); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff create: got %d want 403", got)
	}
}

func TestApprove_RequiresApprovePermission(t *testing.T) {
	id := uuid.NewString()
	// hr_manager has manage but NOT approve → 403.
	fa := appWithRole("hr_manager", &stubRepo{existsInScope: true})
	if got := do(t, fa, fiber.MethodPost, "/api/v1/requisitions/"+id+"/approve", ""); got != fiber.StatusForbidden {
		t.Fatalf("hr_manager approve: got %d want 403", got)
	}
	// regional_director has approve → 200.
	fa2 := appWithRole("regional_director", &stubRepo{existsInScope: true})
	if got := do(t, fa2, fiber.MethodPost, "/api/v1/requisitions/"+id+"/approve", ""); got != fiber.StatusOK {
		t.Fatalf("regional approve: got %d want 200", got)
	}
}

func TestApprove_BadStateIsConflict(t *testing.T) {
	id := uuid.NewString()
	fa := appWithRole("regional_director", &stubRepo{existsInScope: true, approveErr: ErrBadState})
	if got := do(t, fa, fiber.MethodPost, "/api/v1/requisitions/"+id+"/approve", ""); got != fiber.StatusConflict {
		t.Fatalf("approve already-open: got %d want 409", got)
	}
}

func TestApprove_OutOfScopeIsNotFound(t *testing.T) {
	id := uuid.NewString()
	fa := appWithRole("regional_director", &stubRepo{existsInScope: false})
	if got := do(t, fa, fiber.MethodPost, "/api/v1/requisitions/"+id+"/approve", ""); got != fiber.StatusNotFound {
		t.Fatalf("out-of-scope approve: got %d want 404", got)
	}
}

func TestListFilterNormalize(t *testing.T) {
	f := ListFilter{Page: 0, Limit: 0}
	f.normalize()
	if f.Page != 1 || f.Limit != defaultLimit {
		t.Fatalf("normalize defaults: page=%d limit=%d", f.Page, f.Limit)
	}
	f2 := ListFilter{Page: 3, Limit: 9999}
	f2.normalize()
	if f2.Limit != defaultLimit {
		t.Fatalf("over-max limit should reset to default, got %d", f2.Limit)
	}
}
