# Plan: ATS 3.9 — Reports (recruitment-funnel metrics, RBAC-scoped + CSV export)

## Summary
A new HR-facing **ATS Reports** surface that aggregates the Module-3 hiring lifecycle into operational metrics the existing analytics/executive views don't cover: a **reached-funnel + conversion**, **time-to-hire & stage timing**, **offer + onboarding outcomes**, and **interview + approval quality** — all **RBAC-scoped** (store/subregion/all), filtered by a **date range**, and exportable as **CSV** (synchronous download). Backend-heavy: read-only aggregation over existing tables (NO migration); one new dashboard page + nav + i18n.

## User Story
As an **HR user (store / regional / leadership)**, I want a single Reports page showing how our hiring funnel performed over a chosen period — conversion at each stage, how long hires take, offer-acceptance and onboarding-completion rates, interview pass-rate and approval cycle-time — scoped to the stores I'm responsible for and downloadable as CSV, so that I can monitor and report recruitment performance without exporting raw rows.

## Problem → Solution
**Current state:** `internal/reports` provides only a 4-stage whole-table funnel + KPI + sources + a weekly CSV/JSON blob-export pipeline; `executive` provides a leadership-only company snapshot. Neither is RBAC-scoped, neither has a time dimension, and none of time-to-hire / offer-accept-rate / onboarding-completion / interview-pass-rate / approval-cycle-time exists — although every timestamp needed is already in the schema. → **Desired state:** a scoped, date-ranged ATS Reports page + JSON & CSV endpoints computing those metrics, reusing the existing reports package, RBAC scope machinery, and dashboard chart/panel conventions.

## Metadata
- **Complexity**: Large (backend aggregation + scope + 1 page + CSV; no migration, no career-portal)
- **Source PRD**: N/A — client Module-3 slice 3.9, free-form
- **Estimated Files**: ~14 (backend ~5, dashboard ~7, i18n 2)

---

## Key Design Decisions (read first)

1. **Extend `internal/reports`, don't add a package.** New file `ats_report.go` (types + `*Repo` methods) + a small `ats_report_csv.go` (pure flatten/encode) + new routes on the existing `RegisterRoutes` group. Reuses the package's `Repo{pool}`, blob/export infra is untouched.
2. **RBAC-scoped — the key gap.** Every aggregation applies `rbac.Scope.ApplicationsClause` (filters `applications.assigned_store_id` by store/subregion; `all` → no filter). Offer/onboarding/approval/interview queries JOIN `applications a` and apply the clause on that alias. The handler builds the scope from the authenticated user (mirror `applications.scopeFrom`). Store roles (hr_staff/hr_manager/sgm) see only their store — none of the existing reporting does this.
3. **Role gate**: `reportViewRoles` = super_admin, regional_director, operation_director, auditor, sgm, hr_manager, hr_staff (everyone with dashboard access); results are scoped by their RBAC kind. 403 otherwise.
4. **Date range**: `from`/`to` query params (RFC3339 or `YYYY-MM-DD`). Default `to`=now, `from`=now−90d. Half-open `[from, to)`. Each section keys off the natural timestamp (funnel/onboarding/interview/approval by their event time; time-to-hire by `hired_at`; offers by `sent_at`).
5. **Reached-funnel via event flags** (monotonic, defensible — not current-status, which loses history): applied=`created_at` in range; screened=`ai_score`/`must_have_passed` set; interview=EXISTS interview_appointments; offer=EXISTS offer with `sent_at`; hired=`hired_at` set. Step-conversion % computed in Go.
6. **Median** via `percentile_cont(0.5) WITHIN GROUP (...)`; durations in **days** = `EXTRACT(EPOCH FROM (b - a))/86400.0`.
7. **Onboarding completion** compares each hired app's approved-required-doc count to the **config-driven required set** (`cfg.OnboardingRequiredDocs()`), passed into the query as `($docTypes ANY, requiredCount)`. (Mirrors slice 3.8's "every required type approved".)
8. **CSV = synchronous `downloadFile`** (Members pattern), endpoint `GET /api/v1/reports/ats.csv` → `text/csv` attachment; flatten the report to `section,metric,value` rows (same shape family as the existing `EncodeCSV`). NOT the scheduled blob/email pipeline.
9. **No migration, no new table** — pure read aggregation. **No career-portal** changes.

