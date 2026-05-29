// Command worker consumes the AI processing queue. It connects the same backing
// dependencies as the api, exposes a /health probe on WORKER_PORT, and runs an
// asynq server that handles process_application tasks (OCR → parse → persist).
package main

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/branch"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/dedup"
	"github.com/nexto/hr-ats/internal/health"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/pipeline"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/reengage"
	"github.com/nexto/hr-ats/internal/scoring"
	"github.com/nexto/hr-ats/internal/vacancies"
	"github.com/nexto/hr-ats/pkg/blob"
	"github.com/nexto/hr-ats/pkg/bootstrap"
	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/database"
	"github.com/nexto/hr-ats/pkg/httpx"
	"github.com/nexto/hr-ats/pkg/logging"
	"github.com/nexto/hr-ats/pkg/queue"
	appredis "github.com/nexto/hr-ats/pkg/redis"
)

const workerConcurrency = 10

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}
	logging.Configure(cfg.IsDevelopment())

	ctx := context.Background()

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

	var rdb *goredis.Client
	if err := bootstrap.Retry(ctx, "redis", func(ctx context.Context) error {
		r, e := appredis.Connect(ctx, cfg.RedisURL)
		if e != nil {
			return e
		}
		rdb = r
		return nil
	}); err != nil {
		log.Fatal().Err(err).Msg("redis connect failed")
	}
	defer func() { _ = rdb.Close() }()

	var blobClient *blob.Client
	if err := bootstrap.Retry(ctx, "blob", func(ctx context.Context) error {
		b, e := blob.Connect(ctx, cfg.BlobConnString, cfg.BlobContainer)
		if e != nil {
			return e
		}
		blobClient = b
		return nil
	}); err != nil {
		log.Fatal().Err(err).Msg("blob connect failed")
	}

	// Health probe on its own port so docker-compose can check the worker.
	checkers := []health.Checker{
		health.NewChecker("postgres", func(ctx context.Context) error { return pool.Ping(ctx) }),
		health.NewChecker("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() }),
		health.NewChecker("blob", blobClient.HealthCheck),
	}
	probe := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler, DisableStartupMessage: true})
	probe.Get("/health", health.Handler(checkers...))
	go func() {
		addr := "0.0.0.0:" + cfg.WorkerPort
		log.Info().Str("service", "worker").Str("addr", addr).Msg("health probe listening")
		if err := probe.Listen(addr); err != nil {
			log.Fatal().Err(err).Msg("worker health server error")
		}
	}()

	// AI providers (mock by default; azure when configured).
	ocr, parser := ai.New(cfg)
	candidateRepo := candidates.NewRepository(pool)
	appRepo := applications.NewRepository(pool)
	vacancyRepo := vacancies.NewRepository(pool)
	processor := pipeline.NewProcessor(
		ocr, parser, blobClient,
		candidateRepo,
		appRepo,
		dedup.NewService(candidateRepo),
		scoring.NewScorer(cfg),
		branch.NewAssigner(vacancyRepo),
		positions.NewRepository(pool),
	)

	redisOpt, err := queue.RedisOpt(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("queue redis opt failed")
	}
	// Re-engagement (Sprint 5a): notify talent-pool / prior candidates on vacancy open.
	reengageSvc := reengage.NewService(
		reengage.NewRepository(pool),
		notify.NewNotifier(cfg),
		activity.New(pool),
		cfg.PortalBaseURL,
	)

	srv := asynq.NewServer(redisOpt, asynq.Config{Concurrency: workerConcurrency})
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeProcessApplication, processor.HandleProcessApplication)
	mux.HandleFunc(queue.TypeReengageVacancy, reengageSvc.HandleReengageVacancy)

	log.Info().Str("provider", cfg.AIProvider).Msg("worker started; consuming process_application + vacancy:reengage")
	if err := srv.Run(mux); err != nil {
		log.Fatal().Err(err).Msg("asynq server error")
	}
}
