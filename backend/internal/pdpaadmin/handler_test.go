package pdpaadmin

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/rbac"
)

// fakeReader grants pdpa.admin to super_admin (bypass) and a "dpo" role; hr_staff
// is denied.
type fakeReader struct{}

func (fakeReader) ListRoles(context.Context) ([]rbac.Role, error) {
	return []rbac.Role{
		{Key: "super_admin", ScopeKind: rbac.KindAll, IsBuiltin: true},
		{Key: "dpo", ScopeKind: rbac.KindAll, Permissions: []string{rbac.PermPDPAAdmin}},
		{Key: "hr_staff", ScopeKind: rbac.KindStore},
	}, nil
}

func init() {
	a := rbac.NewAuthorizer(fakeReader{}, 0)
	_ = a.Reload(context.Background())
	rbac.SetDefault(a)
}

var errStub = errors.New("stub error")

type stubRepo struct {
	resolveErr error
	countsErr  error
	resolved   int
	lastStatus string
}

func (s *stubRepo) ListDSAR(context.Context, DSARListFilter) ([]DSARRequest, int, error) {
	return []DSARRequest{}, 0, nil
}
func (s *stubRepo) ResolveDSAR(_ context.Context, id uuid.UUID, status, _ string, _ *uuid.UUID) (DSARRequest, error) {
	if s.resolveErr != nil {
		return DSARRequest{}, s.resolveErr
	}
	s.resolved++
	s.lastStatus = status
	return DSARRequest{ID: id, Status: status, AccountID: uuid.New()}, nil
}
func (s *stubRepo) LookupConsents(context.Context, *uuid.UUID, *uuid.UUID) ([]ConsentRecord, error) {
	return []ConsentRecord{}, nil
}
func (s *stubRepo) Counts(context.Context) (int, int, int, string, error) {
	if s.countsErr != nil {
		return 0, 0, 0, "", s.countsErr
	}
	return 3, 1, 1, "1.0", nil
}

func appWithRole(role string, repo Repository) *fiber.App {
	fa := fiber.New()
	fa.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
		return c.Next()
	})
	RegisterRoutes(fa, NewHandler(repo, pdpa.DPOContact{Company: "CP Axtra"}, RetentionInfo{Days: 365, Enabled: true}, nil))
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

func TestOverview_Gating(t *testing.T) {
	cases := []struct {
		role string
		want int
	}{
		{"hr_staff", fiber.StatusForbidden},
		{"dpo", fiber.StatusOK},
		{"super_admin", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			fa := appWithRole(tc.role, &stubRepo{})
			if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/overview", ""); got != tc.want {
				t.Fatalf("%s overview: got %d want %d", tc.role, got, tc.want)
			}
		})
	}
}

func TestListDSAR_RejectsInvalidStatus(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/dsar-requests?status=bogus", ""); got != fiber.StatusBadRequest {
		t.Fatalf("bad status filter: got %d want 400", got)
	}
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/dsar-requests?status=pending", ""); got != fiber.StatusOK {
		t.Fatalf("valid status filter: got %d want 200", got)
	}
}

func TestCompleteDSAR(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("dpo", repo)
	if got := do(t, fa, fiber.MethodPost, "/api/v1/pdpa/admin/dsar-requests/"+uuid.NewString()+"/complete", ""); got != fiber.StatusOK {
		t.Fatalf("complete: got %d want 200", got)
	}
	if repo.resolved != 1 || repo.lastStatus != DSARStatusCompleted {
		t.Fatalf("expected 1 complete, got resolved=%d status=%q", repo.resolved, repo.lastStatus)
	}
}

func TestReject_RequiresReason(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("dpo", repo)
	id := uuid.NewString()
	if got := do(t, fa, fiber.MethodPost, "/api/v1/pdpa/admin/dsar-requests/"+id+"/reject", `{}`); got != fiber.StatusBadRequest {
		t.Fatalf("reject without reason: got %d want 400", got)
	}
	if got := do(t, fa, fiber.MethodPost, "/api/v1/pdpa/admin/dsar-requests/"+id+"/reject", `{"reason":"not eligible"}`); got != fiber.StatusOK {
		t.Fatalf("reject with reason: got %d want 200", got)
	}
	if repo.lastStatus != DSARStatusRejected {
		t.Fatalf("expected rejected, got %q", repo.lastStatus)
	}
}

func TestResolve_ConflictWhenNotPending(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{resolveErr: ErrBadState})
	if got := do(t, fa, fiber.MethodPost, "/api/v1/pdpa/admin/dsar-requests/"+uuid.NewString()+"/complete", ""); got != fiber.StatusConflict {
		t.Fatalf("already-resolved: got %d want 409", got)
	}
}

func TestLookupConsents_RequiresSelector(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/consents", ""); got != fiber.StatusBadRequest {
		t.Fatalf("no selector: got %d want 400", got)
	}
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/consents?account_id="+uuid.NewString(), ""); got != fiber.StatusOK {
		t.Fatalf("with account_id: got %d want 200", got)
	}
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/consents?account_id=not-a-uuid", ""); got != fiber.StatusBadRequest {
		t.Fatalf("bad account_id: got %d want 400", got)
	}
}

func TestOverview_RepoErrorIs500(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{countsErr: errStub})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/overview", ""); got != fiber.StatusInternalServerError {
		t.Fatalf("counts error: got %d want 500", got)
	}
}

func TestLookupConsents_ByCandidateID(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/pdpa/admin/consents?candidate_id="+uuid.NewString(), ""); got != fiber.StatusOK {
		t.Fatalf("candidate_id lookup: got %d want 200", got)
	}
}

func TestForbiddenForStaff(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if got := do(t, fa, fiber.MethodPost, "/api/v1/pdpa/admin/dsar-requests/"+uuid.NewString()+"/complete", ""); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff complete: got %d want 403", got)
	}
}
