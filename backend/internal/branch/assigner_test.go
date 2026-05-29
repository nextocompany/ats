package branch

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/stores"
	"github.com/nexto/hr-ats/internal/vacancies"
)

// fakeVac returns preset vacancies keyed by subregion.
type fakeVac struct {
	bySubregion map[string][]vacancies.OpenVacancy
	total       int
}

func (f fakeVac) FindOpen(_ context.Context, subregion string, _ uuid.UUID) ([]vacancies.OpenVacancy, error) {
	return f.bySubregion[subregion], nil
}
func (f fakeVac) CountOpenForPosition(_ context.Context, _ uuid.UUID) (int, error) {
	return f.total, nil
}
func (fakeVac) Upsert(context.Context, vacancies.Vacancy) error       { return nil }
func (fakeVac) SetStatusByPSID(context.Context, string, string) error { return nil }

func ptrF(v float64) *float64 { return &v }

func TestAssign_NearestStore(t *testing.T) {
	// Two open vacancies in Upper North; เชียงใหม่ centroid ≈ (18.79, 98.99).
	// Store 1 near Chiang Mai, Store 2 in Chiang Rai (farther) → expect store 1.
	vac := fakeVac{bySubregion: map[string][]vacancies.OpenVacancy{
		stores.SubUpperNorth: {
			{StoreNo: 1, Subregion: stores.SubUpperNorth, StoreLat: ptrF(18.80), StoreLng: ptrF(98.97)},
			{StoreNo: 2, Subregion: stores.SubUpperNorth, StoreLat: ptrF(19.91), StoreLng: ptrF(99.84)},
		},
	}}
	a := NewAssigner(vac)

	got, err := a.Assign(context.Background(), "เชียงใหม่", uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.TalentPool {
		t.Fatal("did not expect talent pool")
	}
	if got.StoreNo == nil || *got.StoreNo != 1 {
		t.Errorf("expected nearest store 1, got %v", got.StoreNo)
	}
	if got.Subregion != stores.SubUpperNorth {
		t.Errorf("expected subregion Upper North, got %q", got.Subregion)
	}
}

func TestAssign_NoVacancyTalentPool(t *testing.T) {
	a := NewAssigner(fakeVac{bySubregion: map[string][]vacancies.OpenVacancy{}})
	got, err := a.Assign(context.Background(), "เชียงใหม่", uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TalentPool || got.StoreNo != nil {
		t.Errorf("expected talent pool, got %+v", got)
	}
}

func TestAssign_UnknownProvinceTalentPool(t *testing.T) {
	a := NewAssigner(fakeVac{})
	got, err := a.Assign(context.Background(), "Atlantis", uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TalentPool {
		t.Error("expected talent pool for unknown province")
	}
}

func TestLocationScore(t *testing.T) {
	posID := uuid.New()

	near := NewAssigner(fakeVac{bySubregion: map[string][]vacancies.OpenVacancy{
		stores.SubUpperNorth: {{StoreNo: 1}},
	}})
	if got, _ := near.LocationScore(context.Background(), "เชียงใหม่", posID); got != 20 {
		t.Errorf("expected 20 for in-subregion vacancy, got %d", got)
	}

	elsewhere := NewAssigner(fakeVac{bySubregion: map[string][]vacancies.OpenVacancy{}, total: 3})
	if got, _ := elsewhere.LocationScore(context.Background(), "เชียงใหม่", posID); got != 8 {
		t.Errorf("expected 8 when vacancies exist elsewhere, got %d", got)
	}

	none := NewAssigner(fakeVac{bySubregion: map[string][]vacancies.OpenVacancy{}, total: 0})
	if got, _ := none.LocationScore(context.Background(), "เชียงใหม่", posID); got != 0 {
		t.Errorf("expected 0 with no vacancies, got %d", got)
	}
}
