# Plan: Executive ROI & Performance Dashboard

## Summary
Replace the current Executive dashboard (4 tabs: short-staffed / headcount / pipeline / sourcing — headcount budget is mock-only / pending HRIS) with a **Recruitment ROI & Performance** dashboard for leadership: ROI & cost-per-hire (driven by admin-configured cost assumptions, since the system stores **no** cost data), resume-volume & response & hiring-rate funnel, time-to-hire (apply → contract/offer-accept), and **success cases broken down by branch / region / position**. The old sections are hidden (code kept), the new dashboard takes over `/executive`.

## User Story
As an **executive (regional_director / auditor / super_admin)**, I want to see the recruiting ROI (cost vs hires), volume/response/hire rates, how long apply→signing takes, and success by branch/region/position, so that I can judge the system's business value and where hiring performs best.

## Problem → Solution
- **Now**: `/executive` → `GET /api/v1/executive/overview` returns `Overview{Company, Stores, Pipeline, Sourcing}`; `Company.budget` is fabricated in mock / 0 in live (no HRIS). No ROI, no time-to-hire, no per-dimension success view.
- **Want**: ROI (config-cost-driven) + funnel/volume/response + time-to-hire + success-by-dimension, all period- and dimension-filterable. Hide the budget-dependent old tabs.

## Metadata
- **Complexity**: Large (~13 files: 1 migration, ~4 backend, ~7 frontend)
- **Source PRD**: N/A (free-form via /prp-plan)
- **Next migration**: `000049` (prod schema v48)

## Locked Decisions (from user)
1. **ROI = admin-configured cost assumptions** (new cost-config, gated `settings.admin`). No HRIS integration. [user-chosen]
2. **Hide the old executive dashboard** (keep code, stop rendering); new dashboard replaces it at `/executive`. [user-chosen]
3. **"เซ็นสัญญา" (contract signed) = offer accepted = `applications.hired_at`** (verified written on offer-accept + direct hire). Time-to-hire = `created_at → hired_at`.
4. **Region dimension = dynamic `areas`/`area_stores`** (admin-managed, matches RBAC scoping); `stores.subregion`/`province` as static fallback label.
5. RBAC: reuse `executive.view` (regional_director / auditor / super_admin); cost-config gated `settings.admin`.

---

## UX Design

### Before
```
/executive → CompanySummaryBand + 4 tabs (shortage|headcount|pipeline|sourcing)  ← budget-dependent, hidden
```

### After
```
/executive → Recruitment ROI & Performance
  [global filter: period (month/quarter/year) + dimension (all|region|branch|position)]
  §1 ROI band   : Hires · Cost/hire · Savings vs traditional · ROI% · Vacancy-cost avoided
                  (labeled "based on configured assumptions"; "Set assumptions" link if settings.admin)
  §2 Funnel     : Resumes in (+monthly trend) · Response rate · Conversion→hire · funnel bar
  §3 Time-to-hire: median/avg days apply→offer-accept, + by region/branch/position
  §4 Success    : table switchable by Branch|Region|Position — hires, applications, conv%, avg TTH, top source; top performers highlighted
```

### Interaction Changes
| Touchpoint | Before | After |
|---|---|---|
| `/executive` content | 4 budget tabs | ROI + funnel + TTH + success-by-dimension |
| nav `executive.view` | shows old | shows new (same gate) |
| cost inputs | none | admin cost-config page (settings.admin) |
| old `/overview` API | primary | retained but no longer rendered |

---

