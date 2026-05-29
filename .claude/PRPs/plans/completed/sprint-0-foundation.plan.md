# Plan: Sprint 0 — Foundation (Docker Compose, Migrations, Health Checks)

## Summary
Stand up the complete local development foundation for the AI HR Recruitment platform **before any feature code is written**. This means a working `docker-compose` stack (Postgres 16, Redis 7, Azurite, Go API, Go worker), a full schema migration applied via `golang-migrate`, and `/health` endpoints on the API and worker that actively verify every backing dependency (DB, Redis, Blob) so a single `docker compose up` proves the platform is wired correctly.

## User Story
As a **developer on the HR Recruitment project**,
I want **a turnkey local stack where every service starts, migrations apply cleanly, and health checks confirm DB/Redis/Blob connectivity**,
So that **all later sprints build on a verified foundation and we never discover wiring/dependency problems mid-feature**.

## Problem → Solution
**Current state:** Empty repository. Jumping straight to features (CV parser, scoring, dashboard) would entangle business logic with unproven infrastructure wiring — exactly the dependency-hell the client warned against.
**Desired state:** `docker compose up` brings the entire stack live; `make migrate-up` applies the full schema; `curl localhost:8080/health` and the worker health probe both return `200` with per-dependency status. The conventions (project layout, config, error envelope, repository pattern, migration naming, logging) are established and documented for Sprint 1 to mirror.

## Metadata
- **Complexity**: Large (foundation scaffold across backend + infra; ~25 files)
- **Source PRD**: Inline PRP — "AI-Powered HR Recruitment & Screening System v1.0"
- **PRD Phase**: Sprint 0 (W1–2, "Architecture + Azure AD + Docker setup", 14 MD)
- **Estimated Files**: ~25 (backend skeleton + migrations + docker + scripts)

---

## UX Design

### Before / After
**N/A — internal/infrastructure change.** No end-user UX in Sprint 0. The "user" is the developer; the experience transformation is:

| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Local startup | No repo, nothing runs | `docker compose up` → 5 services healthy | api, worker, postgres, redis, azurite |
| Schema | None | `make migrate-up` applies all core tables | golang-migrate, reversible |
| Health verification | None | `GET /health` returns per-dependency JSON | DB + Redis + Blob probed live |
| Dev auth | None | Mock JWT middleware injects a dev user | Replaces Azure AD locally |

---

## Scope Decision (read first)

This plan covers **Sprint 0 only**. It deliberately establishes — but does not implement — the seams that later sprints fill:

- **IN**: repo scaffold, docker-compose, config loader, DB pool, Redis client, Blob client, full schema migration, health checks, mock JWT middleware, structured logging, error envelope, Makefile, `.env.example`, CI lint/build skeleton.
- **OUT (later sprints)**: any `/api/v1/*` business endpoint, AI clients (OpenAI/Doc Intelligence), LINE/PeopleSoft adapters, branch-assignment logic, the Next.js HR Dashboard and Career Portal apps, seed data population logic (migration creates the tables; seeding the 169 stores / 200 positions is Sprint 1+).

The frontends (`frontend/`, `career-portal/`) are **not** scaffolded in Sprint 0 — the client's gate is explicitly about service foundation + migrations + health checks, which are all backend/infra. Scaffolding Next.js apps now would add unverified surface area with no health-check value. (Flagged in Risks; revisit if the client wants the Next.js shells stood up in S0.)

---

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Fiber v2 | gofiber.io/docs | `app.Get("/health", handler)`; use `fiber.New(fiber.Config{...})`; graceful shutdown via `app.ShutdownWithContext`. |
| pgx v5 pool | github.com/jackc/pgx → `pgxpool` | `pgxpool.New(ctx, dsn)`; `pool.Ping(ctx)` for health; configure `MaxConns` via `pgxpool.ParseConfig`. |
| go-redis v9 | github.com/redis/go-redis/v9 | `redis.NewClient(opts)`; `client.Ping(ctx).Err()` for health. |
| golang-migrate | github.com/golang-migrate/migrate v4 | File naming `NNNNNN_name.up.sql` / `.down.sql`; CLI `migrate -path ... -database "postgres://..." up`. |
| Azurite + azblob | mcr.microsoft.com/azure-storage/azurite; `Azure/azure-sdk-for-go/sdk/storage/azblob` | Default dev connection string + `azblob.NewClientFromConnectionString`; create container on boot; health = list/exists check. |

