# Plan: Executive Overview Dashboard

## Summary
A new company-wide **Executive Overview** dashboard surface (`/executive`) for CP Axtra leadership. It shows Budget vs Actual headcount, total vacancies, store fill-rate (ranked "most short-staffed branches first"), pipeline-by-position, and sourcing-channel performance in a single screen an executive can read in 30 seconds. Built **mock-first behind a provider seam** (`EXECUTIVE_PROVIDER=mock|live`, default `mock`) because Budget/Actual data lives in PeopleSoft/HRIS (deferred) and prod has no live candidate data yet — mock returns deterministic synthetic numbers layered over **real store/position names** so the demo looks live. The `live` path is scaffolded for the future PeopleSoft + ATS-derived computation.

## User Story
As a **CP Axtra executive (super_admin / regional_director / auditor)**,
I want **one screen showing budget vs actual headcount, vacancies, store fill-rate, pipeline, and sourcing performance across the whole company**,
So that **I can answer "which branch is most short-staffed right now?" at a glance during a demo and in real operation later**.

## Problem → Solution
**Current state**: HR dashboard has an operational `/dashboard` (per-store inbox-driven KPIs) and `/analytics` (funnel/sources), but **no executive, company-wide budget/vacancy/fill-rate view**. Budget/Actual data does not exist in the schema; prod candidate data is wiped to zero, so any live-data dashboard renders empty.
**Desired state**: A polished `/executive` page driven by one consolidated endpoint `GET /api/v1/executive/overview`, returning rich, demo-ready data in `mock` mode (real CP Axtra store/position names + deterministic synthetic figures), with a clean seam to swap to `live` (PeopleSoft + ATS) later. A visible "Demo data" badge keeps it honest.

## Metadata
- **Complexity**: Large (new backend package + new frontend page + config seam; ~12–15 files, ~700–900 lines)
- **Source PRD**: N/A (free-form client requirement)
- **PRD Phase**: N/A (standalone)
- **Estimated Files**: ~15 (8 backend, 6 frontend, +2 i18n catalogs)

---

## UX Design

### Before
```
HR Dashboard nav:  Overview · Inbox · Candidates · Search · Analytics · [Members] · [Admin]
                   └─ operational, per-store, no company budget/fill-rate view
```

### After
```
HR Dashboard nav:  Overview · Inbox · Candidates · Search · Analytics · [Executive] · [Members] · [Admin]
                                                              └─ NEW (super_admin/regional_director/auditor only)

┌──────────────────── EXECUTIVE OVERVIEW ──────────────── [Demo data] [TH|EN] ─┐
│  Company headcount                                                            │
│  ┌── Budget 4,200 ──┐  Actual 3,640   Vacancy 560   Fill rate 86.7%          │
│  └──────────────────┘  (hero KPI band — CP Axtra blue)                        │
├───────────────────────────────────────────────────────────────────────────── │
│  Most short-staffed branches            │  Pipeline by position               │
│  Rama III    62% ▓▓▓░░░  -38 heads      │  Cashier       420 ▸ 120 ▸ 38 ▸ 12  │
│  Bangna      71% ▓▓▓▓░░  -24 heads      │  Sales Assoc   310 ▸  90 ▸ 24 ▸  8  │
│  Pinklao     78% ▓▓▓▓░░  -18 heads      │  Stock Ctrl    180 ▸  55 ▸ 12 ▸  4  │
│  Ladprao     83% ▓▓▓▓▓░  -12 heads      │  ...                                │
│  ... (ranked asc by fill-rate)          │                                     │
├───────────────────────────────────────────────────────────────────────────── │
│  Sourcing channel performance (reuse SourcesChart): LINE · Google · Walk-in…  │
└───────────────────────────────────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Sidebar nav | No executive entry | "Executive" entry for KindAll roles only | Gated in `navForRole()` |
| `/executive` route | 404 | Executive Overview page | New `(app)/executive/page.tsx` |
| Data honesty | n/a | "Demo data" badge when `data_source==="mock"` | Prevents mistaking synthetic numbers for real |
| Non-eligible role visiting `/executive` | n/a | "Not available for your role" panel; API returns 403 | UI gate + server gate (server is the real gate) |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/reports/handler.go` | 1–55 | EXACT template for new package: handler struct, `NewHandler`, `RegisterRoutes`, **role-gate map pattern** (`exportRolesAllowed`) |
| P0 | `backend/internal/reports/reports.go` | 1–60, 75–180 | EXACT repo pattern: `Repo struct{ pool }`, `New(pool)`, single-pass aggregate SQL, store/position JOINs, `Source`/`StoreLoad`/`OpenRole` shapes to reuse |
| P0 | `backend/pkg/httpx/response.go` | 8–35 | Response envelope + `httpx.OK` / `httpx.Fail` helpers |
| P0 | `backend/pkg/config/config.go` | 187–189, 206–259, 300–321, 387 | Provider-flag pattern: const, `getenv(...,"mock")`, validation loop, `UsesRealX()` helper |
| P0 | `backend/cmd/api/main.go` | 322–324 | EXACT wiring site — register the new package right after reports |
| P1 | `backend/internal/middleware/mock_jwt.go` | 5–16 | `DevUser{Role,...}` + `UserContextKey` for reading role in handler |
| P1 | `backend/internal/reports/handler.go` (TriggerExport) | ~144–148 | How a handler reads role + returns 403 |
| P1 | `frontend/lib/api.ts` | 9–83 | `api.get` + envelope unwrap + `credentials:"include"` |
| P1 | `frontend/lib/queries.ts` | 38–40 | React Query hook pattern (`useQuery({queryKey, queryFn})`) |
| P1 | `frontend/lib/types.ts` | 9–14 | `Envelope`, `KPI`, `Funnel`, `Source`, `StoreLoad`, `OpenRole`, `Me` shapes |
| P0 | `frontend/components/analytics/Charts.tsx` | 14–157 | `KpiCards` (hero/reporting variants) + supporting-metric grid markup to mirror |
| P0 | `frontend/components/analytics/Operations.tsx` | 1–162 | `WaitingByStore`/`OpenRoles` ranked-bar (RankPanel) pattern — clone for fill-rate + pipeline |
| P1 | `frontend/components/shell/SummaryStrip.tsx` | all | Stat-strip for headline figures |
| P1 | `frontend/components/shell/PageHeader.tsx` | all | Editorial masthead (eyebrow/title/meta/actions) |
| P0 | `frontend/components/shell/nav-config.tsx` | 23–52 | `NavItem`, `NAV`, `navForRole()` — add Executive entry here |
| P1 | `frontend/components/shell/SideNav.tsx` | 43–116 | How nav renders + `useTranslations("nav")` |
| P1 | `frontend/lib/roles.ts` | all | Role constants + gate-function pattern (`canX(role)`) |
| P1 | `frontend/app/(app)/dashboard/page.tsx` | 1–160 | EXACT page template (client component, hooks, sections) |
| P2 | `frontend/messages/en.json` / `th.json` | `nav.*` | i18n key locations |
| P2 | `backend/internal/applications/handler_test.go` | 38–60 | `newTestApp(h)` + `app.Test(req)` unit-test pattern |

