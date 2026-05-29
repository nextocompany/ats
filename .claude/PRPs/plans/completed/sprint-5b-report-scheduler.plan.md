# Plan: Sprint 5b — Report Scheduler (recurring HR reports)

## Summary
Add a **recurring report scheduler**: on a cron schedule, compute the funnel / KPI / source reports (the queries already exist in `internal/reports`), render them to CSV+JSON, store the export in Blob, and deliver a link via the **Notifier seam from 5a**. Past exports are persisted and listable via the authenticated HR API, with a lightweight "Scheduled exports" card on the Analytics page. Introduces the project's first **asynq periodic scheduler** (none exists today).

## User Story
As an **HR director**, I want **the recruitment funnel/KPI/source report delivered to me automatically every week (and on demand)**, so that **I get pipeline visibility without logging in to pull it manually**.

## Problem → Solution
**Current state:** `internal/reports` exposes funnel/kpi/sources on demand via `GET /api/v1/reports/*`, but nothing is scheduled, exported, or delivered. There is **no asynq scheduler** in the codebase (only on-intake enqueue).
**Desired state:** A scheduler enqueues a `report:export` task on a cron; the worker computes a report snapshot, uploads `reports/<period>.{csv,json}` to Blob, records a `report_exports` row, and notifies recipients with a signed URL. HR can also trigger an export on demand and list/download past exports.

## Metadata
- **Complexity**: Medium–Large (new scheduler binary + worker task + reports export/encode + migration + small UI; ~12 files)
- **Source PRD**: Nexto PRP v1.0 — Sprint 5 (report scheduler); roadmap §20
- **Decisions locked**: scheduler = **asynq.Scheduler** in a dedicated `cmd/scheduler` binary (avoids double-dispatch across worker replicas); export = CSV+JSON to Blob; delivery via **5a `notify.Notifier`**; exports persisted in `report_exports`
- **Estimated Files**: ~12
- **Depends on**: **5a** (`internal/notify` seam). If 5a is unmerged, define a minimal local notifier and swap on merge.

---

## UX Design

### After (Analytics page — additive card)
```
Analytics
┌───────────────────────────────────────────┐
│ Funnel · KPI · Sources (existing charts)   │
├───────────────────────────────────────────┤
│ Scheduled exports            [ Export now ]│
│ • 2026-05-25 weekly  funnel,kpi  [CSV][JSON]│
│ • 2026-05-18 weekly  funnel,kpi  [CSV][JSON]│
└───────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Periodic report | none | cron → export + notify | `REPORT_SCHEDULE_CRON` |
| On-demand export | none | `POST /api/v1/reports/exports` | RBAC-gated |
| List exports | none | `GET /api/v1/reports/exports` + Analytics card with download links | signed URLs |

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/reports/reports.go` | 12-102 | `Funnel`/`KPI`/`Source` structs + `Funnel()/KPI()/Sources()` queries to snapshot |
| P0 | `backend/internal/reports/handler.go` | 9-49 | handler + `RegisterRoutes` (`/api/v1/reports/*`) to extend with `/exports` |
| P0 | `backend/pkg/queue/tasks.go` | 11-51 | task const + payload + builder/parser to clone for `report:export` |
| P0 | `backend/cmd/worker/main.go` | 101-126 | provider construction + `mux.HandleFunc` registration |
| P0 | `backend/pkg/blob/blob.go` | 60-104 | `Upload(ctx,name,data,contentType)` + `SignedURL(name, ttl)` for export storage/links |
| P0 | `backend/internal/notify/notify.go` | (5a) | `Notifier.Send` for delivery — **5a dependency** |
| P1 | `backend/pkg/config/config.go` | 55-138 | `getenv`, add `REPORT_SCHEDULE_CRON`, `REPORT_RECIPIENTS` |
| P1 | `backend/cmd/worker/main.go` | 38-41, 84-99 | pool + blob construction to reuse in scheduler/worker |
| P1 | `docker-compose.yml` | all | add the `scheduler` service alongside `worker` |
| P1 | `backend/migrations/000005_ps_public.up.sql` | all | migration style |
| P1 | `frontend/app/(app)/analytics/page.tsx` | all | where to add the "Scheduled exports" card |
| P1 | `frontend/lib/queries.ts` | 81-89 | report hooks pattern (`useFunnel/useKpi/useSources`) to add `useReportExports` |
| P2 | `backend/cmd/importref/main.go` | all | example of a small standalone `cmd/*` binary (shape for `cmd/scheduler`) |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| asynq Scheduler | github.com/hibiken/asynq#scheduler | `asynq.NewScheduler(redisOpt, opts)`; `scheduler.Register(cronspec, task)`; `scheduler.Run()`. Separate process from the server. One scheduler instance only (else duplicate enqueues). |
| cron spec | robfig/cron v3 (asynq uses it) | 5-field cron; default weekly `"0 7 * * 1"` (Mon 07:00). |