### Research Notes
```
KEY_INSIGHT: PRP §2 says "Redis + BullMQ (via go-redis)" but BullMQ is a Node.js library with no Go port.
APPLIES_TO: Sprint 1 queue implementation (NOT Sprint 0).
GOTCHA: For Go, the idiomatic equivalent is hibiken/asynq (Redis-backed task queue). Sprint 0 only needs raw Redis connectivity, so we defer the queue-library decision. Flag to client: confirm asynq vs. a custom go-redis Streams queue before Sprint 1.

KEY_INSIGHT: PRP §14 docker-compose hardcodes DB creds (user/pass) and omits the worker service + healthchecks.
APPLIES_TO: docker-compose.yml in this sprint.
GOTCHA: Add a `worker` service, add `healthcheck:` blocks to postgres/redis, and gate `api`/`worker` startup on `depends_on: condition: service_healthy` so the stack is deterministic.

KEY_INSIGHT: gen_random_uuid() requires pgcrypto (or PG13+ core). PG16 ships it in core, but be explicit.
APPLIES_TO: migration 000001.
GOTCHA: Add `CREATE EXTENSION IF NOT EXISTS pgcrypto;` first so gen_random_uuid() is guaranteed.
```

---

## Conventions This Sprint ESTABLISHES (Sprint 1+ must mirror)

Since the repo is empty, these are the canonical patterns. Capture them here so no future sprint re-invents them.

### PROJECT_LAYOUT
```
backend/
  cmd/api/main.go        # HTTP entrypoint
  cmd/worker/main.go     # queue worker entrypoint
  internal/<domain>/     # feature packages (Sprint 1+: candidates, applications, ...)
  internal/middleware/   # auth (mock JWT), RBAC, logging, request-id
  internal/health/       # health handler + dependency checkers
  pkg/config/            # env config loader
  pkg/database/          # pgxpool connection
  pkg/redis/             # go-redis client
  pkg/blob/              # azblob client
  pkg/httpx/             # shared response envelope + error types
  migrations/            # golang-migrate SQL files
```
Rule (per coding-style.md): organize by **domain**, not by type. Files 200–400 lines typical, 800 max.

### CONFIG_PATTERN
```go
// pkg/config/config.go — load ALL env at startup, fail fast if required vars missing.
type Config struct {
    Env        string // development | production
    HTTPPort   string
    DatabaseURL string
    RedisURL   string
    BlobConnString string
    JWTSecret  string
}
func Load() (*Config, error) {
    c := &Config{
        Env:      getenv("ENV", "development"),
        HTTPPort: getenv("HTTP_PORT", "8080"),
        DatabaseURL: os.Getenv("DB_URL"),
        // ...
    }
    if c.DatabaseURL == "" { return nil, fmt.Errorf("config: DB_URL is required") }
    return c, nil // return new value, never mutate a shared global (immutability rule)
}
```

### RESPONSE_ENVELOPE (per common/patterns.md "API Response Format")
```go
// pkg/httpx/response.go — every API response uses this envelope.
type Envelope[T any] struct {
    Success bool   `json:"success"`
    Data    T      `json:"data,omitempty"`
    Error   string `json:"error,omitempty"`
    Meta    *Meta  `json:"meta,omitempty"` // pagination: total, page, limit
}
func OK[T any](c *fiber.Ctx, data T) error {
    return c.Status(fiber.StatusOK).JSON(Envelope[T]{Success: true, Data: data})
}
func Fail(c *fiber.Ctx, status int, msg string) error {
    return c.Status(status).JSON(Envelope[any]{Success: false, Error: msg})
}
```

### ERROR_HANDLING (per coding-style.md — explicit at every level, never swallow)
```go
if err != nil {
    log.Error().Err(err).Str("dep", "postgres").Msg("health check failed")
    return httpx.Fail(c, fiber.StatusServiceUnavailable, "database unavailable")
}
```
Use a custom Fiber `ErrorHandler` in `fiber.Config` so unhandled errors return the envelope, never a stack trace (no sensitive-data leak — security.md).

