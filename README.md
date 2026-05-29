# AI HR Recruitment & Screening System

End-to-end AI recruitment platform — intake → AI screening/scoring → HR approval → PeopleSoft HCM sync.

> **Status:** Sprint 0 (Foundation). Docker stack, database schema, and health checks are in place. Feature work begins in Sprint 1.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose v2
- [Go 1.22+](https://go.dev/dl/) (1.26 used in CI)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI:
  ```bash
  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  # ensure $(go env GOPATH)/bin is on your PATH
  ```

## Quickstart

```bash
cp .env.example .env          # 1. local config (host-side values)
make up                       # 2. start postgres, redis, azurite, api, worker
make migrate-up               # 3. apply the schema (000001 + 000002)
curl -s localhost:8080/health # 4. api health  → {"success":true,"data":{"checks":{...}}}
curl -s localhost:8081/health # 5. worker health
make down                     # stop the stack
```

A healthy response looks like:

```json
{ "success": true, "data": { "checks": { "postgres": "ok", "redis": "ok", "blob": "ok" } } }
```

If any dependency is down, `/health` returns HTTP `503` and names the failing check.

## Make Targets

Run `make help` for the full list — `up`, `down`, `migrate-up`, `migrate-down`,
`migrate-create name=…`, `build`, `run-api`, `run-worker`, `test`, `lint`, `vet`, `tidy`.

## Project Layout

```
backend/
  cmd/api/        HTTP server entrypoint
  cmd/worker/     queue worker entrypoint (heartbeat + /health in Sprint 0)
  internal/       domain packages (Sprint 1+) + health + middleware
  pkg/            config, database, redis, blob, httpx, logging, bootstrap
  migrations/     golang-migrate SQL files
docker-compose.yml
```

## Conventions (established in Sprint 0, mirror these in Sprint 1+)

- **Response envelope** — every API response uses `pkg/httpx.Envelope[T]` (`{success, data, error, meta}`).
- **Error handling** — `pkg/httpx.ErrorHandler` masks 5xx internals; client (4xx) messages surface.
- **Config** — `pkg/config.Load()` reads all env at startup and fails fast on missing required vars.
- **Repository pattern** — domain packages receive `*pgxpool.Pool` via injection (no globals).
- **Migrations** — `NNNNNN_name.up.sql` / `.down.sql`; every up has a matching reversible down.
- **Logging** — structured zerolog via `pkg/logging.Configure`.
- **Dev auth** — `middleware.MockJWT` injects a fixed `super_admin` user; active only when `ENV=development`.

See `.claude/PRPs/plans/completed/sprint-0-foundation.plan.md` for the full plan.

## Notes

- Local dev uses **Azurite** for Blob Storage and a **mock JWT** in place of Azure AD SSO.
- The job-queue library (PRP mentions "BullMQ", which is Node-only) is deferred to Sprint 1 —
  `hibiken/asynq` is the recommended Go equivalent. Sprint 0 only verifies Redis connectivity.
