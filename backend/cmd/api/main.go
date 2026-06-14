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
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/fit"
	"github.com/nexto/hr-ats/internal/health"
	"github.com/nexto/hr-ats/internal/interview"
	"github.com/nexto/hr-ats/internal/lineauth"
	"github.com/nexto/hr-ats/internal/members"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/peoplesoft"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/profiles"
	"github.com/nexto/hr-ats/internal/public"
	"github.com/nexto/hr-ats/internal/reengage"
	"github.com/nexto/hr-ats/internal/reports"
	"github.com/nexto/hr-ats/internal/search"
	"github.com/nexto/hr-ats/internal/settings"
	"github.com/nexto/hr-ats/internal/users"
	"github.com/nexto/hr-ats/internal/vacancies"
	"github.com/nexto/hr-ats/pkg/blob"
	"github.com/nexto/hr-ats/pkg/bootstrap"
	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/database"
	"github.com/nexto/hr-ats/pkg/email"
	"github.com/nexto/hr-ats/pkg/httpx"
	"github.com/nexto/hr-ats/pkg/logging"
	"github.com/nexto/hr-ats/pkg/queue"
	"github.com/nexto/hr-ats/pkg/ratelimit"
	appredis "github.com/nexto/hr-ats/pkg/redis"
)

const (
	shutdownTimeout  = 10 * time.Second
	maxBodyBytes     = 12 * 1024 * 1024 // headroom over the 10MB resume limit
	publicRateWindow = time.Minute      // rate-limit window for /api/v1/public/* (Max from config)
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
		// Behind a trusted proxy/LB (e.g. the ACA ingress, set via TRUSTED_PROXIES),
		// resolve c.IP() from X-Forwarded-For so the rate limiter keys on the real
		// client. Empty allowlist ⇒ no proxy trusted ⇒ direct peer (dev/CI).
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.TrustedProxyList(),
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID,X-LINE-IdToken",
		AllowCredentials: true,
	}))
	// Security headers (Sprint 6a): helmet sets X-Frame-Options/nosniff/Referrer/
	// HSTS/Permissions-Policy + a baseline CSP for API responses.
	app.Use(helmet.New(helmet.Config{
		XFrameOptions:         "DENY",
		ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: false,
		PermissionPolicy:      "camera=(), microphone=(), geolocation=()",
	}))
	app.Use(middleware.RequestLogger())
	// Settings service backs the runtime "allow all Entra tenants" admin toggle,
	// read by the auth middleware (cached) and managed via /api/v1/admin/settings.
	settingsSvc := settings.NewService(settings.NewRepository(pool))
	authMW, err := middleware.Auth(ctx, cfg, settingsSvc)
	if err != nil {
		log.Fatal().Err(err).Msg("auth middleware init failed")
	}
	app.Use(authMW)

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
	reengageTrigger := reengage.NewTrigger(queueClient)

	// Shared outbound notifier (mock by default; LINE push / email when real).
	notifier := notify.NewNotifier(cfg)

	// Intake + status routes (status PATCH triggers PS sync on hired). Status
	// changes send a best-effort candidate notification (slice 2.3).
	intakeSvc := applications.NewService(candidateRepo, appRepo, blobClient, queueClient)
	intakeHandler := applications.NewHandler(intakeSvc, appRepo, inspector, psService)
	intakeHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	applications.RegisterRoutes(app, intakeHandler)

	// PeopleSoft integration (Direction A webhooks + Direction B sync). Vacancy
	// open fires candidate re-engagement (Sprint 5a).
	peoplesoft.RegisterRoutes(app, peoplesoft.NewHandler(vacancyRepo, positionRepo, psService, cfg.PSProvider, reengageTrigger), cfg.PSWebhookSecret)

	// Re-engagement manual trigger (Sprint 5a).
	reengage.RegisterRoutes(app, reengage.NewHandler(reengageTrigger))

	// Admin system settings (super_admin only) — e.g. the allow-all-tenants toggle.
	settings.RegisterRoutes(app, settings.NewHandler(settingsSvc))

	// Public Career API (consumed by the Next.js portal in Sprint 4). Rate-limited
	// per IP (Sprint 6a) — apply/status are the public abuse surface.
	// Redis-backed storage makes the per-IP window shared across api replicas
	// (Sprint 7) — in-memory storage counted per process, so R replicas allowed
	// R×Max. Fails open on a Redis outage (see pkg/ratelimit).
	app.Use("/api/v1/public", limiter.New(limiter.Config{
		Max:          cfg.RateLimitPublicMax,
		Expiration:   publicRateWindow,
		Storage:      ratelimit.New(rdb),
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
		},
	}))
	pdpaRepo := pdpa.New(pool)
	// Candidate membership (career-portal accounts): signup/login via email-OTP
	// (+ LINE/Google in lineauth/candidateauth), httpOnly session, saved profile +
	// resume. Email is mock (log-only) by default; ACS Email when EMAIL_PROVIDER=real.
	// The session cookie is Secure + SameSite=None outside development (portal and
	// api are cross-site under the apps.io public suffix). Built before the public
	// handler so apply can be account-first.
	emailSender := email.NewSender(cfg)
	caRepo := candidateauth.NewRepository(pool)
	caSvc := candidateauth.NewService(caRepo, emailSender, blobClient, cfg.EmailOTPTTL, cfg.CandidateSessionTTL)
	caHandler := candidateauth.NewHandler(caSvc, cfg.SessionCookieName, !cfg.IsDevelopment())
	// CSRF guard for cookie-authed endpoints: reject cross-origin state-changing
	// requests (the session cookie is SameSite=None in prod, and multipart uploads
	// skip CORS preflight). Safe methods (incl. the GET OAuth login/callback) pass.
	originGuard := candidateauth.EnforceOrigin(cfg.CORSAllowOrigins)
	app.Use("/api/v1/public/auth", originGuard)
	app.Use("/api/v1/public/apply", originGuard)
	candidateauth.RegisterRoutes(app, caHandler)
	// Google Login OAuth (candidate membership). Mock by default → deterministic
	// dev identity; real → full authorize/callback against Google.
	candidateauth.RegisterGoogleRoutes(app, candidateauth.NewGoogleHandler(cfg, caSvc))

	// Public Career API + account-first apply. The handler resolves the member from
	// the session cookie (caSvc); quick-apply (saved resume) is session-gated.
	publicHandler := public.NewHandler(intakeSvc, appRepo, positionRepo, lineVerifier, pdpaRepo, caSvc, cfg.SessionCookieName)
	public.RegisterRoutes(app, publicHandler)
	app.Post("/api/v1/public/apply/quick", candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName), publicHandler.QuickApply)

	// LINE Login OAuth web flow. With the candidate session issuer wired (account-
	// first), the callback creates/links an account and sets the session cookie;
	// the legacy fragment-token path remains when no issuer is supplied.
	lineauth.RegisterRoutes(app, lineauth.NewHandler(cfg, caSvc, lineVerifier))

	// AI pre-interview (slice 2.5): HR invites a shortlisted candidate; the
	// candidate completes an adaptive text chat via an opaque token; the AI writes
	// an evaluation HR reviews. Turns are synchronous (no worker). Mock interviewer
	// by default; Azure OpenAI behind config. Public chat routes ride the rate
	// limiter above; the invite/get routes are authed under /applications.
	interviewRepo := interview.NewRepository(pool)
	interviewSvc := interview.NewService(
		interviewRepo, interview.New(cfg),
		appRepo, positionRepo, candidateRepo, notifier, cfg.PortalBaseURL, cfg.InterviewMaxTurns,
	)
	interviewHandler := interview.NewHandler(interviewSvc, appRepo, cfg.PortalBaseURL)
	interview.RegisterPublicRoutes(app, interviewHandler)

	// HR Dashboard API (Sprint 4a): ranked inbox, bulk, resume signed-URLs,
	// candidate detail/timeline, analytics, PDPA, users/me.
	activityLog := activity.New(pool)
	// Search indexer: no-op unless AI_SEARCH_PROVIDER=azure. Ensure the index
	// exists at startup (best-effort — a transient Search outage must not block
	// the api booting), and keep it fresh on bulk status changes.
	searchIndexer := search.NewIndexer(cfg)
	if cfg.UsesAzureSearch() {
		if err := searchIndexer.EnsureIndex(ctx); err != nil {
			log.Warn().Err(err).Msg("search: ensure index failed at startup (non-fatal)")
		}
	}
	dashboardHandler := applications.NewDashboardHandler(appRepo, blobClient, activityLog)
	dashboardHandler.SetIndexer(search.NewCandidateSync(pool, searchIndexer))
	dashboardHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	applications.RegisterDashboardRoutes(app, dashboardHandler)
	interview.RegisterDashboardRoutes(app, interviewHandler)
	// AI cross-position fit analysis: HR-triggered verdict combining the CV-screening
	// result + the AI pre-interview, matched against the whole Master JD catalogue.
	// Mock summarizer by default; Azure OpenAI behind config. Synchronous (no worker).
	fitSvc := fit.NewService(fit.NewRepository(pool), fit.New(cfg), appRepo, interviewRepo, positionRepo, candidateRepo)
	fit.RegisterDashboardRoutes(app, fit.NewHandler(fitSvc, appRepo))
	// HR member management (career-portal accounts): role-gated directory + lifecycle
	// (super_admin + hr_manager; PDPA erase super_admin-only). Reads candidate_accounts
	// directly; resume URLs are signed on demand and erased on anonymize via the blob
	// client (passed as both signer and deleter).
	members.RegisterDashboardRoutes(app, members.NewHandler(members.NewRepository(pool), activityLog, blobClient, blobClient))
	// Candidate search (Sprint 5c) — registered BEFORE profiles so the static
	// /candidates/search path takes precedence over /candidates/:id. Mock Postgres
	// trigram by default; Azure AI Search behind config.
	search.RegisterRoutes(app, search.NewHandler(search.NewSearcher(cfg, pool)))
	profiles.RegisterRoutes(app, profiles.NewHandler(candidateRepo, appRepo))
	// Analytics + report exports (Sprint 5b): on-demand export rides the same
	// export service the scheduler/worker use; delivery via the notify seam.
	reportRepo := reports.New(pool)
	reportExporter := reports.NewExportService(reportRepo, blobClient, notifier, cfg.ReportRecipientList())
	reports.RegisterRoutes(app, reports.NewHandler(reportRepo, reportExporter, blobClient))
	pdpa.RegisterRoutes(app, pdpa.NewHandler(pdpaRepo))
	users.RegisterRoutes(app, users.NewHandler())

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