### LOGGING_PATTERN
```go
// Structured JSON logging via rs/zerolog. Level from ENV (debug in dev, info in prod).
log.Info().Str("service", "api").Str("addr", addr).Msg("listening")
```

### REPOSITORY_PATTERN (defined now, used Sprint 1+)
```go
// internal/<domain>/repository.go — data access behind an interface (patterns.md Repository Pattern).
type Repository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*Entity, error)
}
type pgRepository struct{ pool *pgxpool.Pool }
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }
```

### MIGRATION_NAMING (golang-migrate)
```
migrations/000001_init_schema.up.sql
migrations/000001_init_schema.down.sql
```
Every `.up.sql` MUST have a matching `.down.sql` (reversible). Zero-padded 6-digit sequence.

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `docker-compose.yml` | CREATE | Full local stack: api, worker, postgres, redis, azurite + healthchecks |
| `.env.example` | CREATE | Documents every env var; copied to `.env` for local dev |
| `.gitignore` | CREATE | Ignore `.env`, build artifacts, `pgdata` |
| `Makefile` | CREATE | `up`, `down`, `migrate-up`, `migrate-down`, `migrate-create`, `lint`, `test`, `run-api`, `run-worker` |
| `backend/go.mod` / `go.sum` | CREATE | Go 1.22 module + deps (fiber, pgx, go-redis, zerolog, azblob, uuid) |
| `backend/Dockerfile` | CREATE | Multi-stage build; shared by api + worker via build target/arg |
| `backend/cmd/api/main.go` | CREATE | HTTP server entrypoint, wires deps, mounts `/health`, graceful shutdown |
| `backend/cmd/worker/main.go` | CREATE | Worker entrypoint, connects deps, exposes health probe |
| `backend/pkg/config/config.go` | CREATE | Env loader, fail-fast validation |
| `backend/pkg/database/postgres.go` | CREATE | `pgxpool` connect + `Ping` |
| `backend/pkg/redis/redis.go` | CREATE | go-redis client + `Ping` |
| `backend/pkg/blob/blob.go` | CREATE | azblob client (Azurite) + ensure container + health check |
| `backend/pkg/httpx/response.go` | CREATE | Response envelope helpers |
| `backend/pkg/httpx/errors.go` | CREATE | Fiber custom ErrorHandler |
| `backend/internal/health/health.go` | CREATE | Health handler aggregating DB/Redis/Blob checkers |
| `backend/internal/middleware/logging.go` | CREATE | Request logging + request-id |
| `backend/internal/middleware/mock_jwt.go` | CREATE | Dev-only mock auth injecting a fixed super_admin user |
| `backend/migrations/000001_init_schema.up.sql` | CREATE | All core tables from PRP §6 |
| `backend/migrations/000001_init_schema.down.sql` | CREATE | Drop in reverse FK order |
| `backend/migrations/000002_indexes.up.sql` | CREATE | Indexes for FK + common filters (status, subregion, ai_score) |
| `backend/migrations/000002_indexes.down.sql` | CREATE | Drop indexes |
| `.github/workflows/ci.yml` | CREATE | go vet + build + golangci-lint + migrate dry-run (no deploy in S0) |
| `README.md` | CREATE | Quickstart: prerequisites, `make up`, `make migrate-up`, health check URLs |
| `backend/internal/health/health_test.go` | CREATE | Unit test for health aggregation logic (mocked checkers) |

## NOT Building
- Any `/api/v1/*` business endpoint (candidates, applications, AI, reports, PeopleSoft, LINE).
- AI clients (Azure OpenAI, Document Intelligence), LINE, PeopleSoft adapters.
- Branch-assignment engine, dedup logic, scoring engine.
- `frontend/` and `career-portal/` Next.js apps.
- Real Azure AD SSO (mock JWT only in S0).
- Seed data population (tables created; loading 169 stores / 200 positions is Sprint 1+).
- The job-queue library wiring (asynq vs. Streams) — Redis connectivity only.

---

## Step-by-Step Tasks

