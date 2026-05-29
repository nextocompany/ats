# Plan: Sprint 5c — Candidate Search (Azure AI Search seam)

## Summary
Add **candidate/application search** for HR: a `search.Searcher` seam that is **mock-default** (Postgres `pg_trgm`/`ILIKE` over candidates + applications, RBAC-scoped) and **Azure AI Search** behind config. Exposed via an authenticated `GET /api/v1/candidates/search` and a new **Search** surface in the HR dashboard. Search works fully locally/CI with zero Azure credentials.

## User Story
As an **HR recruiter**, I want **to search candidates by name, skills, province, status, and score**, so that **I can quickly find people across the pipeline instead of paging through the inbox**.

## Problem → Solution
**Current state:** Candidates/applications are only reachable via filtered list endpoints (status/score/store). There is **no free-text search**, and Azure AI Search (PRP §) is unimplemented.
**Desired state:** A `Searcher` seam — mock (Postgres trigram/ILIKE join, scope-filtered) by default, Azure AI Search REST when configured — behind `GET /api/v1/candidates/search?q=…`. A dashboard **Search** page lets HR query and click through to the candidate detail.

## Metadata
- **Complexity**: Medium–Large (new seam + API + migration + dashboard page; ~13 files)
- **Source PRD**: Nexto PRP v1.0 — Sprint 5 (Azure AI Search); roadmap §20
- **Decisions locked**: search is **mock-default-behind-config** (mirrors ai/peoplesoft/line); mock = Postgres `pg_trgm`; results **RBAC-scoped**; real Azure AI Search **query-only** (index population is out of scope/ops)
- **Estimated Files**: ~13
- **Depends on**: nothing (independent of 5a/5b)

---

## UX Design

### After (new Search page)
```
Search                                   [ ⌕ cashier bangkok            ]
┌─────────────────────────────────────────────────────────────────────┐
│ สมชาย ใจดี      Cashier · scored 88 · กรุงเทพ        → (to detail)    │
│ สมหญิง รักดี    Cashier · scored 81 · นนทบุรี         →               │
│ …  (RBAC-scoped, paginated)                                          │
└─────────────────────────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Find a candidate | filter inbox by status/score | free-text `q` across name/skills/province + filters | F (search) |
| Nav | Overview·Inbox·Candidates·Analytics | + **Search** | `AppHeader.tsx` NAV |
| Backend | list endpoints only | `GET /api/v1/candidates/search` (RBAC-scoped) | mock-default |

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/ai/factory.go` | 8-14 | **mock-default factory** to mirror for `search.NewSearcher(cfg)` |
| P0 | `backend/internal/peoplesoft/client.go` | 9-21 | interface + factory seam shape |
| P0 | `backend/internal/applications/list.go` | 56-124 | dynamic filter + pagination + **scope clause** + row scan — clone for the mock searcher SQL |
| P0 | `backend/internal/rbac/scope.go` | 10-69 | `CandidatesClause`/`ApplicationsClause` for scope-filtered results |
| P0 | `backend/internal/profiles/handler.go` | 38-40 | `scopeFrom(c)` → `rbac.Scope` extraction in handlers |
| P0 | `backend/pkg/httpx/response.go` | 1-37 | `Envelope`/`OK` + `Meta` for paginated results |
| P0 | `backend/internal/applications/dashboard_handler.go` | 55-97 | list handler w/ query params + `Meta` pagination to mirror |
| P1 | `backend/pkg/config/config.go` | 23-53, 68-126 | seam config + predicate; add `AI_SEARCH_PROVIDER` + Azure search creds |
| P1 | `backend/internal/candidates/repository.go` | 100-128 | SQL + `nullable` + scan pattern |
| P1 | `backend/migrations/000005_ps_public.up.sql` | all | migration style (extension + index) |
| P1 | `frontend/lib/api.ts` | 1-52 | envelope client + `buildQuery` |
| P1 | `frontend/lib/queries.ts` | 21-45 | list-query hook pattern → `useCandidateSearch` |
| P1 | `frontend/app/(app)/applications/page.tsx` | all | list/filter/paginate page to mirror for `/search` |
| P1 | `frontend/components/shell/AppHeader.tsx` | 10-15 | NAV array — add "Search" |
| P2 | `backend/internal/ai/azure_parser.go` | 32-114 | Azure REST client shape (endpoint/key/timeout, POST+JSON) for the real searcher |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Azure AI Search query | learn.microsoft.com/azure/search/search-query-overview | `POST {endpoint}/indexes/{index}/docs/search?api-version=2024-07-01` with `api-key` header + `{search, filter, top, skip}` JSON. |
| pg_trgm | postgresql.org/docs/current/pgtrgm.html | `CREATE EXTENSION pg_trgm`; GIN index on text for fast `ILIKE`/similarity; enables fuzzy name/skill search in the mock. |

