# Plan: PDPA Retention Sweep

## Summary
A scheduler-driven background job that erases personally identifiable information (PII) for candidates whose documented retention window (≤ 1 year) has elapsed and who are no longer in an active recruitment pipeline. It **anonymizes in place** (redacts PII columns + deletes resume blobs) rather than hard-deleting rows, because six tables reference `candidates(id)` and the de-identified recruitment/analytics history must survive. The sweep is gated OFF by default behind a `RETENTION_SWEEP_ENABLED` seam so CI, local dev, and mock environments never purge data.

## User Story
As a **data-protection officer for the Thai retail client**, I want **candidate PII automatically erased once the retention period lapses**, so that **the platform honours the ≤ 1-year retention promise made to candidates and stays compliant with Thai PDPA without manual data scrubbing**.

## Problem → Solution
**Current state:** SECURITY.md documents a ≤ 1-year retention/anonymisation promise to candidates, but `docs/SECURITY.md:48-49` explicitly notes "operationalising the retention sweep is a future task." PII (name/phone/email/id_card, resume blobs) accumulates indefinitely in Postgres + Blob.
**Desired state:** A daily, single-replica scheduled job anonymizes expired candidates in bounded batches — redacting candidate PII columns, redacting application PII columns, deleting resume blobs, nulling consent IP addresses, and writing an audit log entry per candidate — idempotently and retry-safely, gated behind an explicit enable flag.

## Metadata
- **Complexity**: Medium-Large
- **Source PRD**: N/A (free-form — Sprint 7 slice from session `2026-06-03-s7-ps-hmac`)
- **PRD Phase**: N/A (standalone)
- **Estimated Files**: 13 (5 new, 8 updated)

---

## UX Design

### Before
N/A — internal/backend change. No HR dashboard or career-portal surface.

### After
N/A — internal/backend change. The only observable effects are: (1) anonymized candidate rows show `full_name = '[ลบข้อมูลแล้ว]'` with null contact fields in any existing list/detail view; (2) an `activity_logs` entry of action `retention_anonymize` per swept candidate. No new endpoints, no UI work.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Scheduler | enqueues `report:export` only | also enqueues `retention:sweep` on `RETENTION_SWEEP_CRON` | second `scheduler.Register` call |
| Worker | handles 3 task types | handles 4 (adds `retention:sweep`) | mirrors `report:export` wiring |
| Candidate row (expired) | full PII retained | name redacted, contact/id PII nulled, `pdpa_anonymized_at` set | aggregate fields (province, status, scores) preserved |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 (critical) | `backend/internal/reports/export_service.go` | 1-117 | Closest service analogue: persist-before-side-effect ordering, best-effort delivery, interface-narrowing for blob |
| P0 (critical) | `backend/internal/reports/worker.go` | 1-37 | Exact asynq handler shape to mirror for the sweep handler |
| P0 (critical) | `backend/pkg/queue/tasks.go` | 90-125 | `ExportReport` task type/payload/New/Parse + `asynq.Unique` pattern to mirror |
| P0 (critical) | `backend/cmd/scheduler/main.go` | 1-48 | How a cron task is registered; add a second `Register` |
| P0 (critical) | `backend/cmd/worker/main.go` | 100-148 | Service construction + `mux.HandleFunc` wiring |
| P0 (critical) | `backend/pkg/config/config.go` | 73-209, 227-233 | Field block, defaults, fail-fast block, `getenv` helper, list-splitter |
| P1 (important) | `backend/internal/pdpa/pdpa.go` | 1-65 | Existing pdpa package: tx pattern (`Begin`/`Exec`/`Commit`/`defer Rollback`), error-wrap style, `Repo` shape |
| P1 (important) | `backend/internal/reengage/repository.go` | 39-90 | Batch SELECT → scan-loop → `rows.Err()` pattern for the eligibility query |
| P1 (important) | `backend/pkg/blob/blob.go` | 60-89 | Upload/SignedURLForStored — mirror for new `Delete`/`DeleteStored`; azblob SDK + key-derivation pattern |
| P1 (important) | `backend/internal/activity/activity.go` | 1-60 | `Writer.Record(ctx, action, entityType, entityID, newValue)` audit contract |
| P2 (reference) | `backend/internal/reports/export_integration_test.go` | 1-60 | `//go:build integration`, `setup(t)`, TRUNCATE…RESTART IDENTITY CASCADE, fake blob/notifier |
| P2 (reference) | `backend/internal/reports/reports_integration_test.go` | 14-22 | `dsn()` helper + default DSN fallback |
| P2 (reference) | `backend/pkg/config/config_test.go` | 1-120 | Unit-test structure for config defaults + fail-fast assertions |
| P2 (reference) | `backend/migrations/000007_report_exports.up.sql` | all | Migration file format + comment style + index naming |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| azblob delete | `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` (already vendored) | `client.DeleteBlob(ctx, container, name, nil)` deletes a blob; wrap "not found" as success via `bloberror.HasCode(err, bloberror.BlobNotFound)` so a re-run after partial progress is idempotent |
| asynq scheduler | `github.com/hibiken/asynq` (already vendored) | `scheduler.Register(cron, task)` can be called multiple times for distinct task types; each tick enqueues a fresh copy |