### Task 1: Repo scaffold + tooling
- **ACTION**: Create root files: `.gitignore`, `.env.example`, `Makefile`, `README.md`.
- **IMPLEMENT**: `.env.example` lists every var from PRP §14 (DB_URL, REDIS_URL, AZURE_BLOB_CONNECTION_STRING, JWT_SECRET, AZURE_* placeholders, PS_* placeholders, ENV, HTTP_PORT). `.gitignore` excludes `.env`, `backend/bin/`, `pgdata/`. `Makefile` targets listed in Files table.
- **MIRROR**: CONFIG_PATTERN (every var documented).
- **GOTCHA**: Never commit a real `.env`. `.env.example` holds placeholders only (security.md — no hardcoded secrets).
- **VALIDATE**: `make help` lists targets; `git status` shows `.env` ignored.

### Task 2: Go module + dependencies
- **ACTION**: `cd backend && go mod init github.com/nexto/hr-ats && go get` the deps.
- **IMPLEMENT**: Deps — `github.com/gofiber/fiber/v2`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, `github.com/rs/zerolog`, `github.com/google/uuid`, `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`. Set `go 1.22` in `go.mod`.
- **MIRROR**: PROJECT_LAYOUT.
- **GOTCHA**: Pin major versions; run `go mod tidy` after writing imports so `go.sum` is complete before the Docker build.
- **VALIDATE**: `go build ./...` compiles (after later tasks add code).

### Task 3: Config loader
- **ACTION**: Create `pkg/config/config.go`.
- **IMPLEMENT**: `Load()` per CONFIG_PATTERN; required vars (`DB_URL`, `REDIS_URL`) fail-fast with a clear error; optional vars get defaults.
- **MIRROR**: CONFIG_PATTERN. Return a new `*Config`; do not mutate package globals (immutability rule).
- **IMPORTS**: `os`, `fmt`.
- **GOTCHA**: Validate at startup, not lazily — fail before binding the port.
- **VALIDATE**: Unset `DB_URL` → `Load()` returns error; set → returns populated config.

### Task 4: Shared httpx (envelope + error handler)
- **ACTION**: Create `pkg/httpx/response.go` and `pkg/httpx/errors.go`.
- **IMPLEMENT**: `Envelope[T]`, `OK`, `Fail`, `Meta`; a Fiber `ErrorHandler` that maps `*fiber.Error` to the envelope and logs+masks unexpected errors as a generic 500.
- **MIRROR**: RESPONSE_ENVELOPE, ERROR_HANDLING.
- **IMPORTS**: `github.com/gofiber/fiber/v2`, `github.com/rs/zerolog/log`.
- **GOTCHA**: Error handler must never echo raw `err.Error()` for 5xx (security.md — no sensitive leak); log full, return generic.
- **VALIDATE**: Trigger a forced error route in a scratch test → JSON envelope, no stack trace.

### Task 5: Database pool (pgx)
- **ACTION**: Create `pkg/database/postgres.go`.
- **IMPLEMENT**: `Connect(ctx, dsn) (*pgxpool.Pool, error)` using `pgxpool.ParseConfig` (set `MaxConns`), then `pool.Ping(ctx)`. Expose `Ping(ctx)` helper for health.
- **MIRROR**: REPOSITORY_PATTERN dependency (pool injected, not global).
- **IMPORTS**: `github.com/jackc/pgx/v5/pgxpool`.
- **GOTCHA**: Apply a connect timeout via `context.WithTimeout`; a hung DB must not block boot indefinitely.
- **VALIDATE**: With Postgres up, `Connect` succeeds + `Ping` returns nil.

### Task 6: Redis client
- **ACTION**: Create `pkg/redis/redis.go`.
- **IMPLEMENT**: `Connect(url) (*redis.Client, error)` via `redis.ParseURL`; `Ping(ctx)` helper.
- **IMPORTS**: `github.com/redis/go-redis/v9`.
- **GOTCHA**: `ParseURL` expects `redis://host:port`; match docker-compose service name `redis`.
- **VALIDATE**: With Redis up, `Ping` returns nil.

### Task 7: Blob client (Azurite)
- **ACTION**: Create `pkg/blob/blob.go`.
- **IMPLEMENT**: `Connect(connString) (*azblob.Client, error)`; on boot ensure a `resumes` container exists (create-if-not-exists, ignore "already exists"); `HealthCheck(ctx)` lists/exists the container.
- **IMPORTS**: `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`.
- **GOTCHA**: Use Azurite's well-known dev connection string in `.env.example`; container create returns a conflict error if it exists — treat as success.
- **VALIDATE**: With Azurite up, container creation idempotent + health check passes.