### Research Notes
```
KEY_INSIGHT: search must work with zero Azure creds (local/CI default).
APPLIES_TO: internal/search mock.
GOTCHA: the mock is NOT a stub — it's a real Postgres query (ILIKE/trigram over candidates.full_name, province, and parsed skills if available, joined to applications for status/score). This keeps the dashboard fully functional locally and is the default.

KEY_INSIGHT: results must respect RBAC scope.
APPLIES_TO: both mock and real.
GOTCHA: mock applies rbac.Scope.CandidatesClause in SQL. Real Azure path post-filters results by the scope-derived store/subregion (or pushes a `filter=` to the index) — never return out-of-scope candidates.

KEY_INSIGHT: index population for real Azure Search is an ops/ingestion concern.
APPLIES_TO: scope boundary.
GOTCHA: 5c implements QUERY only against an externally-maintained index; mock is default. Document that indexing (push candidates → index) is out of scope.
```

---

## Patterns to Mirror

### SEARCH_SEAM (mirror ai/factory.go:8-14)
```go
// internal/search/search.go
type Query struct {
	Text     string
	Status   string
	MinScore *float64
	Page, Limit int
}
type Hit struct {
	CandidateID, FullName, Province, Status string
	AIScore *float64
}
type Searcher interface {
	Search(ctx context.Context, q Query, scope rbac.Scope) ([]Hit, int, error)
}
func NewSearcher(cfg *config.Config, pool *pgxpool.Pool) Searcher {
	if cfg.UsesAzureSearch() {
		return newAzureSearcher(cfg) // REST query-only
	}
	return newPGSearcher(pool)       // default: trigram/ILIKE
}
```

### MOCK_PG_SEARCH (mirror applications/list.go:56-124)
```go
// dynamic args + scope clause + count + LIMIT/OFFSET, scan rows
add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
conds := []string{"(c.full_name ILIKE " + add("%"+q.Text+"%") + " OR c.province ILIKE " + add("%"+q.Text+"%") + ")"}
if sc, a := scope.CandidatesClause(len(args)+1); sc != "" { conds = append(conds, sc); args = append(args, a...) }
```

### CONFIG_PREDICATE (mirror config.go:124-126)
```go
func (c *Config) UsesAzureSearch() bool { return c.AISearchProvider == "azure" }
```

### HANDLER (mirror applications/dashboard_handler.go:55-97 + httpx)
```go
func (h *Handler) Search(c *fiber.Ctx) error {
	q := search.Query{Text: c.Query("q"), Status: c.Query("status"),
		Page: atoiDefault(c.Query("page"),1), Limit: atoiDefault(c.Query("limit"),20)}
	hits, total, err := h.searcher.Search(c.UserContext(), q, scopeFrom(c))
	if err != nil { return err }
	return c.Status(200).JSON(httpx.Envelope[[]search.Hit]{Success:true, Data:hits, Meta:&httpx.Meta{Total:total, Page:q.Page, Limit:q.Limit}})
}
```