### Research Notes
```
KEY_INSIGHT: No asynq.Scheduler exists; this is the first periodic dispatcher.
APPLIES_TO: cmd/scheduler (new).
GOTCHA: run exactly ONE scheduler instance. Put it in its own cmd/scheduler binary + a single compose service (replicas:1). Do NOT start it inside the worker (workers may scale to N → N duplicate enqueues).

KEY_INSIGHT: report data already exists.
APPLIES_TO: internal/reports.
GOTCHA: reuse Funnel()/KPI()/Sources(); add a Snapshot() that gathers all three + CSV/JSON encoders. Do not re-query in the worker handler.

KEY_INSIGHT: blob Upload is idempotent by name + SignedURL gives time-limited links.
APPLIES_TO: export storage + delivery.
GOTCHA: name exports deterministically (reports/<kind>-<period>.csv) so retries overwrite rather than duplicate; deliver via SignedURL(ttl) not the raw blob URL (container isn't public).
```

---

## Patterns to Mirror

### REPORT_SNAPSHOT (extend reports.go)
```go
type Snapshot struct {
	Period  string   `json:"period"`
	Funnel  Funnel   `json:"funnel"`
	KPI     KPI      `json:"kpi"`
	Sources []Source `json:"sources"`
}
func (r *Repo) Snapshot(ctx context.Context, period string) (Snapshot, error) { /* call Funnel/KPI/Sources */ }
func EncodeCSV(s Snapshot) []byte  // funnel+kpi+sources sections
func EncodeJSON(s Snapshot) []byte // json.Marshal
```

### QUEUE_TASK (mirror pkg/queue/tasks.go:11-51)
```go
const TypeExportReport = "report:export"
type ExportReportPayload struct {
	Kind   string `json:"kind"`   // "weekly" | "ondemand"
	Period string `json:"period"` // e.g. "2026-W22" or RFC3339 date
}
```

### SCHEDULER (cmd/scheduler/main.go — new; mirror worker redis setup worker/main.go:115)
```go
redisOpt, _ := queue.RedisOpt(cfg.RedisURL)
s := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{})
task, _ := queue.NewExportReportTask(queue.ExportReportPayload{Kind: "weekly"})
s.Register(cfg.ReportScheduleCron, task)
s.Run()
```

### BLOB_EXPORT (mirror blob.go:60-78)
```go
url, _ := blobClient.Upload(ctx, "reports/"+name+".csv", csv, "text/csv")
link, _ := blobClient.SignedURL("reports/"+name+".csv", 7*24*time.Hour)
```

