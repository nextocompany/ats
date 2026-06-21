package breach

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
)

// fakeReader seeds an authorizer that grants breach.manage to super_admin (via
// the hard bypass) and to a non-builtin role, so rbac.Can works in-process
// without a DB. hr_staff is left without it (the denied case).
type fakeReader struct{}

func (fakeReader) ListRoles(context.Context) ([]rbac.Role, error) {
	return []rbac.Role{
		{Key: "super_admin", ScopeKind: rbac.KindAll, IsBuiltin: true},
		{Key: "dpo", ScopeKind: rbac.KindAll, Permissions: []string{rbac.PermBreachManage}},
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
	getErr     error
	deleteErr  error
	highRisk   bool
	created    int
	resolved   int
	notified   int
	notifiedTo int
	deleted    int
	last       Breach
}

func (s *stubRepo) List(context.Context, ListFilter) ([]Breach, int, error) {
	return []Breach{}, 0, nil
}
func (s *stubRepo) Create(_ context.Context, in CreateInput, _ uuid.UUID) (Breach, error) {
	s.created++
	s.last = Breach{ID: uuid.New(), Title: in.Title, Severity: in.Severity, Status: StatusOpen, DiscoveredAt: in.DiscoveredAt}
	return s.last, nil
}
func (s *stubRepo) GetByID(_ context.Context, id uuid.UUID) (Breach, error) {
	if s.getErr != nil {
		return Breach{}, s.getErr
	}
	return Breach{ID: id, Title: "x", Description: "y", Severity: SeverityHigh, Status: StatusOpen, HighRisk: s.highRisk, DiscoveredAt: time.Now()}, nil
}
func (s *stubRepo) Update(_ context.Context, id uuid.UUID, _ UpdateInput) (Breach, error) {
	if s.getErr != nil {
		return Breach{}, s.getErr
	}
	return Breach{ID: id, Status: StatusOpen}, nil
}
func (s *stubRepo) MarkPDPCNotified(_ context.Context, id uuid.UUID) (Breach, error) {
	if s.getErr != nil {
		return Breach{}, s.getErr
	}
	s.notified++
	n := time.Now()
	return Breach{ID: id, PDPCNotifiedAt: &n}, nil
}
func (s *stubRepo) MarkSubjectsNotified(_ context.Context, id uuid.UUID) (Breach, error) {
	if s.getErr != nil {
		return Breach{}, s.getErr
	}
	s.notifiedTo++
	return Breach{ID: id}, nil
}
func (s *stubRepo) Resolve(_ context.Context, id uuid.UUID, _ uuid.UUID) (Breach, error) {
	if s.getErr != nil {
		return Breach{}, s.getErr
	}
	s.resolved++
	return Breach{ID: id, Status: StatusResolved}, nil
}
func (s *stubRepo) Delete(context.Context, uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted++
	return nil
}

func appWithRole(role string, repo Repository) *fiber.App {
	fa := fiber.New()
	fa.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
		return c.Next()
	})
	RegisterRoutes(fa, NewHandler(repo, DPOContact{Company: "CP Axtra"}))
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
		{"dpo", fiber.StatusOK},
		{"super_admin", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			fa := appWithRole(tc.role, &stubRepo{})
			if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/", ""); got != tc.want {
				t.Fatalf("%s list: got %d want %d", tc.role, got, tc.want)
			}
		})
	}
}

func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"valid", `{"title":"laptop lost","description":"a laptop with CVs","severity":"high"}`, fiber.StatusCreated},
		{"valid defaults severity", `{"title":"x","description":"y"}`, fiber.StatusCreated},
		{"missing title", `{"description":"y"}`, fiber.StatusBadRequest},
		{"missing description", `{"title":"x"}`, fiber.StatusBadRequest},
		{"bad severity", `{"title":"x","description":"y","severity":"meh"}`, fiber.StatusBadRequest},
		{"bad discovered_at", `{"title":"x","description":"y","discovered_at":"not-a-date"}`, fiber.StatusBadRequest},
		{"negative affected", `{"title":"x","description":"y","affected_subjects":-3}`, fiber.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := appWithRole("dpo", &stubRepo{})
			if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/", tc.body); got != tc.want {
				t.Fatalf("create %s: got %d want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestCreate_ForbiddenForStaff(t *testing.T) {
	fa := appWithRole("hr_staff", &stubRepo{})
	body := `{"title":"x","description":"y"}`
	if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/", body); got != fiber.StatusForbidden {
		t.Fatalf("hr_staff create: got %d want 403", got)
	}
}

