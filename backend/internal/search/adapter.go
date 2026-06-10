package search

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CandidateSync upserts a single candidate's document into the search index. It
// structurally satisfies pipeline.CandidateIndexer and applications' indexer
// interface, so neither package needs to import search's concrete type via a
// shared interface — and search never imports them (no cycle).
type CandidateSync struct {
	pool *pgxpool.Pool
	idx  Indexer
}

// NewCandidateSync builds the adapter. With a no-op Indexer (mock), Index is a
// cheap DB read + no-op push; callers can always wire it unconditionally.
func NewCandidateSync(pool *pgxpool.Pool, idx Indexer) *CandidateSync {
	return &CandidateSync{pool: pool, idx: idx}
}

// Index re-projects one candidate and upserts it. Returns nil when the candidate
// has no application (nothing to index).
func (c *CandidateSync) Index(ctx context.Context, candidateID uuid.UUID) error {
	doc, ok, err := FetchDoc(ctx, c.pool, candidateID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return c.idx.UpsertBatch(ctx, []Doc{doc})
}