## Mandatory Reading
| Priority | File | Why |
|---|---|---|
| P0 | `backend/internal/executive/live.go` | real aggregation patterns (storeFills/pipeline/sourcing) — mirror for new queries; provider-seam |
| P0 | `backend/internal/executive/types.go` | payload struct convention to extend with new view types |
| P0 | `backend/internal/executive/service.go:17-22` + `handler.go:25-33` + `routes.go` | service factory, route, `PermExecutiveView` gate, `httpx.OK` envelope |
| P0 | `backend/internal/reports/reports.go:153-161` | canonical `COUNT(*) FILTER (WHERE …)` single-pass aggregation + candidates JOIN for source_channel |
| P0 | `backend/migrations/000001_init_schema.up.sql` (applications:79-100, stores:6-16, candidates:60) + `000042` (areas/area_stores, picked_up_at) + `000046` (status history) + `000023/000048` (offers) | the source columns/dimensions |
| P1 | `backend/internal/settings/{repository.go,handler.go}` + migration `000014_system_settings` | k/v + `settings.admin` gate pattern (cost-config mirrors the gate, but uses its own typed table) |
| P1 | `backend/internal/rbac/permissions.go:11,14` | `PermSettingsAdmin`, `PermExecutiveView` |
| P1 | `backend/cmd/api/main.go:545` | executive wiring (extend with new handler deps: pool) |
| P1 | `frontend/app/(app)/executive/page.tsx` (tabs:27,113-116; band:101; gate:44-67) | the page to rebuild + how old tabs render (to hide) |
| P1 | `frontend/components/executive/*` + `frontend/components/analytics/Charts.tsx:177` | existing exec panels + the **hand-rolled CSS/SVG chart** convention (recharts installed but unused) |
| P1 | `frontend/components/shell/nav-config.tsx:63,108` + `lib/roles.ts:74` + `lib/queries.ts:313` + `lib/types.ts:692` | nav gate, role helper, query hook, types to extend |

## External Documentation
No external research needed — internal aggregation + provider-seam + settings patterns only.

---

## Patterns to Mirror

### EXEC_PROVIDER_SEAM (extend, keep mock default)
```go
// SOURCE: backend/internal/executive/service.go:17-22
func NewService(pool *pgxpool.Pool, provider string) Service {
    if provider == "real" { return &liveService{pool: pool} }
    return &mockService{pool: pool}
}
// New ROI endpoint follows the same mock/live split (mock = deterministic synthetic).
```

### AGGREGATION_SQL (single-pass FILTER; mirror for ROI/funnel/TTH/success)
```go
// SOURCE: backend/internal/reports/reports.go:153-161
// COUNT(*) FILTER (WHERE a.status='hired') etc.; JOIN candidates for source_channel;
// LEFT JOIN stores ON s.store_no = a.assigned_store_id for store/region dims.
```

### TIME_TO_HIRE
```sql
-- created_at → hired_at (hired_at verified written: offer_repository.go:184, repository.go:606)
AVG(EXTRACT(EPOCH FROM (hired_at - created_at))/86400) FILTER (WHERE hired_at IS NOT NULL)
PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (hired_at-created_at))/86400)
```

### RBAC_ROUTE_GATE
```go
// SOURCE: backend/internal/executive/handler.go:32 + settings/handler.go:44
if !rbac.Can(u.Role, rbac.PermExecutiveView) { return fiber.NewError(403, ...) }
// cost-config write: rbac.Can(u.Role, rbac.PermSettingsAdmin)
```

### FRONTEND_HAND_CHART (no recharts; CSS/SVG bars)
```tsx
// SOURCE: frontend/components/analytics/Charts.tsx:177 — horizontal bars as <div style={{width:`${pct}%`}}>
// Mirror this; do NOT introduce recharts (installed but unused — keep convention).
```

### FRONTEND_QUERY_HOOK
```ts
// SOURCE: frontend/lib/queries.ts:313 — useExecutiveOverview → /api/v1/executive/overview
// Add useExecutiveROI(filters) → /api/v1/executive/roi?...
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000049_executive_cost_config.{up,down}.sql` | CREATE | single-row `executive_cost_config(id bool PK default true, currency TEXT default 'THB', system_cost_monthly NUMERIC, traditional_cost_per_hire NUMERIC, vacancy_cost_per_day NUMERIC, updated_by UUID, updated_at)` (CHECK id) |
| `backend/internal/executive/roi.go` | CREATE | ROI/funnel/time-to-hire/success aggregations (live) + cost-config read/write; `ROIView` + dimension breakdown structs |
| `backend/internal/executive/roi_mock.go` | CREATE | deterministic synthetic ROIView (mirrors mock.go; no rand/time.Now) |
| `backend/internal/executive/types.go` | UPDATE | add `ROIView`, `FunnelStat`, `TimeToHire`, `SuccessRow`, `CostConfig` structs |
| `backend/internal/executive/handler.go` + `routes.go` | UPDATE | `GET /api/v1/executive/roi` (PermExecutiveView, parses period+dimension+filters), `GET/PUT /api/v1/executive/cost-config` (GET=ExecutiveView, PUT=SettingsAdmin) |
| `backend/internal/executive/service.go` | UPDATE | extend interface with `ROI(ctx, filters)` + `GetCostConfig`/`SetCostConfig`; mock+live both implement |
| `backend/cmd/api/main.go` | UPDATE | pass pool to the extended service (already does) — verify cost-config repo wired |
| `backend/internal/executive/roi_integration_test.go` | CREATE | PG16: ROI math, funnel counts, TTH median, success-by-dimension, dimension filter scoping |
| `frontend/app/(app)/executive/page.tsx` | UPDATE | render the 4 new sections; **remove the old 4-tab render** (keep old components on disk, just stop importing) |
| `frontend/components/executive/{RoiBand,FunnelVolume,TimeToHirePanel,SuccessByDimension,ExecFilters,CostConfigDialog}.tsx` | CREATE | new sections + global filter + cost-config editor |
| `frontend/lib/queries.ts` + `lib/types.ts` | UPDATE | `useExecutiveROI`, `useExecCostConfig`, `useSetExecCostConfig` + `ROIView`/`SuccessRow` types |
| `frontend/messages/{th,en}.json` | UPDATE | new `executive` keys (ROI labels, sections, assumptions disclaimer) |
| `frontend/components/shell/nav-config.tsx` | KEEP | unchanged (same `executive.view` gate; new page reuses the route) |

