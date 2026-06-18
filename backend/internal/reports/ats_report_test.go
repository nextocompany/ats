package reports

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// fakeATSStore returns a canned report and records the scope it was called with.
type fakeATSStore struct {
	rep       ATSReport
	gotScope  rbac.Scope
	gotFilter ATSFilter
}

func (f *fakeATSStore) ATSReport(_ context.Context, scope rbac.Scope, flt ATSFilter, _ []string) (ATSReport, error) {
	f.gotScope = scope
	f.gotFilter = flt
	return f.rep, nil
}

func atsTestApp(store atsReportStore, u middleware.DevUser) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, u)
		return c.Next()
	})
	RegisterATSRoutes(app, NewATSReportHandler(store, []string{"id_card", "bank_book"}))
	return app
}

func doGet(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func sampleReport() ATSReport {
	return ATSReport{
		From: time.Now().AddDate(0, 0, -30), To: time.Now(),
		Funnel: ATSFunnel{Stages: []ATSFunnelStage{
			{Key: "applied", Count: 100, ConversionPct: 100},
			{Key: "hired", Count: 18, ConversionPct: 18},
		}},
		Timing:     ATSTiming{HiredCount: 18, AvgDaysToHire: 23.4},
		Offers:     ATSOfferOutcomes{Sent: 22, Accepted: 18, AcceptRatePct: 81.8},
		Onboarding: ATSOnboarding{HiredInRange: 18, Completed: 12, CompletionRatePct: 66.7},
		Quality:    ATSQuality{InterviewFeedbackCount: 30, InterviewPassed: 22, InterviewPassRatePct: 73.3},
	}
}

func TestATSReport_RoleGate(t *testing.T) {
	app := atsTestApp(&fakeATSStore{rep: sampleReport()}, middleware.DevUser{Role: "recruiter"})
	if got, _ := doGet(t, app, "/api/v1/reports/ats"); got != fiber.StatusForbidden {
		t.Fatalf("an unlisted role should be 403, got %d", got)
	}
}

func TestATSReport_AllowedJSON(t *testing.T) {
	store := &fakeATSStore{rep: sampleReport()}
	app := atsTestApp(store, middleware.DevUser{Role: "hr_manager"})
	got, body := doGet(t, app, "/api/v1/reports/ats")
	if got != fiber.StatusOK {
		t.Fatalf("hr_manager should be 200, got %d", got)
	}
	var env struct {
		Data ATSReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Timing.HiredCount != 18 || len(env.Data.Funnel.Stages) != 2 {
		t.Fatalf("unexpected report payload: %+v", env.Data)
	}
}

func TestATSReport_ScopeFromUser(t *testing.T) {
	store := &fakeATSStore{rep: sampleReport()}
	sid := 7
	app := atsTestApp(store, middleware.DevUser{Role: "hr_manager", StoreID: &sid})
	if got, _ := doGet(t, app, "/api/v1/reports/ats"); got != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
	if store.gotScope.Kind() != rbac.KindStore || store.gotScope.StoreID == nil || *store.gotScope.StoreID != 7 {
		t.Fatalf("store role should scope to store 7, got %+v", store.gotScope)
	}
}

func TestATSReport_BadDate(t *testing.T) {
	app := atsTestApp(&fakeATSStore{rep: sampleReport()}, middleware.DevUser{Role: "super_admin"})
	if got, _ := doGet(t, app, "/api/v1/reports/ats?from=zzz"); got != fiber.StatusBadRequest {
		t.Fatalf("bad date should be 400, got %d", got)
	}
}

func TestATSReport_InvertedRange(t *testing.T) {
	app := atsTestApp(&fakeATSStore{rep: sampleReport()}, middleware.DevUser{Role: "super_admin"})
	if got, _ := doGet(t, app, "/api/v1/reports/ats?from=2026-02-01&to=2026-01-01"); got != fiber.StatusBadRequest {
		t.Fatalf("to<=from should be 400, got %d", got)
	}
}

func TestATSReport_CSV(t *testing.T) {
	app := atsTestApp(&fakeATSStore{rep: sampleReport()}, middleware.DevUser{Role: "hr_manager"})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/reports/ats.csv", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("csv should be 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("expected text/csv, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "section,metric,value") {
		t.Fatalf("csv missing header, got: %.40q", string(body))
	}
}

// ── pure helpers ────────────────────────────────────────────────────────────

func TestPct(t *testing.T) {
	cases := []struct {
		part, total int
		want        float64
	}{
		{0, 0, 0}, // no divide-by-zero
		{1, 0, 0},
		{18, 22, 81.8},
		{1, 2, 50},
	}
	for _, c := range cases {
		if got := pct(c.part, c.total); got != c.want {
			t.Fatalf("pct(%d,%d) = %v, want %v", c.part, c.total, got, c.want)
		}
	}
}

func TestParseRange(t *testing.T) {
	// Defaults: ~90d window ending now.
	f, err := parseRange("", "")
	if err != nil {
		t.Fatalf("default range err: %v", err)
	}
	if d := f.To.Sub(f.From).Hours() / 24; d < 89 || d > 91 {
		t.Fatalf("default window should be ~90d, got %.1fd", d)
	}
	// Date-only 'to' includes the whole day (advanced by 24h).
	f2, err := parseRange("2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("explicit range err: %v", err)
	}
	if f2.To.Day() != 1 || f2.To.Month() != time.February {
		t.Fatalf("date-only 'to' should advance to next day, got %v", f2.To)
	}
	// Bad + inverted.
	if _, err := parseRange("nope", ""); err == nil {
		t.Fatal("expected error for bad from")
	}
	if _, err := parseRange("2026-02-01", "2026-01-01"); err == nil {
		t.Fatal("expected error for inverted range")
	}
}

func TestEncodeATSCSV(t *testing.T) {
	b, err := EncodeATSCSV(sampleReport())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "section,metric,value") {
		t.Fatal("missing header")
	}
	for _, want := range []string{"funnel:applied,count,100", "offers,sent,22", "timing,hired_count,18"} {
		if !strings.Contains(s, want) {
			t.Fatalf("csv missing row %q", want)
		}
	}
}