## External Documentation
No external research needed — feature uses established internal patterns (Fiber handler+repo, provider seam, React Query, custom CSS charts). No new libraries.

---

## Patterns to Mirror

### NAMING_CONVENTION (backend package)
```go
// SOURCE: backend/internal/reports/reports.go:1-9,46-47
package reports
// Repo computes analytics over applications/candidates.
type Repo struct{ pool *pgxpool.Pool }
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }
```
→ New package `executive`: `type Service` (mock|live behind interface), `type Handler`, `NewHandler`, `RegisterRoutes`.

### ROLE_GATE (in-handler, the established RBAC-for-broad-views idiom)
```go
// SOURCE: backend/internal/reports/handler.go:21-22 + TriggerExport ~144-148
var exportRolesAllowed = map[string]bool{"super_admin": true, "regional_director": true}
// in handler:
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
if !exportRolesAllowed[u.Role] {
    return fiber.NewError(fiber.StatusForbidden, "insufficient role to trigger an export")
}
```
→ Use `var executiveRolesAllowed = map[string]bool{"super_admin": true, "regional_director": true, "auditor": true}`.

### RESPONSE_ENVELOPE
```go
// SOURCE: backend/pkg/httpx/response.go:8-21
func OK[T any](c *fiber.Ctx, data T) error {
    return c.Status(fiber.StatusOK).JSON(Envelope[T]{Success: true, Data: data})
}
func Fail(c *fiber.Ctx, status int, msg string) error {
    return c.Status(status).JSON(Envelope[any]{Success: false, Error: msg})
}
```

### REPOSITORY_PATTERN (single-pass aggregate SQL, store/position JOIN — for live mode)
```go
// SOURCE: backend/internal/reports/reports.go (open-roles ~123-150)
const q = `
  SELECT p.id::text,
         COALESCE(NULLIF(p.title_en,''), p.title_th, 'Unknown role') AS title,
         COALESCE(SUM(v.headcount), 0)::int AS openings,
         COUNT(DISTINCT v.store_id) AS stores
  FROM vacancies v
  JOIN positions p ON p.id = v.position_id
  WHERE v.status = 'open'
  GROUP BY p.id, p.title_en, p.title_th
  ORDER BY openings DESC, title
  LIMIT $1`
```
→ Mock mode reads **real names only**: `SELECT store_no, store_name, subregion FROM stores ORDER BY store_no` and `SELECT id::text, COALESCE(NULLIF(title_en,''),title_th) FROM positions WHERE is_active LIMIT N`; numbers are generated deterministically (below).

