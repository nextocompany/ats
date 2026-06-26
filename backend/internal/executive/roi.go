package executive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// roi.go implements the live Recruitment ROI & Performance aggregations plus the
// admin cost-config read/write. ROI is derived purely from ATS data + the
// admin-configured CostConfig; no finance/HRIS source is involved.
//
// Period semantics (deliberate, per metric):
//   - rolling lookback window: month=1, quarter=3, year=12 months back from now.
//   - resume volume / funnel / response-rate count over the created_at window.
//   - headline hires + time-to-hire + success-row hires count over the hired_at
//     window, so the success table sums to the headline (Σ success.hires == Hires).
//   - system_cost_period = system_cost_monthly * months_in_period (exact, which is
//     why a rolling N-month window beats a calendar-to-date one for the ROI ratio).

const successRowLimit = 20

// monthsInPeriod maps the period selector to a rolling lookback length.
func monthsInPeriod(period string) int {
	switch period {
	case "quarter":
		return 3
	case "year":
		return 12
	default: // "month" and any unknown value
		return 1
	}
}

// normalizeDimension constrains the success-table grouping to a known value.
func normalizeDimension(dim string) string {
	switch dim {
	case "region", "position":
		return dim
	default:
		return "branch"
	}
}

// sqlBuilder accumulates positional WHERE conditions and their arguments so each
// metric query can share the same dimension predicate without placeholder drift.
type sqlBuilder struct {
	conds []string
	args  []any
}

// add appends a condition fragment (a single %d for the next placeholder) + value.
func (b *sqlBuilder) add(frag string, val any) {
	b.args = append(b.args, val)
	b.conds = append(b.conds, fmt.Sprintf(frag, len(b.args)))
}

// and returns the accumulated conditions as a trailing " AND ..." clause.
func (b *sqlBuilder) and() string {
	if len(b.conds) == 0 {
		return ""
	}
	return " AND " + strings.Join(b.conds, " AND ")
}

// dimFilter appends the optional branch/region/position scoping that applies to
// every metric. Region scopes via area_stores; branch via assigned_store_id
// (placement, mirroring live.go), not store_id (applied-to).
func dimFilter(b *sqlBuilder, f ExecFilters) {
	if f.Store != nil {
		b.add("a.assigned_store_id = $%d", *f.Store)
	}
	if f.Position != "" {
		b.add("a.position_id = $%d::uuid", f.Position)
	}
	if f.Region != "" {
		b.add("a.assigned_store_id IN (SELECT store_no FROM area_stores WHERE area_id = $%d::uuid)", f.Region)
	}
}

// ROI computes the full Recruitment ROI & Performance payload (live).
func (l *liveService) ROI(ctx context.Context, f ExecFilters) (ROIView, error) {
	f.Dimension = normalizeDimension(f.Dimension)
	months := monthsInPeriod(f.Period)
	since := time.Now().UTC().AddDate(0, -months, 0)

	cost, err := getCostConfig(ctx, l.pool)
	if err != nil {
		return ROIView{}, err
	}

	hires, avgDays, medianDays, err := l.hireMetrics(ctx, f, since)
	if err != nil {
		return ROIView{}, err
	}
	funnel, err := l.funnel(ctx, f, since)
	if err != nil {
		return ROIView{}, err
	}
	success, err := l.success(ctx, f, since)
	if err != nil {
		return ROIView{}, err
	}

	view := ROIView{
		DataSource: "live",
		Period:     f.Period,
		Dimension:  f.Dimension,
		Cost:       cost,
		Hires:      hires,
		Funnel:     funnel,
		TimeToHire: TimeToHire{Hires: hires, AvgDays: round1(avgDays), MedianDays: round1(medianDays)},
		Success:    success,
	}
	applyROIMath(&view, cost, hires, avgDays, months)
	return view, nil
}

// applyROIMath derives the cost-driven figures with division guards. It is shared
// with the mock service so both paths produce identical math from the same inputs.
func applyROIMath(view *ROIView, cost CostConfig, hires int, avgDays float64, months int) {
	view.CostConfigured = cost.configured()
	if cost.SystemCostMonthly != nil {
		view.SystemCostPeriod = round2(*cost.SystemCostMonthly * float64(months))
	}
	if cost.configured() && hires > 0 {
		view.CostPerHire = round2(view.SystemCostPeriod / float64(hires))
		view.Savings = round2((*cost.TraditionalCostPerHire - view.CostPerHire) * float64(hires))
		if view.SystemCostPeriod > 0 {
			view.ROIPct = round1(view.Savings / view.SystemCostPeriod * 100)
		}
	}
	// Vacancy cost avoided = the carrying cost spared by filling roles faster than
	// the configured traditional baseline: rate/day * days_saved * hires. Requires
	// both vacancy rate AND a traditional TTH baseline; degrades to 0 when unset.
	// (DEVIATION: the plan named this card but gave no formula/baseline; a
	// traditional_time_to_hire_days config column was added so "avoided" is honest.)
	if cost.VacancyCostPerDay != nil && cost.TraditionalTimeToHireDays != nil && hires > 0 {
		daysSaved := *cost.TraditionalTimeToHireDays - avgDays
		if daysSaved < 0 {
			daysSaved = 0
		}
		view.VacancyCostAvoided = round2(*cost.VacancyCostPerDay * daysSaved * float64(hires))
	}
}

