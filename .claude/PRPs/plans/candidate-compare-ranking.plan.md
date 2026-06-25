# Plan: Compare Candidates + Top-10 Ranking (per position)

## Summary
A new HR-dashboard page that, for a chosen **position**, auto-loads every candidate who has **passed screening AND completed the AI interview**, ranks them by a **composite of their screening score + AI interview score**, highlights the **top 10**, and lets HR see a **side-by-side comparison** of the candidates' score dimensions and risk signals. Purpose: speed up overall decision-making when many applicants compete for the same position.

## User Story
As an **HR / hiring decision-maker**, I want to **see all eligible candidates for one position ranked and compared side-by-side**, so that **I can decide who to advance when many people apply for the same role**.

## Problem → Solution
Today HR reviews candidates one-by-one on `applications/[id]` and the only ranked view (`/shortlist`) is `status='shortlisted'`-only, has no position filter, and blends screening with the **human TA** rating — not the AI interview. → A position-scoped Compare page that ranks the post-AI-interview pool by `0.5·screening + 0.5·AI-interview`, with a top-10 leaderboard and a dimension-by-dimension comparison grid.

## Metadata
- **Complexity**: Large (backend repo method + endpoint + new scoring fn + tests; frontend page + hook + types + nav + RBAC; ~12-14 files)
- **Source PRD**: N/A (free-form request)
- **PRD Phase**: N/A
- **Estimated Files**: ~13

### Decisions locked with the user
1. **Ranking metric** = composite `0.5·screening_ai_score + 0.5·ai_interview_score` (both 0-100), with both sub-scores shown separately.
2. **Entry** = position-driven: pick a position → auto-load all eligible, ranked, top-10 highlighted.
3. **Compare scope** = by **position** (`applications.position_id`), within the caller's RBAC scope.

---

## UX Design

### Before
```
applications inbox ──click──> applications/[id]  (one candidate at a time)
/shortlist : status='shortlisted' only, no position filter, screening+human-TA blend
```

### After
```
┌─ /compare ───────────────────────────────────────────────────────────┐
│ [ Position ▼  แคชเชียร์ ]                         eligible: 14 คน      │
│                                                                       │
│ TOP 10 (composite = ½ screening + ½ AI interview)                     │
│ ┌───────────────────────────────────────────────────────────────┐   │
│ │ 1  ●AB  สมชาย   comp 88.5  scr 90 · intv 87  [strong]  ☑ compare│   │
│ │ 2  ●CD  สมหญิง  comp 84.0  scr 82 · intv 86  [recommend]☑      │   │
│ │ 3  ●EF  ...                                              ☐      │   │
│ │ ...                                                            │   │
│ └───────────────────────────────────────────────────────────────┘   │
│                                                                       │
│ SIDE-BY-SIDE  (checked rows, default top 5, max 6)                    │
│              สมชาย      สมหญิง     ...                                 │
│ Composite    88.5★      84.0       ...    (★ = highest)               │
│ Screening    90★        82         ...                                │
│ AI interview 87         86         ...                                │
│ Recommend    strong★    recommend  ...                                │
│ Experience   28/30★     24/30      ...    (bar per dimension)         │
│ Skills       18/20      19/20★     ...                                │
│ Education    9/10★      8/10       ...                                │
│ Language     9/10★      9/10★      ...                                │
│ Location     18/20      20/20★     ...                                │
│ Must-have    ✓          ✓          ...                                │
│ Red flags    -          ความเสี่ยง… ...                               │
│ Applied      3 วันก่อน   1 สัปดาห์   ...                                │
└───────────────────────────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Position picker | none | `Select` from `usePositions()`, persisted in URL `?position_id=` | mirror inbox URL-param pattern |
| Ranked pool | `/shortlist` (shortlisted only) | `/compare` (post-AI-interview pool, per position) | new endpoint |
| Side-by-side | n/a | dimension grid, checkbox toggles which candidates are columns | client-side from one fetch |
| Nav | n/a | new "เปรียบเทียบผู้สมัคร" entry | gated, see RBAC |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/applications/repository.go` | 424-483 | `Shortlist` — the exact repo+SQL+scope pattern to mirror for the compare query |
| P0 | `backend/internal/applications/shortlist_handler.go` | 1-91 | handler+routes+`ShortlistItem`+scope-only RBAC to mirror |
| P0 | `backend/internal/applications/scorecard.go` | 1-45 | `CompositeScore`/`round1` — mirror for `CompareScore` |
| P0 | `backend/internal/applications/transitions.go` | 8-42 | the status machine — the eligible-status set derivation |
| P0 | `backend/internal/applications/model.go` | 18-92 | status consts, `Application`, `ScoreBreakdown` |
| P1 | `backend/internal/interview/repository.go` | 95, 142-167 | `FindByApplicationID`, `SetEvaluation` — interview_sessions columns |
| P1 | `backend/internal/applications/list.go` | 22-32, 74-154 | `ListFilter` + how WHERE/scope clauses are built (position filter goes here-style) |
| P1 | `backend/internal/rbac/scope.go` | 56-90 | `ApplicationsClause`, `scopeFrom` shape |
| P1 | `backend/internal/applications/list_integration_test.go` | 1-82 | `//go:build integration`, `dsn`/`setupList`/`insertApp`, ranking test |
| P0 | `frontend/app/(app)/shortlist/page.tsx` | 14-67 | closest UI: ranked `<ol>`, RBAC gate, `InitialChip`+`ScoreBadge`+`FitLabel` |
| P0 | `frontend/app/(app)/applications/page.tsx` | 72-97, 119-150, 223-277, 376 | URL-param filters, `Select`, selection (`selected[]`+`Checkbox`), `BulkActionBar` |
| P0 | `frontend/lib/queries.ts` | 178-202, 244-250, 584-590 | `useApplications`/`usePositions`/`useApplication`/`useShortlist` hook patterns |
| P1 | `frontend/components/resume/ScoreBreakdown.tsx` | 9-97 | per-dimension bars (experience30/skills20/education10/language10/location20) |
| P1 | `frontend/components/inbox/ScoreBadge.tsx` | 13, all | `ScoreBadge`/`FitLabel`/`ScoreRail`/`fitTier` to reuse |
| P1 | `frontend/lib/roles.ts` | 11-51, 98-134 | `can()`, `PERMS`, `isLineManager`, `can*` wrappers |
| P1 | `frontend/components/shell/nav-config.tsx` | 99-116 | nav registration (`navForRole`, `ALL_NAV`) |
| P2 | `frontend/lib/types.ts` | 19-25, 27-61, 296-305 | `ScoreBreakdown`, `Application`, `ShortlistItem` shapes |
| P2 | `frontend/lib/api.ts` | 9, 77-118 | `api.get`, `buildQuery`, `ApiError` |