### Task 8: Health package
- **ACTION**: Create `internal/health/health.go` + `health_test.go`.
- **IMPLEMENT**: A `Checker` interface (`Name() string`, `Check(ctx) error`); concrete checkers wrapping DB/Redis/Blob; a Fiber handler that runs all checkers (with per-check timeout), returns `200` when all pass or `503` when any fail, with JSON `{ checks: { postgres: "ok", redis: "ok", blob: "ok" } }` inside the envelope.
- **MIRROR**: RESPONSE_ENVELOPE, ERROR_HANDLING.
- **GOTCHA**: Run checks concurrently with a bounded context (e.g. 2s) so one slow dep doesn't stall the probe past Docker's healthcheck interval.
- **VALIDATE**: Unit test with mocked checkers (one failing → 503, all ok → 200). See Testing Strategy.

### Task 9: Middleware (logging + mock JWT)
- **ACTION**: Create `internal/middleware/logging.go` and `mock_jwt.go`.
- **IMPLEMENT**: Logging middleware assigns a request-id and logs method/path/status/latency. Mock JWT middleware (dev only, gated on `ENV=development`) injects a fixed `super_admin` user into `c.Locals("user")` so Sprint 1 handlers can read auth context without real Azure AD.
- **MIRROR**: LOGGING_PATTERN.
- **GOTCHA**: Mock JWT must be a no-op / disabled when `ENV != development` so it can never leak into prod (security.md).
- **VALIDATE**: Request logs include request-id; `/health` is exempt from auth.

### Task 10: API entrypoint
- **ACTION**: Create `cmd/api/main.go`.
- **IMPLEMENT**: `config.Load()` → connect DB/Redis/Blob → `fiber.New` with custom ErrorHandler → mount logging middleware + `GET /health` (health package) → `app.Listen` → graceful shutdown on SIGINT/SIGTERM via `app.ShutdownWithContext`.
- **MIRROR**: All patterns above.
- **IMPORTS**: config, database, redis, blob, health, middleware, httpx, fiber, os/signal.
- **GOTCHA**: Close pools on shutdown. Bind `0.0.0.0:$HTTP_PORT` (not `localhost`) so it's reachable inside Docker.
- **VALIDATE**: `make run-api` (with deps up) → `curl localhost:8080/health` returns 200 envelope.

### Task 11: Worker entrypoint
- **ACTION**: Create `cmd/worker/main.go`.
- **IMPLEMENT**: Connect DB/Redis/Blob; run a minimal loop/ticker that logs "worker alive" and verifies dependency health; expose a lightweight HTTP health probe on a separate port (e.g. `:8081`) reusing the health package so docker-compose can health-check the worker. No queue consumption yet.
- **MIRROR**: health package reuse.
- **GOTCHA**: Worker has no Fiber app by default — add a tiny `fiber.New` solely for `/health` on `:8081`, or use net/http. Keep it minimal.
- **VALIDATE**: `curl localhost:8081/health` returns 200; worker logs heartbeat.

### Task 12: Migrations — schema
- **ACTION**: Create `migrations/000001_init_schema.up.sql` + `.down.sql`.
- **IMPLEMENT**: Up: `CREATE EXTENSION IF NOT EXISTS pgcrypto;` then all 11 tables from PRP §6 in FK-safe order: `stores` → `positions` → `users` → `candidates` → `vacancies` → `applications` → `activity_logs` → `pdpa_consents` → `reengagement_logs` → `notifications`. Down: `DROP TABLE` in reverse order.
- **MIRROR**: MIGRATION_NAMING.
- **GOTCHA**: `applications` and `users` both reference `stores`/each other — order matters; `candidates.is_duplicate_of` is a self-FK (fine inline). `applications.reviewed_by → users(id)` requires `users` first.
- **VALIDATE**: `make migrate-up` applies clean; `make migrate-down` reverts clean; re-run up succeeds (idempotent via golang-migrate version table).

