// Package executive computes the company-wide Executive Overview: budget vs
// actual headcount, vacancies, store fill-rate (most short-staffed first),
// pipeline-by-position, and sourcing performance. It runs behind a provider
// seam (EXECUTIVE_PROVIDER=mock|real): "mock" returns deterministic synthetic
// figures layered over real store/position names so the demo always renders
// rich; "real" computes ATS-derived metrics from the database (budget is
// pending the PeopleSoft/HRIS integration).
package executive

// Overview is the single consolidated payload for the executive dashboard.
type Overview struct {
	DataSource  string             `json:"data_source"`  // "mock" | "live"
	GeneratedAt string             `json:"generated_at"` // RFC3339; stamped by the handler
	Company     CompanyHeadcount   `json:"company"`
	Stores      []StoreFill        `json:"stores"`   // sorted ASC by fill_rate (most short-staffed first)
	Pipeline    []PipelinePosition `json:"pipeline"` // recruitment funnel per position
	Sourcing    []Source           `json:"sourcing"`
}

// CompanyHeadcount is the headline budget/actual/vacancy snapshot for the whole company.
type CompanyHeadcount struct {
	BudgetHeadcount int     `json:"budget_headcount"`
	ActualHeadcount int     `json:"actual_headcount"`
	Vacancy         int     `json:"vacancy"`          // budget - actual
	FillRatePct     float64 `json:"fill_rate_pct"`    // actual/budget*100, 1 dp
	BudgetAvailable bool    `json:"budget_available"` // false in live until PeopleSoft is wired
}

// StoreFill is per-store staffing for the "most short-staffed branches" ranking.
type StoreFill struct {
	StoreNo         int     `json:"store_no"`
	StoreName       string  `json:"store_name"`
	Subregion       string  `json:"subregion"`
	BudgetHeadcount int     `json:"budget_headcount"`
	ActualHeadcount int     `json:"actual_headcount"`
	HeadsShort      int     `json:"heads_short"` // budget - actual
	FillRatePct     float64 `json:"fill_rate_pct"`
}

// PipelinePosition is the recruitment funnel for one position across the company.
type PipelinePosition struct {
	PositionID string `json:"position_id"`
	Title      string `json:"title"`
	Applied    int    `json:"applied"`
	Screening  int    `json:"screening"`
	Interview  int    `json:"interview"`
	Offer      int    `json:"offer"`
	Hired      int    `json:"hired"`
	Openings   int    `json:"openings"`
}

// Source is per-channel sourcing efficiency (mirrors reports.Source).
type Source struct {
	Channel    string  `json:"channel"`
	Applied    int     `json:"applied"`
	Hired      int     `json:"hired"`
	Conversion float64 `json:"conversion"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Recruitment ROI & Performance dashboard
//
// The ATS stores no cost/finance data, so ROI is driven by admin-configured cost
// assumptions (CostConfig, gated settings.admin). Period semantics are explicit
// per metric: a hire counts in the period it was HIRED (hired_at), while resume
// volume / funnel / response-rate count in the period applications were CREATED
// (created_at). Success rows decompose the headline: hires + time-to-hire use the
// hired_at window (so Σ success.hires == ROIView.Hires), applications use the
// created_at window.
// ─────────────────────────────────────────────────────────────────────────────

// CostConfig holds the single-row admin cost assumptions. All cost figures are
// pointers so an unset assumption (nil) drives the ROI empty-state instead of a
// fabricated zero.
type CostConfig struct {
	Currency                  string   `json:"currency"`
	SystemCostMonthly         *float64 `json:"system_cost_monthly"`
	TraditionalCostPerHire    *float64 `json:"traditional_cost_per_hire"`
	VacancyCostPerDay         *float64 `json:"vacancy_cost_per_day"`
	TraditionalTimeToHireDays *float64 `json:"traditional_time_to_hire_days"`
	UpdatedBy                 string   `json:"updated_by,omitempty"`
	UpdatedAt                 string   `json:"updated_at,omitempty"`
}

// configured reports whether every cost figure ROI math needs is present.
func (c CostConfig) configured() bool {
	return c.SystemCostMonthly != nil && c.TraditionalCostPerHire != nil
}

// ExecFilters are the period + dimension controls applied to the whole payload.
type ExecFilters struct {
	Period    string // "month" | "quarter" | "year" (rolling lookback)
	Dimension string // success-table grouping: "branch" | "region" | "position"
	Store     *int   // optional branch filter (assigned_store_id)
	Region    string // optional area id filter (areas.id)
	Position  string // optional position id filter
}

// FunnelStat is the volume/response funnel over the created_at window.
type FunnelStat struct {
	Applied          int     `json:"applied"`
	Screened         int     `json:"screened"`
	Interviewed      int     `json:"interviewed"`
	Offered          int     `json:"offered"`
	Hired            int     `json:"hired"`
	ResponseRate     float64 `json:"response_rate"`      // share of apps picked up by HR, 1 dp
	ConversionToHire float64 `json:"conversion_to_hire"` // hired/applied, 1 dp
}

// TimeToHire is apply→offer-accept duration over the hired_at window.
type TimeToHire struct {
	Hires      int     `json:"hires"`
	AvgDays    float64 `json:"avg_days"`
	MedianDays float64 `json:"median_days"`
}

// SuccessRow is one branch/region/position row decomposing the headline hires.
type SuccessRow struct {
	Key           string  `json:"key"`              // dimension id (store_no/area id/position id) or label
	Label         string  `json:"label"`            // human label
	Applications  int     `json:"applications"`     // created_at window
	Hires         int     `json:"hires"`            // hired_at window
	Conversion    float64 `json:"conversion"`       // hires/applications velocity ratio, 1 dp
	AvgTimeToHire float64 `json:"avg_time_to_hire"` // days, hired_at window
	TopSource     string  `json:"top_source"`
}

// ROIView is the consolidated Recruitment ROI & Performance payload.
type ROIView struct {
	DataSource         string       `json:"data_source"`  // "mock" | "live"
	GeneratedAt        string       `json:"generated_at"` // RFC3339; stamped by the handler
	Period             string       `json:"period"`
	Dimension          string       `json:"dimension"`
	Cost               CostConfig   `json:"cost"`
	CostConfigured     bool         `json:"cost_configured"`
	Hires              int          `json:"hires"`              // status='hired' in hired_at window
	SystemCostPeriod   float64      `json:"system_cost_period"` // monthly * months_in_period
	CostPerHire        float64      `json:"cost_per_hire"`
	Savings            float64      `json:"savings"` // vs traditional cost-per-hire
	ROIPct             float64      `json:"roi_pct"`
	VacancyCostAvoided float64      `json:"vacancy_cost_avoided"` // see roi.go (days-saved * rate * hires)
	Funnel             FunnelStat   `json:"funnel"`
	TimeToHire         TimeToHire   `json:"time_to_hire"`
	Success            []SuccessRow `json:"success"`
}