## External Documentation
No external research needed — feature uses established internal patterns (Fiber handler + pgx repo + RBAC scope + TanStack Query + base-ui).

---

## Patterns to Mirror

### NAMING_CONVENTION
```go
// SOURCE: backend/internal/applications/shortlist_handler.go:18-27
type ShortlistItem struct {
    ApplicationID   uuid.UUID `json:"application_id"`
    CandidateName   string    `json:"candidate_name"`
    PositionID      string    `json:"position_id"`
    PositionTitle   string    `json:"position_title"`
    AssignedStoreID *int      `json:"assigned_store_id"`
    AIScore         *float64  `json:"ai_score"`
    TAAvgOverall    *float64  `json:"ta_avg_overall"`
    Composite       float64   `json:"composite"`
}
```

### COMPOSITE_SCORE_FN
```go
// SOURCE: backend/internal/applications/scorecard.go:26-40 — mirror EXACTLY for CompareScore
const ( compositeAIWeight = 0.6; compositeTAWeight = 0.4 )
func CompositeScore(aiScore, taAvgOverall float64) float64 {
    if taAvgOverall <= 0 { return round1(aiScore) }
    return round1(aiScore*compositeAIWeight + taAvgOverall*20*compositeTAWeight)
}
```

### REPOSITORY_PATTERN (scoped ranked query)
```go
// SOURCE: backend/internal/applications/repository.go:438-453 (Shortlist)
where := "a.status = 'shortlisted'"
if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
    where += " AND " + clause
    args = append(args, cargs...)
}
limitPH := add(limit)
q := `SELECT a.id, COALESCE(c.full_name, ''), a.position_id::text,
        COALESCE(NULLIF(p.title_en,''), p.title_th, ''), a.assigned_store_id,
        a.ai_score, ta.avg_overall
      FROM applications a
      JOIN candidates c ON c.id = a.candidate_id
      JOIN positions p ON p.id = a.position_id
      LEFT JOIN ( ... ) ta ON ta.application_id = a.id
      WHERE ` + where + `
      ORDER BY (CASE WHEN ... END) DESC, a.ai_score DESC NULLS LAST, a.created_at DESC
      LIMIT ` + limitPH
```

