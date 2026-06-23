package members

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// stubRepo is an in-memory Repository double. The lifecycle methods record the
// last call so handler tests can assert routing/role-gating without a database.
type stubRepo struct {
	member    *Member
	resumeURL string
	resumeErr error

	statusErr    error
	anonymizeErr error
	updateErr    error
	noteErr      error
	tagErr       error
	notes        []Note
	tags         []string
	lastStatus   string    // status passed to SetStatus
	lastAnonID   uuid.UUID // id passed to Anonymize
	lastTag      string    // tag passed to AddTag/RemoveTag
	forceLogouts int       // ForceLogout call count
	resolveFrom  uuid.UUID // a candidate id that ResolveAccountID maps to member.ID
}

func (s *stubRepo) List(context.Context, ListFilter, rbac.Scope) ([]Member, int, error) {
	if s.member == nil {
		return nil, 0, nil
	}
	return []Member{*s.member}, 1, nil
}
func (s *stubRepo) GetByID(_ context.Context, id uuid.UUID) (*Member, error) {
	if s.member != nil && s.member.ID == id {
		return s.member, nil
	}
	return nil, ErrNotFound
}
func (s *stubRepo) GetScopedByID(_ context.Context, id uuid.UUID, _ rbac.Scope) (*Member, error) {
	if s.member != nil && s.member.ID == id {
		return s.member, nil
	}
	return nil, ErrNotFound
}
func (s *stubRepo) ResolveAccountID(_ context.Context, candidateID uuid.UUID) (uuid.UUID, error) {
	// When resolveFrom matches the looked-up id, map it to the member's account id
	// (simulates a per-intake candidate id linking to its owning account).
	if s.member != nil && s.resolveFrom != uuid.Nil && s.resolveFrom == candidateID {
		return s.member.ID, nil
	}
	return uuid.Nil, ErrNotFound
}
func (s *stubRepo) ListApplicationsByAccount(context.Context, uuid.UUID) ([]AccountApplication, error) {
	return nil, nil
}
func (s *stubRepo) GetResumeBlobURL(context.Context, uuid.UUID) (string, error) {
	return s.resumeURL, s.resumeErr
}
func (s *stubRepo) Stats(context.Context, rbac.Scope) (Stats, error) { return Stats{}, nil }

func (s *stubRepo) SetStatus(_ context.Context, _ uuid.UUID, status string, _ *uuid.UUID) error {
	s.lastStatus = status
	return s.statusErr
}
func (s *stubRepo) ForceLogout(context.Context, uuid.UUID) error {
	s.forceLogouts++
	return nil
}
func (s *stubRepo) UpdateProfile(context.Context, uuid.UUID, ProfileUpdate) error { return s.updateErr }
func (s *stubRepo) Anonymize(_ context.Context, id uuid.UUID) (string, error) {
	s.lastAnonID = id
	return s.resumeURL, s.anonymizeErr
}
func (s *stubRepo) ListForExport(context.Context, ListFilter, rbac.Scope, int) ([]Member, error) {
	if s.member == nil {
		return nil, nil
	}
	return []Member{*s.member}, nil
}
func (s *stubRepo) AddNote(_ context.Context, _ uuid.UUID, author, body string) (*Note, error) {
	if s.noteErr != nil {
		return nil, s.noteErr
	}
	return &Note{AuthorEmail: author, Body: body}, nil
}
func (s *stubRepo) ListNotes(context.Context, uuid.UUID) ([]Note, error) { return s.notes, nil }
func (s *stubRepo) AddTag(_ context.Context, _ uuid.UUID, tag string) error {
	s.lastTag = tag
	return s.tagErr
}
func (s *stubRepo) RemoveTag(_ context.Context, _ uuid.UUID, tag string) error {
	s.lastTag = tag
	return nil
}
func (s *stubRepo) ListTags(context.Context, uuid.UUID) ([]string, error) { return s.tags, nil }

type stubSigner struct{}

func (stubSigner) SignedURLForStored(string, time.Duration) (string, error) { return "https://x", nil }

// stubDeleter records blob deletions so anonymize tests can assert the resume was
// removed (or, with delErr set, that a delete failure doesn't fail the response).
type stubDeleter struct {
	deleted []string
	delErr  error
}

