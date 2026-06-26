//go:build integration

package executive

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

// roiFixture is the seeded world: two branches in two regions, two positions, and
// six applications with controlled created_at/hired_at so every metric has a known
// expected value. See the comment block in setupROI for the exact ledger.
type roiFixture struct {
	pool    *pgxpool.Pool
	svc     *liveService
	p1, p2  uuid.UUID
	northID uuid.UUID
}

func setupROI(t *testing.T) roiFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx,
		`TRUNCATE applications, candidates, positions, stores, vacancies, areas, area_stores, executive_cost_config RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO stores (store_no, store_name, subregion) VALUES (1,'Branch One','North'),(2,'Branch Two','South')`); err != nil {
		t.Fatalf("seed stores: %v", err)
	}
	var northID, southID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO areas (name) VALUES ('North Area') RETURNING id`).Scan(&northID); err != nil {
		t.Fatalf("seed area north: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO areas (name) VALUES ('South Area') RETURNING id`).Scan(&southID); err != nil {
		t.Fatalf("seed area south: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO area_stores (area_id, store_no) VALUES ($1,1),($2,2)`, northID, southID); err != nil {
		t.Fatalf("seed area_stores: %v", err)
	}
	var p1, p2 uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th, title_en) VALUES ('แคชเชียร์','Cashier') RETURNING id`).Scan(&p1); err != nil {
		t.Fatalf("seed position 1: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO positions (title_th, title_en) VALUES ('สต๊อก','Stock') RETURNING id`).Scan(&p2); err != nil {
		t.Fatalf("seed position 2: %v", err)
	}

	now := time.Now().UTC()
	// Ledger (all within the last fortnight, well inside a year/quarter window):
	//   store 1 / P1: A1 hired(TTH 10d, LINE), A2 hired(TTH 4d, LINE),
	//                 A3 offer(picked-up, Google), A4 interview(Google)
	//   store 2 / P2: B1 hired(TTH 6d, Walk-in), B2 scored(picked-up, Walk-in)
	// → hires=3, TTH days {10,4,6} avg 6.7 median 6; funnel applied 6.
	mk := func(source, status string, store int, pos uuid.UUID, createdAgo, hiredAgo int, pickedUp bool) {
		var candID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO candidates (full_name, source_channel, status) VALUES ('c',$1,'available') RETURNING id`,
			source).Scan(&candID); err != nil {
			t.Fatalf("seed candidate: %v", err)
		}
		created := now.AddDate(0, 0, -createdAgo)
		var hiredAt any
		if hiredAgo >= 0 {
			hiredAt = now.AddDate(0, 0, -hiredAgo)
		}
		var picked any
		if pickedUp {
			picked = created
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO applications (candidate_id, position_id, status, assigned_store_id, created_at, hired_at, picked_up_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			candID, pos, status, store, created, hiredAt, picked); err != nil {
			t.Fatalf("seed app: %v", err)
		}
	}
	mk("LINE", "hired", 1, p1, 12, 2, false)       // A1 TTH 10
	mk("LINE", "hired", 1, p1, 5, 1, false)        // A2 TTH 4
	mk("Google", "offer", 1, p1, 3, -1, true)      // A3
	mk("Google", "interview", 1, p1, 3, -1, false) // A4
	mk("Walk-in", "hired", 2, p2, 9, 3, false)     // B1 TTH 6
	mk("Walk-in", "scored", 2, p2, 2, -1, true)    // B2

	return roiFixture{pool: pool, svc: &liveService{pool: pool}, p1: p1, p2: p2, northID: northID}
}

func TestROI_MathAndFunnel(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()

	// Cost UNSET first → ROI cards must be safe-empty, funnel/TTH still computed.
	v, err := f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "branch"})
	if err != nil {
		t.Fatal(err)
	}
	if v.CostConfigured {
		t.Errorf("cost_configured should be false when unset")
	}
	if v.CostPerHire != 0 || v.Savings != 0 || v.ROIPct != 0 {
		t.Errorf("ROI figures must be 0 when cost unset, got cph=%v sav=%v roi=%v", v.CostPerHire, v.Savings, v.ROIPct)
	}
	if v.Hires != 3 {
		t.Fatalf("hires = %d, want 3", v.Hires)
	}
	if v.TimeToHire.AvgDays != 6.7 || v.TimeToHire.MedianDays != 6 {
		t.Errorf("TTH avg/median = %v/%v, want 6.7/6", v.TimeToHire.AvgDays, v.TimeToHire.MedianDays)
	}
	if v.Funnel.Applied != 6 || v.Funnel.Screened != 1 || v.Funnel.Interviewed != 1 || v.Funnel.Offered != 1 || v.Funnel.Hired != 3 {
		t.Errorf("funnel = %+v, want applied6 screened1 interviewed1 offered1 hired3", v.Funnel)
	}
	if v.Funnel.ResponseRate != 33.3 {
		t.Errorf("response rate = %v, want 33.3", v.Funnel.ResponseRate)
	}

	// Now configure cost and assert the math.
	sysMonthly, trad, vacDay, tradTth := 10000.0, 50000.0, 2000.0, 30.0
	if err := setCostConfig(ctx, f.pool, CostConfig{
		Currency:                  "THB",
		SystemCostMonthly:         &sysMonthly,
		TraditionalCostPerHire:    &trad,
		VacancyCostPerDay:         &vacDay,
		TraditionalTimeToHireDays: &tradTth,
	}, "admin@x.com"); err != nil {
		t.Fatal(err)
	}
	v, err = f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "branch"})
	if err != nil {
		t.Fatal(err)
	}
	if !v.CostConfigured {
		t.Fatal("cost_configured should be true after set")
	}
	if v.SystemCostPeriod != 120000 { // 10000 * 12 months
		t.Errorf("system cost period = %v, want 120000", v.SystemCostPeriod)
	}
	if v.CostPerHire != 40000 { // 120000 / 3
		t.Errorf("cost per hire = %v, want 40000", v.CostPerHire)
	}
	if v.Savings != 30000 { // (50000-40000)*3
		t.Errorf("savings = %v, want 30000", v.Savings)
	}
	if v.ROIPct != 25 { // 30000/120000*100
		t.Errorf("roi pct = %v, want 25", v.ROIPct)
	}
	if v.VacancyCostAvoided != 140000 { // (30 - 20/3)*2000*3
		t.Errorf("vacancy cost avoided = %v, want 140000", v.VacancyCostAvoided)
	}
}

func TestROI_SuccessSumsToHeadline(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()
	v, err := f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "branch"})
	if err != nil {
		t.Fatal(err)
	}
	var sum int
	byStore := map[string]SuccessRow{}
	for _, r := range v.Success {
		sum += r.Hires
		byStore[r.Key] = r
	}
	if sum != v.Hires {
		t.Errorf("Σ success hires = %d, want headline %d", sum, v.Hires)
	}
	if got := byStore["1"]; got.Applications != 4 || got.Hires != 2 || got.TopSource == "" {
		t.Errorf("store 1 row = %+v, want apps4 hires2 topSource set", got)
	}
	if got := byStore["2"]; got.Applications != 2 || got.Hires != 1 {
		t.Errorf("store 2 row = %+v, want apps2 hires1", got)
	}
}

func TestROI_DimensionFilterScopes(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()
	// Region = North area (store 1 only) → headline + funnel scope to store 1.
	v, err := f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "branch", Region: f.northID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if v.Hires != 2 {
		t.Errorf("region-scoped hires = %d, want 2 (store 1)", v.Hires)
	}
	if v.Funnel.Applied != 4 {
		t.Errorf("region-scoped applied = %d, want 4", v.Funnel.Applied)
	}

	// Position = P2 → only store 2's two apps.
	v, err = f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "position", Position: f.p2.String()})
	if err != nil {
		t.Fatal(err)
	}
	if v.Hires != 1 || v.Funnel.Applied != 2 {
		t.Errorf("position-scoped hires/applied = %d/%d, want 1/2", v.Hires, v.Funnel.Applied)
	}
}

func TestROI_RegionGrouping(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()
	v, err := f.svc.ROI(ctx, ExecFilters{Period: "year", Dimension: "region"})
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]int{}
	for _, r := range v.Success {
		labels[r.Label] = r.Hires
	}
	if labels["North Area"] != 2 {
		t.Errorf("North Area hires = %d, want 2", labels["North Area"])
	}
	if labels["South Area"] != 1 {
		t.Errorf("South Area hires = %d, want 1", labels["South Area"])
	}
}

func TestCostConfig_RejectsNegative(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()
	neg := -5.0
	err := setCostConfig(ctx, f.pool, CostConfig{SystemCostMonthly: &neg}, "admin@x.com")
	if err != ErrNegativeCost {
		t.Errorf("want ErrNegativeCost, got %v", err)
	}
}

func TestCostConfig_RoundTrip(t *testing.T) {
	f := setupROI(t)
	ctx := context.Background()
	val := 12345.50
	if err := setCostConfig(ctx, f.pool, CostConfig{Currency: "USD", SystemCostMonthly: &val}, "admin@x.com"); err != nil {
		t.Fatal(err)
	}
	c, err := getCostConfig(ctx, f.pool)
	if err != nil {
		t.Fatal(err)
	}
	if c.Currency != "USD" || c.SystemCostMonthly == nil || *c.SystemCostMonthly != 12345.50 {
		t.Errorf("round-trip = %+v", c)
	}
	if c.TraditionalCostPerHire != nil {
		t.Errorf("unset figure should stay nil, got %v", *c.TraditionalCostPerHire)
	}
	if c.UpdatedBy != "admin@x.com" {
		t.Errorf("updated_by = %q, want admin@x.com", c.UpdatedBy)
	}
}