### SERVICE/HANDLER + SCOPE
```go
// SOURCE: backend/internal/applications/dashboard_handler.go:115-118
func scopeFrom(c *fiber.Ctx) rbac.Scope {
    u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
    return rbac.New(u.Role, u.StoreID, u.Subregion).WithUserID(u.LocalID)
}
// SOURCE: backend/internal/applications/shortlist_handler.go:48-68 — scope-only RBAC (no perm middleware)
app.Get("/api/v1/shortlist", h.List)
items, err := h.apps.Shortlist(c.UserContext(), scopeFrom(c), limit)
return httpx.OK(c, items)
```

### TEST_STRUCTURE (integration)
```go
// SOURCE: backend/internal/applications/list_integration_test.go:1-54
//go:build integration
func dsn() string { if v := os.Getenv("DB_URL"); v != "" { return v }; return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable" }
func setupList(t *testing.T) (*pgRepository, uuid.UUID, uuid.UUID) {
    t.Helper(); pool, _ := pgxpool.New(ctx, dsn()); t.Cleanup(pool.Close)
    pool.Exec(ctx, `TRUNCATE applications, candidates, positions, stores, vacancies RESTART IDENTITY CASCADE`)
    // seed stores/positions/candidates ... returns &pgRepository{pool}, posID, candID
}
// NOTE: dsn()/setupList/insertApp already exist in package applications — REUSE, do not redefine (compile collision).
```

### TEST_STRUCTURE (pure unit, table-driven)
```go
// SOURCE: backend/internal/scoring/weights_test.go:15-36 — mirror for CompareScore tests (no build tag, no DB)
cases := []struct{ name string; ...; want float64 }{ ... }
for _, tc := range cases { t.Run(tc.name, func(t *testing.T){ ... }) }
```

### FRONTEND_QUERY_HOOK
```ts
// SOURCE: frontend/lib/queries.ts:584-590 (useShortlist) + 244-250 (single) + api buildQuery
export function useCompare(positionId: string) {
  return useQuery({
    queryKey: ["compare", positionId],
    queryFn: () => api.get<CompareResponse>("/api/v1/compare" + buildQuery({ position_id: positionId })).then((r) => r.data),
    enabled: !!positionId,
  });
}
```

### FRONTEND_RANKED_LIST
```tsx
// SOURCE: frontend/app/(app)/shortlist/page.tsx:44-67 — ranked <ol>, mirror for the leaderboard
<ol className="flex flex-col gap-3">{data.map((it, i) => (
  <li key={it.application_id}><div className="group flex items-center gap-4 rounded-xl bg-card p-4 ring-1 ring-hairline ...">
    <span className="num w-6 tabular-nums text-muted-foreground">{i + 1}</span>
    <InitialChip name={it.candidate_name || "?"} />
    <span className="num text-base font-semibold text-brand">{it.composite}</span>
    <ScoreBadge score={it.ai_score} /><FitLabel score={it.ai_score} />
  </div></li>))}
</ol>
```

### FRONTEND_RBAC_GATE
```tsx
// SOURCE: frontend/app/(app)/shortlist/page.tsx:15-30
const { data: me } = useMe();
const allowed = isLineManager(me) || me?.role === "super_admin"; // -> replace with canCompareCandidates(me)
if (me && !allowed) { return (<div className="settle"><PageHeader .../><section>...{t("notAvailable")}...</section></div>); }
```

---

## Files to Change

### Backend
| File | Action | Justification |
|---|---|---|
| `backend/internal/applications/compare.go` | CREATE | `CompareItem` struct, `CompareScore()`, `CompareByPosition()` repo method, `eligibleCompareStatuses` |
| `backend/internal/applications/compare_handler.go` | CREATE | `CompareHandler.List` (GET /api/v1/compare), `CompareResponse` envelope |
| `backend/internal/applications/repository.go` | UPDATE | add `CompareByPosition(...)` to the `Repository` interface (decl near `Shortlist`, line ~60) |
| `backend/cmd/api/main.go` | UPDATE | register `applications.RegisterCompareRoutes(app, applications.NewCompareHandler(appRepo))` near shortlist (line ~438) |
| `backend/internal/applications/compare_test.go` | CREATE | pure unit tests for `CompareScore` (table-driven) |
| `backend/internal/applications/compare_integration_test.go` | CREATE | integration: eligibility filter, position scope, ranking order, RBAC scope (reuse `dsn/setupList`) |

### Frontend
| File | Action | Justification |
|---|---|---|
| `frontend/app/(app)/compare/page.tsx` | CREATE | the page: position picker + leaderboard + side-by-side grid |
| `frontend/components/compare/CompareLeaderboard.tsx` | CREATE | top-10 ranked `<ol>` + checkbox-to-include |
| `frontend/components/compare/CompareGrid.tsx` | CREATE | side-by-side dimension table with per-row "best" highlight |
| `frontend/lib/queries.ts` | UPDATE | add `useCompare(positionId)` |
| `frontend/lib/types.ts` | UPDATE | add `CompareItem`, `CompareResponse` |
| `frontend/lib/roles.ts` | UPDATE | add `canCompareCandidates(me)` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | register `COMPARE_NAV` in `navForRole` + `ALL_NAV` |
| `frontend/e2e/compare.spec.ts` | CREATE | Playwright happy-path (only frontend test infra available) |