No further external research needed — feature uses established internal patterns.

---

## Patterns to Mirror

### NAMING_CONVENTION
```go
// SOURCE: backend/pkg/queue/tasks.go:18-20
// TypeExportReport is the asynq task type for generating + delivering a recurring
// or on-demand report export (Sprint 5b).
const TypeExportReport = "report:export"
```
→ New: `const TypeRetentionSweep = "retention:sweep"`. Service type `RetentionService`; constructor `NewRetentionService(...)`. Method `Sweep`. Handler `HandleRetentionSweep`. Package stays `pdpa`.

### ERROR_HANDLING
```go
// SOURCE: backend/internal/pdpa/pdpa.go:29-51
tx, err := r.pool.Begin(ctx)
if err != nil {
	return fmt.Errorf("pdpa: begin: %w", err)
}
defer func() { _ = tx.Rollback(ctx) }()
// ... tx.Exec(...) each wrapped: fmt.Errorf("pdpa: <op>: %w", err)
if err := tx.Commit(ctx); err != nil {
	return fmt.Errorf("pdpa: commit: %w", err)
}
```
→ All new errors wrapped `fmt.Errorf("pdpa: <op>: %w", err)`.

### LOGGING_PATTERN
```go
// SOURCE: backend/internal/reports/worker.go:35
log.Info().Str("kind", kind).Str("period", period).Bool("delivered", exp.Delivered).Msg("reports: export produced")
// SOURCE: backend/internal/reports/export_service.go:78
log.Warn().Err(err).Str("period", period).Msg("reports: mark delivered failed")
```
→ `log.Info().Int("anonymized", n).Int("batch", batch).Msg("pdpa: retention sweep complete")`; best-effort failures use `log.Warn().Err(err)...`. NEVER log PII values (name/phone/email/id_card) — log candidate UUIDs only.

### REPOSITORY_PATTERN (batch select)
```go
// SOURCE: backend/internal/reengage/repository.go:43-71
rows, err := r.pool.Query(ctx, q, positionID)
if err != nil {
	return nil, fmt.Errorf("reengage: matching candidates: %w", err)
}
defer rows.Close()
var out []Target
for rows.Next() {
	var t Target
	if err := rows.Scan(&t.CandidateID, ...); err != nil {
		return nil, fmt.Errorf("reengage: scan target: %w", err)
	}
	out = append(out, t)
}
return out, rows.Err()
```

### SERVICE_PATTERN (persist/side-effect ordering + narrow interface)
```go
// SOURCE: backend/internal/reports/export_service.go:18-36
type BlobStore interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}
type ExportService struct {
	repo *Repo; blob BlobStore; notifier notify.Notifier; recipients []string
}
func NewExportService(repo *Repo, blob BlobStore, notifier notify.Notifier, recipients []string) *ExportService { ... }
```
→ `RetentionService` takes a narrow `BlobDeleter interface { DeleteStored(ctx, storedURL string) error }`, the `*pgxpool.Pool`, an `activity.Writer`, and `retentionDays int`. Commit DB redaction first, then best-effort blob deletes (a blob error must NOT roll back the DB; the URL pointer is already gone).

### HANDLER_PATTERN (asynq)
```go
// SOURCE: backend/internal/reports/worker.go:17-37
func (s *ExportService) HandleExportReport(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseExportReportPayload(t.Payload())
	if err != nil { return err }
	// ... derive defaults, do work, log, return nil
}
```

### CONFIG_PATTERN
```go
// SOURCE: backend/pkg/config/config.go:135-136, 227-233
ReportScheduleCron: getenv("REPORT_SCHEDULE_CRON", "0 7 * * 1"),
// ...
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
```
→ Add `getenvInt(key string, fallback int) int` and `getenvBool(key string, fallback bool) bool` using `strconv.Atoi` / `strconv.ParseBool` (on parse error, return fallback). Place beside `getenv`.

### TEST_STRUCTURE (integration)
```go
// SOURCE: backend/internal/reports/export_integration_test.go:1-50
//go:build integration
func setup(t *testing.T) *Repo {
	t.Helper()
	pool, _ := pgxpool.New(ctx, dsn())
	t.Cleanup(pool.Close)
	pool.Exec(ctx, `TRUNCATE report_exports, applications, candidates, positions RESTART IDENTITY CASCADE`)
	// seed positions/candidates/applications
}
```

