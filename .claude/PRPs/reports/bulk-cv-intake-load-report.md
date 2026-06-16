# Implementation Report: Bulk CV intake + load testing

## Summary
Added HR bulk CV upload (one position, many resumes → one application + pipeline job
per file) and a load-test harness. Reuses `Service.Intake` per file with a filename
placeholder; the pipeline overwrites the name from the parsed profile. Added a
positions list endpoint for the picker, a multipart client on the dashboard, the
`/applications/bulk` page, and `WORKER_CONCURRENCY` for load tuning.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 7.5/10 | held |
| Files Changed | ~16 | 9 new + 7 updated |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Positions list endpoint | ✅ | Reused existing `repo.ListAll` (no new repo method); slim `ListItem` projection; narrow `positionLister` iface |
| 2 | Bulk intake handler | ✅ | role-gated, per-file partial-failure, placeholder name, narrow `bulkIntaker` iface |
| 3 | Mount routes (api) | ✅ | bulk + positions wired |
| 4 | Worker concurrency env | ✅ | `WORKER_CONCURRENCY` (default 10) |
| 5 | Dashboard multipart client | ✅ | `api.postForm` |
| 6 | Types + queries + roles | ✅ | Position/BulkIntakeResult, usePositions/useBulkIntake, canBulkUpload |
| 7 | Bulk upload UI | ✅ | `/applications/bulk` page + `BulkUpload.tsx` (position + multi-file + result table) |
| 8 | Nav entry | ✅ | role-gated `BULK_NAV` via `canBulkUpload` |
| 9 | Load harness | ✅ | `loadtest/intake-load.js` (k6) + `loadtest/README.md` (+ pipeline-drain method) |
| 10 | Backend tests | ✅ | bulk (role/position/empty/mixed) + positions list |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static (go vet) | ✅ Pass | whole module |
| gofmt | ✅ Pass | config.go reformatted (pre-existing cmd/seedresumes untouched) |
| TypeScript | ✅ Pass | `tsc --noEmit` |
| Unit Tests | ✅ Pass | go test exit 0, 27 pkgs; new bulk + positions tests |
| Build | ✅ Pass | `go build ./...` + `next build` (route `/applications/bulk` present) |
| Load test | ⏭ Deferred | operator-run on staging (needs sample CV + auth cookie); not run in this pass |

## Files Changed
| File | Action |
|---|---|
| `backend/internal/positions/handler.go` (+test) | CREATE |
| `backend/internal/applications/bulk_handler.go` (+test) | CREATE |
| `loadtest/intake-load.js`, `loadtest/README.md` | CREATE |
| `frontend/app/(app)/applications/bulk/page.tsx` | CREATE |
| `frontend/components/applications/BulkUpload.tsx` | CREATE |
| `backend/cmd/api/main.go`, `cmd/worker/main.go`, `pkg/config/config.go` | UPDATE |
| `frontend/lib/api.ts`, `lib/types.ts`, `lib/queries.ts`, `lib/roles.ts`, `components/shell/nav-config.tsx` | UPDATE |

## Deviations from Plan
- **No new positions repo method** — the repo already had `ListAll` (active
  positions); the handler reuses it and projects to a slim DTO. Plan predicted a new
  `ListActive`; reuse is cleaner.
- **Load test not executed** — it is an operator artifact requiring a staging stack,
  a sample CV (uncommitted), and an HR session cookie. Script + runbook delivered;
  running it is a deploy-time step (documented in `loadtest/README.md`).

## Issues Encountered
- `multipart.FileHeader` type (not a made-up alias) — fixed import in bulk_handler.go.
- gofmt alignment on the config struct/loader after adding fields — `gofmt -w`.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `internal/applications/bulk_handler_test.go` | 4 | role gate, missing position, empty files, mixed valid/invalid + placeholder name |
| `internal/positions/handler_test.go` | 1 | list shape (slim projection) |

## Deployment Notes
- No migration. Rebuild + roll **api + worker** (worker changed: WORKER_CONCURRENCY).
- New endpoints: `GET /api/v1/positions`, `POST /api/v1/applications/bulk-intake` (both authed; bulk role-gated). Dashboard gains `/applications/bulk`.
- Optional: set `WORKER_CONCURRENCY` (default 10) when load-tuning alongside Azure TPM.

## Next Steps
- [ ] Code review via `/code-review`
- [ ] PR + deploy (api + dashboard + worker)
- [ ] Run load test on staging; record CVs/min + 429 rate → feeds PRP-3