## NOT Building
- No new DB migration / column (uses existing `applications`, `interview_sessions`; weights already in JSONB).
- No recompute of scores — display the stored `ai_score` and `interview_score` verbatim.
- No manual multi-select-from-inbox entry path (user chose position-driven; the in-page checkboxes only toggle which loaded candidates are columns).
- No vacancy/store-level scope (user chose position-level).
- No PDF/CSV export in v1 (listed as a future recommendation).
- No editing/decision actions on this page (read-only decision aid; advancing status stays on `applications/[id]`).
- No changes to the existing `/shortlist` or its composite.

---

## Step-by-Step Tasks

### Task 1: `CompareScore` + eligible-status set (pure)
- **ACTION**: Create `backend/internal/applications/compare.go` with the scoring fn and the eligible-status set.
- **IMPLEMENT**:
  ```go
  // eligibleCompareStatuses: passed screening gate (scored) AND AI interview completed (ai_interviewed)+.
  // Derived from transitions.go: shortlisted/interview/... are reachable only via ai_interviewed.
  var eligibleCompareStatuses = []string{
      StatusAIInterviewed, StatusShortlisted, StatusInterview,
      StatusInterviewed, StatusPendingApproval, StatusOffer, StatusHired,
  }
  const ( compareScreeningWeight = 0.5; compareInterviewWeight = 0.5 )
  // CompareScore blends screening (0..100) and AI-interview (0..100) scores 50/50.
  func CompareScore(screening, interview float64) float64 {
      return round1(screening*compareScreeningWeight + interview*compareInterviewWeight)
  }
  ```
- **MIRROR**: `scorecard.go:26-40` (`CompositeScore`, `round1` already in package — reuse `round1`, do not redefine).
- **IMPORTS**: none new.
- **GOTCHA**: `round1` already exists in `scorecard.go` — reuse it (redefining = compile error).
- **VALIDATE**: `go build ./internal/applications/`.

### Task 2: `CompareItem` + `CompareByPosition` repo method
- **ACTION**: In `compare.go`, add the struct and the scoped ranked query.
- **IMPLEMENT**:
  ```go
  type CompareItem struct {
      ApplicationID    uuid.UUID       `json:"application_id"`
      CandidateName    string          `json:"candidate_name"`
      Status           string          `json:"status"`
      AppliedAt        time.Time       `json:"applied_at"`
      AssignedStoreID  *int            `json:"assigned_store_id"`
      StoreName        string          `json:"store_name"`
      ScreeningScore   *float64        `json:"screening_score"`   // applications.ai_score
      Breakdown        *ScoreBreakdown `json:"breakdown,omitempty"`
      MustHavePassed   *bool           `json:"must_have_passed"`
      AISummary        string          `json:"ai_summary,omitempty"`
      AIRedFlags       string          `json:"ai_red_flags,omitempty"`
      InterviewScore   *float64        `json:"interview_score"`   // interview_sessions.interview_score
      Recommendation   string          `json:"recommendation"`    // strong_recommend|recommend|neutral|caution
      InterviewSummary string          `json:"interview_summary,omitempty"`
      Composite        float64         `json:"composite"`
  }
  func (r *pgRepository) CompareByPosition(ctx context.Context, positionID uuid.UUID, scope rbac.Scope, limit int) ([]CompareItem, error)
  ```
  SQL (build placeholders the same way `Shortlist` does with an `add()` closure + `args`):
  ```sql
  SELECT a.id, COALESCE(c.full_name,''), a.status, a.created_at, a.assigned_store_id,
         COALESCE(st.store_name,''), a.ai_score, a.ai_score_breakdown, a.must_have_passed,
         COALESCE(a.ai_summary,''), COALESCE(a.ai_red_flags,''),
         s.interview_score, COALESCE(s.recommendation,''), COALESCE(s.summary,'')
  FROM applications a
  JOIN candidates c ON c.id = a.candidate_id
  JOIN interview_sessions s ON s.application_id = a.id AND s.status = 'completed'
  LEFT JOIN stores st ON st.store_no = a.assigned_store_id
  WHERE a.position_id = $1
    AND a.status = ANY($2)            -- eligibleCompareStatuses
    [AND <scope.ApplicationsClause>]  -- alias-qualify with a.* (see GOTCHA)
  ORDER BY (COALESCE(a.ai_score,0)*0.5 + COALESCE(s.interview_score,0)*0.5) DESC,
           a.ai_score DESC NULLS LAST, a.created_at DESC
  LIMIT $N
  ```
  After scan: unmarshal `ai_score_breakdown` JSONB into `*ScoreBreakdown` (mirror `repository.go:171-176`), and set `it.Composite = CompareScore(deref(ScreeningScore), deref(InterviewScore))`.