### PROVIDER_SEAM (config)
```go
// SOURCE: backend/pkg/config/config.go:189,228,311,387
ProviderReal = "real"
// in Load():
PSProvider: getenv("PS_PROVIDER", "mock"),
// in validation loop:
{"PS_PROVIDER", c.PSProvider, []string{"mock", ProviderReal}},
// helper:
func (c *Config) UsesRealPeopleSoft() bool { return c.PSProvider == ProviderReal }
```

### CONFIG_VALIDATION_ENTRY
```go
// SOURCE: backend/pkg/config/config.go:300-321 — add one row:
{"EXECUTIVE_PROVIDER", c.ExecutiveProvider, []string{"mock", ProviderReal}},
```

### WIRING (main.go)
```go
// SOURCE: backend/cmd/api/main.go:322-324
reportRepo := reports.New(pool)
reportExporter := reports.NewExportService(reportRepo, blobClient, notifier, cfg.ReportRecipientList())
reports.RegisterRoutes(app, reports.NewHandler(reportRepo, reportExporter, blobClient))
// → ADD right after:
executive.RegisterRoutes(app, executive.NewHandler(executive.NewService(pool, cfg.ExecutiveProvider)))
```

### TEST_STRUCTURE
```go
// SOURCE: backend/internal/applications/handler_test.go:38-60
func newTestApp(h *Handler) *fiber.App {
    app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
    RegisterRoutes(app, h)
    return app
}
// app.Test(httptest.NewRequest(...)) ; assert resp.StatusCode
```

### FRONTEND_QUERY_HOOK
```ts
// SOURCE: frontend/lib/queries.ts:38-40
export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/api/v1/users/me").then((r) => r.data) });
}
```

### FRONTEND_NAV_GATE
```tsx
// SOURCE: frontend/components/shell/nav-config.tsx:43-49
export function navForRole(role?: string): NavItem[] {
  const items = [...NAV];
  if (canBulkUpload(role)) items.push(BULK_NAV);
  if (role === "super_admin" || role === "hr_manager") items.push(MEMBERS_NAV);
  if (role === "super_admin") items.push(ADMIN_NAV);
  return items;
}
```

### FRONTEND_PAGE_SHELL
```tsx
// SOURCE: frontend/app/(app)/dashboard/page.tsx:1-15 + PageHeader usage
"use client";
export default function ExecutivePage() {
  const { data } = useExecutiveOverview();
  // <PageHeader eyebrow="Leadership" title="Executive Overview" .../>
  // sections: <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
}
```

### KPI_CARD_MARKUP (mirror the supporting-metric grid)
```tsx
// SOURCE: frontend/components/analytics/Charts.tsx:70-85
<div className="grid grid-cols-1 gap-px bg-hairline sm:grid-cols-3">
  {supporting.map((m) => (
    <div key={m.label} className="flex flex-col justify-between bg-card px-5 py-5">
      <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{m.label}</p>
      <span className="num block text-3xl font-semibold tabular-nums leading-none tracking-tight">{fmt.format(m.value)}</span>
    </div>
  ))}
</div>
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/executive/types.go` | CREATE | Payload structs (`Overview`, `CompanyHeadcount`, `StoreFill`, `PipelinePosition`, reuse `Source`-shaped) |
| `backend/internal/executive/service.go` | CREATE | `Service` interface + `NewService(pool, provider)` dispatch to mock/live |
| `backend/internal/executive/mock.go` | CREATE | Deterministic synthetic data over real store/position names |
| `backend/internal/executive/live.go` | CREATE | Scaffold: ATS-derived metrics from DB; budget zeroed/flagged (PeopleSoft TODO) |
| `backend/internal/executive/handler.go` | CREATE | Fiber handler, `RegisterRoutes`, role-gate map |
| `backend/internal/executive/mock_test.go` | CREATE | Determinism + shape tests (no DB) |
| `backend/internal/executive/handler_test.go` | CREATE | Route + 403-role-gate test (`app.Test`) |
| `backend/pkg/config/config.go` | UPDATE | Add `ExecutiveProvider` field, `getenv` default, validation row, `UsesRealExecutive()` |
| `backend/cmd/api/main.go` | UPDATE | Wire `executive.RegisterRoutes(...)` after reports (line ~324) |
| `frontend/lib/types.ts` | UPDATE | Add `ExecutiveOverview` + sub-types |
| `frontend/lib/queries.ts` | UPDATE | Add `useExecutiveOverview()` |
| `frontend/lib/roles.ts` | UPDATE | Add `EXECUTIVE_ROLES` + `canViewExecutive(role)` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | Add `EXECUTIVE_NAV` + gate in `navForRole()` |
| `frontend/components/executive/ExecutiveSections.tsx` | CREATE | Fill-rate ranked panel + pipeline-by-position panel + demo badge (mirrors Operations.tsx) |
| `frontend/app/(app)/executive/page.tsx` | CREATE | The page (client component) |
| `frontend/messages/en.json` + `th.json` | UPDATE | `nav.executive` + `executive.*` keys (keep parity) |

