# Implementation Report: Azure AI Search — Index Population (Slice 2.4)

## Summary
Built the missing **index ingestion** half of the AI Search seam (the query path
already existed). Phases A+B+C complete: index schema (`EnsureIndex`, Thai
`th.microsoft` analyzer), the `Indexer` seam (Azure REST push + mock no-op),
doc projection, `cmd/reindex` backfill, best-effort incremental hooks (pipeline +
bulk), Bicep module + `deploySearch` wiring, and a provisioning runbook.
**Validated end-to-end against a real Azure AI Search service** (free tier,
provisioned in prod RG): index created, 16 docs backfilled, Thai-name + province
queries return relevant results. Mock-default invariant preserved throughout.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | XL (3 phases) | XL — all 3 phases delivered |
| Confidence | 7/10 | 9/10 (real-service E2E validated before merge) |
| Files Changed | ~10 | 11 (7 new, 4 edited) |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Index schema + EnsureIndex | ✅ | `index.go`; `th.microsoft`; PUT create-or-update (200/201/204) |
| 2 | Indexer interface + impls + factory | ✅ | `indexer.go`; mergeOrUpload, ≤500/batch, per-doc failure surfaced, no-op |
| 3 | Doc projection | ✅ | `docs.go`; best-app per candidate + subregion/store, no scope |
| 4 | Backfill command | ✅ | `cmd/reindex`; mock→no-op; **verified: 16 docs to real Search** |
| 5 | Pipeline incremental hook | ✅ | `process.go`; `SetIndexer` (setter, no ctor churn) + best-effort post-run |
| 6 | Wire worker + api | ✅ | `CandidateSync` adapter (no import cycle); api `EnsureIndex` at startup |
| 7 | Status-change hook | ✅ | `dashboard_handler.go` Bulk → re-index affected candidates best-effort |
| 8 | Provision + runbook | ✅ | `docs/azure-search-provisioning.md`; **service provisioned** (free, SE Asia) |
| 9 | Bicep module | ✅ | `infra/modules/search.bicep` + `deploySearch` env/secret wiring; `bicep build` clean |
| 10 | Flip + validate | ✅ (staging) | E2E vs real Search: `ศรีสุข`→1, `เชียงใหม่`→5; **prod flip pending deploy** |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go build ./...` + `go vet ./...` clean; `bicep build` clean (1 lint warning, same as openai.bicep) |
| Unit Tests | ✅ Pass | 5 (schema, batch chunking ≤500, partial-failure, hard-status, no-op) |
| Integration | ✅ Pass | projection vs live DB; **full pipeline vs real Azure Search** (index+backfill+Thai query) |
| Build | ✅ Pass | all binaries incl. `cmd/reindex` |
| Full Suite | ✅ Pass | `go test ./...` green — mock invariant intact |

## Files Changed
| File | Action |
|---|---|
| `backend/internal/search/index.go` | CREATED |
| `backend/internal/search/indexer.go` | CREATED |
| `backend/internal/search/docs.go` | CREATED |
| `backend/internal/search/adapter.go` | CREATED |
| `backend/internal/search/indexer_test.go` | CREATED |
| `backend/internal/search/docs_integration_test.go` | CREATED |
| `backend/cmd/reindex/main.go` | CREATED |
| `backend/internal/pipeline/process.go` | UPDATED (indexer dep + hook) |
| `backend/cmd/worker/main.go` | UPDATED (wire indexer) |
| `backend/cmd/api/main.go` | UPDATED (EnsureIndex + dashboard indexer) |
| `backend/internal/applications/dashboard_handler.go` | UPDATED (status hook) |
| `infra/modules/search.bicep` | CREATED |
| `infra/main.bicep` | UPDATED (module + env/secret wiring) |
| `docs/azure-search-provisioning.md` | CREATED |

## Deviations from Plan
1. **Used a `SetIndexer` setter** instead of extending `NewProcessor`/`NewDashboardHandler` signatures — avoids touching the pipeline integration-test caller and keeps ctors stable. Same effect, less churn.
2. **Found `cmd/reindex` config strictness** — `config.Load()` validates the full app config (REDIS_URL, blob, JWT), so the backfill job must inherit the app env. Documented in the runbook rather than relaxing config (out of scope).

## Issues Encountered
- Search `Microsoft.Search` RP wasn't registered on the sub — `az` auto-registered it.
- First `cmd/reindex` runs failed on missing REDIS_URL / blob env (strict config) — resolved by supplying the full app env (and documented).

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `indexer_test.go` | 5 | schema, batching, partial/hard failure, no-op |
| `docs_integration_test.go` | 1 (gated) | projection best-app + subregion/store |

## Next Steps
- [ ] PR → review → merge (backend + infra + docs)
- [ ] Deploy **api + worker** from main (worker keeps the index fresh)
- [ ] Prod cutover: set `AI_SEARCH_PROVIDER=azure` + `AZURE_SEARCH_*` on api+worker, restart, run `cmd/reindex` against prod DB, smoke test
- [ ] Follow-ups: ACA Job for reindex; PDPA-delete index hook; relevance A/B at scale
