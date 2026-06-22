package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// docProjection is the index document source: one row per candidate = their
// highest-scoring application (rn=1), mirroring pgSearcher's ranked CTE but with
// subregion + assigned_store_id (the index's RBAC filter fields) and NO scope
// clause — the index holds every candidate; the QUERY applies the scope filter.
// Duplicate shadows (is_duplicate_of) are excluded, matching the search query.
//
// content is the semantic-search text blob: full name + province + the best
// application's ai_summary (empty for walk-in/bulk candidates with no AI summary
// yet, which caps recall but is harmless). The indexer drops it for keyword-only
// indexes, so projecting it unconditionally is safe.
const docProjection = `
	WITH ranked AS (
		SELECT c.id AS candidate_id, c.full_name,
		       COALESCE(c.province,'') AS province, COALESCE(c.subregion,'') AS subregion,
		       a.assigned_store_id, a.status, a.ai_score,
		       COALESCE(a.ai_summary,'') AS ai_summary,
		       ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY a.ai_score DESC NULLS LAST) AS rn
		FROM candidates c
		JOIN applications a ON a.candidate_id = c.id
		WHERE c.is_duplicate_of IS NULL %s
	)
	SELECT candidate_id, full_name, province, subregion, assigned_store_id, status, ai_score,
	       trim(both ' ' from full_name || ' ' || province || ' ' || ai_summary) AS content
	FROM ranked WHERE rn = 1`

func scanDocs(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]Doc, error) {
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.CandidateID, &d.FullName, &d.Province, &d.Subregion,
			&d.AssignedStoreID, &d.Status, &d.AIScore, &d.Content); err != nil {
			return nil, fmt.Errorf("search: doc scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FetchDoc returns the index document for one candidate (false if the candidate
// has no application, so nothing to index).
func FetchDoc(ctx context.Context, pool *pgxpool.Pool, candidateID uuid.UUID) (Doc, bool, error) {
	q := fmt.Sprintf(docProjection, "AND c.id = $1")
	rows, err := pool.Query(ctx, q+" ORDER BY full_name LIMIT 1", candidateID)
	if err != nil {
		return Doc{}, false, fmt.Errorf("search: fetch doc: %w", err)
	}
	docs, err := scanDocs(rows)
	if err != nil {
		return Doc{}, false, err
	}
	if len(docs) == 0 {
		return Doc{}, false, nil
	}
	return docs[0], true, nil
}

// FetchAllDocs returns a page of index documents (for backfill), ordered stably
// so pagination is deterministic.
func FetchAllDocs(ctx context.Context, pool *pgxpool.Pool, offset, limit int) ([]Doc, error) {
	q := fmt.Sprintf(docProjection, "") + " ORDER BY candidate_id LIMIT $1 OFFSET $2"
	rows, err := pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search: fetch all docs: %w", err)
	}
	return scanDocs(rows)
}