## NOT Building
- **No Thailand map / geo visualization** (decided: ranked list/bar only). `stores.latitude/longitude` left unused.
- **No new DB tables or migration** — mock generates numbers in-memory; live mode reuses existing tables.
- **No PeopleSoft integration** — `live` mode returns budget as unavailable/zero with a flag; real PS sync is a future ticket.
- **No new "executive/ceo" RBAC role** — reuses existing KindAll roles (super_admin/regional_director/auditor).
- **No CSV/PDF export** of the executive view (reports module already owns exports; out of scope here).
- **No real-time/streaming** — simple request/response, React Query default caching.
- **No per-subregion scoping in mock** — mock is company-wide; live mode may later apply `rbac.Scope` for regional_director.

---

## Step-by-Step Tasks

### Task 1: Backend payload types
- **ACTION**: Create `backend/internal/executive/types.go`.
- **IMPLEMENT**:
  ```go
  package executive

  type Overview struct {
      DataSource string            `json:"data_source"` // "mock" | "live"
      GeneratedAt string           `json:"generated_at"` // RFC3339; pass time in, do not call time.Now in tests
      Company    CompanyHeadcount  `json:"company"`
      Stores     []StoreFill       `json:"stores"`        // sorted ASC by fill_rate (most short-staffed first)
      Pipeline   []PipelinePosition `json:"pipeline"`
      Sourcing   []Source          `json:"sourcing"`
  }
  type CompanyHeadcount struct {
      BudgetHeadcount int     `json:"budget_headcount"`
      ActualHeadcount int     `json:"actual_headcount"`
      Vacancy         int     `json:"vacancy"`          // budget - actual
      FillRatePct     float64 `json:"fill_rate_pct"`    // actual/budget*100, 1 dp
      BudgetAvailable bool    `json:"budget_available"` // false in live until PeopleSoft wired
  }
  type StoreFill struct {
      StoreNo         int     `json:"store_no"`
      StoreName       string  `json:"store_name"`
      Subregion       string  `json:"subregion"`
      BudgetHeadcount int     `json:"budget_headcount"`
      ActualHeadcount int     `json:"actual_headcount"`
      HeadsShort      int     `json:"heads_short"`   // budget - actual
      FillRatePct     float64 `json:"fill_rate_pct"`
  }
  type PipelinePosition struct {
      PositionID string `json:"position_id"`
      Title      string `json:"title"`
      Applied    int    `json:"applied"`
      Screening  int    `json:"screening"`
      Interview  int    `json:"interview"`
      Offer      int    `json:"offer"`
      Hired      int    `json:"hired"`
      Openings   int    `json:"openings"`
  }
  type Source struct {
      Channel    string  `json:"channel"`
      Applied    int     `json:"applied"`
      Hired      int     `json:"hired"`
      Conversion float64 `json:"conversion"`
  }
  ```
- **MIRROR**: `reports.go` struct + json-tag style (snake_case tags).
- **GOTCHA**: Use `GeneratedAt string` and inject the timestamp from the handler/caller, NOT `time.Now()` inside the data builder — keeps `mock_test.go` deterministic.
- **VALIDATE**: `go build ./internal/executive/...`

### Task 2: Service interface + dispatch
- **ACTION**: Create `backend/internal/executive/service.go`.
- **IMPLEMENT**:
  ```go
  package executive

  import (
      "context"
      "github.com/jackc/pgx/v5/pgxpool"
  )

  type Service interface {
      Overview(ctx context.Context) (Overview, error)
  }

  // NewService picks mock or live based on provider ("mock" default).
  func NewService(pool *pgxpool.Pool, provider string) Service {
      if provider == "real" {
          return &liveService{pool: pool}
      }
      return &mockService{pool: pool}
  }
  ```
- **MIRROR**: provider dispatch like `peoplesoft.NewService` / config provider seam.
- **IMPORTS**: `context`, `pgxpool`.
- **GOTCHA**: Both impls take `pool` (mock uses it for real names; live for everything). Keep the interface single-method so the frontend has one hook.
- **VALIDATE**: `go build ./internal/executive/...`