## NOT Building
- No HRIS/finance/PeopleSoft cost integration (config assumptions only).
- No headcount-budget / fill-rate (budget data doesn't exist — that's WHY old tabs are hidden).
- No new RBAC role; reuse `executive.view` + `settings.admin`.
- No recharts adoption (keep hand-rolled CSS/SVG).
- No deletion of old executive components/`/overview` endpoint (kept dormant; revertible).
- No per-stage historical durations (status-history is forward-only) — TTH uses created_at→hired_at only.

---

## Step-by-Step Tasks

### Task 1: Migration 000049 — cost-config table
- **ACTION**: create `executive_cost_config` single-row table (boolean PK pattern) with currency + 3 numerics + updated_by/at; seed one row with NULLs/0.
- **MIRROR**: 000014_system_settings style.
- **GOTCHA**: single-row guarantee via `id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id)`; upsert with `ON CONFLICT (id) DO UPDATE`.
- **VALIDATE**: migrate up/down on PG16; one row exists after up.

### Task 2: Cost-config repo + structs + endpoints
- **ACTION**: `CostConfig` struct + `GetCostConfig`/`SetCostConfig(ctx, cfg, updatedBy)`; `GET /executive/cost-config` (ExecutiveView), `PUT` (SettingsAdmin).
- **MIRROR**: settings repo + RBAC_ROUTE_GATE.
- **GOTCHA**: PUT validates non-negative numbers; null-safe reads (cost unset → ROI cards show "set assumptions" empty-state, not a divide error).
- **VALIDATE**: PUT 403 without settings.admin; GET returns the row.

### Task 3: ROI + funnel + TTH + success aggregations (`roi.go` live)
- **ACTION**: `ROI(ctx, ExecFilters{Period, Dimension, Region/Store/Position})` returns `ROIView{Cost CostConfig, Hires int, CostPerHire, Savings, ROIPct, VacancyCostAvoided, Funnel FunnelStat, TimeToHire, Success []SuccessRow}`. Funnel = applied/screened/interviewed/offer/hired counts + response-rate (`picked_up_at IS NOT NULL` share) within period. TTH = avg+median days created_at→hired_at. Success rows grouped by the chosen dimension (store via assigned_store_id→stores; region via area_stores→areas (fallback subregion); position via position_id→positions) with hires/applications/conversion/avg-TTH/top-source.
- **IMPLEMENT** ROI math: `CostPerHire = system_cost_period / NULLIF(hires,0)`; `Savings = (traditional_cost_per_hire - CostPerHire) * hires`; `ROIPct = Savings / NULLIF(system_cost_period,0) * 100`. `system_cost_period` = `system_cost_monthly * months_in_period`.
- **MIRROR**: AGGREGATION_SQL + TIME_TO_HIRE.
- **GOTCHA**: hires defined as `status='hired'` (offer-accept terminal). Period filter on `created_at` for volume but on `hired_at` for hires/TTH (a hire counts in the period it was HIRED, not applied) — be explicit per metric. Division guards (`NULLIF`) everywhere. Dimension filter must respect the same store/region/position joins.
- **VALIDATE**: integration — seed apps across 2 stores/regions/positions with known hires+dates → assert ROI math, funnel counts, TTH median, success grouping, and that a region filter scopes correctly.