### Task 13: Migrations — indexes
- **ACTION**: Create `migrations/000002_indexes.up.sql` + `.down.sql`.
- **IMPLEMENT**: Indexes on FKs and hot filters: `applications(status)`, `applications(ai_score DESC)`, `applications(candidate_id)`, `applications(assigned_store_id)`, `candidates(subregion)`, `candidates(status)`, `candidates(phone)`, `candidates(email)`, `vacancies(store_id, position_id, status)`, `activity_logs(entity_type, entity_id)`.
- **GOTCHA**: `id_card` already UNIQUE (auto-indexed) — don't duplicate. Keep index DDL reversible.
- **VALIDATE**: `migrate-up` then `\di` in psql shows the indexes.

### Task 14: Dockerfile (multi-stage, shared)
- **ACTION**: Create `backend/Dockerfile`.
- **IMPLEMENT**: Stage 1 `golang:1.22-alpine` builds both binaries (`go build -o /out/api ./cmd/api` and `/out/worker ./cmd/worker`). Stage 2 `alpine` with `ca-certificates`; use a build `ARG SVC=api` (or two targets) so the same Dockerfile yields api and worker images.
- **GOTCHA**: Cache `go mod download` layer before copying source. Run as non-root user.
- **VALIDATE**: `docker build` succeeds for both binaries.

### Task 15: docker-compose
- **ACTION**: Create root `docker-compose.yml` (extends PRP §14 with fixes).
- **IMPLEMENT**: Services `postgres` (16-alpine, healthcheck `pg_isready`), `redis` (7-alpine, healthcheck `redis-cli ping`), `azurite`, `api` (port 8080, `depends_on` postgres+redis `service_healthy`), `worker` (port 8081, same deps). All env via `${VAR}` from `.env`. Named volume `pgdata`.
- **MIRROR**: PRP §14 service shape; add worker + healthchecks + dependency gating.
- **GOTCHA**: Don't hardcode secrets — pull from `.env`. Ensure `api`/`worker` wait for healthy deps to avoid race-on-boot migration failures.
- **VALIDATE**: `docker compose up` → all 5 services reach healthy; `docker compose ps` shows healthy status.

### Task 16: Migration run integration into compose flow
- **ACTION**: Wire `make migrate-up` to run against the compose Postgres (and document a one-shot migrate container option).
- **IMPLEMENT**: Makefile `migrate-up` runs `migrate -path backend/migrations -database "$(DB_URL)" up`. Optionally add a `migrate` one-shot service for CI/`docker compose run`.
- **GOTCHA**: golang-migrate CLI must be installed locally (`go install ...migrate`) or run via its Docker image — document both in README.
- **VALIDATE**: After `make up`, `make migrate-up` applies both migrations; health check still 200.

### Task 17: CI skeleton
- **ACTION**: Create `.github/workflows/ci.yml`.
- **IMPLEMENT**: On PR: `go vet ./...`, `go build ./...`, `golangci-lint run`, `go test ./...`, and a migrate up/down dry-run against an ephemeral Postgres service container. No deploy step in S0.
- **GOTCHA**: Use a Postgres service container in the workflow for the migration test.
- **VALIDATE**: Workflow YAML lints; jobs defined.

### Task 18: README quickstart
- **ACTION**: Create root `README.md`.
- **IMPLEMENT**: Prerequisites (Docker, Go 1.22, golang-migrate), steps: `cp .env.example .env` → `make up` → `make migrate-up` → verify `http://localhost:8080/health` and `http://localhost:8081/health`. Document the established conventions (link to this plan).
- **VALIDATE**: A fresh dev following README reaches green health checks.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| health aggregates all-ok | 3 mock checkers return nil | 200, all checks `"ok"` | No |
| health one-failing | redis checker returns err | 503, redis `"error: ..."`, others `"ok"` | Yes |
| health timeout | a checker blocks > ctx timeout | 503, that check reports timeout | Yes |
| config required missing | `DB_URL` unset | `Load()` returns error | Yes |
| config defaults | only required set | port defaults to `8080`, env `development` | No |

