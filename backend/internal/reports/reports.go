// Package reports computes recruitment analytics (F08/F10): funnel, KPI
// pipeline, and sourcing efficiency. Aggregations are single-pass SQL.
package reports

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Funnel is the recruitment funnel (counts by stage).
type Funnel struct {
	Applied  int `json:"applied"`
	PassedAI int `json:"passed_ai"`
	Reviewed int `json:"reviewed"`
	Hired    int `json:"hired"`
}

// KPI is the executive pipeline snapshot.
type KPI struct {
	Applied   int `json:"applied"`
	Passed    int `json:"passed"`
	Onboarded int `json:"onboarded"`
	Waiting   int `json:"waiting"`
}

// Source is per-channel sourcing efficiency.
type Source struct {
	Channel    string  `json:"channel"`
	Applied    int     `json:"applied"`
	Hired      int     `json:"hired"`
	Conversion float64 `json:"conversion"`
}

// Repo computes analytics over applications/candidates.
type Repo struct{ pool *pgxpool.Pool }

// New builds the reports repository.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Funnel returns funnel counts.
func (r *Repo) Funnel(ctx context.Context) (Funnel, error) {
	const q = `
		SELECT
			COUNT(*) AS applied,
			COUNT(*) FILTER (WHERE must_have_passed IS TRUE) AS passed_ai,
			COUNT(*) FILTER (WHERE status IN ('shortlisted','interview','hired')) AS reviewed,
			COUNT(*) FILTER (WHERE status = 'hired') AS hired
		FROM applications`
	var f Funnel
	if err := r.pool.QueryRow(ctx, q).Scan(&f.Applied, &f.PassedAI, &f.Reviewed, &f.Hired); err != nil {
		return Funnel{}, fmt.Errorf("reports: funnel: %w", err)
	}
	return f, nil
}

// KPI returns the pipeline snapshot.
func (r *Repo) KPI(ctx context.Context) (KPI, error) {
	const q = `
		SELECT
			COUNT(*) AS applied,
			COUNT(*) FILTER (WHERE must_have_passed IS TRUE) AS passed,
			COUNT(*) FILTER (WHERE status = 'hired') AS onboarded,
			COUNT(*) FILTER (WHERE status IN ('pending','parsed','scored')) AS waiting
		FROM applications`
	var k KPI
	if err := r.pool.QueryRow(ctx, q).Scan(&k.Applied, &k.Passed, &k.Onboarded, &k.Waiting); err != nil {
		return KPI{}, fmt.Errorf("reports: kpi: %w", err)
	}
	return k, nil
}

// Sources returns per-channel applied/hired counts and conversion.
func (r *Repo) Sources(ctx context.Context) ([]Source, error) {
	const q = `
		SELECT COALESCE(c.source_channel,'unknown') AS channel,
		       COUNT(*) AS applied,
		       COUNT(*) FILTER (WHERE a.status = 'hired') AS hired
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		GROUP BY COALESCE(c.source_channel,'unknown')
		ORDER BY applied DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reports: sources: %w", err)
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.Channel, &s.Applied, &s.Hired); err != nil {
			return nil, fmt.Errorf("reports: sources scan: %w", err)
		}
		if s.Applied > 0 {
			s.Conversion = float64(s.Hired) / float64(s.Applied)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