### Task 3: Mock provider (the demo brain)
- **ACTION**: Create `backend/internal/executive/mock.go`.
- **IMPLEMENT**:
  - `type mockService struct{ pool *pgxpool.Pool }`
  - `Overview(ctx)`:
    1. Load real stores: `SELECT store_no, COALESCE(store_name,'Store'), COALESCE(subregion,'') FROM stores ORDER BY store_no`. If query errors or returns 0 rows, fall back to a small baked list of 6 plausible CP Axtra branch names (so the demo never renders empty even on an empty DB).
    2. For each store, derive deterministic numbers from `store_no` (stable across refreshes): `budget := 180 + (storeNo*37)%220` (range ~180–400); `fillPct := 60 + (storeNo*13)%38` (60–98%); `actual := round(budget*fillPct/100)`; `headsShort := budget-actual`.
    3. Sort stores ASC by `FillRatePct` (most short-staffed first).
    4. Company totals = sum of store budget/actual; `Vacancy = budget-actual`; `FillRatePct = actual/budget*100` (1 dp); `BudgetAvailable=true`.
    5. Pipeline: load up to 8 real positions `SELECT id::text, COALESCE(NULLIF(title_en,''),title_th) FROM positions WHERE is_active=TRUE ORDER BY created_at LIMIT 8` (fallback baked list of retail roles: Cashier, Sales Associate, Stock Controller, Store Supervisor, …). Per position derive a funnel from a seed: `applied := 120 + (i*90)`, `screening := applied*30/100`, `interview := screening*30/100`, `offer := interview*35/100`, `hired := offer*70/100`, `openings := 5 + (i*3)%20`.
    6. Sourcing: baked deterministic channels `LINE`, `Google`, `Walk-in`, `Referral`, `JobsDB`, `Email` with descending applied + conversion.
    7. `DataSource="mock"`.
  - Add a small unexported helper `func deterministic(seed, base, span int) int { return base + (seed*37)%span }` or inline.
- **MIRROR**: SQL style + `COALESCE/NULLIF` from `reports.go`.
- **IMPORTS**: `context`, `fmt`, `sort`, `pgxpool`.
- **GOTCHA**: NO `math/rand` — must be deterministic so the demo is stable and tests are repeatable. NO `time.Now()` in number generation. Fallback list is essential because **prod has 14 real stores but a fresh/empty DB must still demo**.
- **VALIDATE**: `go test ./internal/executive/ -run TestMock`

### Task 4: Live provider scaffold
- **ACTION**: Create `backend/internal/executive/live.go`.
- **IMPLEMENT**:
  - `type liveService struct{ pool *pgxpool.Pool }`
  - `Overview(ctx)`: compute what ATS truly has —
    - Stores fill: budget unavailable → set `BudgetHeadcount=0`, `ActualHeadcount` from `COUNT(applications WHERE status='hired')` grouped by `assigned_store_id`; `FillRatePct=0`; `HeadsShort=0`. (Or `vacancies.headcount` as a proxy openings figure.)
    - Pipeline by position: real GROUP BY over applications/vacancies (reuse the open-roles JOIN + status FILTERs).
    - Sourcing: same query as `reports.Sources` (`candidates.source_channel` JOIN applications).
    - `Company.BudgetAvailable=false`; `DataSource="live"`.
  - Mark budget paths with `// TODO(peoplesoft): wire budget headcount once HRIS sync lands.`
- **MIRROR**: `reports.go` aggregate queries (sources, open-roles, by-store).
- **GOTCHA**: Do NOT fabricate budget in live mode — that's the whole honesty point. `BudgetAvailable=false` lets the UI show "Budget: pending HRIS".
- **VALIDATE**: `go build ./internal/executive/...` (full live correctness is a future ticket; build + shape is enough now).

### Task 5: Handler + routes + role gate
- **ACTION**: Create `backend/internal/executive/handler.go`.
- **IMPLEMENT**:
  ```go
  package executive

  import (
      "time"
      "github.com/gofiber/fiber/v2"
      "github.com/nexto/hr-ats/internal/middleware"
      "github.com/nexto/hr-ats/pkg/httpx"
  )

  var executiveRolesAllowed = map[string]bool{
      "super_admin": true, "regional_director": true, "auditor": true,
  }

  type Handler struct{ svc Service }
  func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

  func RegisterRoutes(app *fiber.App, h *Handler) {
      v1 := app.Group("/api/v1/executive")
      v1.Get("/overview", h.Overview)
  }

  func (h *Handler) Overview(c *fiber.Ctx) error {
      u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
      if !executiveRolesAllowed[u.Role] {
          return fiber.NewError(fiber.StatusForbidden, "executive overview is restricted to leadership roles")
      }
      ov, err := h.svc.Overview(c.UserContext())
      if err != nil {
          return err
      }
      ov.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
      return httpx.OK(c, ov)
  }
  ```
- **MIRROR**: `reports/handler.go` role-map + `httpx.OK`.
- **GOTCHA**: Stamp `GeneratedAt` here (handler), not in the service — keeps service deterministic/testable. In dev mode the mock JWT injects `super_admin`, so the route works locally without auth setup.
- **VALIDATE**: `go test ./internal/executive/ -run TestHandler`

### Task 6: Config flag
- **ACTION**: Update `backend/pkg/config/config.go`.
- **IMPLEMENT**:
  - Struct field (near other providers ~line 104): `ExecutiveProvider string`
  - In `Load()` (near line 247): `ExecutiveProvider: getenv("EXECUTIVE_PROVIDER", "mock"),`
  - In validation loop (line ~316, after GRAPH_PROVIDER): `{"EXECUTIVE_PROVIDER", c.ExecutiveProvider, []string{"mock", ProviderReal}},`
  - Helper (near line 387): `func (c *Config) UsesRealExecutive() bool { return c.ExecutiveProvider == ProviderReal }`