---

## UX Design

### Before
```
Sidebar: Overview · Inbox · Candidates · Search · Analytics [· Executive · …]
Analytics = company-wide snapshot (4-stage funnel / KPI / sources) + scheduled exports.
No scoped, time-ranged, lifecycle-metric report. Store HR cannot see store-only metrics.
```

### After
```
Sidebar: … · Analytics · Reports (new, role-gated)
/reports:
┌──────────────────────────────────────────────────────────────┐
│ Reporting · ATS Reports            [ from ▾ ] [ to ▾ ] [⇩ CSV] │
│ scope: <Store X | Subregion | Company>                         │
│ ── Funnel ─────────────────────────────────────────────────── │
│ Applied 120 → Screened 86 (72%) → Interview 41 (48%) →         │
│   Offer 22 (54%) → Hired 18 (82%)                              │
│ ── Time to hire ───────────────────────────────────────────── │
│ avg 23.4d · median 19d   | to-screen 2.1d · to-offer 15d ·     │
│   offer-response 1.8d                                          │
│ ── Offer & onboarding ─────────────────────────────────────── │
│ offers sent 22 · accept 81% · decline 14%                     │
│ onboarding complete 12/18 (67%) · doc rejection 9%            │
│ ── Interview & approval ───────────────────────────────────── │
│ interview pass 73% · avg rating 3.8/5                         │
│ approval cycle 1.9d · SLA breach 6%                           │
└──────────────────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Sidebar | no Reports | + Reports (role-gated `canViewReports`) | scoped page |
| Date range | n/a | from/to inputs → query key | local state + `buildQuery` |
| Export | scheduled blob only | one-click CSV `downloadFile` | `ats-report.csv` |
| Scope | whole-table everywhere | store/subregion/company per role | first scoped report |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/reports/reports.go` | all | `Repo{pool}`, `New`, `COUNT(*) FILTER` query style, struct/JSON conventions to mirror |
| P0 | `backend/internal/reports/handler.go` | all | `RegisterRoutes` group, how it reads the user/role from locals, `exportRolesAllowed` gate, `clampLimit`, response envelope |
| P0 | `backend/internal/reports/export.go` | 39-77 | `EncodeCSV` (`section,metric,value`) — the CSV flatten template |
| P0 | `backend/internal/rbac/scope.go` | 16-69 | `Scope`, `New`, `Kind`, **`ApplicationsClause(argStart)` exact return signature** (string + args) |
| P0 | `backend/internal/applications/dashboard_handler.go` | 113-160 | `scopeFrom(c)` (build `rbac.New` from `middleware.DevUser`) + `from`/`to` RFC3339 parse pattern |
| P1 | `backend/internal/applications/model.go` | 18-34 | status set; confirm event columns used in funnel |
| P1 | `backend/migrations/000001_init_schema.up.sql` | 79-100 | `applications` timestamp cols: `reviewed_at, hired_at, parsed_at, created_at` |
| P1 | `backend/migrations/000023_offers.up.sql` | all | `offers.sent_at/responded_at/status` |
| P1 | `backend/migrations/000025_onboarding_documents.up.sql` | all | `onboarding_documents.status/doc_type/uploaded_at` |
| P1 | `backend/migrations/000020_interview_feedback.up.sql` | all | `interview_feedback.overall_rating/recommendation/application_id/created_at` (exact names) |
| P1 | `backend/migrations/000022_approval_workflow.up.sql` | all | `approval_requests.application_id/created_at/decided_at`; `approval_steps` FK + `escalated/created_at` (exact names) |
| P1 | `backend/migrations/000019_status_machine.up.sql` | all | `interview_appointments.application_id/created_at` |
| P1 | `backend/pkg/config/config.go` | OnboardingRequiredDocs() | required-doc set for onboarding completion |
| P1 | `backend/cmd/api/main.go` | ~359-362 | where `reports.RegisterRoutes` + executive are wired (pass cfg required docs) |
| P1 | `backend/internal/reports/handler_*` or `reports_integration_test.go` | all | existing test style in the package |
| P0 | `frontend/components/analytics/Charts.tsx` | 20, 190, 346 | `KpiCards`/`FunnelChart`/`SourcesChart` props + OKLCH ramp style to mirror |
| P0 | `frontend/app/(app)/analytics/page.tsx` | all | page skeleton (PageHeader, hooks, grid, `.settle`) |
| P0 | `frontend/app/(app)/members/page.tsx` | 105-115, 195-196 | `downloadFile` CSV export button pattern (`Download` icon, toast on fail) |
| P0 | `frontend/components/shell/nav-config.tsx` | 18-23, 26-32, 57-69 | `NavItem`, `NAV`, `navForRole`, `ALL_NAV` — register Reports |
| P1 | `frontend/lib/roles.ts` | 54-61 | `canView*` allowlist pattern → `canViewReports` |
| P1 | `frontend/lib/queries.ts` | 185-236 | reports hooks (`useFunnel`…/`useReportExports`) pattern |
| P1 | `frontend/lib/types.ts` | 367-386 | `Funnel`/`KPI`/`Source` types — add ATS report types nearby |
| P1 | `frontend/app/(app)/executive/page.tsx` | all | role-gated page + `notAvailable` i18n convention |
| P2 | `frontend/messages/{en,th}.json` | executive block | namespace shape for a new `reports` namespace + `nav.reports` |

