package config

import "testing"

func TestLoad_MissingRequired(t *testing.T) {
	// Arrange: DB_URL explicitly empty.
	t.Setenv("DB_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "x")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error when DB_URL is missing, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Arrange: only required vars set.
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")

	// Act
	c, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HTTPPort != "8080" {
		t.Errorf("expected default HTTPPort 8080, got %q", c.HTTPPort)
	}
	if c.WorkerPort != "8081" {
		t.Errorf("expected default WorkerPort 8081, got %q", c.WorkerPort)
	}
	if c.Env != "development" {
		t.Errorf("expected default Env development, got %q", c.Env)
	}
	if !c.IsDevelopment() {
		t.Error("expected IsDevelopment to be true by default")
	}
	if c.BlobContainer != "resumes" {
		t.Errorf("expected default BlobContainer resumes, got %q", c.BlobContainer)
	}
	if c.AIProvider != "mock" {
		t.Errorf("expected default AIProvider mock, got %q", c.AIProvider)
	}
	if c.UsesAzureAI() {
		t.Error("expected UsesAzureAI false by default")
	}
}

func TestLoad_AzureRequiresKeys(t *testing.T) {
	// Arrange: azure provider selected but no Azure credentials present.
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("AI_PROVIDER", "azure")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "")
	t.Setenv("AZURE_OPENAI_KEY", "")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error when AI_PROVIDER=azure without Azure keys")
	}
}

func TestLoad_PSRealRequiresCreds(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("PS_PROVIDER", "real")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when PS_PROVIDER=real without IB creds")
	}
}

// setRealPSIBCreds sets the four PS Integration Broker creds so that only the
// webhook-secret requirement is left to assert.
func setRealPSIBCreds(t *testing.T) {
	t.Helper()
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("PS_PROVIDER", "real")
	t.Setenv("PS_IB_BASE_URL", "https://ps.example.com")
	t.Setenv("PS_IB_TOKEN_URL", "https://ps.example.com/oauth/token")
	t.Setenv("PS_IB_CLIENT_ID", "client")
	t.Setenv("PS_IB_CLIENT_SECRET", "secret")
}

func TestLoad_PSRealRequiresWebhookSecret(t *testing.T) {
	setRealPSIBCreds(t)
	t.Setenv("PS_WEBHOOK_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when PS_PROVIDER=real without PS_WEBHOOK_SECRET")
	}
}

func TestLoad_PSRealWithWebhookSecret(t *testing.T) {
	setRealPSIBCreds(t)
	t.Setenv("PS_WEBHOOK_SECRET", "shh")
	c, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed with full real PS config, got %v", err)
	}
	if c.PSWebhookSecret != "shh" {
		t.Errorf("expected PSWebhookSecret %q, got %q", "shh", c.PSWebhookSecret)
	}
}

func TestLoad_LINERealRequiresChannel(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("LINE_PROVIDER", "real")
	t.Setenv("LINE_CHANNEL_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when LINE_PROVIDER=real without channel id")
	}
}

func TestLoad_AzureWithKeys(t *testing.T) {
	// Arrange
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("AI_PROVIDER", "azure")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://x.openai.azure.com")
	t.Setenv("AZURE_OPENAI_KEY", "k")
	t.Setenv("AZURE_DOC_INTEL_ENDPOINT", "https://x.cognitiveservices.azure.com")
	t.Setenv("AZURE_DOC_INTEL_KEY", "k2")

	// Act
	c, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.UsesAzureAI() {
		t.Error("expected UsesAzureAI true when AI_PROVIDER=azure")
	}
}

func TestLoad_GeminiRequiresKey(t *testing.T) {
	// Arrange: gemini provider selected but no API key present.
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error when AI_PROVIDER=gemini without GEMINI_API_KEY")
	}
}

func TestLoad_GeminiWithKey(t *testing.T) {
	// Arrange: gemini provider with API key requires no Azure vars.
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gk")

	// Act
	c, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.UsesGeminiAI() {
		t.Error("expected UsesGeminiAI true when AI_PROVIDER=gemini")
	}
	if c.UsesAzureAI() {
		t.Error("expected UsesAzureAI false when AI_PROVIDER=gemini")
	}
	if c.GeminiModel != "gemini-2.0-flash" {
		t.Errorf("expected default GeminiModel gemini-2.0-flash, got %q", c.GeminiModel)
	}
}