### MIGRATION_FORMAT
```sql
-- SOURCE: backend/migrations/000007_report_exports.up.sql
-- <comment explaining intent + idempotency/index rationale>
CREATE TABLE ... ;
CREATE INDEX idx_... ON ... ;
-- down: DROP ...
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000009_pdpa_retention.up.sql` | CREATE | Add `candidates.pdpa_anonymized_at TIMESTAMPTZ` + partial index for sweep efficiency |
| `backend/migrations/000009_pdpa_retention.down.sql` | CREATE | Drop the index + column |
| `backend/internal/pdpa/retention.go` | CREATE | `RetentionService`: eligibility query, per-candidate anonymize tx, `Sweep(ctx, batch)` |
| `backend/internal/pdpa/worker.go` | CREATE | `HandleRetentionSweep` asynq handler (enable-gate + batch derivation) |
| `backend/internal/pdpa/retention_integration_test.go` | CREATE | Integration test for eligibility + anonymization over seeded data |
| `backend/pkg/queue/tasks.go` | UPDATE | Add `TypeRetentionSweep`, `RetentionSweepPayload`, `New*`/`Parse*` (mirror export, with `asynq.Unique`) |
| `backend/pkg/config/config.go` | UPDATE | Add 4 retention fields + defaults + `getenvInt`/`getenvBool`; no fail-fast (safe default OFF) |
| `backend/pkg/config/config_test.go` | UPDATE | +2 tests: retention defaults; enabled+parsed values |
| `backend/pkg/blob/blob.go` | UPDATE | Add `Delete(ctx, name)` + `DeleteStored(ctx, storedURL)` (idempotent on not-found) |
| `backend/cmd/scheduler/main.go` | UPDATE | Register `retention:sweep` on `RetentionSweepCron` (second `Register`) |
| `backend/cmd/worker/main.go` | UPDATE | Construct `RetentionService`, `mux.HandleFunc(queue.TypeRetentionSweep, ...)` |
| `.env.example` (repo ROOT) | UPDATE | Add `RETENTION_SWEEP_ENABLED`, `RETENTION_DAYS`, `RETENTION_SWEEP_CRON`, `RETENTION_SWEEP_BATCH` with a retention-policy comment |
| `docs/SECURITY.md` | UPDATE | Flip line 48-49 "future task" → implemented; document the anonymize-in-place model + enable flag |

## NOT Building

- **Hard deletion of candidate rows** — six FKs reference `candidates(id)`; we anonymize in place to preserve referential integrity and de-identified analytics. Out of scope.
- **An on-demand HTTP/admin endpoint to trigger the sweep** — scheduler-driven only (YAGNI; no caller asked for it). The `report:export` slice has a manual `POST /exports`, but retention is destructive and should not be web-triggerable.
- **A per-candidate "right to erasure on request" API** — that is a separate self-service flow; this sweep is the time-based bulk job only.
- **Configurable per-field or per-status retention policies** — single global `RETENTION_DAYS` window. 
- **Anonymizing the `pdpa_consents` consent ledger rows themselves** — we keep the consent records (legal proof consent was given) but null their `ip_address` (the only PII they carry).
- **Dashboard surfacing of anonymized state / retention metrics** — no frontend work.
- **Hashing/tokenizing PII for reversible pseudonymization** — true erasure (null), not reversible.

---

## Step-by-Step Tasks

### Task 1: Migration — `pdpa_anonymized_at` column + sweep index
- **ACTION**: Create `backend/migrations/000009_pdpa_retention.up.sql` and `.down.sql`.
- **IMPLEMENT** (up):
  ```sql
  -- Sprint 7: PDPA retention sweep. pdpa_anonymized_at marks candidates whose PII
  -- has been erased so the daily sweep is idempotent (re-runs skip set rows). The
  -- partial index keeps the eligibility scan cheap as the table grows: only rows
  -- still pending anonymization are indexed by their retention clock (created_at).
  ALTER TABLE candidates ADD COLUMN pdpa_anonymized_at TIMESTAMPTZ;

  CREATE INDEX idx_candidates_retention_pending
      ON candidates (created_at)
      WHERE pdpa_anonymized_at IS NULL;
  ```
- **IMPLEMENT** (down):
  ```sql
  DROP INDEX IF EXISTS idx_candidates_retention_pending;
  ALTER TABLE candidates DROP COLUMN IF EXISTS pdpa_anonymized_at;
  ```
- **MIRROR**: MIGRATION_FORMAT (000007).
- **GOTCHA**: Migration numbering is strictly sequential; 000008 is the last existing one, so this is 000009. Keep the partial-index `WHERE` clause IMMUTABLE-safe (it is — `IS NULL` only).
- **VALIDATE**: `make migrate-up` applies cleanly; `make migrate-down` then `make migrate-up` round-trips. `\d candidates` shows the new column.

### Task 2: Config — retention fields + getenvInt/getenvBool
- **ACTION**: Add fields to `Config`, defaults in `Load`, and two helpers in `backend/pkg/config/config.go`.
- **IMPLEMENT**:
  - In the struct (near the Report scheduler block, lines 74-77), add:
    ```go
    // Retention sweep (Sprint 7, F-PDPA): daily anonymization of expired candidate
    // PII. Disabled by default — a destructive job must be explicitly enabled per
    // environment so CI/dev/mock never purge. RetentionDays is the ≤1-year window.
    RetentionSweepEnabled bool
    RetentionDays         int
    RetentionSweepCron    string
    RetentionSweepBatch   int
    ```
  - In `Load`'s struct literal (near lines 135-136):
    ```go
    RetentionSweepEnabled: getenvBool("RETENTION_SWEEP_ENABLED", false),
    RetentionDays:         getenvInt("RETENTION_DAYS", 365),
    RetentionSweepCron:    getenv("RETENTION_SWEEP_CRON", "30 3 * * *"), // daily 03:30
    RetentionSweepBatch:   getenvInt("RETENTION_SWEEP_BATCH", 500),
    ```
  - Beside `getenv` (line 227), add:
    ```go
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
    ```