- **MIRROR**: `Shortlist` (`repository.go:438-483`) for the placeholder/args/scope mechanics and JSONB unmarshal from `FindByID` (`repository.go:171-176`).
- **IMPORTS**: `encoding/json`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5`, `internal/rbac` (already imported in package).
- **GOTCHA 1**: `ApplicationsClause` fragments reference bare columns (`assigned_store_id`, `vacancy_id`, `talent_pool`) — the `Shortlist` query also aliases the table `a` yet the clause uses unqualified names; verify against `Shortlist` (it works because those columns are unambiguous in its join). Keep the same joins so column names stay unambiguous; if Postgres reports ambiguity, the `stores` join exposes no clashing names (it has `store_no/store_name`), so bare `assigned_store_id` still resolves to `applications`.
- **GOTCHA 2**: pass `eligibleCompareStatuses` as a `[]string` bound to `$2` via `= ANY($2)` (pgx encodes Go `[]string` → `text[]`). Don't string-build an `IN (...)`.
- **GOTCHA 3**: INNER JOIN on `interview_sessions ... status='completed'` is intentional belt-and-suspenders — it both guarantees the AI interview is done and that `interview_score` is non-null for the composite.
- **VALIDATE**: `go build ./...`; covered by Task 7 integration test.

### Task 3: Add to `Repository` interface
- **ACTION**: Add the method to the `Repository` interface in `repository.go` near `Shortlist` (line ~60).
- **IMPLEMENT**: `CompareByPosition(ctx context.Context, positionID uuid.UUID, scope rbac.Scope, limit int) ([]CompareItem, error)`
- **MIRROR**: the `Shortlist` interface line.
- **GOTCHA**: any test fake/mock implementing `Repository` must gain the method — `go test ./...` reveals these.
- **VALIDATE**: `go build ./... && go vet ./internal/applications/`.

### Task 4: `CompareHandler` + route
- **ACTION**: Create `compare_handler.go` mirroring `shortlist_handler.go`.
- **IMPLEMENT**:
  ```go
  type CompareResponse struct {
      PositionID    string        `json:"position_id"`
      PositionTitle string        `json:"position_title"`
      Candidates    []CompareItem `json:"candidates"`
  }
  type CompareHandler struct { apps Repository; pos positions.Repository }
  func NewCompareHandler(apps Repository, pos positions.Repository) *CompareHandler
  func RegisterCompareRoutes(app *fiber.App, h *CompareHandler) { app.Get("/api/v1/compare", h.List) }
  func (h *CompareHandler) List(c *fiber.Ctx) error {
      pid, err := uuid.Parse(c.Query("position_id"))  // 400 on missing/invalid
      limit := 50; if v := c.Query("limit"); v != "" { /* clamp 1..200 */ }
      items, err := h.apps.CompareByPosition(c.UserContext(), pid, scopeFrom(c), limit)
      // resolve position title via h.pos.FindByID (best-effort, COALESCE en->th); items may be empty []
      return httpx.OK(c, CompareResponse{ ... Candidates: items })
  }
  ```
- **MIRROR**: `shortlist_handler.go:48-68` (scope-only RBAC, `httpx.OK`, limit clamp, `scopeFrom`). `Status` handler (`public/handler.go:509`) for the `pos.FindByID` title lookup pattern.
- **IMPORTS**: `strconv`, `github.com/google/uuid`, `internal/positions`, `pkg/httpx`, fiber.
- **GOTCHA**: ensure `items == nil` → return `[]CompareItem{}` (mirror `MyApplications` nil-guard) so the JSON is `[]` not `null`.
- **VALIDATE**: `go build ./...`; manual curl after deploy (401 unauth, 200 + `{candidates:[...]}` authed).

### Task 5: Wire route in main.go
- **ACTION**: In `backend/cmd/api/main.go` near line 438, add `applications.RegisterCompareRoutes(app, applications.NewCompareHandler(appRepo, positionRepo))`.
- **MIRROR**: `applications.RegisterShortlistRoutes(app, applications.NewShortlistHandler(appRepo))` (`main.go:438`). Confirm `positionRepo` is in scope in main (it is — used by public handler).
- **VALIDATE**: `go build ./cmd/api/`.

### Task 6: Unit tests for `CompareScore`
- **ACTION**: Create `compare_test.go` (no build tag).
- **IMPLEMENT** table-driven cases: `{90,87→88.5}`, `{0,0→0}`, `{100,100→100}`, `{82,86→84.0}`, rounding `{83,86→84.5}`. Assert eligible-set excludes `scored`/`ai_interview`/`pending`/`rejected` and includes `ai_interviewed`+.
- **MIRROR**: `scoring/weights_test.go:15-36`.
- **VALIDATE**: `go test ./internal/applications/ -run TestCompareScore`.

### Task 7: Integration test for `CompareByPosition`
- **ACTION**: Create `compare_integration_test.go` (`//go:build integration`), reuse `dsn()`/`setupList`. Need a helper to insert an application WITH a completed `interview_sessions` row + breakdown JSONB.
- **IMPLEMENT** scenarios:
  1. Two positions; only the queried position's candidates returned.
  2. Eligibility: a `scored` (no interview) and an `ai_interview` (invited, not completed) candidate are EXCLUDED; `ai_interviewed`/`shortlisted`/`interview` INCLUDED.
  3. Ranking: composite `0.5·ai + 0.5·intv` orders rows; assert order for `{ai90,intv80}` vs `{ai70,intv99}` (88.5 > 84.5).
  4. RBAC: a `hr_staff` store-scoped user sees only their store's eligible candidate (mirror `TestList_StoreScope`).
  5. `Breakdown` unmarshals from JSONB.
