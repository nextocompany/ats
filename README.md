# AI HR Recruitment & Screening System

End-to-end AI recruitment platform — intake → AI screening/scoring → HR approval → PeopleSoft HCM sync.

> **Status:** Sprint 3 (PeopleSoft + Public Career API + Reference Import). Vacancies now flow in from
> **PeopleSoft** webhooks (replacing seeded demos) and hired candidates are pushed back (CSV fallback on
> failure). A public **Career API** (`/api/v1/public/*`) lets candidates browse open positions, apply
> (mock LINE auth, reusing the full intake→score→assign pipeline), and check status by opaque token. A CSV
> **importer** loads real Storelist/Master JD data. The pipeline (S1+S2) runs intake → OCR → parse →
> dedup → score → must-have gate → branch assignment. External integrations (AI, PeopleSoft, LINE) are all
> pluggable — **mock by default** (zero external creds); set `AI_PROVIDER=azure` / `PS_PROVIDER=real` /
> `LINE_PROVIDER=real` for the real services. The Next.js Career Portal + HR Dashboard UIs are Sprint 4.

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
make migrate-up               # 3. apply all migrations
make seed                     # 4. load representative stores/positions/vacancies (Sprint 2)
curl -s localhost:8080/health # 5. api health  → {"success":true,"data":{"checks":{...}}}
curl -s localhost:8081/health # 6. worker health
make down                     # stop the stack
```

A healthy response looks like:

```json
{ "success": true, "data": { "checks": { "postgres": "ok", "redis": "ok", "blob": "ok" } } }
```

If any dependency is down, `/health` returns HTTP `503` and names the failing check.

## Submit a Resume (Sprint 1)

```bash
# A position must exist; grab one (or seed your own):
POS=$(docker compose exec -T postgres psql -U hruser -d hr_db -tA \
  -c "INSERT INTO positions (title_th, level) VALUES ('แคชเชียร์','Staff') RETURNING id;" | head -1)

curl -F "resume=@cv.pdf;type=application/pdf" \
     -F "position_id=$POS" -F "full_name=สมชาย ใจดี" -F "source_channel=walk_in" \
     localhost:8080/api/v1/applications
#   → 201 {"application_id":"…","candidate_id":"…","job_id":"…"}

curl -s localhost:8080/api/v1/ai/jobs/<job_id>        # → {"state":"completed"}
curl -s localhost:8080/api/v1/applications/<app_id>   # → status "parsed", parsed_profile_blob_url, ocr_confidence
```

Allowed uploads: `pdf`, `docx`, `jpeg`, `png` (≤10MB). Low OCR confidence (<0.7) sets `needs_manual_review=true`
without aborting.

After parse the pipeline (Sprint 2) **dedups** the candidate, **scores** 0–100 against the position's
Master JD, applies the **must-have gate**, and **assigns a branch**:

```bash
curl -s localhost:8080/api/v1/applications/<app_id>   # → status "scored" | "rejected"
#   ai_score, ai_score_breakdown {experience,skills,education,language,location},
#   ai_summary (3 Thai strengths), must_have_passed, assigned_store_id (or talent_pool), dedup_state
```

Resubmitting the same person (matching id_card, or phone/email + fuzzy name) auto-merges to the canonical
candidate.

## PeopleSoft + Public Career API (Sprint 3)

```bash
# Reference data: synthetic SQL seed (make seed) OR real-format CSV import:
make import DIR=scripts        # → cmd/importref loads stores + Master JD positions (idempotent)

# Direction A — PeopleSoft opens a vacancy (replaces seeded demos):
curl -X POST localhost:8080/api/v1/ps/vacancy-opened -H 'Content-Type: application/json' \
  -d '{"ps_vacancy_id":"V-2026-001","store_id":1,"position_code":"CASHIER","headcount":1,"opened_date":"2026-06-01"}'

# Public Career API (mock LINE auth via X-LINE-IdToken):
curl localhost:8080/api/v1/public/positions
curl -F resume=@cv.pdf;type=application/pdf -F position_id=<id> -F full_name=... -F province=เชียงใหม่ \
  -H 'X-LINE-IdToken: dev-stub' localhost:8080/api/v1/public/apply   # → {status_token}
curl localhost:8080/api/v1/public/status/<status_token>             # minimal projection

# Direction B — mark hired → push to PeopleSoft (CSV-to-Blob fallback on failure):
curl -X PATCH localhost:8080/api/v1/applications/<id>/status -H 'Content-Type: application/json' -d '{"status":"hired"}'
curl localhost:8080/api/v1/ps/health
```

A PeopleSoft outage never blocks a hire — the candidate is written as a CSV export to Blob and
`ps_synced_at` is left unset for later retry. Notifications (LINE push) are Sprint 5.

## HR Dashboard API (Sprint 4a)

The Go API the dashboard UI (Sprint 4b) renders — role-scoped via the authenticated user.

```bash
curl 'localhost:8080/api/v1/applications?status=scored&min_score=50&page=1&limit=20'  # ranked inbox → data[] + meta
curl localhost:8080/api/v1/applications/<id>/resume          # short-lived signed (SAS) URL
curl -X POST localhost:8080/api/v1/applications/bulk -d '{"ids":["<id>"],"action":"status","value":"shortlisted"}'
curl localhost:8080/api/v1/candidates                        # list (scoped, paginated)
curl localhost:8080/api/v1/candidates/<id>                   # candidate + applications
curl localhost:8080/api/v1/candidates/<id>/timeline          # audit history (F16)
curl localhost:8080/api/v1/reports/funnel                    # also /kpi, /sources
curl -X POST localhost:8080/api/v1/pdpa/consent -d '{"candidate_id":"<id>","consent_given":true,"consent_version":"1.0","source_channel":"career_portal"}'
curl localhost:8080/api/v1/users/me
```

**Role scoping** is derived centrally (`internal/rbac`): super_admin/regional/auditor → all; operation_director →
subregion; sgm/hr_manager/hr_staff → store. The dev mock user is `super_admin`; scoping is unit-tested across roles.
The Next.js HR Dashboard + Career Portal UIs (Sprint 4b) build on this API in a **light operations console** direction.

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
