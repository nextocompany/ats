# Implementation Report: Sprint 3 — PeopleSoft + Public Career API + Reference Import

## Summary
Connected the platform to PeopleSoft bi-directionally, exposed the public Career Portal **API**, and added a real-data CSV importer — all Go (backend-first; the Next.js portal UI is deferred to Sprint 4). PeopleSoft webhooks now create/close vacancies (mapped to internal positions); marking an application `hired` pushes it to PeopleSoft (Integration Broker REST) with a CSV-to-Blob fallback. The public API lets candidates browse open positions, apply (mock LINE auth, reusing the full intake→score→assign pipeline), and check status by an opaque token. PS + LINE follow the established mock-default-behind-config seam. Verified end-to-end on the live stack.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | Single-pass; only test-signature fixups after the impl |
| Files Changed | ~26 | 18 created + 13 modified |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000005 | ✅ | ps_position_code + public_token (partial-unique indexes); reversible |
| 2 | Config (PS + LINE) | ✅ | mock defaults; conditional fail-fast; UsesRealPeopleSoft/UsesRealLINE |
| 3 | Vacancies upsert/close/list | ✅ | idempotent upsert by ps_vacancy_id |
| 4 | Positions FindByPSCode + ListPublic | ✅ | public list = positions with ≥1 open vacancy |
| 5 | PS client (mock/REST/fallback) | ✅ | OAuth2 client-credentials REST; mock default |
| 6 | PS service (SyncHired) | ✅ | success→ps_synced_at; failure→CSV fallback, hire preserved |
| 7 | PS webhooks + health + routes | ✅ | unmapped code stored (not dropped) |
| 8 | LINE auth (mock/real) | ✅ | mock accepts stub token |
| 9 | Public Career API | ✅ | list/detail/apply/status; opaque crypto token; minimal projection |
| 10 | Status PATCH + hired→PS | ✅ | HiredSyncer interface (no import cycle) |
| 11 | Reference-data importer | ✅ | cmd/importref + sample CSVs + make import; subregion/centroid derivation |
| 12 | Wire api + docs | ✅ | PS/public routes, LINE verifier, README |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ | `go vet` clean; `golangci-lint` 0 issues |
| Unit Tests (`-race`) | ✅ | config (PS/LINE), auth, peoplesoft (service + webhook) |
| Build | ✅ | `go build` + `docker compose build` |
| Integration (`-tags integration`) | ✅ | pipeline suite passes |
| Edge Cases | ✅ | unmapped code stored, PS-fail→CSV fallback, migration 000005 round-trip |

### End-to-end evidence (live stack, mock providers)
```
PS vacancy-opened (CASHIER) → mapped:true, vacancy upserted
/public/positions → Cashier open_count:1
/public/apply (X-LINE-IdToken: dev-stub) → opaque status_token
/public/status/<token> → status "scored", position "แคชเชียร์" (no internal fields)
PATCH /applications/<id>/status {hired} → status hired, ps_synced_at set, assigned_store_id 1
/ps/health → provider mock
```
The career-portal application traversed the full S1→S2→S3 chain (parsed → scored → assigned → hired → PS-synced).

## Files Changed
18 created, 13 modified. New: `internal/peoplesoft/*`, `internal/public/*`, `internal/auth/line.go`, `cmd/importref`, migration `000005`, sample CSVs. Modified: config, applications (model/repo/handler/routes), positions, vacancies, api main.

## Deviations from Plan
1. **PS scorer/LINE factories use REST/net-http + x/oauth2** (REST decision consistent with S1/S2); added `golang.org/x/oauth2` for client-credentials.
2. **Did not add a `peoplesoft` checker to aggregate `/health`** — a mock PS check is meaningless and a real one would couple boot health to an external system; `/api/v1/ps/health` reports provider status instead. (Plan task 12 mentioned it; low-value, skipped.)
3. **HR LINE notification on vacancy-opened is logged, not written as a `notifications` row** — that row belongs to Sprint 5's notification system; avoided introducing a notifications writer here.
4. **PS `Applicant.ps_vacancy_id` left empty in SyncHired** — resolving the assigned vacancy's PS id requires a vacancy→application link not modeled yet; documented for a later iteration. The push + fallback paths are fully exercised.
5. **PS service depends on narrow `ApplicationStore`/`CandidateStore` interfaces** (not the full repos) for testability.

## Issues Encountered
- Growing `vacancies.Repository` (+Upsert/SetStatusByPSID) and `applications.NewHandler` (+HiredSyncer) broke two existing test files — updated the branch test fake + handler test calls (test-only).

## Tests Written
| Test File | Tests | Area |
|---|---|---|
| `pkg/config/config_test.go` | +2 | PS-real / LINE-real fail-fast |
| `internal/auth/line_test.go` | 2 | mock verify accept/reject; real selection |
| `internal/peoplesoft/service_test.go` | 2 | SyncHired success→synced; PS-fail→CSV fallback |
| `internal/peoplesoft/webhook_test.go` | 4 | mapped/unmapped/bad-payload open; close |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Open PR (branch `feat/sprint-3-peoplesoft-publicapi-import`)
- [ ] Sprint 4: HR Dashboard (Next.js) + Candidate Profile + PDPA, **and** the Next.js Career Portal UI consuming these `/public/*` endpoints.