- **MIRROR**: `list_integration_test.go:56-97` + `findbyid_integration_test.go` (breakdown JSONB insert).
- **GOTCHA**: must INSERT an `interview_sessions` row with `status='completed'` + `interview_score` for each eligible app, else the INNER JOIN drops it. Seed via raw SQL in the test helper.
- **VALIDATE**: `DB_URL=... go test -tags=integration ./internal/applications/ -run TestCompareByPosition`.

### Task 8: Frontend types + hook
- **ACTION**: Add `CompareItem`/`CompareResponse` to `frontend/lib/types.ts`; `useCompare(positionId)` to `frontend/lib/queries.ts`.
- **IMPLEMENT**: types mirror the Go JSON tags exactly (snake_case). Hook mirrors `useShortlist`/single-fetch (`.then(r=>r.data)`, `enabled: !!positionId`).
- **MIRROR**: `types.ts:296-305` (`ShortlistItem`), `queries.ts:584-590`.
- **VALIDATE**: `cd frontend && npx tsc --noEmit`.

### Task 9: `canCompareCandidates` + nav
- **ACTION**: Add `canCompareCandidates(me)` to `roles.ts`; register `COMPARE_NAV` in `nav-config.tsx`.
- **IMPLEMENT**: Default gate = decision-makers + HR: `isLineManager(me) || me?.role === "super_admin" || can(me, PERMS.requisitionManage) || can(me, "applications.view")`. If no clean applications-view perm exists, gate the same as the inbox is gated (check `nav-config.tsx` for how `applications` nav is pushed and mirror it). Add `COMPARE_NAV` to `navForRole` (conditional push) and `ALL_NAV`.
- **MIRROR**: `roles.ts:98-134`, `nav-config.tsx:103-116`.
- **GOTCHA**: backend is scope-only (no perm middleware), so this gate is UX-only; a store user with the nav still only sees their scoped candidates. Keep the gate generous (this is a decision aid, not a sensitive admin action).
- **VALIDATE**: `npx tsc --noEmit`; nav renders for the chosen roles.

### Task 10: Compare page + components
- **ACTION**: Create `compare/page.tsx`, `CompareLeaderboard.tsx`, `CompareGrid.tsx`.
- **IMPLEMENT**:
  - `page.tsx`: `"use client"`, `<Suspense>` (reads `?position_id=`), `useMe()` RBAC gate, `<PageHeader eyebrow title meta=eligible-count actions=<position Select>/>`. Position `Select` from `usePositions()`, value bound to URL param (mirror inbox `setParam`). `useCompare(positionId)`. Empty/loading via `<Skeleton>`; empty-pool message.
  - `CompareLeaderboard`: ranked `<ol>` (mirror shortlist), each row: rank, `InitialChip`, name, `composite` (brand), `ScoreBadge`+`FitLabel` on screening, `interview_score`, recommendation badge, store, a `Checkbox` (include-in-grid). Visually emphasize ranks 1-3 / top-10. Top-10 = `slice(0,10)` emphasized; rest below a divider.
  - `CompareGrid`: columns = checked candidates (default top 5, max 6). Rows = Composite, Screening, AI interview, Recommendation, the five dimensions (bars vs caps 30/20/10/10/20 — reuse `ScoreBreakdown` bar style or inline), Must-have, Red flags, Applied date. Highlight the best cell per row (★) using `var(--score-high)`.
