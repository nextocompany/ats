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
	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/auth"
	"github.com/nexto/hr-ats/internal/breach"
	"github.com/nexto/hr-ats/internal/calendar"
	"github.com/nexto/hr-ats/internal/candidateauth"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/dsar"
	"github.com/nexto/hr-ats/internal/executive"
	"github.com/nexto/hr-ats/internal/fit"
	"github.com/nexto/hr-ats/internal/health"
	"github.com/nexto/hr-ats/internal/hrauth"
	"github.com/nexto/hr-ats/internal/intake"
	"github.com/nexto/hr-ats/internal/interview"
	"github.com/nexto/hr-ats/internal/letters"
	"github.com/nexto/hr-ats/internal/lineauth"
	"github.com/nexto/hr-ats/internal/members"
	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/notify"
	"github.com/nexto/hr-ats/internal/pdpa"
	"github.com/nexto/hr-ats/internal/pdpaadmin"
	"github.com/nexto/hr-ats/internal/peoplesoft"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/profiles"
	"github.com/nexto/hr-ats/internal/public"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/internal/rbacadmin"
	"github.com/nexto/hr-ats/internal/reengage"
	"github.com/nexto/hr-ats/internal/reports"
	"github.com/nexto/hr-ats/internal/requisitions"
	"github.com/nexto/hr-ats/internal/search"
	"github.com/nexto/hr-ats/internal/settings"
	"github.com/nexto/hr-ats/internal/stores"
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
	rbacCacheTTL     = 60 * time.Second // dynamic-RBAC matrix refresh interval per replica
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

	// Dynamic RBAC: install the DB-backed authorizer so handlers/scope resolve
	// role→permission + scope from the editable matrix. Guarded — if the matrix
	// hasn't been migrated/seeded yet (or the read fails), we leave the compiled-in
	// legacy fallback active (identical to pre-RBAC behavior), never fail-open. A
	// background ticker refreshes every replica within the TTL after an admin edit.
	rbacRepo := rbac.NewRepository(pool)
	var rbacAuthz *rbac.Authorizer // non-nil once the dynamic matrix is installed
	if roles, err := rbacRepo.ListRoles(ctx); err != nil {
		log.Warn().Err(err).Msg("rbac: could not load roles; using legacy fallback matrix")
	} else if len(roles) == 0 {
		log.Warn().Msg("rbac: no roles seeded (migration 000028 not applied?); using legacy fallback matrix")
	} else {
		authz := rbac.NewAuthorizer(rbacRepo, rbacCacheTTL)
		if err := authz.Reload(ctx); err != nil {
			log.Warn().Err(err).Msg("rbac: authorizer reload failed; using legacy fallback matrix")
		} else {
			rbac.SetDefault(authz)
			authz.Start(ctx)
			rbacAuthz = authz
			log.Info().Int("roles", len(roles)).Msg("rbac: dynamic authorizer installed")
		}
	}

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
		// Retained as a safe default for any fiber c.IP() callers. The rate limiters
		// do NOT use c.IP() — they key via middleware.RealClientIP (right-most
		// non-trusted X-Forwarded-For entry), which is spoof-resistant whereas
		// fiber's c.IP() returns the raw header when a proxy is trusted.
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
	// Spoof-resistant client-IP resolution for the rate limiters: parse the
	// TRUSTED_PROXIES allowlist once, then key on the right-most non-trusted
	// X-Forwarded-For entry (the address our ingress actually saw). A client
	// cannot prepend hops to escape its bucket. Empty allowlist ⇒ direct peer.
	trustedProxies, badProxies, maskedProxies := middleware.ParseTrustedCIDRs(cfg.TrustedProxyList())
	if len(badProxies) > 0 {
		log.Warn().Strs("entries", badProxies).Msg("TRUSTED_PROXIES: ignoring malformed entries")
	}
	if len(maskedProxies) > 0 {
		log.Warn().Strs("entries", maskedProxies).Msg("TRUSTED_PROXIES: host bits set — trust range widened to network")
	}
	if len(trustedProxies) == 0 && !cfg.IsDevelopment() {
		// Behind ACA the direct TCP peer is always the ingress, so an empty allowlist
		// collapses every client into one rate-limit bucket. Warn loudly, don't fatal.
		log.Warn().Msg("TRUSTED_PROXIES empty in non-dev: rate limiters key on the direct peer (ingress) — all traffic shares one bucket")
	}
	log.Info().Int("trusted_proxy_cidrs", len(trustedProxies)).Msg("client-ip trust configured")
	clientIPKey := func(c *fiber.Ctx) string { return middleware.RealClientIP(c, trustedProxies) }
	app.Use(middleware.ClientIPDebugLogger(cfg.LogClientIPs, trustedProxies))
	// Settings service backs the runtime "allow all Entra tenants" admin toggle,
	// read by the auth middleware (cached) and managed via /api/v1/admin/settings.
	settingsSvc := settings.NewService(settings.NewRepository(pool))
	// HR password sign-in (local accounts alongside Entra SSO): the service both
	// validates session cookies for the auth middleware and serves login/logout +
	// super_admin account management. secureCookie ⇒ Secure + SameSite=None in prod.
	hrAuthSvc := hrauth.NewService(hrauth.NewRepository(pool), cfg.HRSessionTTL)
	// Dynamic RBAC: allow assigning any role defined in rbac_roles (custom roles),
	// not just the seven built-ins. Falls back to the built-in allowlist on error.
	hrAuthSvc.SetRoleValidator(rbacRepo.RoleExists)
	authMW, err := middleware.Auth(ctx, cfg, settingsSvc, hrAuthSvc, hrAuthSvc)
	if err != nil {
		log.Fatal().Err(err).Msg("auth middleware init failed")
	}
	app.Use(authMW)
	// Stash the spoof-resistant client IP per request so PDPA audit handlers can
	// attribute who/where (middleware.AuditActor) without re-threading the trust
	// list. Runs after auth so DevUser is already in locals.
	app.Use(middleware.ResolveClientIP(trustedProxies))
	// CSRF guard for cookie-authed mutations: once the hr_auth cookie exists
	// (SameSite=None in prod), state-changing requests must carry an allowed Origin.
	// Safe methods, no-Origin machine calls, and bearer-authed requests pass.
	app.Use(middleware.EnforceOrigin(cfg.CORSAllowOrigins))

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
	intakeHandler.SetActivity(activity.New(pool)) // record single status changes onto the journey
	storeRepo := stores.NewRepository(pool)
	intakeHandler.SetStores(storeRepo) // validate target store on manual reassignment
	applications.RegisterRoutes(app, intakeHandler)

	// PeopleSoft integration (Direction A webhooks + Direction B sync). Vacancy
	// open fires candidate re-engagement (Sprint 5a).
	peoplesoft.RegisterRoutes(app, peoplesoft.NewHandler(vacancyRepo, positionRepo, psService, cfg.PSProvider, reengageTrigger), cfg.PSWebhookSecret)

	// External intake webhook (MS Forms / SEEK / JobsDB → the same pipeline). HMAC-
	// authed; not mounted unless INTAKE_WEBHOOK_SECRET is set (disabled by default).
	// Rate-limited per client IP to bound cost/queue amplification if the HMAC leaks.
	if cfg.IntakeWebhookSecret != "" {
		app.Use("/api/v1/intake", limiter.New(limiter.Config{
			Max:          cfg.RateLimitIntakeMax,
			Expiration:   publicRateWindow,
			Storage:      ratelimit.New(rdb),
			KeyGenerator: clientIPKey,
			LimitReached: func(c *fiber.Ctx) error {
				return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
			},
		}))
	}
	intake.RegisterRoutes(app, intake.NewHandler(intakeSvc, positionRepo), cfg.IntakeWebhookSecret)

	// Re-engagement manual trigger (Sprint 5a).
	reengage.RegisterRoutes(app, reengage.NewHandler(reengageTrigger))

	// Admin system settings (super_admin only) — e.g. the allow-all-tenants toggle.
	settings.RegisterRoutes(app, settings.NewHandler(settingsSvc))

	// Dynamic RBAC admin API (rbac.admin permission) — CRUD roles + the
	// role→permission matrix + per-role scope. Writes refresh the local
	// authorizer immediately; other replicas converge on the TTL.
	rbacadmin.RegisterRoutes(app, rbacadmin.NewHandler(rbacRepo, rbacAuthz))

	// Public Career API (consumed by the Next.js portal in Sprint 4). Rate-limited
	// per IP (Sprint 6a) — apply/status are the public abuse surface.
	// Redis-backed storage makes the per-IP window shared across api replicas
	// (Sprint 7) — in-memory storage counted per process, so R replicas allowed
	// R×Max. Fails open on a Redis outage (see pkg/ratelimit).
	app.Use("/api/v1/public", limiter.New(limiter.Config{
		Max:          cfg.RateLimitPublicMax,
		Expiration:   publicRateWindow,
		Storage:      ratelimit.New(rdb),
		KeyGenerator: clientIPKey,
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
	// Shared audit log (PDPA Phase 5.1): records who/where for PDPA-relevant events.
	// Built here so candidate-facing handlers below are fully configured (audit wired)
	// before their routes are registered.
	activityLog := activity.New(pool)
	caRepo := candidateauth.NewRepository(pool)
	caSvc := candidateauth.NewService(caRepo, emailSender, blobClient, cfg.EmailOTPTTL, cfg.CandidateSessionTTL).
		WithConsentPolicy(pdpaRepo)
	caHandler := candidateauth.NewHandler(caSvc, cfg.SessionCookieName, !cfg.IsDevelopment())
	caHandler.SetAudit(activityLog) // audit consent withdrawals (before RegisterRoutes)
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
	// Candidate-facing application history (their own, by session account).
	app.Get("/api/v1/public/me/applications", candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName), publicHandler.MyApplications)

	// Offer management (candidate side): a logged-in member lists their offers and
	// accepts/declines. Accept best-effort pushes the hire to PeopleSoft. Routes sit
	// under /api/v1/public/auth (already origin-guarded) behind RequireCandidate.
	offerCandHandler := applications.NewOfferCandidateHandler(appRepo, psService)
	applications.RegisterCandidateOfferRoutes(app, offerCandHandler, candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName))
	// Candidate letter downloads (account-scoped, under the origin-guarded /auth prefix).
	applications.RegisterCandidateLetterRoutes(app, applications.NewLetterCandidateHandler(appRepo, blobClient), candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName))

	// Onboarding documents (candidate side): a hired member lists their checklist
	// and uploads/replaces required documents; upload best-effort notifies store HR.
	onboardingCandHandler := applications.NewOnboardingCandidateHandler(appRepo, blobClient, cfg.OnboardingRequiredDocs())
	onboardingCandHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")
	applications.RegisterCandidateOnboardingRoutes(app, onboardingCandHandler, candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName))

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
	// Portal DSAR self-service (PDPA Phase 3): an authenticated candidate exports
	// their own data (s.30 access + s.31 portability) and erases it (s.33; held for
	// HR when a legal hold applies). Gated by the candidate session.
	dsarEraser := pdpa.NewRetentionService(pool, blobClient, search.NewIndexer(cfg, nil), activityLog, cfg.RetentionDays)
	dsar.RegisterRoutes(app, dsar.NewHandler(dsar.New(pool).WithEraser(dsarEraser), activityLog),
		candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName))
	// Search indexer: no-op unless AI_SEARCH_PROVIDER=azure. A non-nil embedder
	// (set only when AZURE_OPENAI_EMBED_DEPLOYMENT is configured) turns on semantic
	// indexing/query. Ensure the index exists at startup (best-effort: a transient
	// Search outage must not block the api booting, and keep it fresh on bulk
	// status changes.
	searchEmbedder := ai.NewEmbedder(cfg)
	searchIndexer := search.NewIndexer(cfg, searchEmbedder)
	if cfg.UsesAzureSearch() {
		if err := searchIndexer.EnsureIndex(ctx); err != nil {
			log.Warn().Err(err).Msg("search: ensure index failed at startup (non-fatal)")
		}
	}
	dashboardHandler := applications.NewDashboardHandler(appRepo, blobClient, activityLog)
	dashboardHandler.SetIndexer(search.NewCandidateSync(pool, searchIndexer))
	dashboardHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	dashboardHandler.SetLineManagerNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")
	applications.RegisterDashboardRoutes(app, dashboardHandler)
	// Human interview scheduling (state-machine feature): sets status=interview and,
	// for an online interview, creates a Teams meeting + calendar invite via Graph
	// (mock log-only by default; real behind GRAPH_PROVIDER=real).
	scheduleHandler := applications.NewScheduleHandler(appRepo, calendar.NewProvider(cfg), candidateRepo, positionRepo)
	scheduleHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	applications.RegisterScheduleRoutes(app, scheduleHandler)
	// Structured interview feedback recorded by the hiring panel (sgm/hr_manager/
	// super_admin) during the interview stage; many entries per application. HR is
	// pinged (email + Teams, best-effort) when feedback is recorded.
	feedbackHandler := applications.NewFeedbackHandler(appRepo)
	feedbackHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")
	applications.RegisterFeedbackRoutes(app, feedbackHandler)
	// Line-Manager Top-5 shortlist + per-application scorecard summary (TA + LM).
	applications.RegisterShortlistRoutes(app, applications.NewShortlistHandler(appRepo))
	// Multi-level hiring approval chain (Staff → HR Manager → SGM → Regional).
	approvalHandler := applications.NewApprovalHandler(appRepo, cfg.ApprovalSLAHours)
	approvalHandler.SetNotifier(notifier, applications.NewHRDirectory(pool), cfg.DashboardBaseURL, cfg.TeamsWebhookURL != "")
	applications.RegisterApprovalRoutes(app, approvalHandler)
	// Offer management (HR side): compose / edit / send an offer for an
	// offer-stage application; sending notifies the candidate.
	offerHandler := applications.NewOfferHandler(appRepo)
	offerHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	applications.RegisterOfferRoutes(app, offerHandler)
	// Letter generation (HR): interview/offer PDF letters → blob → signed download.
	letterRenderer := letters.NewRenderer(cfg.CompanyName)
	applications.RegisterLetterRoutes(app, applications.NewLetterHandler(appRepo, letterRenderer, blobClient))

	// Onboarding documents (HR side): review the checklist + approve/reject each
	// document; a review best-effort notifies the candidate.
	onboardingHandler := applications.NewOnboardingHandler(appRepo, blobClient, cfg.OnboardingRequiredDocs())
	onboardingHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)
	applications.RegisterOnboardingRoutes(app, onboardingHandler)
	// Bulk CV upload (HR dashboard): many resumes for one position → one application
	// + pipeline job each. Positions list powers the picker.
	applications.RegisterBulkRoutes(app, applications.NewBulkHandler(intakeSvc))
	positions.RegisterRoutes(app, positions.NewHandler(positionRepo))
	stores.RegisterRoutes(app, stores.NewHandler(storeRepo))
	// Requisition management (HR dashboard): open/approve/close manual position
	// openings as rows in the shared `vacancies` table (source='manual'), RBAC-scoped.
	// Approved requisitions flow into branch assignment + executive + portal automatically.
	requisitions.RegisterRoutes(app, requisitions.NewHandler(requisitions.NewRepository(pool)))
	// PDPA breach register (DPO/legal): record personal-data breaches, drive the
	// s.37(4) 72h PDPC-notification countdown, and generate the notification
	// content. Gated to breach.manage; company-wide (no RBAC data-scope). The DPO
	// contact in the notification is resolved dynamically from DPO-flagged accounts.
	breach.RegisterRoutes(app, breach.NewHandler(
		breach.NewRepository(pool),
		pdpaRepo,
		cfg.CompanyName,
		activityLog,
	))
	interview.RegisterDashboardRoutes(app, interviewHandler)
	// AI cross-position fit analysis: HR-triggered verdict combining the CV-screening
	// result + the AI pre-interview, matched against the whole Master JD catalogue.
	// Mock summarizer by default; Azure OpenAI behind config. Synchronous (no worker).
	fitSvc := fit.NewService(fit.NewRepository(pool), fit.New(cfg), appRepo, interviewRepo, positionRepo, candidateRepo)
	fit.RegisterDashboardRoutes(app, fit.NewHandler(fitSvc, appRepo))
	// HR member management (career-portal accounts): role-gated directory + lifecycle
	// (super_admin + hr_manager; PDPA erase super_admin-only). Reads candidate_accounts
	// directly; resume URLs are signed on demand and erased on anonymize via the blob
	// client (passed as both signer and deleter). The eraser cascades account erasure
	// into full PDPA erasure of the applicant data behind it.
	memberEraser := pdpa.NewRetentionService(pool, blobClient, search.NewIndexer(cfg, nil), activityLog, cfg.RetentionDays)
	members.RegisterDashboardRoutes(app, members.NewHandler(members.NewRepository(pool), activityLog, blobClient, blobClient).WithEraser(memberEraser))
	// Candidate search (Sprint 5c) — registered BEFORE profiles so the static
	// /candidates/search path takes precedence over /candidates/:id. Mock Postgres
	// trigram by default; Azure AI Search behind config.
	search.RegisterRoutes(app, search.NewHandler(search.NewSearcher(cfg, pool, searchEmbedder)))
	profiles.RegisterRoutes(app, profiles.NewHandler(candidateRepo, appRepo))
	// Analytics + report exports (Sprint 5b): on-demand export rides the same
	// export service the scheduler/worker use; delivery via the notify seam.
	reportRepo := reports.New(pool)
	reportExporter := reports.NewExportService(reportRepo, blobClient, notifier, cfg.ReportRecipientList())
	reports.RegisterRoutes(app, reports.NewHandler(reportRepo, reportExporter, blobClient))
	// ATS Reports (Module-3 3.9): RBAC-scoped, date-ranged hiring-funnel metrics + CSV.
	reports.RegisterATSRoutes(app, reports.NewATSReportHandler(reportRepo, cfg.OnboardingRequiredDocs()))
	executive.RegisterRoutes(app, executive.NewHandler(executive.NewService(pool, cfg.ExecutiveProvider)))
	// PDPA: published DPO contact (s.41) on the public policy endpoints + the
	// pdpa.admin-gated DPO console (DSAR held-queue, consent lookup, overview).
	pdpaHandler := pdpa.NewHandler(pdpaRepo)
	pdpaHandler.SetCompany(cfg.CompanyName)
	pdpa.RegisterRoutes(app, pdpaHandler)
	pdpaadmin.RegisterRoutes(app, pdpaadmin.NewHandler(
		pdpaadmin.NewRepository(pool),
		pdpaRepo,
		cfg.CompanyName,
		pdpaadmin.RetentionInfo{Days: cfg.RetentionDays, Enabled: cfg.RetentionSweepEnabled},
		activityLog,
	))
	users.RegisterRoutes(app, users.NewHandler())
	// HR password sign-in + super_admin account management (alongside Entra SSO).
	// login/logout are unauthenticated (see middleware.isUnauthedPath); the
	// /admin/users CRUD is gated to super_admin in-handler. The login endpoint is
	// the credential-stuffing surface, so it gets its own tight per-IP rate limit
	// (Redis-backed for cross-replica parity, like the public limiter above).
	app.Use("/api/v1/auth/login", limiter.New(limiter.Config{
		Max:          cfg.RateLimitLoginMax,
		Expiration:   publicRateWindow,
		Storage:      ratelimit.New(rdb),
		KeyGenerator: clientIPKey,
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many login attempts")
		},
	}))
	hrauth.RegisterRoutes(app, hrauth.NewHandler(hrAuthSvc, !cfg.IsDevelopment()))

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