- **MIRROR**: exact `GRAPH_PROVIDER` lines.
- **GOTCHA**: default MUST be `"mock"` — prod stays demo-ready until PeopleSoft lands.
- **VALIDATE**: `go build ./pkg/config/...` + `go vet ./...`

### Task 7: Wire into main.go
- **ACTION**: Update `backend/cmd/api/main.go` after line 324.
- **IMPLEMENT**:
  ```go
  executive.RegisterRoutes(app, executive.NewHandler(executive.NewService(pool, cfg.ExecutiveProvider)))
  ```
  Add import `"github.com/nexto/hr-ats/internal/executive"`.
- **MIRROR**: the reports wiring block 322–324.
- **GOTCHA**: place it inside the same route-registration region (after `reports.RegisterRoutes`). `pool` and `cfg` are already in scope there.
- **VALIDATE**: `go build ./cmd/api/...`

### Task 8: Backend tests
- **ACTION**: Create `mock_test.go` + `handler_test.go`.
- **IMPLEMENT**:
  - `mock_test.go` (no DB — pass `nil` pool; mock must hit fallback baked lists when pool is nil → guard the queries with `if m.pool != nil`): assert `Overview` returns non-empty Stores/Pipeline/Sourcing, `DataSource=="mock"`, stores sorted ASC by fill-rate, company `Vacancy == Budget-Actual`, and that two calls return identical numbers (determinism).
  - `handler_test.go`: build `newTestApp`, inject a `DevUser` via a tiny middleware that sets `c.Locals(middleware.UserContextKey, middleware.DevUser{Role:"super_admin"})`; assert 200 + envelope `success:true`. Second test with `Role:"hr_staff"` → 403.
- **MIRROR**: `applications/handler_test.go:38-60`.
- **GOTCHA**: `mockService.Overview` must tolerate `pool==nil` (fall back to baked lists) so unit tests need no Postgres. Add that nil-guard in Task 3.
- **VALIDATE**: `go test ./internal/executive/...`

### Task 9: Frontend types
- **ACTION**: Update `frontend/lib/types.ts`.
- **IMPLEMENT**: mirror backend json tags exactly:
  ```ts
  export interface ExecutiveOverview {
    data_source: "mock" | "live";
    generated_at: string;
    company: { budget_headcount: number; actual_headcount: number; vacancy: number; fill_rate_pct: number; budget_available: boolean; };
    stores: { store_no: number; store_name: string; subregion: string; budget_headcount: number; actual_headcount: number; heads_short: number; fill_rate_pct: number; }[];
    pipeline: { position_id: string; title: string; applied: number; screening: number; interview: number; offer: number; hired: number; openings: number; }[];
    sourcing: Source[]; // existing Source interface
  }
  ```
- **MIRROR**: existing `Funnel`/`Source` snake_case interfaces.
- **GOTCHA**: keys are snake_case to match Go json tags (the api client does NOT camelize).
- **VALIDATE**: `npx tsc --noEmit`

### Task 10: Frontend query hook
- **ACTION**: Update `frontend/lib/queries.ts`.
- **IMPLEMENT**:
  ```ts
  export function useExecutiveOverview() {
    return useQuery({
      queryKey: ["executive-overview"],
      queryFn: () => api.get<ExecutiveOverview>("/api/v1/executive/overview").then((r) => r.data),
    });
  }
  ```
- **MIRROR**: `useKpi`/`useMe` exactly.
- **VALIDATE**: `npx tsc --noEmit`

### Task 11: Frontend role gate
- **ACTION**: Update `frontend/lib/roles.ts`.
- **IMPLEMENT**:
  ```ts
  export const EXECUTIVE_ROLES = ["super_admin", "regional_director", "auditor"];
  export function canViewExecutive(role?: string): boolean {
    return !!role && EXECUTIVE_ROLES.includes(role);
  }
  ```
- **MIRROR**: `canBulkUpload` pattern.
- **VALIDATE**: `npx tsc --noEmit`

### Task 12: Nav entry
- **ACTION**: Update `frontend/components/shell/nav-config.tsx`.
- **IMPLEMENT**:
  - Import an icon (`TrendingUp` from lucide-react) + `canViewExecutive` from `@/lib/roles`.
  - `export const EXECUTIVE_NAV: NavItem = { href: "/executive", label: "Executive", key: "executive", icon: TrendingUp };`
  - In `navForRole()`: `if (canViewExecutive(role)) items.push(EXECUTIVE_NAV);` (place before MEMBERS_NAV).
  - Ensure `EXECUTIVE_NAV` is included in `ALL_NAV` if that array exists (for breadcrumb in AppHeader).
- **MIRROR**: `BULK_NAV` / `navForRole` lines 41–49.
- **GOTCHA**: nav uses `item.key` for i18n → must add `nav.executive` to both message catalogs (Task 15) or `tNav("executive")` shows the raw key.
- **VALIDATE**: `npx tsc --noEmit`