## External Documentation
No external research needed — internal patterns only (pgx aggregation, rbac scope, next-intl, custom CSS charts, `downloadFile`). `percentile_cont`/`EXTRACT(EPOCH …)` are standard Postgres.

---

## Patterns to Mirror

### REPO_QUERY — FILTER aggregation
```go
// SOURCE: internal/reports/reports.go:43-56 (Funnel)
const q = `SELECT COUNT(*) AS applied,
  COUNT(*) FILTER (WHERE must_have_passed IS TRUE) AS passed_ai, ...
  FROM applications`
var f Funnel
err := r.pool.QueryRow(ctx, q).Scan(&f.Applied, &f.PassedAI, ...)
```

### RBAC scope (apply to every ATS query)
```go
// SOURCE: internal/rbac/scope.go:42-54 — confirm exact signature at implement
clause, args := scope.ApplicationsClause(3) // $1,$2 = date range; scope args start at $3
q := `SELECT ... FROM applications a WHERE a.created_at >= $1 AND a.created_at < $2 ` + clause
rowArgs := append([]any{from, to}, args...)
// offers/onboarding/approval/interview: JOIN applications a ON a.id = x.application_id, same clause on `a`
```

### SCOPE build from user (handler)
```go
// SOURCE: internal/applications/dashboard_handler.go:113-160 (scopeFrom + from/to parse)
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
scope := rbac.New(u.Role, u.StoreID, u.Subregion)
from, to, err := parseRange(c.Query("from"), c.Query("to")) // default to=now, from=now-90d; 400 on bad
```

### CSV flatten
```go
// SOURCE: internal/reports/export.go:49-77 (EncodeCSV: section,metric,value)
w := csv.NewWriter(&buf); w.Write([]string{"section","metric","value"})
w.Write([]string{"funnel","applied", strconv.Itoa(rep.Funnel.Applied)}) // ...
```

### FRONTEND page + panel
```tsx
// SOURCE: frontend/app/(app)/analytics/page.tsx + members/page.tsx:105-115
const { data } = useAtsReport(from, to);
await downloadFile(`/api/v1/reports/ats.csv${buildQuery({from,to})}`, "ats-report.csv");
// panels: <section className="rounded-xl bg-card p-6 ring-1 ring-hairline"> + eyebrow + tabular-nums
```

