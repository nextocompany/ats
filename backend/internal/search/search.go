// Package search provides candidate search for HR. The seam is mock-default:
// a Postgres trigram/ILIKE query (works locally/CI with zero credentials) or
// Azure AI Search behind config. Results are always RBAC-scoped. Mirrors the
// ai/peoplesoft/notify integration seams.
package search

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/config"
)

// Default + max page size for search results.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Query is a candidate search request (all fields except Text optional).
type Query struct {
	Text     string
	Status   string
	MinScore *float64
	Page     int
	Limit    int
}

// normalize clamps pagination into sane bounds.
func (q *Query) normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}
}

// Hit is one search result: a candidate plus their best application's status/score.
type Hit struct {
	CandidateID string   `json:"candidate_id"`
	FullName    string   `json:"full_name"`
	Province    string   `json:"province"`
	Status      string   `json:"status"`
	AIScore     *float64 `json:"ai_score"`
}

// Searcher searches candidates within an RBAC scope, returning a page of hits
// plus the total match count.
type Searcher interface {
	Search(ctx context.Context, q Query, scope rbac.Scope) ([]Hit, int, error)
}

// Embedder turns query/document text into dense vectors for semantic search.
// Defined here (not imported from internal/ai) so search never imports ai; the
// ai package's concrete embedder satisfies this structurally, wired in via the
// command mains. A nil Embedder means semantic is off, index and query both
// degrade gracefully to keyword-only.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewSearcher selects the implementation by config (mock Postgres by default —
// no Azure credentials needed for local/CI). A non-nil embedder enables hybrid
// (keyword + vector) ranking on the Azure path; nil keeps it keyword-only.
func NewSearcher(cfg *config.Config, pool *pgxpool.Pool, embedder Embedder) Searcher {
	if cfg.UsesAzureSearch() {
		return newAzureSearcher(cfg, embedder)
	}
	return newPGSearcher(pool)
}