- **MIRROR**: CONFIG_PATTERN.
- **IMPORTS**: add `"strconv"` to config.go imports.
- **GOTCHA**: Do NOT add a fail-fast in the validation block — the safe default (disabled) means no required vars. A `RetentionDays <= 0` would erase everything immediately; clamp defensively in the service (Task 4), not in config, to keep config a pure reader.
- **VALIDATE**: `go build ./...`; `go test ./pkg/config/...`.

### Task 3: Queue task — `retention:sweep`
- **ACTION**: Append to `backend/pkg/queue/tasks.go`.
- **IMPLEMENT**:
  ```go
  // TypeRetentionSweep is the asynq task type for the daily PDPA retention sweep
  // (Sprint 7): anonymize candidates whose retention window has elapsed.
  const TypeRetentionSweep = "retention:sweep"

  // RetentionSweepPayload is the job body for a retention sweep. Both fields are
  // optional; the handler derives defaults from config when zero.
  type RetentionSweepPayload struct {
      Batch int `json:"batch"` // max candidates per run; 0 → config default
  }

  // retentionUniqueTTL dedups overlapping sweep enqueues during a rolling deploy.
  const retentionUniqueTTL = 1 * time.Hour

  func NewRetentionSweepTask(p RetentionSweepPayload) (*asynq.Task, error) {
      body, err := json.Marshal(p)
      if err != nil {
          return nil, fmt.Errorf("queue: marshal payload: %w", err)
      }
      return asynq.NewTask(
          TypeRetentionSweep, body,
          asynq.MaxRetry(taskMaxRetry),
          asynq.Timeout(taskTimeout),
          asynq.Retention(taskRetention),
          asynq.Unique(retentionUniqueTTL),
      ), nil
  }

  func ParseRetentionSweepPayload(body []byte) (RetentionSweepPayload, error) {
      var p RetentionSweepPayload
      if err := json.Unmarshal(body, &p); err != nil {
          return p, fmt.Errorf("queue: unmarshal payload: %w", err)
      }
      return p, nil
  }
  ```
- **MIRROR**: `ExportReport` task block (tasks.go:90-125).
- **GOTCHA**: `taskTimeout` is 90s. A 500-row batch of small UPDATEs + blob deletes fits comfortably; if blob deletes dominate, the daily re-run drains any remainder. Do not raise the shared timeout.
- **VALIDATE**: `go build ./...`; `go test ./pkg/queue/...` (if present) else build only.

### Task 4: Service — `RetentionService` + `Sweep`
- **ACTION**: Create `backend/internal/pdpa/retention.go`.
- **IMPLEMENT**:
  - Narrow blob interface + struct:
    ```go
    // BlobDeleter is the subset of blob.Client the sweep needs.
    type BlobDeleter interface {
        DeleteStored(ctx context.Context, storedURL string) error
    }

    // RetentionService anonymizes candidates whose retention window has elapsed.
    type RetentionService struct {
        pool          *pgxpool.Pool
        blob          BlobDeleter
        audit         activity.Writer
        retentionDays int
    }

    func NewRetentionService(pool *pgxpool.Pool, blob BlobDeleter, audit activity.Writer, retentionDays int) *RetentionService {
        return &RetentionService{pool: pool, blob: blob, audit: audit, retentionDays: retentionDays}
    }
    ```
  - `Sweep(ctx, batch)` flow:
    1. Defensive clamp: `days := s.retentionDays; if days <= 0 { days = 365 }`. `if batch <= 0 { batch = 500 }`.
    2. Select eligible candidate IDs (eligibility query below), `LIMIT batch`.
    3. For each candidate: gather its resume blob URLs (SELECT non-null `resume_blob_url` from applications), run the anonymize tx (commit), then best-effort delete blobs, then best-effort `audit.Record`.
    4. Return count anonymized.
  - **Eligibility query** (a candidate is eligible when window elapsed AND no active application):
    ```sql
    SELECT c.id
    FROM candidates c
    WHERE c.pdpa_anonymized_at IS NULL
      AND c.created_at < NOW() - make_interval(days => $1)
      AND NOT EXISTS (
          SELECT 1 FROM applications a
          WHERE a.candidate_id = c.id
            AND a.status IN ('pending','parsed','scored')
      )
    ORDER BY c.created_at
    LIMIT $2
    ```
  - **Gather blob URLs** (before redaction, so we still have the pointers):
    ```sql
    SELECT resume_blob_url FROM applications
    WHERE candidate_id = $1 AND resume_blob_url IS NOT NULL AND resume_blob_url <> ''
    ```
  - **Anonymize tx** (mirror `pdpa.Record` tx structure):
    ```sql
    -- candidate PII → redacted; aggregate fields (province, subregion, source_channel, status) kept
    UPDATE candidates SET
        full_name = '[ลบข้อมูลแล้ว]',
        phone = NULL, email = NULL, id_card = NULL,
        address = NULL, date_of_birth = NULL,
        pdpa_anonymized_at = NOW(), updated_at = NOW()
    WHERE id = $1 AND pdpa_anonymized_at IS NULL;  -- re-check guards double-anonymize under concurrent runs

    -- application PII pointers/free-text → redacted; scores/status kept for analytics
    UPDATE applications SET
        resume_blob_url = NULL, resume_original_filename = NULL,
        ai_summary = NULL, ai_red_flags = NULL, updated_at = NOW()
    WHERE candidate_id = $1;

    -- consent ledger kept as legal proof; only its PII (ip) erased
    UPDATE pdpa_consents SET ip_address = NULL WHERE candidate_id = $1;
    ```
  - Best-effort blob delete per URL: `if err := s.blob.DeleteStored(ctx, url); err != nil { log.Warn().Err(err).Str("candidate_id", id).Msg("pdpa: resume blob delete failed") }`.
  - Best-effort audit: `s.audit.Record(ctx, "retention_anonymize", "candidate", id, map[string]any{"reason":"retention_window_elapsed"})` — log a Warn on error, do not abort.