- **MIRROR**: `shortlist/page.tsx` (gate+`<ol>`), `applications/page.tsx:72-97,119-150,223-277` (URL `Select` + `selected[]`+`Checkbox`), `ScoreBreakdown.tsx:9-23` (dimension caps + bar math), `ScoreBadge.tsx` (badges), hand-rolled `ledger-head/ledger-row` for the grid (no `Tabs`/`Table` primitive).
- **IMPORTS**: `useMe`, `usePositions`, `useCompare`, `Select*`, `Checkbox`, `Skeleton`, `PageHeader`, `ScoreBadge`/`FitLabel`, `InitialChip`, `isLineManager`/`canCompareCandidates`.
- **GOTCHA 1**: dimension caps differ from weights — bars use caps (exp30/skl20/edu10/lng10/loc20) per `ScoreBreakdown.tsx:9-15`; do NOT divide by weights.
- **GOTCHA 2**: `ai_score`/`interview_score` are nullable — guard with `?? null` and render "-".
- **GOTCHA 3**: selection (which candidates are columns) is client-side state over the already-fetched list — no second fetch. Cap columns for readability.
- **VALIDATE**: `npx tsc --noEmit && npx eslint app/(app)/compare components/compare && npm run build`.

### Task 11: e2e spec
- **ACTION**: `frontend/e2e/compare.spec.ts` mirroring `dashboard.spec.ts`: navigate to `/compare`, pick a position, assert leaderboard rows render and the grid shows score rows. (Best-effort; gated by seed/auth like existing specs.)
- **MIRROR**: `frontend/e2e/dashboard.spec.ts`.
- **VALIDATE**: `npx playwright test compare` (or document as manual if seed/auth not available locally).

---

## Testing Strategy

### Unit Tests (Go, `compare_test.go`)
| Test | Input | Expected | Edge? |
|---|---|---|---|
| CompareScore default | 90, 87 | 88.5 | |
| CompareScore zero | 0, 0 | 0 | yes |
| CompareScore max | 100, 100 | 100 | yes |
| CompareScore rounding | 83, 86 | 84.5 | yes |
| eligible set excludes pre-interview | `scored`,`ai_interview` | not in set | yes |
| eligible set includes post-interview | `ai_interviewed`..`hired` | in set | |

### Integration Tests (Go, `-tags=integration`)
| Test | Setup | Expected |
|---|---|---|
| position isolation | 2 positions seeded | only queried position returned |
| eligibility filter | scored + ai_interview(invited) + ai_interviewed | only ai_interviewed-and-beyond returned |
| ranking order | {ai90,intv80}=88.5 vs {ai70,intv99}=84.5 | 88.5 first |
| RBAC store scope | store1 + store2 apps, hr_staff(store2) | only store2 |
| breakdown unmarshal | JSONB breakdown seeded | `Breakdown.Experience` etc. populated |

