package peoplesoft

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/scoring"
	"github.com/nexto/hr-ats/internal/vacancies"
	"github.com/nexto/hr-ats/pkg/httpx"
)

type fakeVac struct {
	upserted *vacancies.Vacancy
	closed   string
}

func (f *fakeVac) FindOpen(context.Context, string, uuid.UUID) ([]vacancies.OpenVacancy, error) {
	return nil, nil
}
func (f *fakeVac) CountOpenForPosition(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (f *fakeVac) Upsert(_ context.Context, v vacancies.Vacancy) error {
	f.upserted = &v
	return nil
}
func (f *fakeVac) SetStatusByPSID(_ context.Context, psID, status string) error {
	f.closed = status
	return nil
}

type fakePos struct{ knownCode string }

func (fakePos) FindByID(context.Context, uuid.UUID) (*positions.Position, error) { return nil, nil }
func (f fakePos) FindByPSCode(_ context.Context, code string) (*positions.Position, error) {
	if code == f.knownCode {
		return &positions.Position{ID: uuid.New()}, nil
	}
	return nil, errors.New("not found")
}
func (fakePos) ListPublic(context.Context) ([]positions.PublicPosition, error) { return nil, nil }
func (fakePos) ListAll(context.Context) ([]positions.Position, error)          { return nil, nil }
func (fakePos) UpdateScoreWeights(context.Context, uuid.UUID, scoring.Weights) error {
	return nil
}

type fakeReengage struct {
	called  int
	lastPos uuid.UUID
}

func (f *fakeReengage) OnVacancyOpened(_ context.Context, positionID uuid.UUID) error {
	f.called++
	f.lastPos = positionID
	return nil
}

func testApp(h *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, h, "") // no secret → group open (mock/dev behavior)
	return app
}

func post(t *testing.T, app *fiber.App, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestVacancyOpened_MappedCode(t *testing.T) {
	vac := &fakeVac{}
	h := NewHandler(vac, fakePos{knownCode: "CASHIER"}, nil, "mock", nil)
	app := testApp(h)

	code := post(t, app, "/api/v1/ps/vacancy-opened",
		`{"ps_vacancy_id":"V-1","store_id":1,"position_code":"CASHIER","headcount":2,"opened_date":"2026-06-01"}`)
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if vac.upserted == nil || vac.upserted.PositionID == nil {
		t.Error("expected vacancy upserted with mapped position id")
	}
}

func TestVacancyOpened_FiresReengageWhenMapped(t *testing.T) {
	re := &fakeReengage{}
	h := NewHandler(&fakeVac{}, fakePos{knownCode: "CASHIER"}, nil, "mock", re)
	if code := post(t, testApp(h), "/api/v1/ps/vacancy-opened",
		`{"ps_vacancy_id":"V-9","position_code":"CASHIER","headcount":1}`); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if re.called != 1 {
		t.Fatalf("expected re-engagement fired once for mapped vacancy, got %d", re.called)
	}
}

func TestVacancyOpened_SkipsReengageWhenUnmapped(t *testing.T) {
	re := &fakeReengage{}
	h := NewHandler(&fakeVac{}, fakePos{knownCode: "CASHIER"}, nil, "mock", re)
	if code := post(t, testApp(h), "/api/v1/ps/vacancy-opened",
		`{"ps_vacancy_id":"V-10","position_code":"UNKNOWN","headcount":1}`); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if re.called != 0 {
		t.Fatalf("expected no re-engagement for unmapped vacancy, got %d", re.called)
	}
}

func TestVacancyOpened_UnknownCodeStillStored(t *testing.T) {
	vac := &fakeVac{}
	h := NewHandler(vac, fakePos{knownCode: "CASHIER"}, nil, "mock", nil)
	app := testApp(h)

	code := post(t, app, "/api/v1/ps/vacancy-opened",
		`{"ps_vacancy_id":"V-2","position_code":"UNKNOWN","headcount":1}`)
	if code != fiber.StatusOK {
		t.Fatalf("expected 200 (event not dropped), got %d", code)
	}
	if vac.upserted == nil {
		t.Fatal("expected vacancy stored even when unmapped")
	}
	if vac.upserted.PositionID != nil {
		t.Error("expected nil position id for unmapped code")
	}
}

func TestVacancyOpened_BadPayload(t *testing.T) {
	h := NewHandler(&fakeVac{}, fakePos{}, nil, "mock", nil)
	if code := post(t, testApp(h), "/api/v1/ps/vacancy-opened", `{"headcount":1}`); code != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for missing ps_vacancy_id, got %d", code)
	}
}

func TestVacancyClosed(t *testing.T) {
	vac := &fakeVac{}
	h := NewHandler(vac, fakePos{}, nil, "mock", nil)
	if code := post(t, testApp(h), "/api/v1/ps/vacancy-closed", `{"ps_vacancy_id":"V-1","status":"cancelled"}`); code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if vac.closed != "cancelled" {
		t.Errorf("expected status cancelled, got %q", vac.closed)
	}
}
