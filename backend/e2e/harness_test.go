//go:build e2e

// Package e2e drives the whole system over HTTP against the live docker stack
// (make up + migrate + seed), fully offline via the deterministic mocks. It uses
// the DB only for test setup and for polling async pipeline completion (polling
// the public /status endpoint would hit the 6a rate limiter).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func apiBase() string {
	if v := os.Getenv("E2E_API_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn())
	if err != nil {
		t.Fatalf("e2e: connect db (stack up + migrated?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedPositionStore truncates and seeds one store + one lenient, PS-mapped
// position so the webhook can open a vacancy and the mock profile scores+passes.
func seedPositionStore(t *testing.T, pool *pgxpool.Pool, psCode string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE applications, candidates, positions, stores, vacancies, reengagement_contacts, report_exports RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stores (store_no, store_name, subregion, province) VALUES (1,'E2E Store','East','Bangkok')`); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	var posID uuid.UUID
	// Lenient must-have so the mock profile (ปวส., 24mo) passes the gate.
	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th, ps_position_code, must_have_criteria, is_active)
		 VALUES ('แคชเชียร์ E2E', $1, '{"min_education_level":1,"min_experience_months":6}'::jsonb, TRUE) RETURNING id`,
		psCode).Scan(&posID); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return posID
}

// --- HTTP helpers ---

func postJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(apiBase()+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func patchJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, apiBase()+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	return resp
}

// getEnvelope GETs a path and decodes the {success,data,meta} envelope's data into out.
func getEnvelope(t *testing.T, path string, out any) (int, json.RawMessage) {
	t.Helper()
	resp, err := http.Get(apiBase() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Meta    json.RawMessage `json:"meta"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if out != nil && len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, out)
	}
	return resp.StatusCode, env.Meta
}

// applyMultipart submits the public apply form (deterministic mock resume) and
// returns the status_token. Mirrors the career-portal FormData + X-LINE-IdToken.
func applyMultipart(t *testing.T, positionID uuid.UUID, fullName string) string {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("position_id", positionID.String())
	_ = w.WriteField("full_name", fullName)
	// phone + email are required on apply since #165 (UAT bug #6).
	_ = w.WriteField("phone", "0810000000")
	_ = w.WriteField("email", "e2e.fullflow@example.com")
	_ = w.WriteField("consent_given", "true")
	_ = w.WriteField("consent_version", "1.0")
	// Explicit part Content-Type — the API maps it to a file type; the multipart
	// default (octet-stream) is rejected with 415.
	ph := textproto.MIMEHeader{}
	ph.Set("Content-Disposition", `form-data; name="resume"; filename="cv.pdf"`)
	ph.Set("Content-Type", "application/pdf")
	fw, _ := w.CreatePart(ph)
	_, _ = fw.Write([]byte("%PDF-1.4 e2e resume"))
	_ = w.Close()

	req, _ := http.NewRequest(http.MethodPost, apiBase()+"/api/v1/public/apply", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-LINE-IdToken", "e2e-stub")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("apply expected 201, got %d: %s", resp.StatusCode, b)
	}
	var env struct {
		Data struct {
			StatusToken string `json:"status_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Data.StatusToken == "" {
		t.Fatal("apply returned empty status_token")
	}
	return env.Data.StatusToken
}

// pollDB waits (bounded) for fn to return true, querying the DB for async state.
func pollDB(t *testing.T, deadline time.Duration, fn func(ctx context.Context) bool) {
	t.Helper()
	ctx := context.Background()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn(ctx) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("e2e: condition not met before deadline")
}

func waitHealthy(t *testing.T) {
	t.Helper()
	for i := 0; i < 40; i++ {
		resp, err := http.Get(apiBase() + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("e2e: api not healthy at %s", apiBase())
}

func TestHarness_Health(t *testing.T) {
	waitHealthy(t)
	_ = fmt.Sprint // keep fmt imported for failure messages
}