### Task 4: Mock ROIView + service wiring
- **ACTION**: `roi_mock.go` deterministic ROIView over real dimension names (mirror mock.go fallback); extend `Service` interface + both impls; wire in main.go.
- **GOTCHA**: mock must not use rand/time.Now (stable for tests/demo). Default `EXECUTIVE_PROVIDER=mock` keeps demo working.
- **VALIDATE**: `go build`; /executive/roi returns mock payload when provider=mock.

### Task 5: Frontend — new sections + filters + cost editor
- **ACTION**: rebuild `page.tsx` to render `<ExecFilters>` + `<RoiBand>` + `<FunnelVolume>` + `<TimeToHirePanel>` + `<SuccessByDimension>`; remove old-tab imports (keep files). `CostConfigDialog` (visible if settings.admin) PUTs cost-config. Charts hand-rolled (CSS bars). Add `useExecutiveROI(filters)` + cost-config hooks + types + i18n.
- **MIRROR**: FRONTEND_HAND_CHART, FRONTEND_QUERY_HOOK, existing page gate (`canViewExecutive`).
- **GOTCHA**: ROI cards show a clear "based on configured assumptions" disclaimer; empty-state when cost unset (link to dialog). Keep the existing `canViewExecutive` gate + DataSourceBadge (demo/live). Period+dimension in URL state if practical.
- **VALIDATE**: `pnpm tsc --noEmit` + `eslint`; old tabs no longer render; new sections render against mock.

### Task 6: Hide old + verify gate
- **ACTION**: confirm `/executive` shows only the new dashboard; old components dormant (no import); nav unchanged.
- **VALIDATE**: page renders new; 403 for non-exec roles; super_admin sees new.

---

## Testing Strategy
### Unit / Integration (Go, PG16)
| Test | Expected |
|---|---|
| ROI math | costPerHire/savings/ROIPct correct; div-by-zero guarded (0 hires, 0 cost) |
| funnel counts | applied/screened/interviewed/offer/hired + response-rate share correct |
| time-to-hire | avg+median days created_at→hired_at; ignores un-hired |
| success by dimension | grouping by store/region/position; conversion + top-source per row |
| dimension filter | region/store/position filter scopes the whole payload |
| cost-config | PUT gated settings.admin; GET returns row; null cost → safe empty ROI |
| migration 000049 | up/down reversible; single row enforced |

### Edge Cases
- [ ] zero hires in period (no div error); cost unset; empty dimension
- [ ] hire counted in HIRED period not applied period
- [ ] store with no region/area mapping (fallback label)
- [ ] non-exec role 403; settings.admin-only cost write

## Validation Commands
```bash
cd backend && go build ./... && go vet ./... && go test ./internal/executive/...
DB_URL=... go test -tags=integration ./internal/executive/
cd frontend && pnpm tsc --noEmit && pnpm lint
~/go/bin/migrate -path backend/migrations -database "$DB_URL" up   # →v49
```

## Acceptance Criteria
- [ ] `/executive` shows ROI + funnel + TTH + success-by-dimension; old tabs gone
- [ ] ROI computed from admin cost-config; clearly labeled assumptions; safe when unset
- [ ] success/TTH/funnel filter by branch / region / position
- [ ] same `executive.view` gate; cost-config write gated `settings.admin`
- [ ] mock provider still powers the demo; all validation passes

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| ROI seen as "real" when it's assumptions | Medium | Medium | prominent disclaimer + "set assumptions" provenance |
| Period semantics confusion (applied vs hired) | Medium | Medium | per-metric explicit date column; documented + tested |
| Region mapping gaps (store not in any area) | Medium | Low | fallback to `stores.subregion`/province label; "unmapped" bucket |
| Old tabs referenced elsewhere | Low | Low | keep components on disk; only remove the page import |
| `hired` vs `offer` terminal ambiguity | Low | Medium | hires = `status='hired'` (post offer-accept); aligns with hired_at |

## Notes
- Prod schema v48 → adds **000049** (v49). Deploy: migration + api + dashboard (career-portal/worker untouched). New env: none required (cost via DB config; `EXECUTIVE_PROVIDER` stays, flip to `real` when ready).
- The mock provider stays the default so the leadership demo keeps working with synthetic figures; flip `EXECUTIVE_PROVIDER=real` for live aggregations.
- Old `/overview` endpoint + components remain dormant (revertible) — hiding is at the page-render layer, not deletion.