// hireMetrics returns headline hires (hired_at window) plus avg/median days
// created_at→hired_at over the same cohort.
func (l *liveService) hireMetrics(ctx context.Context, f ExecFilters, since time.Time) (int, float64, float64, error) {
	b := &sqlBuilder{}
	b.add("a.hired_at >= $%d", since)
	dimFilter(b, f)
	q := `
		SELECT COUNT(*) AS hires,
		       COALESCE(AVG(EXTRACT(EPOCH FROM (a.hired_at - a.created_at)) / 86400), 0) AS avg_days,
		       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (
		           ORDER BY EXTRACT(EPOCH FROM (a.hired_at - a.created_at)) / 86400), 0) AS median_days
		FROM applications a
		WHERE a.status = 'hired' AND a.hired_at IS NOT NULL` + b.and()
	var hires int
	var avg, median float64
	if err := l.pool.QueryRow(ctx, q, b.args...).Scan(&hires, &avg, &median); err != nil {
		return 0, 0, 0, fmt.Errorf("executive: hire metrics: %w", err)
	}
	return hires, avg, median, nil
}

// funnel returns the volume/response funnel over the created_at window.
func (l *liveService) funnel(ctx context.Context, f ExecFilters, since time.Time) (FunnelStat, error) {
	b := &sqlBuilder{}
	b.add("a.created_at >= $%d", since)
	dimFilter(b, f)
	q := `
		SELECT COUNT(*) AS applied,
		       COUNT(*) FILTER (WHERE a.status IN ('parsed','scored','shortlisted','ai_interview','ai_interviewed')) AS screened,
		       COUNT(*) FILTER (WHERE a.status IN ('interview','interviewed')) AS interviewed,
		       COUNT(*) FILTER (WHERE a.status = 'offer') AS offered,
		       COUNT(*) FILTER (WHERE a.status = 'hired') AS hired,
		       COUNT(*) FILTER (WHERE a.picked_up_at IS NOT NULL) AS responded
		FROM applications a
		WHERE TRUE` + b.and()
	var fn FunnelStat
	var responded int
	if err := l.pool.QueryRow(ctx, q, b.args...).Scan(
		&fn.Applied, &fn.Screened, &fn.Interviewed, &fn.Offered, &fn.Hired, &responded,
	); err != nil {
		return FunnelStat{}, fmt.Errorf("executive: funnel: %w", err)
	}
	fn.ResponseRate = pct(responded, fn.Applied)
	fn.ConversionToHire = pct(fn.Hired, fn.Applied)
	return fn, nil
}