### Edge Cases Checklist
- [ ] Postgres not yet ready when api starts → `depends_on: service_healthy` prevents; connect has timeout fallback.
- [ ] Redis down at health-check time → 503 with redis flagged.
- [ ] Azurite container already exists → create is idempotent (no error).
- [ ] Migration run twice → golang-migrate no-ops (version table).
- [ ] `migrate-down` then `migrate-up` → clean round-trip (reversibility).
- [ ] Mock JWT disabled when `ENV != development`.

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./... && golangci-lint run
```
EXPECT: Zero issues.

### Build
```bash
cd backend && go build ./...
docker compose build
```
EXPECT: Both binaries + images build.

### Unit Tests
```bash
cd backend && go test ./... -cover
```
EXPECT: All pass; health + config covered (target 80%+ on tested packages).

### Stack + Migrations + Health (the Sprint 0 gate)
```bash
cp .env.example .env
make up                 # docker compose up -d
make migrate-up         # apply 000001 + 000002
docker compose ps       # all services "healthy"
curl -fsS http://localhost:8080/health   # api: 200 envelope, all deps ok
curl -fsS http://localhost:8081/health   # worker: 200, all deps ok
```
EXPECT: `docker compose ps` shows all healthy; both `/health` return `{"success":true,"data":{"checks":{"postgres":"ok","redis":"ok","blob":"ok"}}}`.

### Database Validation
```bash
docker compose exec postgres psql -U user -d hr_db -c "\dt"   # 11 tables + schema_migrations
docker compose exec postgres psql -U user -d hr_db -c "\di"   # indexes from 000002
make migrate-down && make migrate-up                          # reversibility
```
EXPECT: All core tables present; clean down/up round-trip.

### Manual Validation
- [ ] `cp .env.example .env` then `make up` → 5 services healthy within ~30s.
- [ ] `make migrate-up` applies without error; re-running is a no-op.
- [ ] Stop Redis (`docker compose stop redis`) → `/health` returns 503 flagging redis; restart → 200.
- [ ] Both api (8080) and worker (8081) health endpoints green.

---

## Acceptance Criteria (Sprint 0 gate — must all pass before Sprint 1)
- [ ] `docker compose up` brings api, worker, postgres, redis, azurite to **healthy**.
- [ ] `make migrate-up` applies the full schema (11 tables) + indexes; `migrate-down` reverts cleanly.
- [ ] **API `/health` returns 200 verifying DB + Redis + Blob.**
- [ ] **Worker `/health` returns 200 verifying DB + Redis + Blob.**
- [ ] All validation commands pass; no type/vet/lint errors.
- [ ] Conventions (layout, config, envelope, error handling, logging, repository, migration naming) documented in this plan + README.
- [ ] `.env` is git-ignored; no secrets committed.

## Completion Checklist
- [ ] Code follows the conventions this sprint establishes.
- [ ] Error handling explicit; 5xx never leak internals.
- [ ] Structured logging in place.
- [ ] Health unit tests pass; config tests pass.
- [ ] No hardcoded secrets (env-driven).
- [ ] README quickstart verified on a clean checkout.
- [ ] Self-contained — Sprint 1 can start without re-deriving infrastructure.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| BullMQ named but no Go port exists | High | Med | Defer queue lib to S1; recommend `hibiken/asynq`. Confirm with client before S1. |
| docker-compose race (api boots before DB ready) | Med | High | `depends_on: condition: service_healthy` + connect timeouts. |
| Azurite connection-string mismatch | Med | Med | Use Azurite's documented dev conn string in `.env.example`; verify container create idempotency. |
| golang-migrate CLI not installed locally | Med | Low | Document `go install` + Docker-image alternative in README. |
| Frontend shells expected in S0 | Low | Med | Plan excludes Next.js apps (no health-check value); flag for client confirmation. |
| `gen_random_uuid()` unavailable | Low | High | Explicit `CREATE EXTENSION IF NOT EXISTS pgcrypto;` in migration 000001. |

## Notes
- Module path `github.com/nexto/hr-ats` is a placeholder — adjust to the real org/repo before `go mod init` if different.
- This plan intentionally defines patterns (envelope, repository, migration naming) that Sprint 1 mirrors verbatim, so feature work inherits a consistent foundation.
- PeopleSoft / LINE / Azure AD env vars are present in `.env.example` as placeholders only; their adapters are built in later sprints.
