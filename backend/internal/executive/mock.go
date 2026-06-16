package executive

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// mockService returns deterministic synthetic figures layered over the real
// store and position names from the database. Numbers are derived purely from
// stable seeds (store_no / index) — no math/rand, no time.Now — so the demo is
// identical across refreshes and the unit tests are repeatable. When the
// database is empty or unavailable (nil pool), it falls back to baked CP Axtra
// names so the executive demo never renders empty.
type mockService struct {
	pool *pgxpool.Pool
}

type storeRef struct {
	no        int
	name      string
	subregion string
}

// bakedStores is the fallback branch list used when the DB has no stores.
var bakedStores = []storeRef{
	{1, "Lotus's Rama III", "Bangkok"},
	{2, "Lotus's Bangna", "Bangkok"},
	{3, "Lotus's Pinklao", "Bangkok"},
	{4, "Lotus's Ladprao", "Bangkok"},
	{5, "Lotus's Rangsit", "Central"},
	{6, "Lotus's Chiang Mai", "North"},
	{7, "Lotus's Khon Kaen", "Northeast"},
	{8, "Lotus's Hat Yai", "South"},
	{9, "Lotus's Phuket", "South"},
	{10, "Lotus's Chonburi", "East"},
}

// bakedPositions is the fallback role list used when the DB has no positions.
var bakedPositions = []PipelinePosition{
	{PositionID: "pos-cashier", Title: "Cashier"},
	{PositionID: "pos-sales-associate", Title: "Sales Associate"},
	{PositionID: "pos-stock-controller", Title: "Stock Controller"},
	{PositionID: "pos-store-supervisor", Title: "Store Supervisor"},
	{PositionID: "pos-fresh-food", Title: "Fresh Food Clerk"},
	{PositionID: "pos-customer-service", Title: "Customer Service"},
	{PositionID: "pos-security", Title: "Security Officer"},
	{PositionID: "pos-baker", Title: "Baker"},
}

func (m *mockService) Overview(ctx context.Context) (Overview, error) {
	stores := m.loadStores(ctx)
	positions := m.loadPositions(ctx)

	storeFills := make([]StoreFill, 0, len(stores))
	var totalBudget, totalActual int
	for _, s := range stores {
		budget := 180 + seedSpan(s.no, 220)  // ~180–400 budgeted heads
		fillPct := 60 + seedSpan(s.no*7, 39) // 60–98% filled
		actual := budget * fillPct / 100
		if actual > budget {
			actual = budget
		}
		totalBudget += budget
		totalActual += actual
		storeFills = append(storeFills, StoreFill{
			StoreNo:         s.no,
			StoreName:       s.name,
			Subregion:       s.subregion,
			BudgetHeadcount: budget,
			ActualHeadcount: actual,
			HeadsShort:      budget - actual,
			FillRatePct:     pct(actual, budget),
		})
	}

	// Most short-staffed first (ascending fill-rate; tie-break by larger gap).
	sort.SliceStable(storeFills, func(i, j int) bool {
		if storeFills[i].FillRatePct != storeFills[j].FillRatePct {
			return storeFills[i].FillRatePct < storeFills[j].FillRatePct
		}
		return storeFills[i].HeadsShort > storeFills[j].HeadsShort
	})

	pipeline := make([]PipelinePosition, 0, len(positions))
	for i, p := range positions {
		applied := 120 + i*90
		screening := applied * 30 / 100
		interview := screening * 30 / 100
		offer := interview * 35 / 100
		hired := offer * 70 / 100
		pipeline = append(pipeline, PipelinePosition{
			PositionID: p.PositionID,
			Title:      p.Title,
			Applied:    applied,
			Screening:  screening,
			Interview:  interview,
			Offer:      offer,
			Hired:      hired,
			Openings:   5 + seedSpan(i+1, 20),
		})
	}

	return Overview{
		DataSource: "mock",
		Company: CompanyHeadcount{
			BudgetHeadcount: totalBudget,
			ActualHeadcount: totalActual,
			Vacancy:         totalBudget - totalActual,
			FillRatePct:     pct(totalActual, totalBudget),
			BudgetAvailable: true,
		},
		Stores:   storeFills,
		Pipeline: pipeline,
		Sourcing: mockSourcing(),
	}, nil
}

// loadStores returns real stores when available, else the baked fallback.
func (m *mockService) loadStores(ctx context.Context) []storeRef {
	if m.pool == nil {
		return bakedStores
	}
	rows, err := m.pool.Query(ctx, `
		SELECT store_no, COALESCE(NULLIF(store_name,''), 'Store'), COALESCE(subregion,'')
		FROM stores
		ORDER BY store_no`)
	if err != nil {
		return bakedStores
	}
	defer rows.Close()
	var out []storeRef
	for rows.Next() {
		var s storeRef
		if err := rows.Scan(&s.no, &s.name, &s.subregion); err != nil {
			return bakedStores
		}
		out = append(out, s)
	}
	if rows.Err() != nil || len(out) == 0 {
		return bakedStores
	}
	return out
}

// loadPositions returns up to 8 real active positions, else the baked fallback.
func (m *mockService) loadPositions(ctx context.Context) []PipelinePosition {
	if m.pool == nil {
		return bakedPositions
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id::text, COALESCE(NULLIF(title_en,''), title_th, 'Position')
		FROM positions
		WHERE is_active = TRUE
		ORDER BY created_at
		LIMIT 8`)
	if err != nil {
		return bakedPositions
	}
	defer rows.Close()
	var out []PipelinePosition
	for rows.Next() {
		var p PipelinePosition
		if err := rows.Scan(&p.PositionID, &p.Title); err != nil {
			return bakedPositions
		}
		out = append(out, p)
	}
	if rows.Err() != nil || len(out) == 0 {
		return bakedPositions
	}
	return out
}

// mockSourcing is a deterministic, descending channel breakdown.
func mockSourcing() []Source {
	raw := []struct {
		channel        string
		applied, hired int
	}{
		{"LINE", 1280, 96},
		{"Google", 940, 71},
		{"Walk-in", 610, 58},
		{"Referral", 430, 62},
		{"JobsDB", 360, 28},
		{"Email", 180, 11},
	}
	out := make([]Source, 0, len(raw))
	for _, r := range raw {
		out = append(out, Source{
			Channel:    r.channel,
			Applied:    r.applied,
			Hired:      r.hired,
			Conversion: pct(r.hired, r.applied),
		})
	}
	return out
}

// seedSpan maps a stable seed into [0, span) deterministically.
func seedSpan(seed, span int) int {
	if span <= 0 {
		return 0
	}
	if seed < 0 {
		seed = -seed
	}
	return (seed * 37) % span
}

// pct returns n/d*100 rounded to 1 decimal place (0 when d==0).
func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	v := float64(n) / float64(d) * 100
	return float64(int(v*10+0.5)) / 10
}