- **MIRROR**: SERVICE_PATTERN (export_service), ERROR_HANDLING + tx (pdpa.go), REPOSITORY_PATTERN (reengage select loop).
- **IMPORTS**: `context`, `fmt`, `time` (only if used), `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/rs/zerolog/log`, `github.com/nexto/hr-ats/internal/activity`.
- **GOTCHA**:
  - The candidate UPDATE keeps `WHERE ... AND pdpa_anonymized_at IS NULL` so two overlapping sweeps can't double-process (the unique-TTL on the task already makes this rare).
  - Commit the DB tx BEFORE deleting blobs — a blob failure must not roll back redaction (the DB pointer is already gone; the orphaned blob is unreachable and the next run won't re-find it since the candidate is now marked). This mirrors reports' "persist before side-effect."
  - NEVER log redacted values; log `candidate_id` UUIDs only.
  - Add `activity.ActionRetentionAnonymize = "retention_anonymize"` const to `activity.go` for consistency with the existing action constants (optional but matches the codebase; if added, update the call site).
- **VALIDATE**: `go build ./...`; `golangci-lint run ./internal/pdpa/...`.

### Task 5: Worker handler — `HandleRetentionSweep`
- **ACTION**: Create `backend/internal/pdpa/worker.go`.
- **IMPLEMENT**:
  ```go
  func (s *RetentionService) HandleRetentionSweep(ctx context.Context, t *asynq.Task) error {
      p, err := queue.ParseRetentionSweepPayload(t.Payload())
      if err != nil {
          return err
      }
      n, err := s.Sweep(ctx, p.Batch)
      if err != nil {
          return err
      }
      log.Info().Int("anonymized", n).Msg("pdpa: retention sweep complete")
      return nil
  }
  ```
- **MIRROR**: HANDLER_PATTERN (reports/worker.go).
- **NOTE**: The enable-gate lives at the wiring layer (Task 7) — the scheduler only registers the cron when `RetentionSweepEnabled`. The handler itself stays pure so a manually-enqueued task in an enabled env still works. (Alternative: gate inside handler. Chosen: gate at scheduler so disabled envs never enqueue, and the worker need not carry the flag.)
- **IMPORTS**: `context`, `github.com/hibiken/asynq`, `github.com/rs/zerolog/log`, `github.com/nexto/hr-ats/pkg/queue`.
- **VALIDATE**: `go build ./...`.

### Task 6: Blob — `Delete` + `DeleteStored`
- **ACTION**: Add two methods to `backend/pkg/blob/blob.go`.
- **IMPLEMENT**:
  ```go
  // Delete removes the named blob. A missing blob is treated as success so the
  // retention sweep is idempotent across re-runs.
  func (c *Client) Delete(ctx context.Context, name string) error {
      _, err := c.client.DeleteBlob(ctx, c.container, name, nil)
      if err != nil && !bloberror.HasCode(err, bloberror.BlobNotFound) {
          return fmt.Errorf("blob: delete %q: %w", name, err)
      }
      return nil
  }

  // DeleteStored derives the blob key from a previously stored full URL and
  // deletes it. Mirrors SignedURLForStored's key derivation.
  func (c *Client) DeleteStored(ctx context.Context, storedURL string) error {
      marker := "/" + c.container + "/"
      i := strings.Index(storedURL, marker)
      if i < 0 {
          return fmt.Errorf("blob: cannot derive key from %q", storedURL)
      }
      return c.Delete(ctx, storedURL[i+len(marker):])
  }
  ```
- **MIRROR**: `SignedURLForStored` (blob.go:82-89) for key derivation; `ensureContainer` (blob.go:41) for the `bloberror.HasCode` idempotency idiom.
- **GOTCHA**: `bloberror` is already imported (blob.go:14). No new imports.
- **VALIDATE**: `go build ./...`; `gosec ./pkg/blob/...` exit 0.

### Task 7: Scheduler wiring — register `retention:sweep`
- **ACTION**: Update `backend/cmd/scheduler/main.go`.
- **IMPLEMENT**: after the existing `report:export` registration, gate + register:
  ```go
  if cfg.RetentionSweepEnabled {
      sweepTask, err := queue.NewRetentionSweepTask(queue.RetentionSweepPayload{})
      if err != nil {
          log.Fatal().Err(err).Msg("build retention sweep task failed")
      }
      sweepID, err := scheduler.Register(cfg.RetentionSweepCron, sweepTask)
      if err != nil {
          log.Fatal().Err(err).Str("cron", cfg.RetentionSweepCron).Msg("register retention sweep failed")
      }
      log.Info().Str("cron", cfg.RetentionSweepCron).Str("entry_id", sweepID).Msg("scheduler: retention:sweep registered")
  } else {
      log.Info().Msg("scheduler: retention sweep disabled (RETENTION_SWEEP_ENABLED=false)")
  }
  ```
- **MIRROR**: existing `report:export` Register block (scheduler/main.go:38-46).
- **GOTCHA**: Scheduler must remain single-replica (documented in its header) — the `asynq.Unique` TTL is a belt-and-braces guard for rolling deploys, not a license to scale it.
- **VALIDATE**: `go build ./cmd/scheduler`.

### Task 8: Worker wiring — construct service + register handler
- **ACTION**: Update `backend/cmd/worker/main.go`.
- **IMPLEMENT**:
  - After the `exportSvc` construction (worker/main.go ~126), add:
    ```go
    // Retention sweep (Sprint 7): anonymize expired candidate PII.
    retentionSvc := pdpa.NewRetentionService(pool, blobClient, activity.New(pool), cfg.RetentionDays)
    ```
  - In the mux block (after the export handler, ~141):
    ```go
    mux.HandleFunc(queue.TypeRetentionSweep, retentionSvc.HandleRetentionSweep)
    ```
  - Update the startup log message to mention `retention:sweep`.
- **MIRROR**: `reengageSvc` / `exportSvc` construction + `mux.HandleFunc` lines.
- **IMPORTS**: add `"github.com/nexto/hr-ats/internal/pdpa"` (and `internal/activity` is already imported).
- **GOTCHA**: `blobClient` is `*blob.Client`, which now satisfies `pdpa.BlobDeleter` via the new `DeleteStored`. `activity.New(pool)` returns `*activity.Log` which satisfies `activity.Writer`.
- **VALIDATE**: `go build ./cmd/worker`.

### Task 9: Env + docs
- **ACTION**: Update repo-root `.env.example` and `docs/SECURITY.md`.
- **IMPLEMENT** (`.env.example`, near the report scheduler vars):
  ```bash
  # PDPA retention sweep (Sprint 7). Disabled by default — enable ONLY in environments
  # that should erase candidate PII after the retention window. RETENTION_DAYS is the
  # documented ≤1-year promise. Cron is single-replica (runs on the scheduler service).
  RETENTION_SWEEP_ENABLED=false
  RETENTION_DAYS=365
  RETENTION_SWEEP_CRON=30 3 * * *
  RETENTION_SWEEP_BATCH=500
  ```
- **IMPLEMENT** (`docs/SECURITY.md`, replace lines 48-49):
  ```markdown
  - **Retention**: candidate PII (name/phone/email/id_card/address/DOB, resume blobs) is anonymized in
    place ≤ 1 year after intake by a daily scheduled sweep (`retention:sweep`, Sprint 7). Rows are
    de-identified, not deleted, to preserve referential integrity + aggregate analytics; resume blobs are
    removed from storage and consent-ledger IPs nulled. The sweep is gated behind `RETENTION_SWEEP_ENABLED`
    (off by default) and skips candidates still in an active pipeline. Each anonymization writes a
    `retention_anonymize` audit log entry.
  ```
- **GOTCHA**: Use the Read tool on `docs/SECURITY.md` and `.env.example` BEFORE editing — the Edit tool requires a prior Read (a session lesson: shell grep/sed does not count).
- **VALIDATE**: `grep -n RETENTION .env.example` shows 4 keys; SECURITY.md no longer says "future task".

### Task 10: Config tests
- **ACTION**: Add to `backend/pkg/config/config_test.go`.
- **IMPLEMENT**:
  ```go
  func TestLoad_RetentionDefaults(t *testing.T) {
      t.Setenv("DB_URL", "postgres://localhost/db")
      t.Setenv("REDIS_URL", "redis://localhost:6379")
      t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
      c, err := Load()
      if err != nil { t.Fatalf("unexpected error: %v", err) }
      if c.RetentionSweepEnabled { t.Error("expected retention sweep disabled by default") }
      if c.RetentionDays != 365 { t.Errorf("expected RetentionDays 365, got %d", c.RetentionDays) }
      if c.RetentionSweepBatch != 500 { t.Errorf("expected batch 500, got %d", c.RetentionSweepBatch) }
  }

  func TestLoad_RetentionEnabledParsed(t *testing.T) {
      t.Setenv("DB_URL", "postgres://localhost/db")
      t.Setenv("REDIS_URL", "redis://localhost:6379")
      t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
      t.Setenv("RETENTION_SWEEP_ENABLED", "true")
      t.Setenv("RETENTION_DAYS", "30")
      t.Setenv("RETENTION_SWEEP_BATCH", "10")
      c, err := Load()
      if err != nil { t.Fatalf("unexpected error: %v", err) }
      if !c.RetentionSweepEnabled { t.Error("expected enabled") }
      if c.RetentionDays != 30 { t.Errorf("expected 30, got %d", c.RetentionDays) }
      if c.RetentionSweepBatch != 10 { t.Errorf("expected 10, got %d", c.RetentionSweepBatch) }
  }
  ```
- **MIRROR**: `TestLoad_Defaults` / `TestLoad_PSRealWithWebhookSecret` (config_test.go).
- **VALIDATE**: `go test ./pkg/config/...`.

### Task 11: Integration test — sweep behaviour
- **ACTION**: Create `backend/internal/pdpa/retention_integration_test.go`.
- **IMPLEMENT**:
  - `//go:build integration`, package `pdpa`.
  - `dsn()` helper (copy the reports default-fallback form) + a `fakeBlobDeleter` recording deleted URLs + an in-memory `fakeAudit` implementing `activity.Writer`.
  - `setup(t)`: `pgxpool.New(dsn())`, `t.Cleanup(pool.Close)`, `TRUNCATE pdpa_consents, applications, candidates, positions RESTART IDENTITY CASCADE`.
  - Seed three candidates:
    1. **expired + terminal app** (`created_at = NOW() - 400 days`, one `rejected` application with a `resume_blob_url`) → expect anonymized.
    2. **expired + active app** (`created_at = NOW() - 400 days`, one `pending` application) → expect NOT anonymized (active pipeline).
    3. **recent** (`created_at = NOW()`, no apps) → expect NOT anonymized (window not elapsed).
  - Run `NewRetentionService(pool, fakeBlob, fakeAudit, 365).Sweep(ctx, 500)`.
  - Assert: return count == 1; candidate 1 `full_name = '[ลบข้อมูลแล้ว]'`, `phone/email/id_card IS NULL`, `pdpa_anonymized_at IS NOT NULL`; its application `resume_blob_url IS NULL`; `fakeBlob` recorded the one URL; `fakeAudit` recorded one `retention_anonymize`; candidates 2 & 3 unchanged (`pdpa_anonymized_at IS NULL`).
  - Add a second test: run `Sweep` twice → second run returns 0 (idempotent), `fakeBlob` not called again.
- **MIRROR**: TEST_STRUCTURE (reports/export_integration_test.go), dsn() (reports_integration_test.go).
- **GOTCHA**: Seeding a custom `created_at` requires an explicit column in the INSERT (the default is `NOW()`); use `INSERT INTO candidates (full_name, source_channel, status, created_at) VALUES (...)`. Seed `id_card` with unique values per candidate (it's `UNIQUE`); anonymization nulls them (multiple NULLs are allowed under UNIQUE).
- **VALIDATE**: `make up && make migrate-up` then `cd backend && go test -tags integration ./internal/pdpa/... -count=1`.

### Task 12: activity action const (small consistency task)
- **ACTION**: Add `ActionRetentionAnonymize = "retention_anonymize"` to the const block in `backend/internal/activity/activity.go` (lines 15-21) and use it at the Task 4 call site.
- **MIRROR**: existing `ActionReengage` const.
- **VALIDATE**: `go build ./...`.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `TestLoad_RetentionDefaults` | only required env | enabled=false, days=365, batch=500 | default safety |
| `TestLoad_RetentionEnabledParsed` | enabled/days/batch env set | parsed values | parse path |

### Integration Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `TestSweep_AnonymizesExpiredTerminal` | 3 seeded candidates (expired+terminal / expired+active / recent) | count=1; only #1 redacted; blob+audit recorded | eligibility boundaries |
| `TestSweep_Idempotent` | run twice | 2nd run count=0; no re-delete | retry safety |

### Edge Cases Checklist
- [x] Window not elapsed → skipped (candidate 3)
- [x] Active pipeline (pending/parsed/scored) → skipped (candidate 2)
- [x] Already anonymized → skipped (idempotency test + `WHERE pdpa_anonymized_at IS NULL` re-check)
- [x] Candidate with no applications → still eligible (NOT EXISTS is vacuously true)
- [x] Resume blob missing in storage → `Delete` treats not-found as success
- [x] Blob delete failure → logged, DB redaction already committed (no rollback)
- [x] `RetentionDays <= 0` misconfig → service clamps to 365 (no mass-erase)
- [x] `id_card` UNIQUE under null-out → multiple NULLs allowed

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./... && go build ./...
```
EXPECT: Zero errors.

### Unit Tests
```bash
cd backend && go test ./pkg/config/... ./pkg/queue/... ./pkg/blob/... ./internal/pdpa/...
```
EXPECT: All pass (unit-level; integration files are tag-gated out).

### Lint + Security (mirrors CI)
```bash
cd backend && golangci-lint run ./... && gosec -exclude-generated ./... && GOTOOLCHAIN=go1.26.4 govulncheck ./...
```
EXPECT: golangci-lint `0 issues`; gosec exit 0; govulncheck clean.

### Full Test Suite
```bash
cd backend && go test ./...
```
EXPECT: 15+ pkgs ok, no regressions.

### Database Validation
```bash
make up && make migrate-up && make migrate-down && make migrate-up
```
EXPECT: 000009 applies + rolls back + re-applies cleanly.

### Integration
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./internal/pdpa/... -count=1
```
EXPECT: sweep integration tests pass.

### Manual Validation
- [ ] With stack up + migrated, manually age a candidate: `UPDATE candidates SET created_at = NOW() - INTERVAL '400 days' WHERE id = '<terminal-app candidate>';`
- [ ] Enqueue a one-off sweep (enabled env) or call `Sweep` via the integration harness; confirm the row's `full_name = '[ลบข้อมูลแล้ว]'`, contact fields NULL, `pdpa_anonymized_at` set.
- [ ] Confirm an `activity_logs` row with `action='retention_anonymize'` exists for that candidate.
- [ ] Confirm `RETENTION_SWEEP_ENABLED=false` (default) → scheduler logs "retention sweep disabled" and never enqueues.

---

## Acceptance Criteria
- [ ] Migration 000009 adds `pdpa_anonymized_at` + partial index; round-trips up/down.
- [ ] Config exposes the 4 retention vars with safe defaults (disabled, 365, daily 03:30, 500); no fail-fast.
- [ ] `retention:sweep` task type + payload + handler exist and are wired into scheduler (gated) and worker.
- [ ] `Sweep` anonymizes only candidates past the window with no active application; idempotent on re-run.
- [ ] Resume blobs deleted via `blob.DeleteStored`; consent IPs nulled; `retention_anonymize` audit written.
- [ ] All validation commands pass; no type/lint/security regressions.

## Completion Checklist
- [ ] Code follows discovered patterns (tx, narrow interface, persist-before-side-effect, batch select loop)
- [ ] Error handling wrapped `pdpa: <op>: %w`
- [ ] Logging via zerolog; NO PII values logged (UUIDs only)
- [ ] Tests follow `//go:build integration` + AAA + config unit-test patterns
- [ ] No hardcoded values (window/batch/cron from config; redaction sentinel is an intentional constant)
- [ ] `docs/SECURITY.md` updated; `.env.example` updated
- [ ] No scope additions beyond the NOT Building list
- [ ] Self-contained — implementable without further codebase searching

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Misconfigured `RETENTION_DAYS=0` mass-erases | Low | Critical | Service clamps `<=0` → 365; disabled by default; integration test asserts window boundary |
| Sweep runs in an env that shouldn't purge | Low | Critical | `RETENTION_SWEEP_ENABLED=false` default; scheduler only registers cron when enabled |
| Anonymizing a candidate still mid-pipeline | Low | High | Eligibility excludes `pending/parsed/scored` applications |
| Blob delete failure orphans a resume after DB redaction | Medium | Low | Best-effort + logged; DB pointer already gone; blob unreachable; not re-found next run |
| Double-anonymize under overlapping runs | Very Low | Low | `asynq.Unique` TTL + `WHERE pdpa_anonymized_at IS NULL` re-check in UPDATE |
| `hired` candidate PII erased while PeopleSoft still needs it | Low | Medium | `hired` is terminal; PII already pushed to PS at hire (S3); retention promise covers all candidates. Documented decision — revisit if PS needs longer retention |

## Notes
- **Why anonymize, not delete:** `applications`, `pdpa_consents`, `reengagement_logs`, `reengagement_contacts`, `notifications`, and `candidates.is_duplicate_of` all FK to `candidates(id)`. Hard delete would cascade-destroy recruitment analytics + audit history or fail on FK. De-identification satisfies PDPA erasure while preserving aggregate reporting (funnel/KPI/sources reports read non-PII columns).
- **Retention clock = `candidates.created_at`** (when PII entered the system) — simplest defensible choice; documented. A "last activity" clock was considered but rejected as YAGNI for v1.
- **Enable-gate at scheduler, handler stays pure** — disabled envs never enqueue; an enabled env can still manually enqueue for ops/testing. Mirrors the mock-default seam philosophy used across the codebase (`AI_PROVIDER`, `PS_PROVIDER`, etc.).
- **Consent ledger kept** as legal proof consent was given (PDPA accountability), with its only PII (`ip_address`) nulled.
- Session continuity: branch `feat/s7-pdpa-retention`, NO commit attribution, squash-merge; CI is green so it merges without `--admin`. After implement → `/code-review` → `/prp-pr`.
