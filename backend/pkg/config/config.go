// Package config loads and validates all runtime configuration from the
// environment at startup. It fails fast when a required variable is missing so
// the process never binds a port in a half-configured state.
package config

import (
	"fmt"
	"os"
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

	// AIProvider selects the OCR/parse implementation: "mock" (default) or "azure".
	AIProvider string

	// Azure AI settings — required only when AIProvider == "azure".
	AzureOpenAIEndpoint   string
	AzureOpenAIKey        string
	AzureOpenAIDeployment string
	AzureDocIntelEndpoint string
	AzureDocIntelKey      string

	// PeopleSoft Integration Broker — "mock" (default) or "real".
	PSProvider             string
	PSIBBaseURL            string
	PSIBTokenURL           string
	PSIBClientID           string
	PSIBClientSecret       string
	PSCSVFallbackContainer string

	// LINE Login — "mock" (default) or "real".
	LINEProvider  string
	LINEChannelID string

	// CORSAllowOrigins is the comma-separated allowlist for browser clients.
	CORSAllowOrigins string
}

// Provider values selecting real (vs mock) integrations.
const (
	AIProviderAzure = "azure"
	ProviderReal    = "real"
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

		PSProvider:             getenv("PS_PROVIDER", "mock"),
		PSIBBaseURL:            os.Getenv("PS_IB_BASE_URL"),
		PSIBTokenURL:           os.Getenv("PS_IB_TOKEN_URL"),
		PSIBClientID:           os.Getenv("PS_IB_CLIENT_ID"),
		PSIBClientSecret:       os.Getenv("PS_IB_CLIENT_SECRET"),
		PSCSVFallbackContainer: getenv("PS_CSV_FALLBACK_CONTAINER", "ps-export"),

		LINEProvider:  getenv("LINE_PROVIDER", "mock"),
		LINEChannelID: os.Getenv("LINE_CHANNEL_ID"),

		CORSAllowOrigins: getenv("CORS_ALLOW_ORIGINS", "http://localhost:3000,http://localhost:3001"),
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
	if c.UsesAzureAI() {
		if c.AzureOpenAIEndpoint == "" || c.AzureOpenAIKey == "" {
			return nil, fmt.Errorf("config: AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_KEY are required when AI_PROVIDER=azure")
		}
		if c.AzureDocIntelEndpoint == "" || c.AzureDocIntelKey == "" {
			return nil, fmt.Errorf("config: AZURE_DOC_INTEL_ENDPOINT and AZURE_DOC_INTEL_KEY are required when AI_PROVIDER=azure")
		}
	}
	if c.UsesRealPeopleSoft() {
		if c.PSIBBaseURL == "" || c.PSIBTokenURL == "" || c.PSIBClientID == "" || c.PSIBClientSecret == "" {
			return nil, fmt.Errorf("config: PS_IB_BASE_URL, PS_IB_TOKEN_URL, PS_IB_CLIENT_ID, PS_IB_CLIENT_SECRET are required when PS_PROVIDER=real")
		}
	}
	if c.UsesRealLINE() && c.LINEChannelID == "" {
		return nil, fmt.Errorf("config: LINE_CHANNEL_ID is required when LINE_PROVIDER=real")
	}

	return c, nil
}

// UsesRealPeopleSoft reports whether the real PS Integration Broker client should be used.
func (c *Config) UsesRealPeopleSoft() bool { return c.PSProvider == ProviderReal }

// UsesRealLINE reports whether real LINE id-token verification should be used.
func (c *Config) UsesRealLINE() bool { return c.LINEProvider == ProviderReal }

// UsesAzureAI reports whether real Azure AI clients should be constructed.
func (c *Config) UsesAzureAI() bool {
	return c.AIProvider == AIProviderAzure
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
