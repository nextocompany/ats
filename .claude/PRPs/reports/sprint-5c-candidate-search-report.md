# Implementation Report: Sprint 5c — Candidate Search

## Summary
Added HR candidate search: a `search.Searcher` seam that is **mock-default** (Postgres
`pg_trgm`/ILIKE over candidates joined to applications, RBAC-scoped) and **Azure AI Search**
behind config. Exposed at `GET /api/v1/candidates/search` and a new dashboard **Search**
page. Works fully locally/CI with zero Azure credentials. Completes Sprint 5.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium–Large | Medium–Large (as predicted) |
| Confidence | 8/10 | High — all levels green, live route-precedence + scope verified |
| Files Changed | ~13 | 14 (9 created, 5 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Config — search toggle | ✅ | `AI_SEARCH_PROVIDER` (default mock), `AZURE_SEARCH_*`, `UsesAzureSearch()` + validation |
| 2 | Migration | ✅ | `000008_search_trgm` — `pg_trgm` + GIN trigram indexes on `full_name`/`province`; round-trips |
| 3 | Search seam + mock PG searcher | ✅ | trigram ILIKE, one hit per candidate (best app via `ROW_NUMBER`), RBAC `CandidatesClause` |
| 4 | Real Azure searcher | ✅ | query-only REST; OData scope filter pushed to index; index population out of scope |
| 5 | API handler + wiring | ✅ | `GET /api/v1/candidates/search`; registered **before** profiles for path precedence |
| 6 | Frontend search page + nav | ✅ | debounced URL-synced query, paginated results → candidate detail; "Search" nav entry |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; `golangci-lint` 0 issues; frontend `eslint` + `tsc` clean |
| Unit Tests | ✅ Pass | factory selection, query normalize, OData scope filter (+escaping), Azure response map (httptest) |
| Build | ✅ Pass | backend `go build ./...`; `next build` (incl. `/search`) |
| Integration | ✅ Pass | PG searcher: name-ranked, **one-hit-per-candidate dedup**, **store-scope isolation** |
| Live e2e | ✅ Pass | `?q=สมชาย` → scoped hit; empty `q` → 400; `/candidates/:id` still 200 (precedence) |

## Files Changed

| File | Action |
|---|---|
| `internal/search/{search,pg,azure,handler}.go` | CREATED |
| `internal/search/{search_test,pg_integration_test}.go` | CREATED |
| `migrations/000008_search_trgm.{up,down}.sql` | CREATED |
| `frontend/app/(app)/search/page.tsx` | CREATED |
| `pkg/config/config.go` | UPDATED (search provider config) |
| `cmd/api/main.go` | UPDATED (search wiring, registered before profiles) |
| `frontend/lib/{types,queries}.ts` | UPDATED (SearchHit/Filter + `useCandidateSearch`) |
| `frontend/components/shell/AppHeader.tsx` | UPDATED (Search nav) |

## Deviations from Plan
- **Search fields = name + province** (not skills): the `candidates` table has no skills column (skills live in the parsed-profile blob), so the mock searches `full_name`/`province`. Documented.
- **No skills/address in trigram index**: indexed `full_name` + `province` (the searched columns).

## Issues Encountered
- **Route collision (caught live)**: `/candidates/search` was matched by `/candidates/:id` (profiles, registered first) → "invalid candidate id". Fixed by registering `search.RegisterRoutes` **before** `profiles.RegisterRoutes` so the static segment wins; verified `/candidates/:id` still resolves for real UUIDs.
- Integration tests truncate seed data (existing pattern) — re-seeded for the live check.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/search/search_test.go` | 6 | factory, normalize, scope OData (+escape), Azure mapping |
| `internal/search/pg_integration_test.go` | 3 | name ranking, dedup-per-candidate, store-scope isolation |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr` → squash-merge to `main`
- [ ] Sprint 5 complete (5a/5b/5c). Next roadmap: S6–7 (PWA/E2E/security), S8 (UAT/go-live).
