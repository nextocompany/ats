package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

// pgSearcher is the mock-default searcher: a Postgres trigram/ILIKE query over
// candidates joined to their applications. It is a real, useful search (not a
// stub) so the dashboard works fully without Azure.
type pgSearcher struct{ pool *pgxpool.Pool }

func newPGSearcher(pool *pgxpool.Pool) *pgSearcher { return &pgSearcher{pool: pool} }

func (s *pgSearcher) Search(ctx context.Context, q Query, scope rbac.Scope) ([]Hit, int, error) {
	q.normalize()

	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	like := "%" + q.Text + "%"
	conds := []string{
		"c.is_duplicate_of IS NULL",
		"(c.full_name ILIKE " + add(like) + " OR COALESCE(c.province,'') ILIKE " + add(like) + ")",
	}
	if q.Status != "" {
		conds = append(conds, "a.status = "+add(q.Status))
	}
	if q.MinScore != nil {
		conds = append(conds, "a.ai_score >= "+add(*q.MinScore))
	}
	// Scope the APPLICATIONS (not just candidate visibility): the result exposes a
	// candidate's best application status/score, so the best pick must be chosen
	// only among in-scope applications — otherwise a candidate with an out-of-scope
	// (e.g. other-store) application could leak that application's data.
	if sc, scArgs := scope.ApplicationsClause(len(args) + 1); sc != "" {
		conds = append(conds, sc)
		args = append(args, scArgs...)
	}

	// One hit per candidate: their highest-scoring in-scope application (rn = 1).
	ranked := `
		WITH ranked AS (
			SELECT c.id AS candidate_id, c.full_name, COALESCE(c.province,'') AS province,
			       a.status, a.ai_score,
			       ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY a.ai_score DESC NULLS LAST) AS rn
			FROM candidates c
			JOIN applications a ON a.candidate_id = c.id
			WHERE ` + strings.Join(conds, " AND ") + `
		)`

	var total int
	if err := s.pool.QueryRow(ctx, ranked+` SELECT COUNT(*) FROM ranked WHERE rn = 1`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search: count: %w", err)
	}

	limitPH := add(q.Limit)
	offsetPH := add((q.Page - 1) * q.Limit)
	pageQ := ranked + `
		SELECT candidate_id, full_name, province, status, ai_score
		FROM ranked WHERE rn = 1
		ORDER BY ai_score DESC NULLS LAST, full_name
		LIMIT ` + limitPH + ` OFFSET ` + offsetPH

	rows, err := s.pool.Query(ctx, pageQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search: query: %w", err)
	}
	defer rows.Close()

	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.CandidateID, &h.FullName, &h.Province, &h.Status, &h.AIScore); err != nil {
			return nil, 0, fmt.Errorf("search: scan: %w", err)
		}
		out = append(out, h)
	}
	return out, total, rows.Err()
}
