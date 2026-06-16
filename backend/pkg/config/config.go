// Package config loads and validates all runtime configuration from the
// environment at startup. It fails fast when a required variable is missing so
// the process never binds a port in a half-configured state.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every runtime setting. It is constructed once via Load and then
// treated as immutable — callers read from it, never mutate it.
type Config struct {
	Env            string
	HTTPPort       string
	WorkerPort     string
	DatabaseURL    string
	RedisURL       string
	BlobConnString string
	BlobContainer  string
	JWTSecret      string

	// AIProvider selects the OCR/parse implementation: "mock" (default), "azure",
	// or "gemini" (Google Gemini REST API, free-tier friendly).
	AIProvider string

	// Azure AI settings — required only when AIProvider == "azure".
	AzureOpenAIEndpoint   string
	AzureOpenAIKey        string
	AzureOpenAIDeployment string
	AzureDocIntelEndpoint string
	AzureDocIntelKey      string

	// Gemini AI settings — required only when AIProvider == "gemini".
	GeminiAPIKey string
	GeminiModel  string

	// InterviewMaxTurns caps the number of questions the AI pre-interview asks
	// (slice 2.5). The interviewer reuses the Azure OpenAI deployment above.
	InterviewMaxTurns int

	// AISearchProvider selects candidate search: "mock" (Postgres trigram, default)
	// or "azure" (Azure AI Search query). Required Azure fields gate on "azure".
	AISearchProvider    string
	AzureSearchEndpoint string
	AzureSearchKey      string
	AzureSearchIndex    string

	// PeopleSoft Integration Broker — "mock" (default) or "real".
	PSProvider             string
	PSIBBaseURL            string
	PSIBTokenURL           string
	PSIBClientID           string
	PSIBClientSecret       string
	PSCSVFallbackContainer string

	// PSWebhookSecret is the shared secret PeopleSoft uses to sign inbound webhook
	// bodies (X-PS-Signature: hex HMAC-SHA256). Required when PSProvider == "real".
	PSWebhookSecret string

	// HR auth (Sprint 6a) — "mock" (dev super_admin, default) or "real" (Azure AD /
	// Entra ID JWT validation). The middleware preserves the DevUser locals contract.
	AuthProvider    string
	AzureADTenantID string
	AzureADClientID string // expected token audience
	// AzureADAllowedTenants is the comma-separated allowlist of Entra tenant IDs
	// (the token `tid` claim) permitted to sign in. Empty ⇒ single-tenant: only
	// AzureADTenantID is accepted (unchanged behaviour). Set to two or more to
	// enable multi-tenant SSO (e.g. partner organisations) — the app registration
	// must also be multi-tenant (signInAudience=AzureADMultipleOrgs).
	AzureADAllowedTenants string

	// LINE Login — "mock" (default) or "real".
	LINEProvider  string
	LINEChannelID string
	// LINE Login OAuth web flow (real mode): the channel secret signs the
	// authorization-code → token exchange, and the callback URL must exactly match
	// the one registered on the LINE Login channel.
	LINEChannelSecret    string
	LINELoginCallbackURL string
	// LINELoginBotPrompt asks LINE to offer "add the Official Account as a friend"
	// during login ("aggressive" = dedicated screen, "normal" = inline option, ""
	// = off). Requires the Login channel to be linked to the Messaging API channel
	// in the LINE console; ignored by LINE otherwise.
	LINELoginBotPrompt string

	// Notifications (re-engagement, report delivery) — "mock" (default) or "real".
	NotifyProvider  string
	NotifyLINEToken string // LINE Messaging API channel access token (push)
	NotifyEmailFrom string // from-address for email delivery (real)
	// TeamsWebhookURL is an MS Teams Incoming Webhook the HR channel receives
	// notifications on. Empty = Teams channel disabled (independent of LINE/email).
	TeamsWebhookURL string

	// Interview calendar (Microsoft Graph) — "mock" (default) or "real". When real,
	// online interviews create a Teams meeting + a calendar event on the service
	// mailbox with the candidate as an email attendee (app-only client credentials).
	GraphProvider         string
	GraphTenantID         string
	GraphClientID         string
	GraphClientSecret     string
	GraphOrganizerMailbox string // service mailbox the event is created on (e.g. interviews@…)
	GraphTimeZone         string // Windows tz name for event start/end (default "SE Asia Standard Time")

	// Google Login (candidate membership) — "mock" (default) or "real". The OAuth
	// web flow mirrors LINE Login; the client secret signs the code→token exchange
	// and the callback URL must exactly match the one registered in Google Cloud.
	GoogleProvider     string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string

	// Email delivery for candidate email-OTP (passwordless signup/login) —
	// "mock" (default, log-only) or "real" (Azure Communication Services Email REST).
	// ACS has no Go SDK: acsSender signs requests with the access key (shared-key
	// HMAC, like Azure Storage) and POSTs to {endpoint}/emails:send.
	EmailProvider     string
	ACSEmailEndpoint  string // e.g. https://<resource>.<region>.communication.azure.com
	ACSEmailAccessKey string // base64 access key from the ACS connection string
	ACSEmailSender    string // verified sender address (e.g. DoNotReply@<domain>)

	// Candidate session (httpOnly cookie) + email-OTP tuning.
	CandidateSessionTTL time.Duration // how long a login stays valid (default 30d)
	SessionCookieName   string        // cookie name (default "cp_session")
	EmailOTPTTL         time.Duration // OTP validity window (default 10m)

	// HR password sign-in (local accounts, alongside Entra SSO). Sessions are
	// httpOnly cookies with a server-side opaque token — shorter-lived than the
	// candidate session as the admin console is higher-privilege.
	HRSessionTTL time.Duration // how long an HR password login stays valid (default 12h)

	// PortalBaseURL is the public Career Portal origin used to build apply links
	// in outbound notifications.
	PortalBaseURL string

	// DashboardBaseURL is the HR dashboard origin used to build deep links in
	// HR-facing notifications (email/Teams). Distinct from PortalBaseURL, which is
	// candidate-facing.
	DashboardBaseURL string

	// Report scheduler (Sprint 5b): cron spec for the recurring export, and the
	// comma-separated recipient list notified with the export link.
	ReportScheduleCron string
	ReportRecipients   string

	// Retention sweep (Sprint 7, F-PDPA): daily anonymization of expired candidate
	// PII. Disabled by default — a destructive job must be explicitly enabled per
	// environment so CI/dev/mock never purge. RetentionDays is the ≤1-year window.
	RetentionSweepEnabled bool
	RetentionDays         int
	RetentionSweepCron    string
	RetentionSweepBatch   int

	// Auth cleanup (candidate membership): delete expired/consumed email_otps and
	// expired/revoked candidate_sessions on the scheduler's cadence. Unlike the
	// retention sweep this is benign housekeeping (it only removes already-dead
	// auth artifacts, never user data), so it defaults ENABLED.
	AuthCleanupEnabled bool
	AuthCleanupCron    string
	AuthCleanupBatch   int

	// Public API rate limit (Sprint 7): max requests per IP per minute on
	// /api/v1/public/*. Enforced cluster-wide via the Redis-backed store.
	RateLimitPublicMax int

	// Login rate limit: max POST /api/v1/auth/login attempts per IP per minute —
	// the credential-stuffing surface. Deliberately tight.
	RateLimitLoginMax int

	// CORSAllowOrigins is the comma-separated allowlist for browser clients.
	CORSAllowOrigins string

	// TrustedProxies is the comma-separated allowlist of proxy IPs/CIDRs (e.g. the
	// ACA ingress range) whose X-Forwarded-For is trusted for client-IP resolution.
	// Empty (dev/CI) ⇒ no proxy trusted ⇒ c.IP() is the direct peer.
	TrustedProxies string
}