// success returns the dimension breakdown decomposing the headline hires.
func (l *liveService) success(ctx context.Context, f ExecFilters, since time.Time) ([]SuccessRow, error) {
	// applications use the created_at window; hires + avg-TTH use the hired_at
	// window (so Σ hires == headline). A row is included if it falls in EITHER
	// window ($1); per-metric FILTERs (also $1) keep each figure on its own window.
	b := &sqlBuilder{}
	b.args = append(b.args, since)
	b.conds = append(b.conds, "(a.created_at >= $1 OR (a.status = 'hired' AND a.hired_at >= $1))")
	dimFilter(b, f)

	var keyExpr, labelExpr, joins string
	switch f.Dimension {
	case "region":
		keyExpr = "COALESCE(ar.id::text, 'unmapped:' || COALESCE(NULLIF(s.subregion,''),'Unmapped'))"
		labelExpr = "COALESCE(ar.name, NULLIF(s.subregion,''), 'Unmapped')"
		joins = `
			LEFT JOIN stores s ON s.store_no = a.assigned_store_id
			LEFT JOIN area_stores ast ON ast.store_no = s.store_no
			LEFT JOIN areas ar ON ar.id = ast.area_id AND ar.active = TRUE`
	case "position":
		keyExpr = "p.id::text"
		labelExpr = "COALESCE(NULLIF(p.title_en,''), p.title_th, 'Position')"
		joins = `JOIN positions p ON p.id = a.position_id`
	default: // branch
		// LEFT JOIN (not INNER) so hired apps with a NULL assigned_store_id (direct
		// hire / legacy / hired-before-assigned) land in an "Unassigned" bucket
		// instead of vanishing — this is what keeps Σ success.hires == headline.
		keyExpr = "COALESCE(s.store_no::text, 'unassigned')"
		labelExpr = "COALESCE(NULLIF(s.store_name,''), 'Unassigned')"
		joins = `LEFT JOIN stores s ON s.store_no = a.assigned_store_id`
	}

	q := fmt.Sprintf(`
		SELECT %s AS key, %s AS label,
		       COUNT(*) FILTER (WHERE a.created_at >= $1) AS applications,
		       COUNT(*) FILTER (WHERE a.status = 'hired' AND a.hired_at >= $1) AS hires,
		       COALESCE(AVG(EXTRACT(EPOCH FROM (a.hired_at - a.created_at)) / 86400)
		           FILTER (WHERE a.status = 'hired' AND a.hired_at >= $1), 0) AS avg_tth,
		       mode() WITHIN GROUP (ORDER BY COALESCE(c.source_channel,'unknown')) AS top_source
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id %s
		WHERE %s
		GROUP BY 1, 2
		ORDER BY hires DESC, applications DESC
		LIMIT %d`,
		keyExpr, labelExpr, joins,
		strings.Join(b.conds, " AND "), successRowLimit)

	rows, err := l.pool.Query(ctx, q, b.args...)
	if err != nil {
		return nil, fmt.Errorf("executive: success: %w", err)
	}
	defer rows.Close()
	var out []SuccessRow
	for rows.Next() {
		var r SuccessRow
		var avgTTH float64
		if err := rows.Scan(&r.Key, &r.Label, &r.Applications, &r.Hires, &avgTTH, &r.TopSource); err != nil {
			return nil, fmt.Errorf("executive: success scan: %w", err)
		}
		r.AvgTimeToHire = round1(avgTTH)
		r.Conversion = pct(r.Hires, r.Applications)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ─── cost config (DB-backed, shared by mock + live) ──────────────────────────

// getCostConfig reads the single cost-config row. A nil pool or missing row
// returns an empty (unset) config so the ROI empty-state renders.
func getCostConfig(ctx context.Context, pool *pgxpool.Pool) (CostConfig, error) {
	if pool == nil {
		return CostConfig{Currency: "THB"}, nil
	}
	const q = `
		SELECT currency, system_cost_monthly, traditional_cost_per_hire,
		       vacancy_cost_per_day, traditional_time_to_hire_days, updated_by, updated_at
		FROM executive_cost_config WHERE id = TRUE`
	var c CostConfig
	var updatedAt time.Time
	if err := pool.QueryRow(ctx, q).Scan(
		&c.Currency, &c.SystemCostMonthly, &c.TraditionalCostPerHire,
		&c.VacancyCostPerDay, &c.TraditionalTimeToHireDays, &c.UpdatedBy, &updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CostConfig{Currency: "THB"}, nil
		}
		return CostConfig{}, fmt.Errorf("executive: get cost config: %w", err)
	}
	c.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return c, nil
}

// GetCostConfig / SetCostConfig satisfy the Service cost-config seam (live).
func (l *liveService) GetCostConfig(ctx context.Context) (CostConfig, error) {
	return getCostConfig(ctx, l.pool)
}

func (l *liveService) SetCostConfig(ctx context.Context, c CostConfig, updatedBy string) error {
	return setCostConfig(ctx, l.pool, c, updatedBy)
}

// setCostConfig upserts the single cost-config row. Negative figures are rejected.
func setCostConfig(ctx context.Context, pool *pgxpool.Pool, c CostConfig, updatedBy string) error {
	if pool == nil {
		return errors.New("executive: cost config requires a database")
	}
	for _, v := range []*float64{c.SystemCostMonthly, c.TraditionalCostPerHire, c.VacancyCostPerDay, c.TraditionalTimeToHireDays} {
		if v != nil && *v < 0 {
			return ErrNegativeCost
		}
	}
	currency := strings.TrimSpace(c.Currency)
	if currency == "" {
		currency = "THB"
	}
	const q = `
		INSERT INTO executive_cost_config
		    (id, currency, system_cost_monthly, traditional_cost_per_hire,
		     vacancy_cost_per_day, traditional_time_to_hire_days, updated_by, updated_at)
		VALUES (TRUE, $1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
		    currency = EXCLUDED.currency,
		    system_cost_monthly = EXCLUDED.system_cost_monthly,
		    traditional_cost_per_hire = EXCLUDED.traditional_cost_per_hire,
		    vacancy_cost_per_day = EXCLUDED.vacancy_cost_per_day,
		    traditional_time_to_hire_days = EXCLUDED.traditional_time_to_hire_days,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = NOW()`
	if _, err := pool.Exec(ctx, q, currency, c.SystemCostMonthly, c.TraditionalCostPerHire,
		c.VacancyCostPerDay, c.TraditionalTimeToHireDays, updatedBy); err != nil {
		return fmt.Errorf("executive: set cost config: %w", err)
	}
	return nil
}

// ErrNegativeCost is returned when a cost-config write contains a negative figure.
var ErrNegativeCost = errors.New("cost figures must be non-negative")

// round1 rounds to 1 decimal place; round2 to 2 (currency).
func round1(v float64) float64 { return float64(int64(v*10+sign(v)*0.5)) / 10 }
func round2(v float64) float64 { return float64(int64(v*100+sign(v)*0.5)) / 100 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