### NAV registration
```tsx
// SOURCE: frontend/components/shell/nav-config.tsx:57-69
const REPORTS_NAV = { href: "/reports", label: "Reports", key: "reports", icon: BarChart3 };
// in navForRole: if (canViewReports(role)) items.push(REPORTS_NAV);  + add to ALL_NAV
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/reports/ats_report.go` | CREATE | ATSReport types + `*Repo` aggregation methods (funnel/timing/offer/onboarding/interview/approval) |
| `backend/internal/reports/ats_report_csv.go` | CREATE | pure flatten → `section,metric,value` CSV |
| `backend/internal/reports/ats_report_handler.go` | CREATE | scope build + date parse + role gate + JSON & CSV handlers + `RegisterATSRoutes` (or extend handler.go) |
| `backend/internal/reports/ats_report_test.go` | CREATE | handler tests (fake store: role gate/scope/date 400/JSON/CSV) + pure flatten + conversion helper |
| `backend/cmd/api/main.go` | UPDATE | wire ATS report handler with `cfg.OnboardingRequiredDocs()` + the pool/scope |
| `frontend/app/(app)/reports/page.tsx` | CREATE | Reports page: date range + 4 panels + CSV button + role gate |
| `frontend/components/reports/ReportSections.tsx` | CREATE | the 4 metric panels (funnel/timing/outcomes/quality) |
| `frontend/lib/types.ts` | UPDATE | `AtsReport` + sub-types |
| `frontend/lib/queries.ts` | UPDATE | `useAtsReport(from, to)` |
| `frontend/lib/roles.ts` | UPDATE | `canViewReports` + `REPORT_VIEW_ROLES` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | `REPORTS_NAV` + `navForRole` + `ALL_NAV` |
| `frontend/messages/{en,th}.json` | UPDATE | `reports` namespace + `nav.reports` |

## NOT Building
- **No migration / new table** — read-only over existing tables.
- **No career-portal** changes (HR-only report).
- **No scheduled/persisted ATS-report export** — sync CSV only (the weekly blob/email pipeline stays for the existing snapshot export).
- **No new charting library** — custom CSS/SVG panels following the existing convention (recharts stays unused).
- **No trend/time-series line charts** — single-period aggregates (a by-month trend is a future follow-up; out of scope to keep it tight).
- **No edits to the existing `/analytics` snapshot endpoints or `executive`** — additive only.
- **No company budget/fill-rate** (that's executive's domain, PeopleSoft-pending).

---

## Step-by-Step Tasks

### Task 1: Confirm exact schema column names
- **ACTION**: Read migrations 000019/000020/000022 (+ 000001/000023/000025) for exact column names before writing SQL.
- **VALIDATE**: note `interview_feedback`(application_id, overall_rating, recommendation, created_at), `approval_requests`(application_id, created_at, decided_at), `approval_steps`(<request FK>, escalated, created_at), `interview_appointments`(application_id), `offers`(sent_at, responded_at, status), `applications`(created_at, parsed_at, reviewed_at, hired_at, ai_score, must_have_passed, assigned_store_id), `onboarding_documents`(application_id, doc_type, status, uploaded_at). Confirm `rbac.ApplicationsClause` return signature.

### Task 2: Types — `ats_report.go` (types half)
- **ACTION**: Define the composite report structs.
- **IMPLEMENT**:
  - `type ATSFilter struct { From, To time.Time }`
  - `type ATSFunnelStage struct { Key string; Count int; ConversionPct float64 }` (Conversion vs previous stage; first = 100)
  - `type ATSFunnel struct { Stages []ATSFunnelStage }`
  - `type ATSTiming struct { HiredCount int; AvgDaysToHire, MedianDaysToHire, AvgDaysToScreen, AvgDaysToOffer, AvgOfferResponseDays float64 }` (pointer or 0 when no data — use `float64` rounded to 1dp in Go; `0` when N=0)
  - `type ATSOfferOutcomes struct { Sent, Accepted, Declined int; AcceptRatePct, DeclineRatePct, AvgResponseDays float64 }`
  - `type ATSOnboarding struct { HiredInRange, Completed int; CompletionRatePct float64; DocsReviewed, DocsRejected int; DocRejectionRatePct float64 }`
  - `type ATSQuality struct { InterviewFeedbackCount, InterviewPassed int; InterviewPassRatePct, AvgInterviewRating float64; ApprovalDecided int; AvgApprovalCycleDays float64; ApprovalSteps, ApprovalBreached int; ApprovalSLABreachPct float64 }`
  - `type ATSReport struct { From, To time.Time; Scope string; Funnel ATSFunnel; Timing ATSTiming; Offers ATSOfferOutcomes; Onboarding ATSOnboarding; Quality ATSQuality }` (all `json:"snake_case"`).