### Task 13: Executive sections component
- **ACTION**: Create `frontend/components/executive/ExecutiveSections.tsx`.
- **IMPLEMENT** (client component):
  - `ShortStaffedPanel({ stores })`: ranked rows (already sorted asc by fill-rate from API), each row = store name + subregion + `fill_rate_pct%` + a horizontal bar (`width: fill_rate_pct%`, color ramp blue→clay where low fill = clay/red using `--score-low` for <70%, `--score-mid` 70–85%, `--brand` ≥85%) + `-{heads_short} heads`. Mirror the bar markup from `Operations.tsx`.
  - `PipelinePanel({ rows })`: per-position compact funnel `applied ▸ screening ▸ interview ▸ offer ▸ hired` with `openings` badge. Tabular-nums.
  - `DemoBadge({ source })`: when `source==="mock"`, render a small pill "Demo data" (use `Pill tone="pending"` from `people/PeopleBits.tsx`); when `"live"` and `budget_available===false`, show "Budget: pending HRIS".
- **MIRROR**: `Operations.tsx` ranked-bar (RankPanel) + `globals.css` score ramp tokens (`--score-low/mid/high`).
- **GOTCHA**: animate `width`/`transform` only (per web perf rules) with `transition-[width] duration-500`. Use `tabular-nums`/`num` classes for figures.
- **VALIDATE**: `npx tsc --noEmit`

### Task 14: Executive page
- **ACTION**: Create `frontend/app/(app)/executive/page.tsx`.
- **IMPLEMENT** (client component):
  ```tsx
  "use client";
  import { useExecutiveOverview, useMe } from "@/lib/queries";
  import { canViewExecutive } from "@/lib/roles";
  import { PageHeader } from "@/components/shell/PageHeader";
  import { KpiCards } from "@/components/analytics/Charts"; // or build a bespoke headcount band
  import { SourcesChart } from "@/components/analytics/Charts";
  import { ShortStaffedPanel, PipelinePanel, DemoBadge } from "@/components/executive/ExecutiveSections";
  ```
  - Gate: `const { data: me } = useMe(); if (me && !canViewExecutive(me.role)) return <NotAvailablePanel/>;`
  - Header: `<PageHeader eyebrow="Leadership" title="Executive Overview" meta="Company headcount, vacancies & pipeline" actions={<DemoBadge source={data?.data_source}/>} />`
  - Headcount KPI band: a hero card showing Budget / Actual / Vacancy / Fill rate (build a small bespoke band mirroring `KpiCards` hero markup, since the `KPI` shape differs — see Patterns).
  - Grid: `ShortStaffedPanel` (left) + `PipelinePanel` (right) in `grid gap-6 lg:grid-cols-2`.
  - Full-width `SourcesChart sources={data.sourcing}` (reuse as-is; `Source` shape matches).
  - Loading: `<Skeleton/>` fallbacks like `dashboard/page.tsx`.
- **MIRROR**: `app/(app)/dashboard/page.tsx` structure + `settle space-y-8` container.
- **GOTCHA**: This page is rendered inside `(app)/layout.tsx` (sidebar/header auto-wrap). It's a client page → any shared chrome it pulls MUST use `useTranslations`, never `getTranslations` (the bug that 500'd the portal in PRP-4).
- **VALIDATE**: `npm run build` (frontend uses plain `next build` — NO `--webpack` flag, unlike career-portal).

### Task 15: i18n keys (parity)
- **ACTION**: Update `frontend/messages/en.json` + `frontend/messages/th.json`.
- **IMPLEMENT**:
  - `nav.executive`: en `"Executive"`, th `"ผู้บริหาร"`.
  - `executive` block: `title`, `eyebrow`, `meta`, `demoData`, `budgetPending`, `shortStaffed`, `pipeline`, `sourcing`, `notAvailable` — both catalogs.
- **MIRROR**: existing `nav` block structure.
- **GOTCHA**: Run the parity guard after — `scripts/check-i18n-parity.mjs` fails CI on key drift between catalogs. Add the SAME keys to both files.
- **VALIDATE**: `node scripts/check-i18n-parity.mjs`

---

## Testing Strategy

### Unit Tests (backend)
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `TestMockOverview_Shape` | `NewService(nil,"mock")` | non-empty stores/pipeline/sourcing, `data_source=="mock"` | nil pool → baked fallback ✓ |
| `TestMockOverview_Deterministic` | call twice | identical numbers both calls | determinism ✓ |
| `TestMockOverview_SortedByFill` | mock output | `stores[i].fill_rate_pct <= stores[i+1]` | ordering ✓ |
| `TestCompany_VacancyMath` | mock output | `vacancy == budget - actual` | invariant ✓ |
| `TestHandler_Overview_AllowsLeadership` | `DevUser{Role:"super_admin"}` | 200, `success:true` | — |
| `TestHandler_Overview_DeniesStaff` | `DevUser{Role:"hr_staff"}` | 403 | role gate ✓ |

