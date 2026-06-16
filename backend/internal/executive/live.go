package executive

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// liveService computes executive metrics from real ATS data. Budget/Actual
// headcount targets live in PeopleSoft/HRIS, which is not yet integrated, so
// budget figures are reported as unavailable (BudgetAvailable=false) rather
// than fabricated. Fill-rate, pipeline, and sourcing are derived from the
// applications/candidates/vacancies tables.
//
// TODO(peoplesoft): wire budget headcount + true fill-rate once the HRIS sync
// lands; then this path produces the full overview and EXECUTIVE_PROVIDER=real
// can be flipped on with zero frontend change.
type liveService struct {
	pool *pgxpool.Pool
}

const livePipelineLimit = 12

func (l *liveService) Overview(ctx context.Context) (Overview, error) {
	stores, totalActual, err := l.storeFills(ctx)
	if err != nil {
		return Overview{}, err
	}
	pipeline, err := l.pipeline(ctx)
	if err != nil {
		return Overview{}, err
	}
	sourcing, err := l.sourcing(ctx)
	if err != nil {
		return Overview{}, err
	}
	openVacancies, err := l.openVacancies(ctx)
	if err != nil {
		return Overview{}, err
	}

	return Overview{
		DataSource: "live",
		Company: CompanyHeadcount{
			BudgetHeadcount: 0,
			ActualHeadcount: totalActual,
			Vacancy:         openVacancies,
			FillRatePct:     0,
			BudgetAvailable: false, // pending PeopleSoft/HRIS
		},
		Stores:   stores,
		Pipeline: pipeline,
		Sourcing: sourcing,
	}, nil
}

// storeFills returns actual hires per store. Budget is unavailable, so fill-rate
// and heads-short are left zero until HRIS provides targets.
func (l *liveService) storeFills(ctx context.Context) ([]StoreFill, int, error) {
	const q = `
		SELECT s.store_no,
		       COALESCE(NULLIF(s.store_name,''), 'Store') AS store_name,
		       COALESCE(s.subregion,'') AS subregion,
		       COUNT(a.id) FILTER (WHERE a.status = 'hired') AS hired
		FROM stores s
		LEFT JOIN applications a ON a.assigned_store_id = s.store_no
		GROUP BY s.store_no, s.store_name, s.subregion
		ORDER BY s.store_no`
	rows, err := l.pool.Query(ctx, q)
	if err != nil {
		return nil, 0, fmt.Errorf("executive: store fills: %w", err)
	}
	defer rows.Close()
	var out []StoreFill
	var total int
	for rows.Next() {
		var s StoreFill
		if err := rows.Scan(&s.StoreNo, &s.StoreName, &s.Subregion, &s.ActualHeadcount); err != nil {
			return nil, 0, fmt.Errorf("executive: store fills scan: %w", err)
		}
		total += s.ActualHeadcount
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// pipeline returns the recruitment funnel per position across the company.
func (l *liveService) pipeline(ctx context.Context) ([]PipelinePosition, error) {
	const q = `
		SELECT p.id::text,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, 'Position') AS title,
		       COUNT(a.id) AS applied,
		       COUNT(a.id) FILTER (WHERE a.status IN ('parsed','scored','shortlisted','ai_interview','ai_interviewed')) AS screening,
		       COUNT(a.id) FILTER (WHERE a.status IN ('interview','interviewed')) AS interview,
		       COUNT(a.id) FILTER (WHERE a.status = 'offer') AS offer,
		       COUNT(a.id) FILTER (WHERE a.status = 'hired') AS hired,
		       COALESCE((SELECT SUM(v.headcount) FROM vacancies v WHERE v.position_id = p.id AND v.status = 'open'), 0)::int AS openings
		FROM positions p
		LEFT JOIN applications a ON a.position_id = p.id
		GROUP BY p.id, p.title_en, p.title_th
		ORDER BY applied DESC, title
		LIMIT $1`
	rows, err := l.pool.Query(ctx, q, livePipelineLimit)
	if err != nil {
		return nil, fmt.Errorf("executive: pipeline: %w", err)
	}
	defer rows.Close()
	var out []PipelinePosition
	for rows.Next() {
		var p PipelinePosition
		if err := rows.Scan(&p.PositionID, &p.Title, &p.Applied, &p.Screening, &p.Interview, &p.Offer, &p.Hired, &p.Openings); err != nil {
			return nil, fmt.Errorf("executive: pipeline scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// sourcing returns per-channel applied/hired/conversion (mirrors reports.Sources).
func (l *liveService) sourcing(ctx context.Context) ([]Source, error) {
	const q = `
		SELECT COALESCE(c.source_channel,'unknown') AS channel,
		       COUNT(*) AS applied,
		       COUNT(*) FILTER (WHERE a.status = 'hired') AS hired
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		GROUP BY COALESCE(c.source_channel,'unknown')
		ORDER BY applied DESC`
	rows, err := l.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executive: sourcing: %w", err)
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.Channel, &s.Applied, &s.Hired); err != nil {
			return nil, fmt.Errorf("executive: sourcing scan: %w", err)
		}
		s.Conversion = pct(s.Hired, s.Applied)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Keep a stable order for ties (defensive; query already orders by applied).
	sort.SliceStable(out, func(i, j int) bool { return out[i].Applied > out[j].Applied })
	return out, nil
}

// openVacancies returns the total open headcount across the company.
func (l *liveService) openVacancies(ctx context.Context) (int, error) {
	const q = `SELECT COALESCE(SUM(headcount),0)::int FROM vacancies WHERE status = 'open'`
	var n int
	if err := l.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("executive: open vacancies: %w", err)
	}
	return n, nil
}