// Provider values selecting real (vs mock) integrations.
const (
	AIProviderAzure  = "azure"
	AIProviderGemini = "gemini"
	ProviderReal     = "real"
)

// Load reads configuration from the environment and returns a populated Config.
// Required variables (DB_URL, REDIS_URL) cause a clear error when absent.
func Load() (*Config, error) {
	c := &Config{
		Env:            getenv("ENV", "development"),
		HTTPPort:       getenv("HTTP_PORT", "8080"),
		WorkerPort:     getenv("WORKER_PORT", "8081"),
		DatabaseURL:    os.Getenv("DB_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
		BlobConnString: os.Getenv("AZURE_BLOB_CONNECTION_STRING"),
		BlobContainer:  getenv("AZURE_BLOB_CONTAINER", "resumes"),
		JWTSecret:      os.Getenv("JWT_SECRET"),

		AIProvider:            getenv("AI_PROVIDER", "mock"),
		AzureOpenAIEndpoint:   os.Getenv("AZURE_OPENAI_ENDPOINT"),
		AzureOpenAIKey:        os.Getenv("AZURE_OPENAI_KEY"),
		AzureOpenAIDeployment: getenv("AZURE_OPENAI_DEPLOYMENT", "hr-screening-gpt4o"),
		AzureDocIntelEndpoint: os.Getenv("AZURE_DOC_INTEL_ENDPOINT"),
		AzureDocIntelKey:      os.Getenv("AZURE_DOC_INTEL_KEY"),

		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		GeminiModel:  getenv("GEMINI_MODEL", "gemini-2.0-flash"),

		InterviewMaxTurns: getenvInt("INTERVIEW_MAX_TURNS", 6),

		AuthProvider:          getenv("AUTH_PROVIDER", "mock"),
		AzureADTenantID:       os.Getenv("AZURE_AD_TENANT_ID"),
		AzureADClientID:       os.Getenv("AZURE_AD_CLIENT_ID"),
		AzureADAllowedTenants: os.Getenv("AZURE_AD_ALLOWED_TENANTS"),

		AISearchProvider:    getenv("AI_SEARCH_PROVIDER", "mock"),
		AzureSearchEndpoint: os.Getenv("AZURE_SEARCH_ENDPOINT"),
		AzureSearchKey:      os.Getenv("AZURE_SEARCH_KEY"),
		AzureSearchIndex:    getenv("AZURE_SEARCH_INDEX", "candidates"),

		PSProvider:             getenv("PS_PROVIDER", "mock"),
		PSIBBaseURL:            os.Getenv("PS_IB_BASE_URL"),
		PSIBTokenURL:           os.Getenv("PS_IB_TOKEN_URL"),
		PSIBClientID:           os.Getenv("PS_IB_CLIENT_ID"),
		PSIBClientSecret:       os.Getenv("PS_IB_CLIENT_SECRET"),
		PSCSVFallbackContainer: getenv("PS_CSV_FALLBACK_CONTAINER", "ps-export"),
		PSWebhookSecret:        os.Getenv("PS_WEBHOOK_SECRET"),

		LINEProvider:         getenv("LINE_PROVIDER", "mock"),
		LINEChannelID:        os.Getenv("LINE_CHANNEL_ID"),
		LINEChannelSecret:    os.Getenv("LINE_CHANNEL_SECRET"),
		LINELoginCallbackURL: os.Getenv("LINE_LOGIN_CALLBACK_URL"),
		LINELoginBotPrompt:   getenv("LINE_LOGIN_BOT_PROMPT", "aggressive"),

		NotifyProvider:  getenv("NOTIFY_PROVIDER", "mock"),
		NotifyLINEToken: os.Getenv("NOTIFY_LINE_TOKEN"),
		NotifyEmailFrom: os.Getenv("NOTIFY_EMAIL_FROM"),
		TeamsWebhookURL: os.Getenv("TEAMS_WEBHOOK_URL"),

		GraphProvider:         getenv("GRAPH_PROVIDER", "mock"),
		GraphTenantID:         os.Getenv("GRAPH_TENANT_ID"),
		GraphClientID:         os.Getenv("GRAPH_CLIENT_ID"),
		GraphClientSecret:     os.Getenv("GRAPH_CLIENT_SECRET"),
		GraphOrganizerMailbox: os.Getenv("GRAPH_ORGANIZER_MAILBOX"),
		GraphTimeZone:         getenv("GRAPH_TIMEZONE", "SE Asia Standard Time"),

		GoogleProvider:     getenv("GOOGLE_PROVIDER", "mock"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleCallbackURL:  os.Getenv("GOOGLE_CALLBACK_URL"),

		EmailProvider:     getenv("EMAIL_PROVIDER", "mock"),
		ACSEmailEndpoint:  os.Getenv("ACS_EMAIL_ENDPOINT"),
		ACSEmailAccessKey: os.Getenv("ACS_EMAIL_ACCESS_KEY"),
		ACSEmailSender:    os.Getenv("ACS_EMAIL_SENDER"),

		CandidateSessionTTL: getenvDuration("CANDIDATE_SESSION_TTL", 720*time.Hour), // 30 days
		SessionCookieName:   getenv("CANDIDATE_SESSION_COOKIE", "cp_session"),
		EmailOTPTTL:         getenvDuration("EMAIL_OTP_TTL", 10*time.Minute),
		HRSessionTTL:        getenvDuration("HR_SESSION_TTL", 12*time.Hour),

		PortalBaseURL:    getenv("PORTAL_BASE_URL", "http://localhost:3001"),
		DashboardBaseURL: getenv("DASHBOARD_BASE_URL", "http://localhost:3000"),

		ReportScheduleCron: getenv("REPORT_SCHEDULE_CRON", "0 7 * * 1"), // Mon 07:00
		ReportRecipients:   os.Getenv("REPORT_RECIPIENTS"),

		RetentionSweepEnabled: getenvBool("RETENTION_SWEEP_ENABLED", false),
		RetentionDays:         getenvInt("RETENTION_DAYS", 365),
		RetentionSweepCron:    getenv("RETENTION_SWEEP_CRON", "30 3 * * *"), // daily 03:30
		RetentionSweepBatch:   getenvInt("RETENTION_SWEEP_BATCH", 500),

		AuthCleanupEnabled: getenvBool("AUTH_CLEANUP_ENABLED", true),
		AuthCleanupCron:    getenv("AUTH_CLEANUP_CRON", "15 3 * * *"), // daily 03:15 (offset from retention)
		AuthCleanupBatch:   getenvInt("AUTH_CLEANUP_BATCH", 500),

		RateLimitPublicMax: getenvInt("RATE_LIMIT_PUBLIC_MAX", 30),
		RateLimitLoginMax:  getenvInt("RATE_LIMIT_LOGIN_MAX", 10),

		CORSAllowOrigins: getenv("CORS_ALLOW_ORIGINS", "http://localhost:3000,http://localhost:3001"),
		TrustedProxies:   os.Getenv("TRUSTED_PROXIES"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DB_URL is required")
	}
	if c.RedisURL == "" {
		return nil, fmt.Errorf("config: REDIS_URL is required")
	}
	if c.BlobConnString == "" {
		return nil, fmt.Errorf("config: AZURE_BLOB_CONNECTION_STRING is required")
	}

	// Catch typo'd provider flags (e.g. AI_PROVIDER=real) that would otherwise
	// silently fall back to mock. AI/Search use "azure"; the rest use "real".
	for _, p := range []struct {
		name    string
		val     string
		allowed []string
	}{
		{"AI_PROVIDER", c.AIProvider, []string{"mock", AIProviderAzure, AIProviderGemini}},
		{"AI_SEARCH_PROVIDER", c.AISearchProvider, []string{"mock", AIProviderAzure}},
		{"AUTH_PROVIDER", c.AuthProvider, []string{"mock", ProviderReal}},
		{"PS_PROVIDER", c.PSProvider, []string{"mock", ProviderReal}},
		{"LINE_PROVIDER", c.LINEProvider, []string{"mock", ProviderReal}},
		{"NOTIFY_PROVIDER", c.NotifyProvider, []string{"mock", ProviderReal}},
		{"GOOGLE_PROVIDER", c.GoogleProvider, []string{"mock", ProviderReal}},
		{"EMAIL_PROVIDER", c.EmailProvider, []string{"mock", ProviderReal}},
		{"GRAPH_PROVIDER", c.GraphProvider, []string{"mock", ProviderReal}},
	} {
		if !isOneOf(p.val, p.allowed) {
			return nil, fmt.Errorf("config: %s must be one of %v, got %q", p.name, p.allowed, p.val)
		}
	}

	if c.UsesAzureAI() {
		if c.AzureOpenAIEndpoint == "" || c.AzureOpenAIKey == "" {
			return nil, fmt.Errorf("config: AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_KEY are required when AI_PROVIDER=azure")
		}
		if c.AzureDocIntelEndpoint == "" || c.AzureDocIntelKey == "" {
			return nil, fmt.Errorf("config: AZURE_DOC_INTEL_ENDPOINT and AZURE_DOC_INTEL_KEY are required when AI_PROVIDER=azure")
		}
	}
	if c.UsesGeminiAI() && c.GeminiAPIKey == "" {
		return nil, fmt.Errorf("config: GEMINI_API_KEY is required when AI_PROVIDER=gemini")
	}
	if c.UsesRealGraph() {
		if c.GraphTenantID == "" || c.GraphClientID == "" || c.GraphClientSecret == "" || c.GraphOrganizerMailbox == "" {
			return nil, fmt.Errorf("config: GRAPH_TENANT_ID, GRAPH_CLIENT_ID, GRAPH_CLIENT_SECRET, GRAPH_ORGANIZER_MAILBOX are required when GRAPH_PROVIDER=real")
		}
	}
	if c.UsesRealPeopleSoft() {
		if c.PSIBBaseURL == "" || c.PSIBTokenURL == "" || c.PSIBClientID == "" || c.PSIBClientSecret == "" {
			return nil, fmt.Errorf("config: PS_IB_BASE_URL, PS_IB_TOKEN_URL, PS_IB_CLIENT_ID, PS_IB_CLIENT_SECRET are required when PS_PROVIDER=real")
		}
		if c.PSWebhookSecret == "" {
			return nil, fmt.Errorf("config: PS_WEBHOOK_SECRET is required when PS_PROVIDER=real")
		}
	}
	if c.UsesRealLINE() {
		if c.LINEChannelID == "" {
			return nil, fmt.Errorf("config: LINE_CHANNEL_ID is required when LINE_PROVIDER=real")
		}
		if c.LINEChannelSecret == "" || c.LINELoginCallbackURL == "" {
			return nil, fmt.Errorf("config: LINE_CHANNEL_SECRET and LINE_LOGIN_CALLBACK_URL are required when LINE_PROVIDER=real")
		}
	}
	if c.UsesRealNotify() && c.NotifyLINEToken == "" {
		return nil, fmt.Errorf("config: NOTIFY_LINE_TOKEN is required when NOTIFY_PROVIDER=real")
	}
	if c.UsesRealGoogle() {
		if c.GoogleClientID == "" || c.GoogleClientSecret == "" || c.GoogleCallbackURL == "" {
			return nil, fmt.Errorf("config: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_CALLBACK_URL are required when GOOGLE_PROVIDER=real")
		}
	}
	if c.UsesRealEmail() {
		if c.ACSEmailEndpoint == "" || c.ACSEmailAccessKey == "" || c.ACSEmailSender == "" {
			return nil, fmt.Errorf("config: ACS_EMAIL_ENDPOINT, ACS_EMAIL_ACCESS_KEY and ACS_EMAIL_SENDER are required when EMAIL_PROVIDER=real")
		}
	}
	if c.UsesAzureSearch() && (c.AzureSearchEndpoint == "" || c.AzureSearchKey == "") {
		return nil, fmt.Errorf("config: AZURE_SEARCH_ENDPOINT and AZURE_SEARCH_KEY are required when AI_SEARCH_PROVIDER=azure")
	}
	if c.UsesRealAuth() && (c.AzureADTenantID == "" || c.AzureADClientID == "") {
		return nil, fmt.Errorf("config: AZURE_AD_TENANT_ID and AZURE_AD_CLIENT_ID are required when AUTH_PROVIDER=real")
	}
	if !c.IsDevelopment() {
		if c.JWTSecret == "" {
			return nil, fmt.Errorf("config: JWT_SECRET is required when ENV != development")
		}
		if strings.Contains(c.CORSAllowOrigins, "localhost") || strings.Contains(c.CORSAllowOrigins, "127.0.0.1") {
			return nil, fmt.Errorf("config: CORS_ALLOW_ORIGINS must be set to real origins (not localhost) when ENV != development")
		}
	}

	return c, nil
}

// UsesRealPeopleSoft reports whether the real PS Integration Broker client should be used.
func (c *Config) UsesRealPeopleSoft() bool { return c.PSProvider == ProviderReal }

// UsesRealLINE reports whether real LINE id-token verification should be used.
func (c *Config) UsesRealLINE() bool { return c.LINEProvider == ProviderReal }

// UsesRealAuth reports whether real Azure AD (Entra) JWT validation should be
// used for the HR API. Mock (dev super_admin) is the default.
func (c *Config) UsesRealAuth() bool { return c.AuthProvider == ProviderReal }

// AllowedTenantList returns the Entra tenant IDs permitted to sign in: the
// trimmed AZURE_AD_ALLOWED_TENANTS entries when set, otherwise a single-element
// fallback to the home tenant (AzureADTenantID) so single-tenant deployments are
// unchanged.
func (c *Config) AllowedTenantList() []string {
	var out []string
	for _, t := range strings.Split(c.AzureADAllowedTenants, ",") {
		if s := strings.TrimSpace(t); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 && c.AzureADTenantID != "" {
		out = []string{c.AzureADTenantID}
	}
	return out
}

// IsMultiTenantAuth reports whether more than one Entra tenant may sign in, which
// switches the verifier to the shared `organizations` issuer plus a `tid`
// allowlist instead of a single pinned-tenant issuer.
func (c *Config) IsMultiTenantAuth() bool { return len(c.AllowedTenantList()) > 1 }

// UsesRealNotify reports whether the real notifier (LINE push / email) should be
// constructed. Mock (log-only) is the default so local/CI need no credentials.
func (c *Config) UsesRealNotify() bool { return c.NotifyProvider == ProviderReal }

// UsesRealGraph reports whether the real Microsoft Graph calendar provider should
// be constructed. Mock (log-only, returns a fake join URL) is the default.
func (c *Config) UsesRealGraph() bool { return c.GraphProvider == ProviderReal }

// UsesRealGoogle reports whether the real Google Login OAuth flow should be used
// for candidate membership. Mock (stub bounce) is the default.
func (c *Config) UsesRealGoogle() bool { return c.GoogleProvider == ProviderReal }

// UsesRealEmail reports whether the real ACS Email sender should be constructed
// for candidate email-OTP. Mock (log-only) is the default so local/CI need no creds.
func (c *Config) UsesRealEmail() bool { return c.EmailProvider == ProviderReal }

// ReportRecipientList splits the comma-separated REPORT_RECIPIENTS into trimmed,
// non-empty entries.
func (c *Config) ReportRecipientList() []string {
	var out []string
	for _, r := range strings.Split(c.ReportRecipients, ",") {
		if t := strings.TrimSpace(r); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// TrustedProxyList splits TRUSTED_PROXIES into trimmed, non-empty entries (IPs or
// CIDRs). Empty ⇒ nil ⇒ Fiber trusts no proxy ⇒ c.IP() is the direct peer.
func (c *Config) TrustedProxyList() []string {
	var out []string
	for _, p := range strings.Split(c.TrustedProxies, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// UsesAzureAI reports whether real Azure AI clients should be constructed.
func (c *Config) UsesAzureAI() bool {
	return c.AIProvider == AIProviderAzure
}

// UsesGeminiAI reports whether the Google Gemini AI clients (OCR, parser, LLM
// scorer) should be constructed. Selected by AI_PROVIDER=gemini.
func (c *Config) UsesGeminiAI() bool {
	return c.AIProvider == AIProviderGemini
}

// UsesAzureSearch reports whether the real Azure AI Search client should be used
// for candidate search. Mock (Postgres trigram) is the default.
func (c *Config) UsesAzureSearch() bool {
	return c.AISearchProvider == AIProviderAzure
}

// IsDevelopment reports whether the process is running in local development.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isOneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