func TestLoad_RetentionDefaults(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.RetentionSweepEnabled {
		t.Error("expected retention sweep disabled by default")
	}
	if c.RetentionDays != 365 {
		t.Errorf("expected RetentionDays 365, got %d", c.RetentionDays)
	}
	if c.RetentionSweepBatch != 500 {
		t.Errorf("expected RetentionSweepBatch 500, got %d", c.RetentionSweepBatch)
	}
	if c.RetentionSweepCron != "30 3 * * *" {
		t.Errorf("expected default cron '30 3 * * *', got %q", c.RetentionSweepCron)
	}
}

func TestLoad_RetentionEnabledParsed(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("RETENTION_SWEEP_ENABLED", "true")
	t.Setenv("RETENTION_DAYS", "30")
	t.Setenv("RETENTION_SWEEP_BATCH", "10")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.RetentionSweepEnabled {
		t.Error("expected RetentionSweepEnabled true")
	}
	if c.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", c.RetentionDays)
	}
	if c.RetentionSweepBatch != 10 {
		t.Errorf("expected RetentionSweepBatch 10, got %d", c.RetentionSweepBatch)
	}
}

func TestLoad_RateLimitDefault(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.RateLimitPublicMax != 30 {
		t.Errorf("expected default RateLimitPublicMax 30, got %d", c.RateLimitPublicMax)
	}

	t.Setenv("RATE_LIMIT_PUBLIC_MAX", "5")
	c2, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c2.RateLimitPublicMax != 5 {
		t.Errorf("expected RateLimitPublicMax 5, got %d", c2.RateLimitPublicMax)
	}
}

// setProdRequired sets the always-required vars plus a non-development ENV so the
// prod-only guards (JWT, CORS) are exercised.
func setProdRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	t.Setenv("ENV", "production")
}

func TestLoad_NonDevRequiresJWT(t *testing.T) {
	setProdRequired(t)
	t.Setenv("JWT_SECRET", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://hr.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when JWT_SECRET is empty outside development")
	}

	t.Setenv("JWT_SECRET", "s3cr3t")
	if _, err := Load(); err != nil {
		t.Fatalf("expected success with JWT + real CORS in prod, got %v", err)
	}
}

func TestLoad_NonDevRejectsLocalhostCORS(t *testing.T) {
	setProdRequired(t)
	t.Setenv("JWT_SECRET", "s3cr3t")
	// Default CORS (localhost) must be rejected in prod.
	if _, err := Load(); err == nil {
		t.Fatal("expected error when CORS_ALLOW_ORIGINS is localhost outside development")
	}

	t.Setenv("CORS_ALLOW_ORIGINS", "https://hr.example.com,https://careers.example.com")
	if _, err := Load(); err != nil {
		t.Fatalf("expected success with real CORS origins in prod, got %v", err)
	}
}

func TestLoad_InvalidProviderValue(t *testing.T) {
	base := func() {
		t.Setenv("DB_URL", "postgres://localhost/db")
		t.Setenv("REDIS_URL", "redis://localhost:6379")
		t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
	}
	// AI_PROVIDER uses "azure"; "real" is a typo that must fail fast.
	base()
	t.Setenv("AI_PROVIDER", "real")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for AI_PROVIDER=real")
	}

	// AUTH_PROVIDER uses "real"; "azure" is a typo that must fail fast.
	t.Setenv("AI_PROVIDER", "mock")
	t.Setenv("AUTH_PROVIDER", "azure")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for AUTH_PROVIDER=azure")
	}
}

func TestLoad_TrustedProxyList(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.TrustedProxyList()) != 0 {
		t.Errorf("expected empty TrustedProxyList by default, got %v", c.TrustedProxyList())
	}

	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8 , 100.64.0.1 ,")
	c2, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := c2.TrustedProxyList()
	want := []string{"10.0.0.0/8", "100.64.0.1"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("expected %v, got %v", want, got)
	}
}
