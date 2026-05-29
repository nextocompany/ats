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
}

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

	return c, nil
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