### Edge Cases Checklist
- [ ] Position with 0 eligible candidates → `{candidates: []}` (not null) → "ยังไม่มีผู้สมัครที่ผ่านรอบ AI" empty state
- [ ] Candidate with null `ai_score` or null `interview_score` (shouldn't happen given INNER JOIN, but guard) → composite from COALESCE 0
- [ ] More eligible than `limit` (200) → truncated, ranked; consider a "showing top N" note
- [ ] Store-scoped user with no visible candidates → empty (not 403)
- [ ] Invalid/missing `position_id` query → 400
- [ ] Grid with 1 candidate (no "best" comparison) → render without ★ noise

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l internal/applications/ && go vet ./internal/applications/ ./cmd/api/
cd ../frontend && npx tsc --noEmit && npx eslint "app/(app)/compare" components/compare lib/queries.ts lib/types.ts lib/roles.ts
```
EXPECT: zero output / zero errors.

### Unit Tests
```bash
cd backend && go test ./internal/applications/ -run TestCompareScore
```
EXPECT: PASS.

### Integration Tests
```bash
cd backend && DB_URL="postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable" \
  go test -tags=integration ./internal/applications/ -run TestCompareByPosition
```
EXPECT: PASS (docker stack up + migrated).

### Full Suites
```bash
cd backend && go test ./... ; cd ../frontend && npm run build
```
EXPECT: no regressions; build clean.

### Database Validation
No migration — confirm schema unchanged: `~/go/bin/migrate ... version` stays at current (46).

### Manual Validation
- [ ] `cd frontend && npm run dev`; log in; open `/compare`; pick a position with eligible candidates
- [ ] Leaderboard ranks by composite; top-10 emphasized; recommendation badges show
- [ ] Toggle checkboxes → grid columns update; per-row ★ marks the best
- [ ] Store-scoped user sees only their candidates; a no-eligible position shows the empty state
- [ ] `curl -s "$API/api/v1/compare?position_id=<uuid>"` → 401 unauth; authed → `{candidates:[...]}`

---

## Design Recommendations — "what else to display" (the user explicitly asked)

Prioritized; v1 = the grid above. Recommended additions:

**P1 (high decision value, low cost)**
1. **Per-dimension winner highlight (★)** — already in the grid; the single biggest "who's stronger where" aid.
2. **AI interview recommendation badge** — color-coded `strong_recommend`(green) / `recommend`(blue) / `neutral`(grey) / `caution`(amber/clay). A fast qualitative signal beside the numeric score.
3. **Red flags / concerns row** — surface `ai_red_flags` (screening) + interview `concerns` so risks are visible at a glance, not buried.
4. **Must-have gate row** — `must_have_passed` ✓/✗; a hard disqualifier should never be hidden behind a high score.
5. **Pool reference line** — show the pool average/median composite so a "high" score is read relative to the cohort.

**P2 (nice, more effort)**
6. **Delta vs #1** — each candidate's composite gap to the top, so HR sees how close the race is.
7. **Strengths summary (AI)** — 1-2 line `ai_summary`/interview `strengths` per candidate column (collapsible) for qualitative colour.
8. **Cross-fit hint** — `ai_suggested_positions`: flag a candidate who may fit another role better (avoid losing good people).
9. **Recency / source** — `applied_at` ("3 วันก่อน") + `source_channel`; tie-breakers and channel insight.
10. **Export / share** — CSV or print-friendly view of the comparison for an offline hiring panel (reuse `downloadFile` in `api.ts:89`).

**P3 (later)**
11. Radar/spider chart per candidate over the 5 dimensions (needs a chart lib — `executive`/`reports` may already have one; check before adding).
12. Save a "compare set" / annotate a decision note per position.

Keep v1 to the grid + P1 rows; fold P2/P3 into follow-ups.

---

## Acceptance Criteria
- [ ] Picking a position auto-loads all eligible (post-AI-interview) candidates for it, within RBAC scope
- [ ] Candidates ranked by `0.5·screening + 0.5·AI-interview`; top-10 emphasized
- [ ] Side-by-side grid shows the 5 dimensions + both scores + recommendation + must-have + red flags, with per-row best highlight
- [ ] `scored`/`ai_interview`(invited) candidates are excluded
- [ ] All validation commands pass; backend unit+integration green; frontend builds

## Completion Checklist
- [ ] Mirrors discovered patterns (Shortlist repo/handler, scorecard composite, shortlist `<ol>`, inbox selection)
- [ ] Error handling + `httpx.OK` + nil→`[]` guard match codebase
- [ ] RBAC scope enforced in the query (not just the nav gate)
- [ ] Tests follow `dsn/setupList` + table-driven conventions
- [ ] No recompute of scores; stored values displayed
- [ ] No migration; no scope creep beyond v1 grid + P1 rows
- [ ] Self-contained — no codebase search needed during implementation

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `ApplicationsClause` column ambiguity once `stores`/`interview_sessions` joined | Low | Med | mirror Shortlist's working joins; alias `a.` on shared cols; integration test catches it |
| INNER JOIN on completed session drops a legacy `ai_interviewed` row with no session | Low | Low | acceptable (no interview_score = can't compute composite); documented; optional LEFT JOIN fallback if it appears |
| Side-by-side too wide on many candidates | Med | Low | default top-5 columns, max 6, checkbox toggle; horizontal scroll on the grid |
| Frontend e2e-only (no unit runner) leaves grid "best" logic lightly tested | Med | Low | keep the best-per-row a small pure helper; cover composite math in Go; e2e smoke + manual |
| Nav gate too narrow/broad for the audience | Low | Low | default generous (decision-makers + HR + super_admin); backend scope is the real boundary |

## Notes
- The persisted weights live inside `ai_score_breakdown` JSONB (`weights` key) but the read struct doesn't decode them; v1 does NOT need them (composite uses stored `ai_score`). If a future "explain the weighting" row is wanted, extend a read struct to unmarshal that key — no migration.
- The existing `/shortlist` (status='shortlisted', screening+human-TA) is intentionally left untouched; Compare is a different pool (post-AI-interview) and a different composite (screening+AI-interview). Two complementary views.
- Status set + composite are the load-bearing definitions — both are unit-tested at the source so they can't silently drift.
