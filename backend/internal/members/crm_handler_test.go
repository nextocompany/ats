package members

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestNotes_RoleGate(t *testing.T) {
	id := uuid.NewString()
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodGet, "/api/v1/admin/members/"+id+"/notes", ""); code != fiber.StatusForbidden {
		t.Fatalf("list notes hr_staff want 403, got %d", code)
	}
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+id+"/notes", `{"body":"hi"}`); code != fiber.StatusForbidden {
		t.Fatalf("add note hr_staff want 403, got %d", code)
	}
}

func TestAddNote_EmptyBody400(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/notes", `{"body":"   "}`); code != fiber.StatusBadRequest {
		t.Fatalf("blank note should 400, got %d", code)
	}
}

func TestAddNote_Created(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/notes", `{"body":"good candidate"}`); code != fiber.StatusCreated {
		t.Fatalf("add note want 201, got %d", code)
	}
}

func TestAddNote_MissingMember404(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{noteErr: ErrNotFound})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/notes", `{"body":"x"}`); code != fiber.StatusNotFound {
		t.Fatalf("note on missing member want 404, got %d", code)
	}
}

func TestAddTag_NormalizesAndStores(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/tags", `{"tag":"  Retail  "}`); code != fiber.StatusOK {
		t.Fatalf("add tag want 200, got %d", code)
	}
	if repo.lastTag != "retail" {
		t.Fatalf("tag should be trimmed+lowercased to 'retail', got %q", repo.lastTag)
	}
}

func TestAddTag_Empty400(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/"+uuid.NewString()+"/tags", `{"tag":""}`); code != fiber.StatusBadRequest {
		t.Fatalf("empty tag should 400, got %d", code)
	}
}

func TestRemoveTag_RequiresQuery400(t *testing.T) {
	fa := appWithRole("hr_manager", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodDelete, "/api/v1/admin/members/"+uuid.NewString()+"/tags", ""); code != fiber.StatusBadRequest {
		t.Fatalf("remove tag without ?tag should 400, got %d", code)
	}
}

func TestRemoveTag_OK(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	if code := doJSON(t, fa, fiber.MethodDelete, "/api/v1/admin/members/"+uuid.NewString()+"/tags?tag=North", ""); code != fiber.StatusOK {
		t.Fatalf("remove tag want 200, got %d", code)
	}
	if repo.lastTag != "north" {
		t.Fatalf("removed tag should normalize to 'north', got %q", repo.lastTag)
	}
}

func TestBulk_RoleGate(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/bulk", `{"ids":["x"],"action":"suspend"}`); code != fiber.StatusForbidden {
		t.Fatalf("bulk hr_staff want 403, got %d", code)
	}
}

func TestBulk_UnsupportedAction400(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/bulk", `{"ids":["`+uuid.NewString()+`"],"action":"erase"}`); code != fiber.StatusBadRequest {
		t.Fatalf("bulk erase (unsupported) want 400, got %d", code)
	}
}

func TestBulk_EmptyIDs400(t *testing.T) {
	fa := appWithRole("super_admin", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/bulk", `{"ids":[],"action":"suspend"}`); code != fiber.StatusBadRequest {
		t.Fatalf("bulk empty ids want 400, got %d", code)
	}
}

func TestBulk_TagApplied(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("hr_manager", repo)
	id := uuid.NewString()
	if code := doJSON(t, fa, fiber.MethodPost, "/api/v1/admin/members/bulk", `{"ids":["`+id+`"],"action":"tag","value":"VIP"}`); code != fiber.StatusOK {
		t.Fatalf("bulk tag want 200, got %d", code)
	}
	if repo.lastTag != "vip" {
		t.Fatalf("bulk tag should normalize to 'vip', got %q", repo.lastTag)
	}
}

func TestExport_RoleGate(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	if code := doJSON(t, fa, fiber.MethodGet, "/api/v1/admin/members/export.csv", ""); code != fiber.StatusForbidden {
		t.Fatalf("export hr_staff want 403, got %d", code)
	}
}

func TestExport_CSVContent(t *testing.T) {
	m := &Member{ID: uuid.New(), FullName: "สมชาย", Email: "s@x.com", Status: StatusActive, EmailLinked: true, AppsCount: 2}
	fa := appWithRole("hr_manager", &stubRepo{member: m})
	resp, err := fa.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/admin/members/export.csv", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("export want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("want text/csv, got %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "members.csv") {
		t.Fatalf("want attachment members.csv, got %q", cd)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if !strings.HasPrefix(out, "name,email,phone,province,providers,status,applications,joined") {
		t.Fatalf("missing/incorrect CSV header: %q", out)
	}
	if !strings.Contains(out, "สมชาย") || !strings.Contains(out, "email") {
		t.Fatalf("CSV row missing expected data: %q", out)
	}
}

func TestExport_NeutralisesFormulaInjection(t *testing.T) {
	// A phone like "+66..." or a name like "=cmd" must not be emitted as a live
	// spreadsheet formula — csvSafe prefixes a single quote.
	m := &Member{ID: uuid.New(), FullName: "=SUM(A1)", Phone: "+66123", Status: StatusActive}
	fa := appWithRole("super_admin", &stubRepo{member: m})
	resp, err := fa.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/admin/members/export.csv", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if !strings.Contains(out, "'=SUM(A1)") {
		t.Fatalf("formula-like name should be quote-prefixed, got %q", out)
	}
	if !strings.Contains(out, "'+66123") {
		t.Fatalf("formula-like phone should be quote-prefixed, got %q", out)
	}
}
