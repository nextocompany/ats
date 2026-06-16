package executive

import (
	"context"
	"testing"
)

// newMock builds a mock service with a nil pool so tests use the baked
// fallback lists and need no Postgres.
func newMock() *mockService { return &mockService{pool: nil} }

func TestMockOverview_Shape(t *testing.T) {
	ov, err := newMock().Overview(context.Background())
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if ov.DataSource != "mock" {
		t.Fatalf("data_source = %q, want mock", ov.DataSource)
	}
	if len(ov.Stores) == 0 {
		t.Fatal("expected non-empty stores (baked fallback)")
	}
	if len(ov.Pipeline) == 0 {
		t.Fatal("expected non-empty pipeline (baked fallback)")
	}
	if len(ov.Sourcing) == 0 {
		t.Fatal("expected non-empty sourcing")
	}
	if !ov.Company.BudgetAvailable {
		t.Fatal("mock company budget should be available")
	}
}

func TestMockOverview_SortedByFill(t *testing.T) {
	ov, _ := newMock().Overview(context.Background())
	for i := 1; i < len(ov.Stores); i++ {
		if ov.Stores[i-1].FillRatePct > ov.Stores[i].FillRatePct {
			t.Fatalf("stores not sorted asc by fill-rate at %d: %.1f > %.1f",
				i, ov.Stores[i-1].FillRatePct, ov.Stores[i].FillRatePct)
		}
	}
}

func TestCompany_VacancyMath(t *testing.T) {
	ov, _ := newMock().Overview(context.Background())
	if got := ov.Company.Vacancy; got != ov.Company.BudgetHeadcount-ov.Company.ActualHeadcount {
		t.Fatalf("vacancy = %d, want budget-actual = %d", got, ov.Company.BudgetHeadcount-ov.Company.ActualHeadcount)
	}
	for _, s := range ov.Stores {
		if s.HeadsShort != s.BudgetHeadcount-s.ActualHeadcount {
			t.Fatalf("store %d heads_short = %d, want %d", s.StoreNo, s.HeadsShort, s.BudgetHeadcount-s.ActualHeadcount)
		}
		if s.ActualHeadcount > s.BudgetHeadcount {
			t.Fatalf("store %d actual %d exceeds budget %d", s.StoreNo, s.ActualHeadcount, s.BudgetHeadcount)
		}
	}
}

func TestMockOverview_Deterministic(t *testing.T) {
	a, _ := newMock().Overview(context.Background())
	b, _ := newMock().Overview(context.Background())
	if a.Company != b.Company {
		t.Fatalf("company not deterministic: %+v vs %+v", a.Company, b.Company)
	}
	if len(a.Stores) != len(b.Stores) {
		t.Fatalf("store count differs: %d vs %d", len(a.Stores), len(b.Stores))
	}
	for i := range a.Stores {
		if a.Stores[i] != b.Stores[i] {
			t.Fatalf("store %d not deterministic: %+v vs %+v", i, a.Stores[i], b.Stores[i])
		}
	}
}

func TestPct_RoundsToOneDecimal(t *testing.T) {
	cases := []struct {
		n, d int
		want float64
	}{
		{0, 0, 0},
		{1, 3, 33.3},
		{2, 3, 66.7},
		{1, 1, 100},
	}
	for _, c := range cases {
		if got := pct(c.n, c.d); got != c.want {
			t.Errorf("pct(%d,%d) = %v, want %v", c.n, c.d, got, c.want)
		}
	}
}