func TestNotifyAndResolve(t *testing.T) {
	id := uuid.NewString()
	repo := &stubRepo{}
	fa := appWithRole("dpo", repo)
	if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/"+id+"/notify-pdpc", ""); got != fiber.StatusOK {
		t.Fatalf("notify-pdpc: got %d want 200", got)
	}
	if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/"+id+"/resolve", ""); got != fiber.StatusOK {
		t.Fatalf("resolve: got %d want 200", got)
	}
	if repo.notified != 1 || repo.resolved != 1 {
		t.Fatalf("expected 1 notify + 1 resolve, got notify=%d resolve=%d", repo.notified, repo.resolved)
	}
}

func TestList_RejectsInvalidFilter(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/?status=hacked", ""); got != fiber.StatusBadRequest {
		t.Fatalf("bad status filter: got %d want 400", got)
	}
	if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/?severity=nope", ""); got != fiber.StatusBadRequest {
		t.Fatalf("bad severity filter: got %d want 400", got)
	}
}

func TestCreate_RejectsFutureDiscoveredAt(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	body := `{"title":"x","description":"y","discovered_at":"2099-01-01T00:00:00Z"}`
	if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/", body); got != fiber.StatusBadRequest {
		t.Fatalf("future discovered_at: got %d want 400", got)
	}
}

func TestUpdate_RejectsResolveViaPatch(t *testing.T) {
	repo := &stubRepo{}
	fa := appWithRole("dpo", repo)
	body := `{"status":"resolved"}`
	if got := do(t, fa, fiber.MethodPatch, "/api/v1/breaches/"+uuid.NewString(), body); got != fiber.StatusBadRequest {
		t.Fatalf("PATCH status=resolved: got %d want 400 (must use /resolve)", got)
	}
	// A legal transition (contained) is accepted.
	if got := do(t, fa, fiber.MethodPatch, "/api/v1/breaches/"+uuid.NewString(), `{"status":"contained"}`); got != fiber.StatusOK {
		t.Fatalf("PATCH status=contained: got %d want 200", got)
	}
}

func TestNotifySubjects_GuardsHighRisk(t *testing.T) {
	id := uuid.NewString()
	// Low-risk breach → 409, and the stamp is never written.
	low := &stubRepo{highRisk: false}
	fa := appWithRole("dpo", low)
	if got := do(t, fa, fiber.MethodPost, "/api/v1/breaches/"+id+"/notify-subjects", ""); got != fiber.StatusConflict {
		t.Fatalf("notify-subjects low-risk: got %d want 409", got)
	}
	if low.notifiedTo != 0 {
		t.Fatalf("low-risk breach must not be stamped, got %d", low.notifiedTo)
	}
	// High-risk breach → stamped.
	high := &stubRepo{highRisk: true}
	fa2 := appWithRole("dpo", high)
	if got := do(t, fa2, fiber.MethodPost, "/api/v1/breaches/"+id+"/notify-subjects", ""); got != fiber.StatusOK {
		t.Fatalf("notify-subjects high-risk: got %d want 200", got)
	}
	if high.notifiedTo != 1 {
		t.Fatalf("high-risk breach should be stamped once, got %d", high.notifiedTo)
	}
}

func TestDelete(t *testing.T) {
	id := uuid.NewString()
	// Un-notified draft → deletable.
	ok := &stubRepo{}
	fa := appWithRole("dpo", ok)
	if got := do(t, fa, fiber.MethodDelete, "/api/v1/breaches/"+id, ""); got != fiber.StatusOK {
		t.Fatalf("delete draft: got %d want 200", got)
	}
	if ok.deleted != 1 {
		t.Fatalf("expected 1 delete, got %d", ok.deleted)
	}
	// Already-notified (protected) → 409 via ErrBadState.
	protected := &stubRepo{deleteErr: ErrBadState}
	fa2 := appWithRole("dpo", protected)
	if got := do(t, fa2, fiber.MethodDelete, "/api/v1/breaches/"+id, ""); got != fiber.StatusConflict {
		t.Fatalf("delete notified: got %d want 409", got)
	}
}

