// Command api is the HTTP server entrypoint. It loads config, connects to every
// backing dependency (with startup retry), mounts the /health endpoint, and
// shuts down gracefully.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/health"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/peoplesoft"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/public"
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

const (
	shutdownTimeout = 10 * time.Second
	maxBodyBytes    = 12 * 1024 * 1024 // headroom over the 10MB resume limit
)

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

	// Queue client + inspector (asynq over the same Redis).
	queueClient, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("queue client init failed")
	}
	defer func() { _ = queueClient.Close() }()

	inspector, err := queue.NewInspector(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("queue inspector init failed")
	}
	defer func() { _ = inspector.Close() }()

	app := fiber.New(fiber.Config{
		ErrorHandler:          httpx.ErrorHandler,
		DisableStartupMessage: true,
		BodyLimit:             maxBodyBytes,
	})
	app.Use(middleware.RequestLogger())
	app.Use(middleware.MockJWT(cfg.IsDevelopment()))

	checkers := []health.Checker{
		health.NewChecker("postgres", func(ctx context.Context) error { return pool.Ping(ctx) }),
		health.NewChecker("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() }),
		health.NewChecker("blob", blobClient.HealthCheck),
		health.NewChecker("queue", func(ctx context.Context) error {
			_, perr := inspector.Queues()
			return perr
		}),
	}
	app.Get("/health", health.Handler(checkers...))

	// Repositories.
	candidateRepo := candidates.NewRepository(pool)
	appRepo := applications.NewRepository(pool)
	positionRepo := positions.NewRepository(pool)
	vacancyRepo := vacancies.NewRepository(pool)

	// External integrations (mock by default; real behind config).
	psClient := peoplesoft.NewClient(cfg)
	psService := peoplesoft.NewService(psClient, appRepo, candidateRepo, blobClient, cfg.PSCSVFallbackContainer)
	lineVerifier := auth.NewVerifier(cfg)

	// Intake + status routes (status PATCH triggers PS sync on hired).
	intakeSvc := applications.NewService(candidateRepo, appRepo, blobClient, queueClient)
	applications.RegisterRoutes(app, applications.NewHandler(intakeSvc, appRepo, inspector, psService))

	// PeopleSoft integration (Direction A webhooks + Direction B sync).
	peoplesoft.RegisterRoutes(app, peoplesoft.NewHandler(vacancyRepo, positionRepo, psService, cfg.PSProvider))

	// Public Career API (consumed by the Next.js portal in Sprint 4).
	public.RegisterRoutes(app, public.NewHandler(intakeSvc, appRepo, positionRepo, lineVerifier))

	go func() {
		addr := "0.0.0.0:" + cfg.HTTPPort
		log.Info().Str("service", "api").Str("addr", addr).Msg("listening")
		if err := app.Listen(addr); err != nil {
			log.Fatal().Err(err).Msg("http server error")
		}
	}()

	waitForShutdown(app)
}

func waitForShutdown(app *fiber.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down api")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
}