### Edge Cases Checklist
- [x] Empty DB (no stores/positions) → mock falls back to baked lists (never empty demo)
- [x] nil pool (unit test) → baked lists
- [ ] Role missing/empty → 403 (treated as non-leadership)
- [x] Determinism across refreshes (no rand/time in data)
- [ ] Frontend: `data===undefined` (loading) → skeletons; `error` → graceful message

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./...
cd ../frontend && npx tsc --noEmit
```
EXPECT: Zero errors. (If gofmt flags new files: `gofmt -w internal/executive/*.go pkg/config/config.go`.)

### Unit Tests
```bash
cd backend && go test ./internal/executive/... ./pkg/config/...
```
EXPECT: All pass (no Postgres needed — mock handles nil pool).

### Full Test Suite
```bash
cd backend && go test ./...
```
EXPECT: No regressions (exit 0 across all packages).

### i18n Parity
```bash
node scripts/check-i18n-parity.mjs
```
EXPECT: frontend catalog parity OK (new `nav.executive` + `executive.*` present in both en/th).

### Browser Validation (local)
```bash
# backend
cd backend && go run ./cmd/api    # dev mode → mock JWT super_admin, EXECUTIVE_PROVIDER defaults mock
# frontend
cd frontend && npm run dev
```
EXPECT: `/executive` renders headcount band + "most short-staffed branches" ranked list + pipeline + sourcing, with a "Demo data" badge; nav shows "Executive" entry.

### Manual Validation
- [ ] Nav "Executive" visible (dev = super_admin).
- [ ] Headcount band shows Budget > Actual, Vacancy = Budget−Actual, Fill rate %.
- [ ] "Most short-staffed branches" sorted worst-first, low fill = clay/red bar, `-N heads` shown.
- [ ] Pipeline lists real position titles with descending funnel.
- [ ] Sourcing chart renders channels.
- [ ] "Demo data" badge present (mock mode).
- [ ] TH/EN switch translates nav + headings.
- [ ] Simulate non-leadership: API `/api/v1/executive/overview` returns 403 for `hr_staff`.

---

## Acceptance Criteria
- [ ] All tasks completed
- [ ] `go build ./... && go vet ./...` clean; `npx tsc --noEmit` clean
- [ ] `go test ./...` green; executive unit tests cover shape/determinism/sort/role-gate
- [ ] i18n parity passes
- [ ] `/executive` page matches UX (headcount band, short-staffed ranked list, pipeline, sourcing, demo badge)
- [ ] Endpoint role-gated (403 for non-leadership), nav-gated for the 3 roles
- [ ] Default `EXECUTIVE_PROVIDER=mock`; demo renders rich even on empty DB

## Completion Checklist
- [ ] Follows reports/handler + repo patterns
- [ ] Provider seam mirrors PS/GRAPH provider flags exactly (incl. validation row + `UsesRealExecutive`)
- [ ] Envelope + error handling via `httpx`
- [ ] React Query hook + snake_case types match Go json tags
- [ ] Shared client-page chrome uses `useTranslations` (not `getTranslations`)
- [ ] No `math/rand`/`time.Now()` in mock data (deterministic)
- [ ] No hardcoded secrets; no new migration
- [ ] Self-contained — no further codebase search needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Empty prod DB makes demo look broken | Medium | High | Mock generates over real names but **falls back to baked store/position lists** when DB empty/nil |
| Executives mistake synthetic numbers for real | Medium | High | Visible "Demo data" badge; `data_source` in payload; honest `budget_available=false` in live |
| New nav/role gate regresses existing nav | Low | Medium | Additive `navForRole` branch only; tsc + manual nav check |
| i18n key drift fails CI | Medium | Low | Add keys to BOTH catalogs; run `check-i18n-parity.mjs` |
| gofmt dirties new files | Medium | Low | `gofmt -w` before commit (known session pattern) |
| Frontend `KpiCards` shape mismatch (KPI vs headcount) | Medium | Low | Build a small bespoke headcount band mirroring KpiCards markup rather than forcing the `KPI` interface |

## Notes
- **Deploy** (when shipping): backend `az acr build … --build-arg SVC=api` + `az containerapp update -n hrats-prod-api --image …:$TAG` (no env change needed — `EXECUTIVE_PROVIDER` defaults to mock); dashboard build needs the 4 `NEXT_PUBLIC_AZURE_AD_*` build-args; verify active revision via `revision list --query "[?properties.active]"`. No migration. **Smoke MANY dashboard routes after deploy** (PRP-4 lesson).
- **Future "live" ticket**: implement `live.go` fully (ATS-derived fill/pipeline/sourcing already trivial via reports queries) + PeopleSoft budget sync → flip `EXECUTIVE_PROVIDER=real`. The seam means zero frontend change.
- **Per-role scoping**: mock is company-wide for all 3 roles; if regional_director should see only their subregion later, apply `rbac.Scope` inside `liveService` (KindSubregion clause exists in `internal/rbac/scope.go`).
- Decisions captured: ranked list (no map), KindAll roles (no new role/migration), mock-first behind provider seam.