### EXPORTS_REPO (mirror candidates/repository.go:100-128 scan pattern)
```go
func (r *Repo) RecordExport(ctx, e Export) error
func (r *Repo) ListExports(ctx, limit int) ([]Export, error)
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/reports/export.go` | CREATE | `Snapshot()` + `EncodeCSV`/`EncodeJSON` |
| `backend/internal/reports/exports_repo.go` | CREATE | `report_exports` persistence (Record/List) |
| `backend/internal/reports/export_service.go` | CREATE | orchestrate: snapshot → encode → blob upload → record → notify |
| `backend/internal/reports/handler.go` | UPDATE | add `GET/POST /api/v1/reports/exports` |
| `backend/internal/reports/export_test.go` | CREATE | CSV/JSON encode + service (fake blob/notify) tests |
| `backend/pkg/queue/tasks.go` | UPDATE | `TypeExportReport` + payload + builder + parser |
| `backend/cmd/scheduler/main.go` | CREATE | asynq.Scheduler — periodic enqueue of `report:export` |
| `backend/cmd/worker/main.go` | UPDATE | build export service; `mux.HandleFunc(queue.TypeExportReport, ...)` |
| `backend/cmd/api/main.go` | UPDATE | wire export service into reports handler (blob + notify) |
| `backend/pkg/config/config.go` | UPDATE | `REPORT_SCHEDULE_CRON` (default `"0 7 * * 1"`), `REPORT_RECIPIENTS` |
| `backend/migrations/0000NN_report_exports.{up,down}.sql` | CREATE | `report_exports` table (via `make migrate-create`) |
| `docker-compose.yml` | UPDATE | add `scheduler` service (build target, `replicas: 1`) |
| `Makefile` | UPDATE | `build` target builds the scheduler binary too |
| `frontend/lib/types.ts` | UPDATE | `ReportExport` type |
| `frontend/lib/queries.ts` | UPDATE | `useReportExports()` + `useTriggerExport()` |
| `frontend/app/(app)/analytics/page.tsx` | UPDATE | "Scheduled exports" card (list + Export now + download links) |

## NOT Building (later / out of scope)
- Per-user report subscriptions / custom schedules per recipient (one global cron).
- PDF rendering / charts in the export (CSV + JSON only).
- Configurable report builder UI (fixed funnel/kpi/sources snapshot).
- Multi-tenant recipient management (recipients from `REPORT_RECIPIENTS` env).
- Email infra — delivery rides the 5a notifier seam (mock default).

---

## Step-by-Step Tasks

### Task 1: Config — schedule + recipients
- **ACTION**: Add `ReportScheduleCron` (`getenv("REPORT_SCHEDULE_CRON","0 7 * * 1")`) and `ReportRecipients` (`getenv("REPORT_RECIPIENTS","")`, comma-split helper).
- **MIRROR**: `config.go:55-138`.
- **VALIDATE**: `go build ./...`.

### Task 2: reports snapshot + encoders
- **ACTION**: `export.go` — `Snapshot(ctx, period)` calls existing `Funnel/KPI/Sources`; `EncodeCSV`/`EncodeJSON`.
- **MIRROR**: REPORT_SNAPSHOT; `reports.go:42-102`.
- **VALIDATE**: unit test: snapshot of seeded data encodes non-empty CSV with funnel/kpi/sources sections.

