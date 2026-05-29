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
}
