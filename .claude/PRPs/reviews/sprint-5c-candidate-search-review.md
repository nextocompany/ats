# Code Review: Sprint 5c — Candidate Search (pre-PR)

**Reviewed**: 2026-05-30
**Branch**: `feat/sprint-5c-candidate-search`
**Reviewer**: go-reviewer agent + maintainer triage
**Decision**: APPROVE (HIGH scope leak fixed; rest documented)

## Summary
Independent Go review confirmed SQL/OData parameterisation is sound and found one
genuine **HIGH** RBAC scope leak. Fixed it (plus a LOW and two test gaps); the rest are
documented. Re-validated: static, race, and integration (incl. a new no-cross-store-leak
test) all pass.

## Findings & Disposition

| # | Sev | Finding | Disposition |
|---|---|---|---|
| pg.go:62-66 | HIGH | Store-scoped users could see a candidate's **out-of-scope (other-store) application** status/score: the old `scoped` CTE gated candidate *visibility* but `ranked` joined ALL their applications and picked the global best | **Fixed** — scope the **applications** via `ApplicationsClause` inside the ranked join, so the best pick is chosen only among in-scope applications. New integration test `TestSearch_StoreScopeNoCrossStoreLeak` locks it (store-1 user sees score 60, not the out-of-scope 95) |
| pg.go:69-80 | MED | Shared `args` slice across count+page queries fragile | **Fixed** — single `ranked` CTE; `limitPH`/`offsetPH` extracted (matches `list.go`) |
| azure.go:90 | LOW | Azure error body discarded | **Fixed** — surface a truncated body in the error |
| search_test.go | MED | Factory test only covered `"mock"`, not the empty-string default | **Fixed** — asserts both `"mock"` and `""` → `pgSearcher` |
| SQL injection (pg.go) | — | All user values bound via `$N` (`add()` helper); `%`-wrapping done in Go pre-bind | **Verified safe** — no finding |
| OData injection (azure.go) | — | `escapeOData` doubles single quotes (correct OData rule); `MinScore`/store are numeric | **Verified safe** — no finding |
| azure.go:122 | MED | `scope.Subregion` interpolated into OData | **Documented** — value originates from the JWT claim (trust boundary), and escaping is sufficient; claim validation belongs at the auth layer, not 5c |
| handler.go:26 | — | `scopeFrom` zero-value on missing auth | **Documented** — fails closed (KindStore + nil store → no results); matches the app-wide pattern |
| main.go:169 | MED | Route precedence by registration order | **Documented** — Fiber matches static `/candidates/search` before `/candidates/:id` when registered first; commented + live-verified `/candidates/:id` still resolves |
| search.go naming | LOW | `NewSearcher` vs `NewClient` | **Won't change** — single-interface factory is appropriate |

## Validation (post-fix)

| Check | Result |
|---|---|
| `go vet` / `golangci-lint` | Pass (0 issues) |
| `go test -race ./internal/search/...` | Pass (factory incl. empty-string, normalize, OData scope+escape, Azure mapping) |
| `go test -tags integration ./internal/search/...` | Pass — name ranking, dedup-per-candidate, store-scope isolation, **no cross-store leak** |
| Frontend lint / tsc / build | Pass |
| Live: `?q=` scoped hit, empty `q`→400, `/candidates/:id` still 200 | Pass |

## Files Reviewed
9 created + 5 updated, plus the review fixes to `pg.go`, `azure.go`, `search_test.go`,
and `pg_integration_test.go`.