- **MIRROR**: `reports.go` struct/JSON style.
- **GOTCHA**: round all percentages/days in Go (`math.Round(x*10)/10`); guard divide-by-zero (rate=0 when denom=0). `Scope` = a human label ("Company" / "Subregion" / "Store <id>") for the CSV/header.
- **VALIDATE**: `go build ./internal/reports/`.

### Task 3: Repo aggregation methods — `ats_report.go`
- **ACTION**: Add methods on `*Repo` that take `(ctx, scope rbac.Scope, f ATSFilter, requiredDocs []string)` and return the composite (or one method per section called by an orchestrator `ATSReport(...)`).
- **IMPLEMENT** (apply scope clause to each; `a` = applications alias):
  - **Funnel** — single row over `applications a` WHERE `a.created_at >= $1 AND a.created_at < $2` + scope: `applied=COUNT(*)`, `screened=COUNT(*) FILTER (WHERE a.ai_score IS NOT NULL OR a.must_have_passed IS NOT NULL)`, `interview=COUNT(*) FILTER (WHERE EXISTS(SELECT 1 FROM interview_appointments ia WHERE ia.application_id=a.id))`, `offer=COUNT(*) FILTER (WHERE EXISTS(SELECT 1 FROM offers o WHERE o.application_id=a.id AND o.sent_at IS NOT NULL))`, `hired=COUNT(*) FILTER (WHERE a.hired_at IS NOT NULL)`. Build `Stages` + conversion in Go.
  - **Timing** — hired apps (`a.hired_at` in range): `COUNT(*)`, `AVG`/`percentile_cont(0.5)` of `(hired_at-created_at)` days; plus `AVG(parsed_at-created_at)` (parsed in range), `AVG(o.sent_at-a.created_at)` and `AVG(o.responded_at-o.sent_at)` over offers joined (sent in range). Use `COALESCE(...,0)`/nullable scan into `*float64` then round.
  - **Offers** — offers with `o.sent_at` in range, JOIN applications + scope: sent/accepted/declined counts + `AVG(responded_at-sent_at)`; rates in Go.
  - **Onboarding** — completion CTE (hired in range; approved-required-doc count `>= $N` where required types = `$docTypes`), + doc rejection over `onboarding_documents.uploaded_at` in range.
  - **Quality** — interview_feedback (created in range): total, `recommendation='pass'`, `AVG(overall_rating)`; approval_requests (created in range): decided + `AVG(decided_at-created_at)`; approval_steps (created in range): escalated/total. Rates in Go.
  - Orchestrator `func (r *Repo) ATSReport(ctx, scope, f, requiredDocs) (ATSReport, error)` runs them sequentially, sets From/To/Scope.
- **MIRROR**: `reports.go` query execution + error wrapping (`fmt.Errorf("reports: ...: %w", err)`).
- **GOTCHA**: nullable aggregates (`AVG` over empty set → NULL) → scan into `*float64`, default 0. Append scope args after the per-query fixed args; `ApplicationsClause(argStart)` must use the correct starting index per query (funnel=3, onboarding after `$docTypes,$count`). Keep each query ≤ ~25 lines.
- **VALIDATE**: `go build ./internal/reports/`; `go vet`.