func (d *stubDeleter) Delete(_ context.Context, name string) error {
	d.deleted = append(d.deleted, name)
	return d.delErr
}
func (d *stubDeleter) DeleteStored(_ context.Context, url string) error {
	d.deleted = append(d.deleted, url)
	return d.delErr
}

// appWithRole builds a Fiber app whose middleware injects a DevUser of the given
// role, then mounts the member routes over the stub repo (nil blob deleter).
func appWithRole(role string, repo Repository) *fiber.App {
	return appWithRoleBlob(role, repo, nil)
}

// appWithRoleBlob is appWithRole with an explicit blob deleter for anonymize tests.
func appWithRoleBlob(role string, repo Repository, blob blobDeleter) *fiber.App {
	fa := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	fa.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
		return c.Next()
	})
	RegisterDashboardRoutes(fa, NewHandler(repo, nil, stubSigner{}, blob))
	return fa
}

func status(t *testing.T, fa *fiber.App, method, path string) int {
	t.Helper()
	resp, err := fa.Test(httptest.NewRequest(method, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// The unified person list is open to any authenticated role (scoped server-side),
// so a store role reads it — it is no longer admin-only.
func TestList_AllowedForNonAdminRole(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members"); code != fiber.StatusOK {
		t.Fatalf("hr_staff should read the scoped list (200), got %d", code)
	}
}

// With no auth context at all the read fails closed (401).
func TestList_UnauthenticatedRejected(t *testing.T) {
	fa := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterDashboardRoutes(fa, NewHandler(&stubRepo{}, nil, stubSigner{}, nil))
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members"); code != fiber.StatusUnauthorized {
		t.Fatalf("unauthenticated list should 401, got %d", code)
	}
}

func TestList_AllowedForHRManager(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members"); code != fiber.StatusOK {
		t.Fatalf("hr_manager should get 200, got %d", code)
	}
}

func TestList_AllowedForSuperAdmin(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members"); code != fiber.StatusOK {
		t.Fatalf("super_admin should get 200, got %d", code)
	}
}

func TestDetail_BadID(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/not-a-uuid"); code != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestDetail_NotFound(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+uuid.NewString()); code != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestDetail_OK(t *testing.T) {
	id := uuid.New()
	fa := appWithRole("super_admin", &stubRepo{member: &Member{ID: id, FullName: "สมชาย", Status: StatusActive}})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+id.String()); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

// Stats is relaxed alongside the list (scoped aggregate, no PII), so a store role
// reads it too.
// A per-intake candidate id (e.g. a search/inbox link) resolves to its owning
// account and returns that person's detail.
func TestDetail_ResolvesCandidateIDToAccount(t *testing.T) {
	acct := uuid.New()
	cand := uuid.New()
	fa := appWithRole("super_admin", &stubRepo{
		member:      &Member{ID: acct, FullName: "สมหญิง", Status: StatusActive},
		resolveFrom: cand,
	})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+cand.String()); code != fiber.StatusOK {
		t.Fatalf("candidate id should resolve to account detail (200), got %d", code)
	}
}

func TestStats_AllowedForNonAdmin(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/stats"); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestList_InvalidStatusFilter400(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members?status=banana"); code != fiber.StatusBadRequest {
		t.Fatalf("invalid status should 400, got %d", code)
	}
}

func TestList_InvalidFromDate400(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members?from=2024-06-14"); code != fiber.StatusBadRequest {
		t.Fatalf("date-only 'from' should 400, got %d", code)
	}
}

func TestResume_Forbidden(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+uuid.NewString()+"/resume"); code != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestResume_BadID(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/not-a-uuid/resume"); code != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestResume_NoResumeOnFile404(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{resumeURL: ""}) // empty url, no error
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+uuid.NewString()+"/resume"); code != fiber.StatusNotFound {
		t.Fatalf("no resume should 404, got %d", code)
	}
}

func TestResume_OK(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{resumeURL: "blob/r.pdf"})
	if code := status(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+uuid.NewString()+"/resume"); code != fiber.StatusOK {
		t.Fatalf("expected 200 signed url, got %d", code)
	}
}