### FRONTEND_HOOK (mirror frontend/lib/queries.ts:21-45)
```ts
export function useCandidateSearch(f: SearchFilter) {
  return useQuery({
    queryKey: ["candidate-search", f],
    queryFn: () => api.get<Hit[]>("/api/v1/candidates/search" + buildQuery({ q: f.q, status: f.status, page: f.page, limit: f.limit })),
    enabled: f.q.trim().length > 0,
  });
}
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/search/search.go` | CREATE | `Query`/`Hit`/`Searcher` + `NewSearcher` factory |
| `backend/internal/search/pg.go` | CREATE | mock-default Postgres trigram/ILIKE searcher (RBAC-scoped) |
| `backend/internal/search/azure.go` | CREATE | real Azure AI Search REST query client (built only when configured) |
| `backend/internal/search/handler.go` | CREATE | `GET /api/v1/candidates/search` + `RegisterRoutes` + `scopeFrom` |
| `backend/internal/search/pg_integration_test.go` | CREATE | seeded search + scope-filter integration test |
| `backend/internal/search/search_test.go` | CREATE | query parsing / factory selection unit test |
| `backend/pkg/config/config.go` | UPDATE | `AISearchProvider` + `AzureSearchEndpoint/Key/Index` + `UsesAzureSearch()` + validation |
| `backend/cmd/api/main.go` | UPDATE | `search.NewSearcher(cfg, pool)` + `search.RegisterRoutes` |
| `backend/migrations/0000NN_search_trgm.{up,down}.sql` | CREATE | `CREATE EXTENSION pg_trgm` + GIN index on `candidates(full_name, province)` |
| `frontend/lib/types.ts` | UPDATE | `SearchHit` + `SearchFilter` |
| `frontend/lib/queries.ts` | UPDATE | `useCandidateSearch` |
| `frontend/app/(app)/search/page.tsx` | CREATE | search input + results list + pagination |
| `frontend/components/shell/AppHeader.tsx` | UPDATE | add `{ href: "/search", label: "Search" }` |
| `frontend/e2e/dashboard.spec.ts` | UPDATE | search flow assertion (or new `search.spec.ts`) |

## NOT Building (later / out of scope)
- **Index population / ingestion** into Azure AI Search (push candidates → index) — query-only; mock default.
- Semantic / vector / embeddings search (keyword + trigram only).
- Saved searches, faceted aggregations, highlighting.
- Search over resume blob *contents* (search structured fields, not OCR text).
- Cross-entity global search (candidates/applications only).

---

## Step-by-Step Tasks

### Task 1: Config — search provider toggle
- **ACTION**: Add `AISearchProvider` (`getenv("AI_SEARCH_PROVIDER","mock")`), `AzureSearchEndpoint/Key/Index` (`os.Getenv`, index default e.g. `"candidates"`); `UsesAzureSearch()`; validate endpoint+key present when azure.
- **MIRROR**: `config.go:23-53, 68-126`.
- **VALIDATE**: `go build ./...`; default → `UsesAzureSearch()==false`.

