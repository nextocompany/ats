// Command reindex backfills the Azure AI Search candidate index: it ensures the
// index schema exists, then pages every candidate (with an application) into the
// index in batches. Idempotent (mergeOrUpload) — safe to re-run. A no-op when
// AI_SEARCH_PROVIDER=mock, so it is harmless in local/CI environments.
//
// Run as a one-off (operator or ACA Job), after migrations + data seed:
//
//	AI_SEARCH_PROVIDER=azure AZURE_SEARCH_ENDPOINT=... AZURE_SEARCH_KEY=<admin> \
//	  go run ./cmd/reindex
package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/search"
	"github.com/nexto/hr-ats/pkg/bootstrap"
	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/database"
	"github.com/nexto/hr-ats/pkg/logging"
)

// pageSize matches the indexer's batch ceiling — one DB page per push.
const pageSize = 500

func main() {
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

	indexer := search.NewIndexer(cfg)
	if err := indexer.EnsureIndex(ctx); err != nil {
		log.Fatal().Err(err).Msg("ensure index failed")
	}
	log.Info().Str("index", cfg.AzureSearchIndex).Msg("index ensured; starting backfill")

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
