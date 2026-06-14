package members

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// doJSON sends a request with an optional JSON body and returns the status code.
func doJSON(t *testing.T, fa *fiber.App, method, path, body string) int {
	t.Helper()
	var r = httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	resp, err := fa.Test(r)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestSetStatus_ForbiddenForNonAdmin(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString()+"/status", `{"status":"suspended"}`)
	if code != fiber.StatusForbidden {
		t.Fatalf("hr_staff should get 403, got %d", code)
	}
}

func TestSetStatus_AllowedForHRManager(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString()+"/status", `{"status":"suspended"}`)
	if code != fiber.StatusOK {
		t.Fatalf("hr_manager should get 200, got %d", code)
	}
	if repo.lastStatus != StatusSuspended {
		t.Fatalf("expected repo to receive 'suspended', got %q", repo.lastStatus)
	}
}

func TestSetStatus_RejectsAnonymizedTarget(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	// 'anonymized' must not be settable via the status route (erase has its own route).
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString()+"/status", `{"status":"anonymized"}`)
	if code != fiber.StatusBadRequest {
		t.Fatalf("anonymized via status route should 400, got %d", code)
	}
}

func TestSetStatus_NotFound(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{statusErr: ErrNotFound})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString()+"/status", `{"status":"active"}`)
	if code != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestSetStatus_Anonymized409(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{statusErr: ErrAnonymized})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString()+"/status", `{"status":"suspended"}`)
	if code != fiber.StatusConflict {
		t.Fatalf("anonymized member should 409, got %d", code)
	}
}

func TestForceLogout_Forbidden(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/force-logout", "")
	if code != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestForceLogout_OK(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/force-logout", "")
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if repo.forceLogouts != 1 {
		t.Fatalf("expected 1 force-logout, got %d", repo.forceLogouts)
	}
}

func TestUpdateProfile_EmptyBody400(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+uuid.NewString(), `{}`)
	if code != fiber.StatusBadRequest {
		t.Fatalf("empty update should 400, got %d", code)
	}
}

func TestUpdateProfile_OK(t *testing.T) {
	id := uuid.New()
	fa := appWithRole("super_admin", &stubRepo{member: &Member{ID: id, Status: StatusActive}})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+id.String(), `{"phone":"0810000009"}`)
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestUpdateProfile_EmailTaken409(t *testing.T) {
	id := uuid.New()
	fa := appWithRole("super_admin", &stubRepo{member: &Member{ID: id, Status: StatusActive}, updateErr: ErrEmailTaken})
	code := doJSON(t, fa, fiber.MethodPatch, "/api/v1/admin/members/"+id.String(), `{"email":"taken@example.com"}`)
	if code != fiber.StatusConflict {
		t.Fatalf("duplicate email should 409, got %d", code)
	}
}

// TestAnonymize_ForbiddenForHRManager is the critical erase role gate: hr_manager
// can suspend/edit but must NOT be able to run the irreversible PDPA erasure.
func TestAnonymize_ForbiddenForHRManager(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{})
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/anonymize", "")
	if code != fiber.StatusForbidden {
		t.Fatalf("hr_manager must be 403 on anonymize, got %d", code)
	}
}

func TestAnonymize_OKForSuperAdmin_DeletesBlob(t *testing.T) {
	id := uuid.New()
	del := &stubDeleter{}
	fa := appWithRoleBlob("super_admin", &stubRepo{resumeURL: "accounts/x/r.pdf"}, del)
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+id.String()+"/anonymize", "")
	if code != fiber.StatusOK {
		t.Fatalf("super_admin anonymize should 200, got %d", code)
	}
	if len(del.deleted) != 1 || del.deleted[0] != "accounts/x/r.pdf" {
		t.Fatalf("expected resume blob deleted by key, got %v", del.deleted)
	}
}

func TestAnonymize_BlobDeleteFailureStillSucceeds(t *testing.T) {
	del := &stubDeleter{delErr: errExpected}
	fa := appWithRoleBlob("super_admin", &stubRepo{resumeURL: "accounts/x/r.pdf"}, del)
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/anonymize", "")
	if code != fiber.StatusOK {
		t.Fatalf("blob delete failure must not fail the response, got %d", code)
	}
}

func TestAnonymize_Conflict(t *testing.T) {
	fa := appWithRoleBlob("super_admin", &stubRepo{anonymizeErr: ErrAnonymized}, &stubDeleter{})
	code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/anonymize", "")
	if code != fiber.StatusConflict {
		t.Fatalf("already-anonymized should 409, got %d", code)
	}
}

// errExpected is a throwaway error for failure-path assertions.
var errExpected = &stubError{"boom"}

type stubError struct{ s string }

func (e *stubError) Error() string { return e.s }
