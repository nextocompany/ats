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