### Task 2: migration — pg_trgm + index
- **ACTION**: `make migrate-create name=search_trgm`. Up: `CREATE EXTENSION IF NOT EXISTS pg_trgm;` + `CREATE INDEX idx_candidates_fullname_trgm ON candidates USING gin (full_name gin_trgm_ops);` (+ province). Down: drop indexes (leave extension or drop if safe).
- **MIRROR**: `migrations/000005_ps_public.up.sql`.
- **GOTCHA**: use `make migrate-create` (don't hardcode number). `CREATE EXTENSION` needs superuser — the dev `hruser` has it in compose; note for prod.
- **VALIDATE**: migrate up/down/up.

### Task 3: search seam + mock PG searcher
- **ACTION**: `search.go` (types + factory), `pg.go` (Postgres searcher: join candidates+applications, ILIKE/trigram on text, optional status/min_score, **scope clause**, count + paginate, scan to `Hit`).
- **MIRROR**: SEARCH_SEAM, MOCK_PG_SEARCH; `applications/list.go:56-124`; `rbac/scope.go:56-69`.
- **GOTCHA**: a candidate may have multiple applications — `SELECT DISTINCT` or aggregate to best score to avoid dupes; apply scope so store/subregion users never see others.
- **VALIDATE**: integration test — seed candidates, `q="cashier"` returns scoped hits; store-scoped user sees only theirs.

### Task 4: real Azure searcher (query-only)
- **ACTION**: `azure.go` — REST `POST {endpoint}/indexes/{index}/docs/search?api-version=2024-07-01` with `api-key` header; map response → `[]Hit`; post-filter by scope. Built only under `UsesAzureSearch()`.
- **MIRROR**: `ai/azure_parser.go:32-114` (REST client + JSON).
- **GOTCHA**: never constructed without creds; out-of-scope hits dropped after the index returns.
- **VALIDATE**: `go build ./internal/search`; unit test maps a canned JSON response → hits (no live Azure).

### Task 5: API handler + wiring
- **ACTION**: `handler.go` (`Search` + `RegisterRoutes` `GET /api/v1/candidates/search`, `scopeFrom`); wire `search.NewSearcher(cfg, pool)` in `cmd/api/main.go` after reports registration.
- **MIRROR**: HANDLER; `dashboard_handler.go:55-97`; `httpx` envelope; route registration.
- **VALIDATE**: `curl "/api/v1/candidates/search?q=cashier"` → envelope with `data` + `meta.total`.

### Task 6: frontend search page + nav
- **ACTION**: `types.ts` (`SearchHit`/`SearchFilter`); `queries.ts` (`useCandidateSearch`, debounced `q`); `app/(app)/search/page.tsx` (search input synced to `?q=`, results list linking to candidate detail, pagination, empty/loading states); add NAV entry in `AppHeader.tsx`.
- **MIRROR**: FRONTEND_HOOK; `applications/page.tsx`; `lib/api.ts:44-51` `buildQuery`.
- **GOTCHA**: query disabled until `q` non-empty; debounce input to avoid a request per keystroke.
- **VALIDATE**: `cd frontend && pnpm lint && pnpm build`; Playwright: type query → results render → click → candidate detail.

---

## Testing Strategy
### Unit
| Test | Input | Expected | Edge? |
|---|---|---|---|
| factory selection | mock vs azure cfg | correct impl | — |
| azure response map | canned JSON | `[]Hit` | — |
| query normalize | page/limit defaults | clamped | yes |

### Integration (mock PG)
| Test | Input | Expected | Edge? |
|---|---|---|---|
| name search | `q="สมชาย"` | matching candidate | — |
| scope filter | store-scoped user | only own-store candidates | yes |
| pagination | limit=1 | total correct, 1 row | yes |
| empty query | `q=""` | 400 or empty (defined) | yes |

### Edge Cases Checklist
- [ ] Empty `q` → no DB scan / clear contract (400 or empty)
- [ ] No matches → empty list, `meta.total=0`
- [ ] Candidate with multiple applications → single hit (best score)
- [ ] Store/subregion user → strictly scoped results
- [ ] Special chars / `%` in `q` → parameterized, no injection

## Validation Commands
### Static + unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Integration (stack up)
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./internal/search/...
```
### Frontend
```bash
cd frontend && pnpm lint && pnpm exec tsc --noEmit && pnpm build
```
### Manual
- [ ] `GET /api/v1/candidates/search?q=cashier` → scoped envelope + meta
- [ ] Dashboard Search page: type → results → click → candidate detail
- [ ] `AI_SEARCH_PROVIDER=mock` (default) works with no Azure creds

## Acceptance Criteria
- [ ] `search.Searcher` seam: mock Postgres default (zero creds), Azure behind `AI_SEARCH_PROVIDER=azure`.
- [ ] `GET /api/v1/candidates/search` returns RBAC-scoped, paginated hits in the envelope.
- [ ] Dashboard **Search** page + nav entry: query → results → candidate detail.
- [ ] vet/lint/`go test -race` + integration pass; frontend lint/tsc/build clean; migration round-trips.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Scope leak (out-of-scope candidates) | Med | High | scope clause in SQL + post-filter for Azure; integration test asserts scoping |
| `CREATE EXTENSION` perms in prod | Med | Med | dev has it; document prod pre-provisioning of pg_trgm |
| Real Azure index absent | Med | Low | mock is default; real is query-only behind config, documented |
| Duplicate hits per candidate | Med | Low | DISTINCT/aggregate to best score |
| SQL injection via `q` | Low | High | parameterized args only (mirror list.go) |

## Notes
- Independent of 5a/5b — can be implemented in any order.
- The mock searcher makes search a first-class local feature; Azure AI Search is a deploy-time swap that needs an externally-populated index.
