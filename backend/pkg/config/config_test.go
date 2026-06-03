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