func TestGet_NotFound(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{getErr: ErrNotFound})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/"+uuid.NewString(), ""); got != fiber.StatusNotFound {
		t.Fatalf("get missing: got %d want 404", got)
	}
}

func TestNotification_Endpoint(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/"+uuid.NewString()+"/notification", ""); got != fiber.StatusOK {
		t.Fatalf("notification: got %d want 200", got)
	}
}

func TestBadIDIsBadRequest(t *testing.T) {
	fa := appWithRole("dpo", &stubRepo{})
	if got := do(t, fa, fiber.MethodGet, "/api/v1/breaches/not-a-uuid", ""); got != fiber.StatusBadRequest {
		t.Fatalf("bad id: got %d want 400", got)
	}
}

// ── pure-logic tests: the 72h deadline + notification content ────────────────

func TestComputeDeadline(t *testing.T) {
	discovered := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	due := discovered.Add(NotificationWindow)

	t.Run("within window", func(t *testing.T) {
		now := discovered.Add(20 * time.Hour)
		d := computeDeadline(discovered, nil, now)
		if d.Overdue {
			t.Fatalf("should not be overdue at +20h")
		}
		if d.HoursRemaining != 52 { // 72 - 20
			t.Fatalf("hours remaining: got %d want 52", d.HoursRemaining)
		}
		if !d.DueAt.Equal(due) {
			t.Fatalf("due at: got %v want %v", d.DueAt, due)
		}
	})

	t.Run("overdue", func(t *testing.T) {
		now := discovered.Add(80 * time.Hour)
		d := computeDeadline(discovered, nil, now)
		if !d.Overdue {
			t.Fatalf("should be overdue at +80h")
		}
		if d.HoursRemaining != -8 { // floored: 8h past the 72h deadline
			t.Fatalf("hours remaining: got %d want -8", d.HoursRemaining)
		}
	})

	t.Run("barely overdue floors to negative", func(t *testing.T) {
		// 30 minutes past the deadline must report -1, not a contradictory 0.
		now := discovered.Add(NotificationWindow + 30*time.Minute)
		d := computeDeadline(discovered, nil, now)
		if !d.Overdue {
			t.Fatalf("should be overdue 30m past deadline")
		}
		if d.HoursRemaining != -1 {
			t.Fatalf("hours remaining: got %d want -1 (floored)", d.HoursRemaining)
		}
	})

	t.Run("notified discharges obligation", func(t *testing.T) {
		notified := discovered.Add(10 * time.Hour)
		now := discovered.Add(200 * time.Hour) // long past the deadline
		d := computeDeadline(discovered, &notified, now)
		if d.Overdue {
			t.Fatalf("notified breach must never be overdue")
		}
		if !d.Notified {
			t.Fatalf("Notified should be true")
		}
		if d.HoursRemaining != 0 {
			t.Fatalf("notified breach should report 0 remaining, got %d", d.HoursRemaining)
		}
	})
}

func TestGenerateNotification(t *testing.T) {
	discovered := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	b := Breach{
		ID:               uuid.New(),
		Title:            "Lost laptop",
		Description:      "An unencrypted laptop with candidate CVs was lost.",
		Severity:         SeverityHigh,
		Status:           StatusOpen,
		AffectedSubjects: 42,
		DataCategories:   "name, email, resume",
		DiscoveredAt:     discovered,
		HighRisk:         true,
	}
	now := discovered.Add(time.Hour)

	withDPO := GenerateNotification(b, DPOContact{Company: "CP Axtra", DPOName: "Jane", DPOEmail: "dpo@cpaxtra.test", DPOPhone: "02-000-0000"}, now)
	if !strings.Contains(withDPO.Subject, "Lost laptop") {
		t.Fatalf("subject missing title: %q", withDPO.Subject)
	}
	for _, want := range []string{"CP Axtra", "Jane", "dpo@cpaxtra.test", "42", "name, email, resume", "37(4)"} {
		if !strings.Contains(withDPO.Body, want) {
			t.Fatalf("body missing %q\n%s", want, withDPO.Body)
		}
	}

	// Missing DPO fields → visible placeholders, never silently blank.
	noDPO := GenerateNotification(b, DPOContact{Company: "CP Axtra"}, now)
	if !strings.Contains(noDPO.Body, "DPO email") {
		t.Fatalf("expected DPO email placeholder in body:\n%s", noDPO.Body)
	}
}
