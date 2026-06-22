// Command reindex backfills the Azure AI Search candidate index: it ensures the
// index schema exists, then pages every candidate (with an application) into the
// index in batches. Idempotent (mergeOrUpload) — safe to re-run. A no-op when
// AI_SEARCH_PROVIDER=mock, so it is harmless in local/CI environments.
//
// With semantic search (AZURE_OPENAI_EMBED_DEPLOYMENT set) the backfill also
// embeds each candidate. Adding the vector field to a pre-existing keyword-only
// index is not an in-place PUT: pass -recreate to DROP and rebuild the index
// with the vector schema before backfilling.
//
// DEPLOY ORDERING IS MANDATORY: run `reindex -recreate` (from the new code, with
// the embed env) and let it finish BEFORE any api/worker carrying the embed env
// serves traffic. A hybrid query against an index that lacks content_vector is a
// hard 4xx, not a keyword fallback. The recreate empties search until the
// backfill completes, so schedule the search-empty window accordingly.
//
// Run as a one-off (operator or ACA Job), after migrations + data seed:
//
//	AI_SEARCH_PROVIDER=azure AZURE_SEARCH_ENDPOINT=... AZURE_SEARCH_KEY=<admin> \
//	AZURE_OPENAI_EMBED_DEPLOYMENT=text-embedding-3-small AZURE_OPENAI_ENDPOINT=... \
//	AZURE_OPENAI_KEY=... go run ./cmd/reindex -recreate
package main

import (
	"context"
	"flag"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/search"
	"github.com/nexto/hr-ats/pkg/bootstrap"
	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/database"
	"github.com/nexto/hr-ats/pkg/logging"
)

// pageSize matches the indexer's batch ceiling — one DB page per push.
const pageSize = 500

func main() {
	recreate := flag.Bool("recreate", false, "drop and rebuild the index before backfilling (required to migrate a keyword-only index to the vector schema)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}
	logging.Configure(cfg.IsDevelopment())
	ctx := context.Background()

	if !cfg.UsesAzureSearch() {
		log.Info().Msg("AI_SEARCH_PROVIDER is mock — search uses Postgres trigram, nothing to index")
		return
	}

	var pool *pgxpool.Pool
	if err := bootstrap.Retry(ctx, "postgres", func(ctx context.Context) error {
		p, e := database.Connect(ctx, cfg.DatabaseURL)
		if e != nil {
			return e
		}
		pool = p
		return nil
	}); err != nil {
		log.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer pool.Close()

	indexer := search.NewIndexer(cfg, ai.NewEmbedder(cfg))
	if *recreate {
		if err := indexer.DropIndex(ctx); err != nil {
			log.Fatal().Err(err).Msg("drop index failed")
		}
		log.Warn().Str("index", cfg.AzureSearchIndex).Bool("semantic", cfg.UsesSemanticSearch()).
			Msg("index dropped for recreate; search is empty until backfill completes")
	}
	if err := indexer.EnsureIndex(ctx); err != nil {
		log.Fatal().Err(err).Msg("ensure index failed")
	}
	log.Info().Str("index", cfg.AzureSearchIndex).Bool("semantic", cfg.UsesSemanticSearch()).
		Msg("index ensured; starting backfill")

	total := 0
	for offset := 0; ; offset += pageSize {
		docs, err := search.FetchAllDocs(ctx, pool, offset, pageSize)
		if err != nil {
			log.Fatal().Err(err).Int("offset", offset).Msg("fetch docs failed")
		}
		if len(docs) == 0 {
			break
		}
		if err := indexer.UpsertBatch(ctx, docs); err != nil {
			log.Fatal().Err(err).Int("offset", offset).Msg("upsert batch failed")
		}
		total += len(docs)
		log.Info().Int("indexed", total).Msg("backfill progress")
		if len(docs) < pageSize {
			break
		}
	}
	log.Info().Int("total", total).Msg("backfill complete")
}
