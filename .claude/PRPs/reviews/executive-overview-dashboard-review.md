# Code Review: Executive Overview Dashboard (local, uncommitted)

**Reviewed**: 2026-06-17
**Author**: Ittiporn Roongmaneephan
**Branch**: `feat/executive-overview-dashboard` → `main`
**Decision**: APPROVE with comments

## Summary
A new company-wide Executive Overview (`/executive`) backed by a mock-first provider seam (`EXECUTIVE_PROVIDER=mock|real`). The implementation faithfully mirrors established codebase patterns — reports-style Fiber handler + repo, the `mock|real` provider seam (config field + validation row + `UsesRealX()` helper), `httpx` envelope, in-handler role-gate map, React Query hook, and the Ledger/CP-Axtra UI primitives. No security issues, no secrets, no debug output; all files well under size limits. Validation green across both stacks. Findings are all LOW (scaffold/cosmetic).

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
None.

### LOW
1. **`live.go` does not sort stores by fill-rate** (`backend/internal/executive/live.go` — `storeFills`, orders by `store_no`). The `Overview` contract and mock sort stores ascending by `fill_rate_pct` ("most short-staffed first"). In live mode `fill_rate_pct` is always `0` (budget unavailable), so the ranking is moot until PeopleSoft lands — but when budget arrives this path must add the sort to match the contract. Already covered by the `TODO(peoplesoft)` note; flagging so it isn't missed in the future ticket.
2. **No unit test for `live.go`** (`backend/internal/executive/`). The live provider's queries are untested (they need a Postgres fixture / integration tag). Acceptable for now — `live` is an explicit scaffold returning `budget_available=false`, and the SQL mirrors the already-tested `reports` aggregates — but the future "go live" ticket should add an `//go:build integration` test like `internal/applications/list_integration_test.go`.
3. **`DemoBadge` returning `null` still renders an empty actions container** (`frontend/.../executive/page.tsx:42` → `PageHeader` actions wrapper). `PageHeader` renders `{actions && <div…>}`; since the `<DemoBadge/>` element is always a truthy JSX value, an empty `flex gap-2` div renders when the badge resolves to `null` (live mode + budget available). Purely cosmetic (no visible artifact). Optional: only pass the `actions` prop when a badge will show.
4. **Panel-internal labels are hardcoded English** (`ExecutiveSections.tsx`) while the masthead/badge/gate are translated. This is a deliberate, documented deviation matching the existing `Operations.tsx`/`Charts.tsx` analytics components (also untranslated). Note only — consistent with the codebase.

## Validation Results

| Check | Result |
|---|---|
| Type check (`tsc --noEmit`) | Pass (exit 0) |
| Lint (`eslint` changed files) | Pass (exit 0) |
| Go vet / gofmt | Pass (clean) |
| Tests (`go test ./...`, `-race` on executive) | Pass (7 new tests; full suite exit 0) |
| Build (`next build`, `go build ./...`) | Pass (`/executive` route present) |
| i18n parity (`check-i18n-parity.mjs`) | Pass (40 keys th/en) |

## Security Review
- **No secrets**: no hardcoded keys/tokens; `EXECUTIVE_PROVIDER` is a non-secret flag.
- **No injection surface**: executive endpoint takes no user input; mock SQL is static, live SQL is static/parameterized (`$1`).
- **AuthZ**: server-side role gate (`executiveRolesAllowed`) is the real gate; frontend `canViewExecutive` + `enabled` fetch guard prevents needless 403s. Role gate covered by `handler_test.go` (200 for 3 leadership roles, 403 for staff/blank).
- **Honesty**: mock data is flagged via `data_source` + visible "Demo data" badge; live budget is `budget_available=false` rather than fabricated.

## Files Reviewed
- `backend/internal/executive/{types,service,mock,live,handler}.go` — Added
- `backend/internal/executive/{mock_test,handler_test}.go` — Added
- `backend/pkg/config/config.go` — Modified (provider field + validation + helper)
- `backend/cmd/api/main.go` — Modified (route wiring)
- `frontend/lib/{types,queries,roles}.ts` — Modified
- `frontend/components/shell/nav-config.tsx` — Modified (nav entry + gate)
- `frontend/components/executive/ExecutiveSections.tsx` — Added
- `frontend/app/(app)/executive/page.tsx` — Added
- `frontend/messages/{en,th}.json` — Modified (i18n keys)