### Task 4: CSV flatten — `ats_report_csv.go`
- **ACTION**: `func EncodeATSCSV(rep ATSReport) ([]byte, error)` → `section,metric,value` rows (funnel stages + each section's metrics; percentages as `"82.0"`, days as `"23.4"`).
- **MIRROR**: `export.go:49-77` (`encoding/csv`).
- **VALIDATE**: covered by Task 7 test (valid CSV, has header + known rows).

### Task 5: Handler — `ats_report_handler.go`
- **ACTION**: Add an HR-facing handler + routes.
- **IMPLEMENT**:
  - `type atsReportStore interface { ATSReport(ctx, scope rbac.Scope, f ATSFilter, requiredDocs []string) (ATSReport, error) }` (narrow; `*Repo` satisfies — enables a fake in tests).
  - `ATSReportHandler{ repo atsReportStore; requiredDocs []string }` + `NewATSReportHandler(repo, requiredDocs)`.
  - `var reportViewRoles = map[string]bool{super_admin, regional_director, operation_director, auditor, sgm, hr_manager, hr_staff}` + `canViewReport(role)`.
  - `RegisterATSRoutes(app, h)` → `GET /api/v1/reports/ats` (JSON), `GET /api/v1/reports/ats.csv` (CSV).
  - `parseRange(fromQ, toQ string) (time.Time, time.Time, error)` — accept RFC3339 or `2006-01-02`; default to=now, from=now-90d; `to<=from` or unparseable → error.
  - Both handlers: read `middleware.DevUser` from locals → role gate (403) → `scope := rbac.New(...)` → parse range (400) → `rep, err := h.repo.ATSReport(...)`. JSON → `httpx.OK(c, rep)`. CSV → `EncodeATSCSV` → `c.Set("Content-Type","text/csv; charset=utf-8")`, `c.Set("Content-Disposition","attachment; filename=ats-report.csv")`, `c.Send(b)`.
  - `scopeLabel(scope)` → "Company"/"Subregion: X"/"Store <id>" for `rep.Scope`.
- **MIRROR**: `reports/handler.go` (role read + route group) + `executive/handler.go` (role gate) + `letter_handler` (sentinel→HTTP not needed here; mostly 400/403).
- **GOTCHA**: reports package must import `internal/rbac` + `internal/middleware` (handler.go likely already imports middleware for the export gate — confirm). Don't break the existing `RegisterRoutes`.
- **VALIDATE**: `go build ./internal/reports/ ./cmd/api/`.

### Task 6: Wire in `cmd/api/main.go`
- **ACTION**: Construct + register the ATS report handler near the existing `reports.RegisterRoutes` (~line 359).
- **IMPLEMENT**: `reports.RegisterATSRoutes(app, reports.NewATSReportHandler(reportsRepo, cfg.OnboardingRequiredDocs()))` (reuse the existing `reports.New(pool)` repo instance if one is already built there; else build it).
- **GOTCHA**: ATS routes sit after `app.Use(authMW)` (HR auth) like the other `/api/v1/reports/*` routes.
- **VALIDATE**: `go build ./cmd/api/`.

### Task 7: Backend tests — `ats_report_test.go`
- **ACTION**: Handler + pure-function tests (no DB).
- **IMPLEMENT**:
  - `fakeATSStore` implementing `atsReportStore` returning a canned `ATSReport`.
  - fiber `app.Test` (mirror `offer_test.go` setup with `middleware.DevUser` locals): role gate (recruiter→403 / hr_manager→200), bad date (`from=zzz`→400), `to<=from`→400, JSON 200 shape, CSV 200 + `Content-Type: text/csv` + body starts with `section,metric,value`.
  - Pure: `EncodeATSCSV` (header + a known row), conversion helper (e.g. `pct(part,total)` divide-by-zero→0), `parseRange` defaults + errors.
- **MIRROR**: `internal/applications/offer_test.go` test harness; package-local fakes.
- **GOTCHA**: the SQL itself isn't unit-testable without a DB (Docker disk-full) — note it; aggregation correctness verified by operator on staging + the existing `reports_integration_test.go` style (DB-gated).
- **VALIDATE**: `go test ./internal/reports/ -race`.

### Task 8: Frontend types + hook
- **ACTION**: `frontend/lib/types.ts` + `lib/queries.ts`.
- **IMPLEMENT**: `AtsReport` + sub-types mirroring the Go JSON (snake_case). `useAtsReport(from?: string, to?: string)` → `GET /api/v1/reports/ats${buildQuery({from,to})}` (queryKey `["ats-report", from, to]`, `enabled` always, `retry:false`; 403→null like other gated hooks).
- **MIRROR**: `useFunnel`/`useExecutiveOverview` (queries.ts).
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 9: Reports page + panels
- **ACTION**: `app/(app)/reports/page.tsx` + `components/reports/ReportSections.tsx`.
- **IMPLEMENT**:
  - Page: `"use client"`; `useMe` + `canViewReports(me?.role)` gate (render `notAvailable` panel for disallowed, mirror executive); `useTranslations("reports")`; date-range local state (`from`/`to`, default last 90d as `YYYY-MM-DD`); `<PageHeader eyebrow title>`; date inputs + `Export CSV` button (`downloadFile` + toast on fail, mirror Members); render the 4 panels from `data`.
  - `ReportSections.tsx`: `FunnelPanel` (stages with count + conversion %, tapered bars / OKLCH ramp like `FunnelChart`), `TimingPanel` (stat numbers: avg/median to-hire + stage timings, `tabular-nums`, `--text-stat`), `OutcomesPanel` (offer accept/decline %, onboarding completion + doc rejection), `QualityPanel` (interview pass% + avg rating, approval cycle + SLA breach%). All in `rounded-xl bg-card p-6 ring-1 ring-hairline` shells with `eyebrow` labels; colors via brand/score-ramp CSS vars (no emerald).
- **MIRROR**: `analytics/page.tsx`, `Charts.tsx` (FunnelChart ramp + KpiCards), `members/page.tsx` (CSV button), `executive/page.tsx` (role gate).
- **GOTCHA**: empty/zero state per panel (N=0 → "no data in range"). `mutate` not relevant (read-only). Use `useTranslations` (client). Format days `x.toFixed(1)` + unit, percentages `Math.round`.
- **VALIDATE**: `pnpm exec tsc --noEmit && pnpm exec eslint app components lib`.

### Task 10: roles + nav
- **ACTION**: `lib/roles.ts` + `components/shell/nav-config.tsx`.
- **IMPLEMENT**: `REPORT_VIEW_ROLES` (mirror backend `reportViewRoles`) + `canViewReports(role)`. `REPORTS_NAV = {href:"/reports", label:"Reports", key:"reports", icon: BarChart3}`; push in `navForRole` when `canViewReports`; add to `ALL_NAV`.
- **MIRROR**: `canViewExecutive` + `EXECUTIVE_NAV` registration.
- **VALIDATE**: `pnpm exec tsc --noEmit`.

### Task 11: i18n
- **ACTION**: add `reports` namespace + `nav.reports` to `frontend/messages/{en,th}.json`.
- **IMPLEMENT** keys: `eyebrow,title,meta,notAvailable,notAvailableHint,from,to,exportCsv,exportFailed,scopeCompany,scopeStore,scopeSubregion,noData`, funnel stage labels `stage_applied/screened/interview/offer/hired`, timing `avgToHire,medianToHire,toScreen,toOffer,offerResponse,days`, outcomes `offersSent,acceptRate,declineRate,onboardingComplete,docRejection`, quality `interviewPass,avgRating,approvalCycle,slaBreach`, plus `nav.reports`. Identical dotted key set th↔en.
- **MIRROR**: `executive`/`onboarding` namespaces.
- **GOTCHA**: career-portal NOT touched (parity is per-app); only frontend th/en.
- **VALIDATE**: `node scripts/check-i18n-parity.mjs`.

### Task 12: Full validation sweep
- **VALIDATE**: see Validation Commands.

---

## Testing Strategy

### Unit / Handler Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Role gate deny | role recruiter | 403 | yes |
| Role gate allow (store) | hr_manager | 200, scope=store | no |
| Bad date | `from=zzz` | 400 | yes |
| Inverted range | `to<=from` | 400 | yes |
| JSON shape | hr_manager | 200 + ats_report json | no |
| CSV | `…/ats.csv` | 200 + `text/csv` + `section,metric,value` header | no |
| EncodeATSCSV | canned report | header + known funnel/offer rows | yes |
| pct() | (0,0) | 0 (no divide-by-zero) | yes |
| parseRange default | empty from/to | now-90d … now | yes |

### Edge Cases Checklist
- [ ] Empty period (N=0) → all rates 0, no divide-by-zero, panels show "no data"
- [ ] Store role → scoped to own store; all-role → unscoped
- [ ] Bad / inverted date range → 400
- [ ] Null aggregates (AVG over empty) → 0
- [ ] CSV content-type + attachment filename

---

## Validation Commands

### Backend
```bash
cd backend && go build ./... && go vet ./... && gofmt -l internal/reports cmd/api
go test ./internal/reports/ -race
```
EXPECT: clean; tests pass. (DB-backed aggregation SQL validated by operator on staging — local Docker Postgres disk-full.)

### Dashboard
```bash
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
```
EXPECT: zero type errors; eslint clean except pre-existing (`components/shell/AppHeader.tsx`, `LocaleSwitcher.tsx`); build green.

### i18n parity
```bash
node scripts/check-i18n-parity.mjs
```
EXPECT: exit 0.

### Manual Validation (post-deploy, operator)
- [ ] `/reports` visible to HR roles, hidden from others; date range changes numbers
- [ ] store HR sees only their store's metrics; leadership sees company
- [ ] Export CSV downloads `ats-report.csv` with the displayed numbers
- [ ] funnel conversion %, time-to-hire, offer accept rate, onboarding completion, interview pass, approval SLA all populate on real data

---

## Acceptance Criteria
- [ ] `GET /api/v1/reports/ats` + `/ats.csv` — role-gated, RBAC-scoped, date-ranged, all 4 metric families
- [ ] `/reports` page: date range + 4 panels + CSV export, role-gated nav
- [ ] All validation commands pass; tests written + passing; no type/lint errors (beyond pre-existing); i18n parity green
- [ ] No migration; no career-portal change; existing analytics/executive untouched

## Completion Checklist
- [ ] Every aggregation applies `rbac.Scope` (store/subregion/all)
- [ ] Divide-by-zero guarded; null aggregates → 0; values rounded
- [ ] Reuses reports package + dashboard chart/CSS conventions (no emerald, no new chart lib)
- [ ] Typed status/stage maps (no `as`); `useTranslations` (client)
- [ ] Self-contained — no questions during implementation

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Aggregation SQL can't be unit-tested locally (Docker disk-full) | High | Medium | Handler/pure tests cover wiring; operator validates SQL on staging; keep queries simple + reviewed |
| `ApplicationsClause` arg-index integration bugs | Medium | Medium | Centralize arg-building per query; test scope=all (no clause) vs store in handler tests via fake |
| Reached-funnel via event flags ≠ "current status" intuition | Medium | Low | Label stages clearly; document the event-flag definition in code + report meta |
| Onboarding completion needs config required set | Low | Low | Pass `cfg.OnboardingRequiredDocs()` through handler (already exists) |
| Scope creep (4 families is large) | Medium | Medium | One read-only page, no migration; panels are simple stat cards; defer trends/charts |

## Notes
- **Slice independence:** fresh off `main` (f067003) — single branch `feat/ats-reports`, one PR (no stacking).
- **Deploy (operator):** rebuild/roll **api** (new read endpoints) + **dashboard** (5 Entra build-args). **No migration, no worker/scheduler, no career-portal.** Nothing to set — works on existing data immediately.
- **This is the last Module-3 slice** (after 3.4/3.1, 3.5, 3.6, 3.3, 3.8).
- **Recurring lessons:** typed maps not `as`; round/guard rates; `useTranslations` for client chrome; reuse `downloadFile` for sync CSV; keep funnel definitions documented.
