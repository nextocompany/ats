# Plan: Azure AI Search — Index Population (Sprint 8 slice 2.4)

## Summary
The Azure AI Search **query** path is fully built (`internal/search/azure.go`) but the index it
queries is never created or populated, so `AI_SEARCH_PROVIDER=azure` would return nothing. This
plan builds the missing half: an **index management + push (ingestion)** layer — create the
`candidates` index (Thai analyzer), **backfill** all existing candidates, and **keep it updated**
on intake/scoring and status changes — then flip the flag in staging and validate relevance vs the
existing Postgres trigram baseline.

## User Story
As an HR user, I want candidate search powered by Azure AI Search so that searching Thai names and
provinces returns linguistically-relevant matches (tokenized for Thai) faster and better than the
trigram `ILIKE` baseline — across the whole national roster, always within my RBAC scope.

## Problem → Solution
`azureSearcher.Search` POSTs to an index that has no schema and no documents (query-only seam) →
add an `Indexer` seam (mock no-op + Azure REST push) that ensures the index schema, backfills via a
one-off command, and upserts incrementally from the pipeline + status-change paths.

## Metadata
- **Complexity**: XL (new ingestion subsystem + cmd + infra + pipeline wiring) — **split into 3 phases**
- **Source PRD**: `.claude/PRPs/plans/sprint-8-go-live-roadmap.md` (slice 2.4)
- **PRD Phase**: Phase 2 — Provision + Flip Real Seams → 2.4 Azure AI Search
- **Estimated Files**: ~10 (4 new backend, 3 edited backend, 1 cmd, 1 infra module, 1 runbook doc)
- **Decision (locked)**: keep-updated covers the two real mutation paths — **(a) pipeline end (new/rescored apps)** and **(b) operator status changes (bulk)**. Both best-effort, non-blocking.
- **Mock-default invariant**: when `AI_SEARCH_PROVIDER=mock` (CI/local), the Indexer is a **no-op** — zero behavior change, no Azure creds needed (mirrors the existing `NewSearcher` seam).

---