### Task 3: migration — report_exports
- **ACTION**: `make migrate-create name=report_exports`. Up: `CREATE TABLE report_exports (id uuid PK default gen_random_uuid(), kind varchar(20), period varchar(40), csv_blob varchar(256), json_blob varchar(256), delivered bool default false, created_at timestamptz default now());`.
- **MIRROR**: `migrations/000005_ps_public.up.sql`.
- **GOTCHA**: use `make migrate-create` (don't hardcode the number — 5a/5c may take 000006).
- **VALIDATE**: migrate up/down/up.

### Task 4: exports repo + export service
- **ACTION**: `exports_repo.go` (Record/List). `export_service.go` — `Export(ctx, kind, period)`: snapshot → encode → `blob.Upload` csv+json → `RecordExport` → `notify.Send` to each recipient with `SignedURL`. Notify failure → log warn, mark delivered=false, do not fail.
- **MIRROR**: BLOB_EXPORT; EXPORTS_REPO; `peoplesoft/service.go:47-92` fail-safe.
- **GOTCHA**: deterministic blob names so retries overwrite; SignedURL ttl ≥ schedule interval.
- **VALIDATE**: unit test with fake blob + fake notifier: returns export row; notify error doesn't fail.

### Task 5: queue task + worker handler
- **ACTION**: Add `TypeExportReport` to `tasks.go`. In `cmd/worker/main.go` build the export service (pool→reports.Repo, blob, notify) and register `mux.HandleFunc(queue.TypeExportReport, handler)`.
- **MIRROR**: `tasks.go:11-51`; `worker/main.go:101-126`.
- **VALIDATE**: `go build ./cmd/worker`; integration: enqueue export → blob objects + `report_exports` row + `[mock notify]` log.

### Task 6: scheduler binary + compose
- **ACTION**: `cmd/scheduler/main.go` — load config, `asynq.NewScheduler`, register the cron → `report:export` weekly task, `Run()`. Add a `scheduler` service to `docker-compose.yml` (same image/build, command runs the scheduler binary, `deploy.replicas: 1`). Update `Makefile` `build` to compile it.
- **MIRROR**: SCHEDULER; `cmd/importref/main.go` for binary shape; compose `worker` service block.
- **GOTCHA**: exactly one scheduler replica.
- **VALIDATE**: `go build ./cmd/scheduler`; `docker compose up -d --build scheduler` logs registered cron entry.

### Task 7: API endpoints
- **ACTION**: Extend `reports/handler.go`: `GET /api/v1/reports/exports` (list recent, with signed download URLs) + `POST /api/v1/reports/exports` (trigger on-demand → either enqueue `report:export` or call export service directly; RBAC: super_admin/regional_director). Wire export service in `cmd/api/main.go`.
- **MIRROR**: `reports/handler.go:15-49`; envelope helpers `httpx.OK/Created`; RBAC role gate.
- **VALIDATE**: `curl POST /api/v1/reports/exports` → 201; `GET` lists it with download links.

### Task 8: dashboard card
- **ACTION**: `frontend/lib/types.ts` add `ReportExport`; `queries.ts` add `useReportExports()` + `useTriggerExport()`; Analytics page gets a "Scheduled exports" card (list rows with CSV/JSON links + "Export now" button → mutation → invalidate).
- **MIRROR**: `frontend/lib/queries.ts:81-89` + Analytics page structure; envelope client `lib/api.ts`.
- **VALIDATE**: `cd frontend && pnpm lint && pnpm build`; Playwright: card renders, "Export now" adds a row.

---

## Testing Strategy
### Unit
| Test | Input | Expected | Edge? |
|---|---|---|---|
| EncodeCSV | snapshot | CSV with 3 sections | — |
| export service | fake blob+notify | export row, 2 blobs, send called | — |
| export service notify fail | notifier err | returns ok, delivered=false | yes |
| list exports | N rows | newest-first, capped | yes |

### Edge Cases Checklist
- [ ] Empty dataset → export still produced (zeros), delivered
- [ ] Blob upload failure → task errors → asynq retry (transient)
- [ ] Notify failure → export persisted, delivered=false (no retry storm)
- [ ] No recipients configured → export stored, delivery skipped (logged)
- [ ] Scheduler restarted → no duplicate enqueue (single replica)

## Validation Commands
### Static + unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Build all binaries
```bash
cd backend && go build ./cmd/api ./cmd/worker ./cmd/scheduler
```
### Integration (stack up)
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./internal/reports/...
```
### Frontend
```bash
cd frontend && pnpm lint && pnpm exec tsc --noEmit && pnpm build
```
### Manual
- [ ] `POST /api/v1/reports/exports` → blob `reports/*.csv|json` created; `report_exports` row; `[mock notify]` log
- [ ] `GET /api/v1/reports/exports` → row with working signed download links
- [ ] scheduler container logs the registered weekly cron

## Acceptance Criteria
- [ ] `cmd/scheduler` enqueues `report:export` on `REPORT_SCHEDULE_CRON`; single replica in compose.
- [ ] Worker computes snapshot → CSV+JSON in Blob → `report_exports` row → notify (via 5a seam).
- [ ] `GET/POST /api/v1/reports/exports` work, RBAC-gated; Analytics card lists + triggers + downloads.
- [ ] vet/lint/`go test -race` pass; all three binaries build; migration round-trips; frontend builds.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Duplicate scheduling across replicas | Med | Med | dedicated `cmd/scheduler`, `replicas: 1`, documented |
| Signed URL expiry < schedule | Low | Med | ttl ≥ interval (default 7d for weekly) |
| 5a notifier not merged yet | Med | Low | depend on 5a; or stub a local notifier and swap |
| Blob/notify coupling fails export | Low | Med | fail-safe delivery; transient blob errors retried by asynq |

## Notes
- Reuses 5a's `notify.Notifier`. Sequence: land 5a, then 5b.
- A new `cmd/scheduler` binary + compose service is the cleanest single-dispatcher; `make up` already does `--build`.