## UX Design
**Internal/backend change — no user-facing UX change.** The HR Search page (`frontend/app/(app)/search/page.tsx`)
already calls `/api/v1/candidates/search`; its results simply become Azure-backed when the flag flips.
Observable difference: better relevance/ranking on Thai text. No frontend edits.

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/search/azure.go` | all | The query side + the **exact REST call pattern** (api-key header, api-version, error-body surfacing, `scopeFilter`/`escapeOData`) to mirror for push/ensure. Defines the index field contract. |
| P0 | `backend/internal/search/search.go` | all | `Searcher` interface + `NewSearcher` **seam factory** to mirror for `NewIndexer`; `Query`/`Hit`; `UsesAzureSearch` gate |
| P0 | `backend/internal/search/pg.go` | all | The `ranked` CTE = the **doc projection to mirror** (one row per candidate = best application). Add `subregion` + `assigned_store_id` to it; drop the scope clause (index holds all; query filters). |
| P0 | `backend/internal/pipeline/process.go` | 40-64, 65-97, 198-225 | `NewProcessor` ctor (add an indexer dep), `HandleProcessApplication` (best-effort hook after `run` succeeds), the scored/rejected terminal points |
| P1 | `backend/pkg/config/config.go` | 40-45, 138-141, 188-230, 295-298 | Search env fields, the `Validate` allow-list, `UsesAzureSearch()`. Already complete — no new config needed beyond confirming the key is an **admin** key. |
| P1 | `backend/cmd/worker/main.go` | 1-160 | Bootstrap pattern (config → pool → redis → DI → `mux.HandleFunc`) to **mirror for `cmd/reindex`** and to wire the Indexer into the Processor |
| P1 | `backend/internal/applications/dashboard_handler.go` | Bulk (~105-145) | `SetStatus` loop = the **api-side incremental hook** (re-index affected candidates after a status change) |
| P2 | `backend/internal/search/pg_integration_test.go` | all | Integration-test pattern (real pool, gated) to mirror for an indexer/projection test |
| P2 | `docs/azure-openai-provisioning.md` | all | The **runbook format** to mirror for `docs/azure-search-provisioning.md` |
| P2 | `infra/modules/*.bicep` + `infra/main.bicep` | search/`deploySearch` | Existing `deploySearch` toggle (default false) to wire a real `search.bicep` module |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Push/load index | learn.microsoft.com/azure/search/search-how-to-load-search-index | `POST /indexes/{name}/docs/index?api-version=2024-07-01`, body `{"value":[{"@search.action":"mergeOrUpload", ...}]}`. **Batch ≤ 1000 docs / 16 MB**, whichever first. |
| mergeOrUpload | (same) | Behaves like `upload` if new, `merge` if the key exists — idempotent upsert. Key = `candidate_id`. |
| Thai analyzer | learn.microsoft.com/azure/search/index-add-language-analyzers | Thai = **`th.microsoft`** (Lucene alt `th.lucene`). Set via the `"analyzer"` property on a `searchable` `Edm.String` field. Critical: Thai has no spaces → default analyzer mis-tokenizes names. |
| Create index | learn.microsoft.com/rest/api/searchservice/indexes/create | `PUT /indexes/{name}?api-version=2024-07-01` with `{name, fields:[...]}`. Idempotent for create-or-update of schema. Needs the **admin** api-key (query key is read-only). |
| api-version | existing `azure.go` `searchAPIVersion = "2024-07-01"` | Reuse the **same constant** for index + push calls. |

```
KEY_INSIGHT: The query code already assumes index fields: candidate_id, full_name, province, status, ai_score, subregion, assigned_store_id (see azure.go scopeFilter + azureSearchResponse). The index schema MUST match these names exactly.
APPLIES_TO: index schema (Task 1) + doc projection (Task 3)
GOTCHA: scopeFilter emits `assigned_store_id eq -1` and `subregion eq '...'` → those fields must be filterable; assigned_store_id is Edm.Int32, subregion Edm.String. ai_score must be filterable (ge) and ideally sortable.

KEY_INSIGHT: AZURE_SEARCH_KEY is used today only for query; push + index-create need an ADMIN key.
APPLIES_TO: runbook + config note
GOTCHA: Provision so AZURE_SEARCH_KEY = an ADMIN key (works for query too). A query-only key will 403 on index/push.
```

---

## Patterns to Mirror

### SEAM_FACTORY (mirror for NewIndexer)
```go
// SOURCE: backend/internal/search/search.go:59-66
func NewSearcher(cfg *config.Config, pool *pgxpool.Pool) Searcher {
	if cfg.UsesAzureSearch() {
		return newAzureSearcher(cfg)
	}
	return newPGSearcher(pool)
}
```
`NewIndexer(cfg) Indexer` → `azureIndexer` when `UsesAzureSearch()`, else `noopIndexer`.

### AZURE_REST_CALL (mirror for EnsureIndex + push)
```go
// SOURCE: backend/internal/search/azure.go:58-98
req.Header.Set("api-key", s.key); req.Header.Set("Content-Type", "application/json")
resp, err := s.http.Do(req) ; ...
if resp.StatusCode != http.StatusOK {          // index/push success codes differ — see GOTCHA
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("search: azure status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
```

### DOC_PROJECTION (mirror pg.go ranked CTE; add subregion + assigned_store_id, drop scope)
```go
// SOURCE: backend/internal/search/pg.go:39-60  (the `ranked` CTE)
// New projection (no scope clause — the index holds everything; the QUERY scopes):
//   WITH ranked AS (
//     SELECT c.id AS candidate_id, c.full_name, COALESCE(c.province,'') province,
//            COALESCE(c.subregion,'') subregion, a.assigned_store_id, a.status, a.ai_score,
//            ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY a.ai_score DESC NULLS LAST) rn
//     FROM candidates c JOIN applications a ON a.candidate_id=c.id
//     WHERE c.is_duplicate_of IS NULL [AND c.id = $1 for single])
//   SELECT ... FROM ranked WHERE rn=1
```

### CMD_BOOTSTRAP (mirror for cmd/reindex)
```go
// SOURCE: backend/cmd/worker/main.go:42-60
cfg, err := config.Load(); logging.Configure(cfg.IsDevelopment())
var pool *pgxpool.Pool
bootstrap.Retry(ctx, "postgres", func(ctx) error { p,e:=database.Connect(ctx,cfg.DatabaseURL); pool=p; return e })
```

### PIPELINE_DI (add a small indexer interface — NO import cycle)
```go
// SOURCE: backend/internal/pipeline/process.go:52-63 (NewProcessor)
// Add to pipeline package (pipeline must NOT import search → define a local interface):
type CandidateIndexer interface { Index(ctx context.Context, candidateID uuid.UUID) error }
// noopCandidateIndexer{} default; concrete impl wired in cmd/worker closes over (pool, search.Indexer).
```

### ERROR/LOGGING (best-effort, non-blocking)
```go
// SOURCE: backend/internal/pipeline/process.go:217-222 (zerolog usage)
if err := pr.indexer.Index(ctx, candID); err != nil {
	logger.Warn().Err(err).Msg("search index update failed (non-fatal)")  // NEVER return — search staleness must not fail the pipeline
}
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/search/index.go` | CREATE | Index schema JSON + `EnsureIndex` (PUT) — Azure index admin |
| `backend/internal/search/indexer.go` | CREATE | `Indexer` interface + `azureIndexer` (push/mergeOrUpload) + `noopIndexer` + `NewIndexer` factory |
| `backend/internal/search/docs.go` | CREATE | `Doc` struct + `FetchDoc(pool,id)` + `FetchAllDocs(pool,offset,limit)` (the projection) |
| `backend/internal/search/indexer_test.go` | CREATE | Unit: schema JSON shape, batch chunking ≤1000, mock no-op; doc projection (gated integration) |
| `backend/cmd/reindex/main.go` | CREATE | One-off backfill: EnsureIndex → page FetchAllDocs → UpsertBatch (≤1000) → progress log |
| `backend/internal/pipeline/process.go` | UPDATE | `CandidateIndexer` dep on `Processor`; best-effort `Index(candID)` after `run` succeeds |
| `backend/cmd/worker/main.go` | UPDATE | Build `search.NewIndexer(cfg)` + adapter, inject into `NewProcessor` |
| `backend/cmd/api/main.go` | UPDATE | Build indexer; `EnsureIndex` at startup (azure only); inject into bulk handler |
| `backend/internal/applications/dashboard_handler.go` | UPDATE | After `SetStatus` in Bulk, best-effort re-index affected candidates |
| `infra/modules/search.bicep` (+ `main.bicep` wire) | CREATE/UPDATE | Real Azure AI Search module behind existing `deploySearch` toggle |
| `docs/azure-search-provisioning.md` | CREATE | Runbook: `az search service create`, admin key, env/secrets, flip + smoke test |

## NOT Building
- **No change to the query path** (`azure.go` Search) — it already works; this plan only feeds its index.
- **No frontend change** — Search page is provider-agnostic.
- **No asynq index-task** — incremental updates are synchronous best-effort at the two mutation sites; a dedicated `TypeIndexCandidate` queue task is a future robustness upgrade (noted in Risks), not v1.
- **No semantic/vector search, scoring profiles, synonym maps, or suggesters** — lexical Thai analyzer only for v1.
- **No automated index deletion/migration tooling** — schema changes are handled by re-running `cmd/reindex` (mergeOrUpload is additive; a breaking field change is a manual drop+recreate, documented in the runbook).
- **No CI test against a live Search service** — integration tests are build-tag/env gated like `pg_integration_test.go`.

---

## Step-by-Step Tasks

> **Phase A (core ingestion): Tasks 1–4 · Phase B (incremental + wiring): Tasks 5–7 · Phase C (provision + flip + validate): Tasks 8–10.** Each phase is independently shippable; A leaves the seam mock-safe.

### Task 1: Index schema + EnsureIndex (`internal/search/index.go`)
- **ACTION**: Define the `candidates` index schema and an idempotent `EnsureIndex`.
- **IMPLEMENT**: A Go struct/`map` → JSON matching the field contract: `candidate_id` (Edm.String, `key:true`, retrievable), `full_name` (Edm.String, searchable, retrievable, `analyzer:"th.microsoft"`), `province` (Edm.String, searchable, retrievable, `analyzer:"th.microsoft"`), `subregion` (Edm.String, filterable, retrievable), `assigned_store_id` (Edm.Int32, filterable, retrievable), `status` (Edm.String, filterable, retrievable), `ai_score` (Edm.Double, filterable, sortable, retrievable). `EnsureIndex(ctx)`: `PUT {endpoint}/indexes/{index}?api-version={searchAPIVersion}` with the schema; treat 200/201 as success.
- **MIRROR**: `AZURE_REST_CALL` (azure.go). Reuse `searchAPIVersion` const.
- **IMPORTS**: same as azure.go (`bytes, encoding/json, net/http, io, fmt, strings`).
- **GOTCHA**: PUT create-or-update returns **201 (created) or 204/200**; accept 200/201/204. Field names MUST equal what `azure.go` `scopeFilter`/`azureSearchResponse` already use. The admin key is required (query key → 403).
- **VALIDATE**: `go build ./internal/search/`; unit asserts the JSON contains `"analyzer":"th.microsoft"` on full_name and `key:true` on candidate_id.

### Task 2: Indexer interface + impls + factory (`internal/search/indexer.go`)
- **ACTION**: `Indexer` seam with Azure push + no-op, plus `NewIndexer`.
- **IMPLEMENT**: `type Indexer interface { EnsureIndex(ctx) error; UpsertBatch(ctx, []Doc) error }`. `azureIndexer` (same fields as `azureSearcher`): `UpsertBatch` POSTs `/indexes/{index}/docs/index?api-version=...` with `{"value":[{"@search.action":"mergeOrUpload", ...doc}]}`, **chunking input into ≤500 docs/request** (safe under the 1000/16MB cap). `noopIndexer` returns nil for all. `func NewIndexer(cfg) Indexer { if cfg.UsesAzureSearch() { return newAzureIndexer(cfg) }; return noopIndexer{} }`.
- **MIRROR**: `SEAM_FACTORY` + `AZURE_REST_CALL`.
- **IMPORTS**: as azure.go.
- **GOTCHA**: docs/index returns **200 even on partial failure** — parse the response `value[].status`/`errorMessage` and return an error if any doc failed. `@search.action` and field keys are JSON tags; `assigned_store_id`/`ai_score` are nullable (`*int`/`*float64`) → omit or send null, never 0.
- **VALIDATE**: unit: a 1200-doc slice produces 3 batched requests (round-trip via an `httptest.Server`); no-op indexer makes zero HTTP calls.

### Task 3: Doc projection (`internal/search/docs.go`)
- **ACTION**: SQL projection producing one `Doc` per candidate (best application), for single + paged reads.
- **IMPLEMENT**: `type Doc struct { CandidateID, FullName, Province, Subregion, Status string; AssignedStoreID *int; AIScore *float64 }`. `FetchDoc(ctx, pool, id uuid.UUID) (Doc, bool, error)` and `FetchAllDocs(ctx, pool, offset, limit int) ([]Doc, error)` using the `ranked` CTE from pg.go **plus** `c.subregion`, `a.assigned_store_id`, **without** the scope clause, `WHERE rn=1`.
- **MIRROR**: `DOC_PROJECTION` (pg.go).
- **IMPORTS**: `context`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/google/uuid`.
- **GOTCHA**: a candidate with **no application** has no doc (the JOIN drops them) — acceptable (search returns candidates-with-applications, matching pg.go today). `is_duplicate_of IS NULL` filter must stay (don't index dup shadows).
- **VALIDATE**: gated integration test (mirror `pg_integration_test.go`) — seed 2 candidates, assert `FetchAllDocs` returns 2 docs with correct best-app status/score/store.

### Task 4: Backfill command (`cmd/reindex/main.go`)
- **ACTION**: One-off binary: ensure index, then page all candidates → batch upsert.
- **IMPLEMENT**: Bootstrap (config + pool). If `!cfg.UsesAzureSearch()` → log "search is mock; nothing to index" and exit 0. Else `idx := search.NewIndexer(cfg); idx.EnsureIndex(ctx)`; loop `FetchAllDocs(offset, 500)` → `idx.UpsertBatch` until a short page; log running count.
- **MIRROR**: `CMD_BOOTSTRAP` (cmd/worker).
- **IMPORTS**: as cmd/worker (subset — no redis/asynq needed).
- **GOTCHA**: run **after** migrations + data seed at cutover. Idempotent (mergeOrUpload) — safe to re-run. This is the **ACA Job** for backfill (infra Task) — exit non-zero on failure so the job is retryable.
- **VALIDATE**: `go build ./cmd/reindex/`; against local Azurite-less mock it's a clean no-op; against a real service (staging) it reports N docs and `azureSearcher.Search("*")` returns them.

### Task 5: Pipeline incremental hook (`internal/pipeline/process.go`)
- **ACTION**: Add a `CandidateIndexer` dep and best-effort index after a successful run.
- **IMPLEMENT**: Add `type CandidateIndexer interface { Index(ctx, candidateID uuid.UUID) error }` + `noopCandidateIndexer`; add field `indexer CandidateIndexer` to `Processor`; extend `NewProcessor(... , idx CandidateIndexer)` (default to no-op if nil). In `HandleProcessApplication`, after `run()` returns nil, call `pr.indexer.Index(ctx, candID)` and **log-warn on error, never fail**.
- **MIRROR**: `PIPELINE_DI` + `ERROR/LOGGING`.
- **IMPORTS**: `github.com/google/uuid` (already imported).
- **GOTCHA**: NO import of `search` from `pipeline` (cycle) — depend on the local interface only. The concrete adapter lives in cmd/worker.
- **VALIDATE**: `go build ./...`; existing pipeline tests pass with the no-op default; `go vet`.

### Task 6: Wire indexer into worker + api (`cmd/worker/main.go`, `cmd/api/main.go`)
- **ACTION**: Construct the real indexer adapter and inject it.
- **IMPLEMENT**: In both: `idx := search.NewIndexer(cfg)`. Define a tiny adapter `candidateIndexer{pool, idx}` implementing `pipeline.CandidateIndexer.Index(ctx,id)` = `d,ok,err := search.FetchDoc(ctx,pool,id); if ok { idx.UpsertBatch(ctx,[]search.Doc{d}) }`. Worker: pass to `NewProcessor`. API: `if cfg.UsesAzureSearch() { idx.EnsureIndex(ctx) }` at startup (log-warn on failure, don't crash); hold `idx`+pool for the bulk handler.
- **MIRROR**: `CMD_BOOTSTRAP`, `SEAM_FACTORY`.
- **GOTCHA**: api EnsureIndex at startup must be best-effort (a transient Search outage must not block the api booting). Place the adapter in a shared spot (e.g. `search.NewCandidateIndexer(pool, idx)` returning a `pipeline.CandidateIndexer`-compatible type) to avoid duplicating it in two `main.go`s — but keep the interface in `pipeline` to avoid the cycle (search may import pipeline? No — define the adapter type in search returning a struct with an `Index` method; both packages structurally satisfy `pipeline.CandidateIndexer`).
- **VALIDATE**: `go build ./...`; mock path: zero Azure calls; both binaries start.

### Task 7: Status-change incremental hook (`internal/applications/dashboard_handler.go`)
- **ACTION**: Re-index candidates whose status changed via Bulk.
- **IMPLEMENT**: Inject a `pipeline.CandidateIndexer` (or a minimal `Index(ctx, candidateID)` func) into `DashboardHandler`. In the Bulk loop, after a successful `SetStatus`, resolve the application's `candidate_id` and best-effort `Index(candidateID)` (log-warn on error).
- **MIRROR**: `ERROR/LOGGING`; the existing Bulk loop structure.
- **GOTCHA**: Bulk operates on application IDs; map app → candidate_id (one extra lookup, or have `SetStatus` return it). Keep best-effort; a stale status filter is preferable to a failed bulk action.
- **VALIDATE**: `go build ./...`; mock path unchanged; handler test (if present) still green.

### Task 8: Provision Azure AI Search + runbook (`docs/azure-search-provisioning.md`)
- **ACTION**: Operator runbook to create the service and wire env.
- **IMPLEMENT**: `az search service create -g hrats-prod-rg -n hrats-prod-search --sku basic -l southeastasia` (Free sku = 1 partition, 50MB, 3 indexes — OK for pilot; Basic for headroom). Get **admin** key (`az search admin-key show`). Set ACA secrets/env on api + worker: `AI_SEARCH_PROVIDER=azure`, `AZURE_SEARCH_ENDPOINT=https://hrats-prod-search.search.windows.net`, `AZURE_SEARCH_KEY=<admin>` (secret), `AZURE_SEARCH_INDEX=candidates`. Document the flip → restart revision → run `cmd/reindex` (ACA Job) → smoke test.
- **MIRROR**: `docs/azure-openai-provisioning.md` format.
- **GOTCHA**: SE-Asia availability — confirm the region offers AI Search on the MPN sub (fallback region like the OpenAI eastus case — see [[azure-ai-enabled]]). Secret-only env change does NOT roll an ACA revision → `az containerapp revision restart` (gotcha #6 in [[azure-aca-infra]]).
- **VALIDATE**: runbook dry-read; `curl` the index `GET /indexes/candidates` returns the schema; `/docs/$count` > 0 after reindex.

### Task 9: Bicep module (`infra/modules/search.bicep` + `main.bicep`)
- **ACTION**: IaC for the Search service behind the existing `deploySearch` toggle.
- **IMPLEMENT**: `Microsoft.Search/searchServices` (sku basic, 1 replica/partition); output endpoint + (keys via listAdminKeys for secret wiring). Wire into `main.bicep` under `if (deploySearch)`; add `AZURE_SEARCH_*` to api+worker env/secret refs.
- **MIRROR**: existing `infra/modules/openai.bicep` / `docintel.bicep` module shape.
- **GOTCHA**: `bicep build` must stay clean; keep `deploySearch=false` default so existing deploys are unaffected. Keys via `listAdminKeys()` not hardcoded.
- **VALIDATE**: `az bicep build -f infra/main.bicep` clean.

### Task 10: Flip in staging + validate relevance
- **ACTION**: Enable on a staging/pilot env, reindex, and compare to trigram.
- **IMPLEMENT**: Apply runbook on the pilot, run `cmd/reindex`, then exercise `/api/v1/candidates/search?q=<thai name>` and province queries; compare result quality + ordering to `AI_SEARCH_PROVIDER=mock`. Verify RBAC scope (store/subregion users get filtered results) and pagination/count parity.
- **VALIDATE**: see Manual Validation checklist.

---

## Testing Strategy

### Unit / Component Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| index schema JSON | EnsureIndex body | contains `analyzer:"th.microsoft"` (full_name, province), `key:true` (candidate_id), `Edm.Int32` assigned_store_id | — |
| batch chunking | 1200 Docs | 3 POSTs (≤500 each) to `/docs/index` (httptest) | max-size |
| partial-failure parse | docs/index 200 with a `value[].status:false` | UpsertBatch returns error | failure mode |
| no-op indexer | any | zero HTTP calls, nil error | mock invariant |
| doc projection (gated) | 2 seeded candidates, multi-app | 2 Docs, best-app status/score/store, dups excluded | rn=1, is_duplicate_of |
| nullable fields | candidate, rejected app (no store) | Doc.AssignedStoreID == nil (not 0) | null vs 0 |

### Edge Cases Checklist
- [ ] Candidate with no application → no doc (not an error)
- [ ] Rejected/unassigned candidate → null `assigned_store_id` (store-scoped users correctly don't match)
- [ ] Thai name tokenization (`th.microsoft`) — partial-name query matches
- [ ] Search service transient 5xx during pipeline → pipeline still completes (best-effort)
- [ ] `AI_SEARCH_PROVIDER=mock` → no Azure calls anywhere (CI safe)
- [ ] Backfill re-run → idempotent (mergeOrUpload), no dupes
- [ ] >1000 candidates → batches, all indexed

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./internal/search/ ./internal/pipeline/ ./cmd/reindex/
```
EXPECT: clean

### Unit Tests
```bash
cd backend && go test ./internal/search/ -run 'Indexer|Index|Doc' -count=1
```
EXPECT: pass (no Azure creds — httptest + no-op)

### Full Suite (no regressions)
```bash
cd backend && go test ./... -count=1
```
EXPECT: pass (mock-default; integration tests gated/skip without a DB/Search)

### Bicep
```bash
az bicep build -f infra/main.bicep
```
EXPECT: clean; `deploySearch` default false

### Backfill smoke (staging, real Search)
```bash
AI_SEARCH_PROVIDER=azure AZURE_SEARCH_ENDPOINT=... AZURE_SEARCH_KEY=<admin> \
  go run ./cmd/reindex
curl -s -H "api-key: <key>" "$AZURE_SEARCH_ENDPOINT/indexes/candidates/docs/\$count?api-version=2024-07-01"
```
EXPECT: count == number of candidates-with-applications

### Manual Validation
- [ ] `GET /indexes/candidates` returns the expected schema (Thai analyzer present)
- [ ] Search a Thai name fragment → relevant candidate ranks top; province search works
- [ ] Store-scoped + subregion-scoped users get correctly filtered results (no leakage)
- [ ] Apply a new CV → within seconds the candidate is searchable (pipeline hook)
- [ ] Bulk status change → search status filter reflects it (status hook)
- [ ] Relevance/ordering ≥ trigram baseline on a sample of real Thai queries

---

## Acceptance Criteria
- [ ] All tasks complete; `go build ./...` + `go vet` clean; `go test ./...` green
- [ ] `AI_SEARCH_PROVIDER=mock` path unchanged (no Azure calls, CI green) — the seam invariant
- [ ] `cmd/reindex` backfills all candidates-with-applications idempotently
- [ ] New applications + status changes keep the index fresh (best-effort, non-blocking)
- [ ] Staging: Azure search returns scoped, relevant Thai results ≥ trigram baseline
- [ ] Runbook + bicep module exist; `bicep build` clean; `deploySearch` default false

## Completion Checklist
- [ ] Mirrors the search seam factory + azure REST + pg projection patterns exactly
- [ ] Index field names match `azure.go`'s existing query contract
- [ ] Best-effort hooks never fail the pipeline or a bulk action
- [ ] No import cycle (pipeline ↔ search) — local `CandidateIndexer` interface
- [ ] Admin-key requirement documented; no secrets hardcoded
- [ ] Self-contained — implementable without further searching

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Synchronous best-effort hook adds pipeline latency / coupling | Med | Low | Best-effort + short HTTP timeout; upgrade path = asynq `TypeIndexCandidate` task (noted, not v1) |
| Index goes stale (a mutation path not hooked, e.g. PDPA retention delete) | Med | Med | v1 hooks scoring + bulk; **deletes** not yet hooked → document a periodic `cmd/reindex` re-sync; add a `delete` action hook in a follow-up |
| AI Search not available on MPN sub in SE-Asia | Med | Med | Mirror the OpenAI region workaround ([[azure-ai-enabled]]) — pick an available region; endpoint is region-independent in code |
| Admin vs query key confusion → 403 on push | Med | Low | Runbook: AZURE_SEARCH_KEY = admin key; Task-1 GOTCHA |
| Thai analyzer relevance underwhelms vs trigram | Low | Med | Task 10 explicit A/B; `th.lucene` fallback; keep mock as instant rollback (flip flag) |
| Index schema change later needs drop+recreate | Low | Low | mergeOrUpload is additive; document drop+recreate+reindex in runbook |

## Notes
- **Instant rollback**: this is a flag flip — set `AI_SEARCH_PROVIDER=mock` to revert to the trigram baseline with zero code change. De-risks go-live.
- **Deploy shape**: backend-only (api + worker images) + one ACA Job (reindex) + the Search resource. No frontend, no DB migration.
- **Sequence with go-live**: provision (Task 8/9) has lead time → start early; backfill (Task 4) runs at cutover **after** the real data seed (D5 in the roadmap).
- Companion context: the seam mirrors AI/Notify/PeopleSoft; the `deploySearch` bicep toggle already exists ([[azure-aca-infra]]).
