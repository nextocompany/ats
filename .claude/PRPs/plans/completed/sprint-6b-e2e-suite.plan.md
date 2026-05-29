# Plan: Sprint 6b — Cross-System E2E Suite + CI

## Summary
A comprehensive **end-to-end test suite** that exercises the whole product as one system — PeopleSoft vacancy webhook → career-portal apply → async pipeline (OCR→parse→dedup→score→assign) → HR dashboard review → hire → PeopleSoft sync → re-engagement → report export → candidate search — running fully offline against the deterministic mocks, and wired into **CI** with the docker stack + seeded data. Closes the gap between today's per-surface tests (dashboard Playwright, portal Playwright, package-level Go integration tests) and a real cross-system guarantee.

## User Story
As the **team preparing for go-live**, I want **one automated suite that proves the full recruitment flow works across services**, so that **a regression in any handoff (webhook→portal→pipeline→dashboard→PS) is caught before release**.

## Problem → Solution
**Current state:** Strong but siloed tests — `frontend/e2e` (dashboard), `career-portal/e2e` (portal apply), and `-tags integration` package tests (pipeline, list, reports, search, reengage). Nothing drives a single application all the way through the system. CI (`.github/workflows/ci.yml`) runs Go unit + vet + lint + migrate round-trip only — **no integration tests, no Playwright, no stack**.
**Desired state:** A `backend/e2e/` cross-system suite (Go, `-tags e2e`) that runs against the live docker stack and chains the real HTTP contracts end-to-end, plus the existing Playwright suites run in CI against the stack. One `make e2e` and a CI job that boots the stack, seeds, and runs all of it.

## Metadata
- **Complexity**: Large (new cross-system suite + CI orchestration; ~10 files, mostly tests + CI)
- **Source PRD**: Nexto PRP v1.0 — Sprint 6 (E2E); roadmap §20 (S6–7)
- **Decisions locked**: cross-system flow tested in **Go** (`backend/e2e`, `//go:build e2e`) hitting the live API over HTTP (deterministic, no browser flake); existing Playwright suites kept and run in CI against the stack; all external integrations stay **mock** so the suite is fully offline
- **Estimated Files**: ~10
- **Depends on**: nothing functionally; runs best after 6a (so it also exercises auth=mock + rate limits), but independent

---

## UX Design
Internal/test + CI change — no user-facing UX.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Cross-system coverage | none | `backend/e2e` full-flow suite | Go over HTTP |
| CI | Go unit + migrate only | + stack boot, integration tests, e2e, Playwright | gated on PRs |
| Local run | per-surface manual | `make e2e` (one command) | boots stack + seeds + runs |

---

## Mandatory Reading (the contract + patterns to reuse)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/pipeline/process_integration_test.go` | all | the closest existing cross-system test (seed→process→assert); reuse the fixture/seed/poll patterns |
| P0 | `backend/internal/applications/list_integration_test.go` | 1-55 | the `dsn()` helper + `TRUNCATE … RESTART IDENTITY CASCADE` + seed pattern every integration test shares |
| P0 | `docker-compose.yml` | all | services (postgres/redis/azurite/api/worker/scheduler) + internal hostnames + env the e2e relies on |
| P0 | `Makefile` | all | `up`/`migrate-up`/`seed`/`test`/`test-integration` targets to compose into `make e2e` |
| P0 | `.github/workflows/ci.yml` | all | the Go-only CI to extend (add stack + integration + e2e + Playwright jobs) |
| P0 | (routes — the HTTP contract to chain) | — | see the route table below |
| P1 | `backend/internal/ai/mock.go` | 8-50 | deterministic OCR/parse output (สมชาย ใจดี / 0812345678 / ปวส. / 24-mo cashier) the asserts key on |
| P1 | `backend/internal/auth/line.go` | 37-45 | mock LINE verifier accepts any non-empty `X-LINE-IdToken` — apply call uses a stub token |
| P1 | `backend/internal/peoplesoft/mock.go` | 9-20 | mock PS sync logs, no external call — hire→sync works offline |
| P1 | `career-portal/e2e/portal.spec.ts` | all | existing portal apply Playwright flow to keep + run in CI |
| P1 | `frontend/e2e/dashboard.spec.ts` | all | dashboard Playwright; auth via `hr_session=dev` cookie (bypasses `/login`) |
| P2 | `backend/internal/applications/model.go` | 11-19 | status constants (`pending/parsed/scored/rejected/hired`) the e2e polls/asserts |
| P2 | `backend/internal/pipeline/process.go` | 65-220 | how the worker processes the task (so the e2e knows what to poll for) |

### HTTP contract to chain (from routes files)
```
POST /api/v1/ps/vacancy-opened            (peoplesoft)  → opens a vacancy (maps position; fires reengage)
GET  /api/v1/public/positions             (public)      → portal sees the open position
POST /api/v1/public/apply                 (public)      → multipart apply; X-LINE-IdToken stub → {status_token}
GET  /api/v1/public/status/:token         (public)      → candidate status by token
GET  /api/v1/applications                 (dashboard)   → ranked/scoped/paginated inbox
PATCH/api/v1/applications/:id/status       (applications)→ set "hired" → triggers PS sync
POST /api/v1/applications/bulk            (dashboard)   → bulk status/reject
POST /api/v1/positions/:id/reengage       (reengage)    → enqueue re-engagement
GET  /api/v1/reports/funnel|kpi|sources   (reports)     → analytics reflect the flow
POST /api/v1/reports/exports              (reports)     → on-demand export
GET  /api/v1/candidates/search?q=         (search)      → candidate findable post-pipeline
```

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Playwright in CI | playwright.dev/docs/ci | install browsers with `pnpm exec playwright install --with-deps chromium`; run against a started server |
| GitHub Actions services vs compose | docs.github.com/actions | the repo already uses a `postgres` service in CI; for the full stack, `docker compose up -d --build` in a job step is simplest and matches local |
| asynq processing latency | hibiken/asynq | tasks are async — the e2e must **poll** application status (not assume immediate), with a bounded timeout |

### Research Notes
```
KEY_INSIGHT: the flow is async (apply → asynq → worker pipeline).
APPLIES_TO: backend/e2e.
GOTCHA: after POST /apply, poll GET /status/:token (or GET /applications) until status leaves "pending"/"parsed", with a deadline (~30s) and interval (~500ms). Do NOT assert synchronously.

KEY_INSIGHT: mocks make the whole flow deterministic + offline.
APPLIES_TO: whole suite.
GOTCHA: keep AI_PROVIDER/PS_PROVIDER/LINE_PROVIDER/NOTIFY_PROVIDER/AI_SEARCH_PROVIDER = mock (defaults). The mock parser always yields สมชาย ใจดี / 0812345678 → repeated applies DEDUP-merge; the e2e must account for that (use distinct positions or assert the merge, don't expect N distinct candidates).

KEY_INSIGHT: dashboard Playwright bypasses login via a cookie.
APPLIES_TO: CI Playwright.
GOTCHA: tests set `hr_session=dev` (SESSION_COOKIE) to pass middleware.ts; replicate in CI. Career-portal has no auth gate.

KEY_INSIGHT: hire→PS sync is in the status PATCH path.
APPLIES_TO: e2e hire step.
GOTCHA: PATCH /applications/:id/status {status:"hired"} sets hired_at + calls mock PS sync (logged, never fails the hire). Assert status=hired; PS sync success is mock-logged (assert via /ps/health provider=mock or the app's ps_synced flag if exposed).
```

---

## Patterns to Mirror

### E2E_HARNESS (Go, build-tagged; mirror integration `dsn()` + a base URL)
```go
//go:build e2e
package e2e
func apiBase() string { if v := os.Getenv("E2E_API_URL"); v != "" { return v }; return "http://localhost:8080" }
func dsn() string { /* same as integration tests */ }
// helpers: postJSON, postMultipart(resume), getJSON[T], pollStatus(token, until, deadline)
```

### POLL (async processing wait)
```go
func pollUntil(t *testing.T, deadline time.Duration, fn func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn() { return }
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("e2e: condition not met before deadline")
}
```

### MULTIPART_APPLY (mirror career-portal/lib/queries buildApplyForm + public handler fields)
```go
// fields: position_id, full_name, consent_given=true, consent_version, resume (pdf bytes)
// header: X-LINE-IdToken: e2e-stub
```

### CI_JOB (extend .github/workflows/ci.yml)
```yaml
e2e:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - run: docker compose up -d --build
    - run: <wait for api /health>
    - run: make migrate-up && make seed
    - run: cd backend && go test -tags e2e ./e2e/...
    - run: cd career-portal && pnpm i && pnpm exec playwright install --with-deps chromium && pnpm build && pnpm start & ... && pnpm exec playwright test
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/e2e/harness_test.go` | CREATE | shared helpers: api base, http JSON/multipart, poll, db reset/seed |
| `backend/e2e/full_flow_test.go` | CREATE | the headline test: vacancy-opened → apply → pipeline → inbox → hire → PS sync |
| `backend/e2e/reengage_flow_test.go` | CREATE | rejected/talent-pool candidate + vacancy-opened/manual trigger → contact recorded |
| `backend/e2e/reports_search_flow_test.go` | CREATE | after hires: funnel/kpi reflect counts; on-demand export row; search finds the candidate |
| `Makefile` | UPDATE | `e2e` target (up + migrate + seed + `go test -tags e2e` + Playwright) |
| `.github/workflows/ci.yml` | UPDATE | add `integration` + `e2e` jobs (boot stack) and a `playwright` job (both web apps) |
| `backend/e2e/README.md` | CREATE | how to run locally; what each flow asserts; mock determinism notes |
| `frontend/e2e/dashboard.spec.ts` | UPDATE (if needed) | ensure CI-stable (cookie auth, deterministic waits) |
| `career-portal/e2e/portal.spec.ts` | UPDATE (if needed) | ensure CI-stable against the booted stack |
| `frontend/playwright.config.ts` / `career-portal/playwright.config.ts` | UPDATE (if needed) | `webServer` block so Playwright boots the app in CI |

## NOT Building (later / out of scope)
- **Real external integrations** in the suite (Azure/PS/LINE stay mock — the point is deterministic offline coverage).
- **Load/performance/soak testing** (functional E2E only).
- **Visual regression baselines** beyond the existing portal screenshots.
- **Contract tests against a real PeopleSoft** (mock webhook payloads only).
- **A second browser matrix** (chromium only, matching the cached-browser constraint).
- **Rewriting the package-level `-tags integration` tests** — they stay; e2e complements them.

---

## Step-by-Step Tasks

### Task 1: E2E harness
- **ACTION**: `backend/e2e/harness_test.go` (`//go:build e2e`): `apiBase()`, `dsn()`, `httpJSON`/`postMultipart`/`getInto[T]`, `pollUntil`, and a `resetAndSeed(t)` that TRUNCATEs + seeds a store/position/vacancy (reuse integration seed SQL).
- **MIRROR**: integration `dsn()` + TRUNCATE/seed; POLL pattern.
- **GOTCHA**: e2e talks to the API over HTTP (not the repo directly) AND to the DB for setup/asserts. Use `E2E_API_URL` (default :8080).
- **VALIDATE**: `go test -tags e2e ./backend/e2e -run TestHarness` (a trivial /health ping) passes against `make up`.

### Task 2: Full-flow test
- **ACTION**: `full_flow_test.go`: (1) POST `/ps/vacancy-opened` (maps a seeded position) → 200; (2) GET `/public/positions` shows it; (3) POST `/public/apply` (multipart, stub LINE token) → `status_token`; (4) poll `/public/status/:token` until status ∈ {scored,rejected}; (5) GET `/applications` (super_admin) shows the ranked app; (6) PATCH `/applications/:id/status` {hired} → 200; (7) assert hired (and PS sync mock-logged / ps health provider=mock).
- **MIRROR**: HTTP contract table; MULTIPART_APPLY; mock determinism (assert สมชาย ใจดี).
- **GOTCHA**: async — poll, don't sleep-once. The mock scorer’s outcome (scored vs rejected) depends on the seeded position's must-have; seed a lenient position so it scores+assigns.
- **VALIDATE**: `go test -tags e2e ./backend/e2e -run TestFullFlow` green against the stack.

### Task 3: Re-engagement flow
- **ACTION**: `reengage_flow_test.go`: seed a rejected/talent-pool applicant for a position; POST `/positions/:id/reengage` → 202/201; assert a `reengagement_contacts` row (DB) + worker mock-notify (poll for the row). Also assert the vacancy-opened webhook path enqueues re-engagement.
- **MIRROR**: 5a behavior; POLL for the async worker effect.
- **VALIDATE**: `-run TestReengage` green.

### Task 4: Reports + search flow
- **ACTION**: `reports_search_flow_test.go`: after a hire, GET `/reports/funnel` + `/kpi` reflect ≥1 applied/hired; POST `/reports/exports` → 201 + a `report_exports` row; GET `/candidates/search?q=สมชาย` finds the candidate (scoped, super_admin).
- **MIRROR**: 5b/5c behavior + envelope assertions.
- **VALIDATE**: `-run TestReportsSearch` green.

### Task 5: Makefile + local runner
- **ACTION**: `make e2e`: `up` (with `--build`) → wait `/health` → `migrate-up` → `seed` → `go test -tags e2e ./e2e/...`. Add a `backend/e2e/README.md`.
- **MIRROR**: existing Makefile targets.
- **GOTCHA**: wait-for-health loop before tests (api boot is async). Tests reset their own data, but `seed` provides baseline reference rows (stores/positions/vacancies).
- **VALIDATE**: `make e2e` runs end-to-end locally.

### Task 6: CI — stack + integration + e2e + Playwright
- **ACTION**: Extend `.github/workflows/ci.yml`: keep the existing unit job; add an `integration` job and an `e2e` job that `docker compose up -d --build`, wait for health, `make migrate-up && make seed`, run `go test -tags integration ./...` and `go test -tags e2e ./e2e/...`; add a `playwright` job that builds + starts each web app and runs its Playwright suite (chromium, `--with-deps`). Add `webServer` to the Playwright configs so they self-start in CI.
- **MIRROR**: CI_JOB; existing CI service/step style.
- **GOTCHA**: cache pnpm + go; install only chromium; the dashboard Playwright needs the `hr_session=dev` cookie (already in its spec) and the API up; bound job timeouts.
- **VALIDATE**: push the branch → CI green (or run `act`/inspect logs); all jobs pass.

---

## Testing Strategy
This sprint **is** tests. "Unit" = the e2e helpers compile/behave; the deliverable is the flows below.

### Flow coverage
| Flow | Asserts |
|---|---|
| Full flow | vacancy open → portal apply → pipeline scores/assigns → inbox shows → hire → PS sync (mock) |
| Re-engagement | trigger → contact row + mock notify; webhook enqueues |
| Reports | funnel/kpi reflect counts; export row created |
| Search | hired candidate findable, scope-correct |

### Edge Cases Checklist
- [ ] Async wait bounded (no infinite poll; clear failure on timeout)
- [ ] Duplicate apply (mock parser → same person) dedup-merges (don't expect 2 candidates)
- [ ] Consent missing on apply → 400 (negative case)
- [ ] Rate limiting (if 6a merged) doesn't break the e2e (stay under cap or expect 429 deliberately)
- [ ] CI stack health-gated before tests run
- [ ] Tests reset DB state between flows (no cross-test bleed)

## Validation Commands
### Local e2e (stack up)
```bash
make e2e
# or manually:
make up && make migrate-up && make seed
cd backend && go test -tags e2e ./e2e/... -count=1
```
### Playwright (both apps, stack up)
```bash
cd career-portal && pnpm exec playwright test
cd frontend && pnpm exec playwright test
```
### Existing suites still green
```bash
cd backend && go test -race ./... && go test -tags integration ./...
```
### CI
```bash
# push branch; confirm unit + integration + e2e + playwright jobs all green
```

## Acceptance Criteria
- [ ] `backend/e2e` suite chains vacancy→apply→pipeline→inbox→hire→PS sync, plus reengage, reports, and search flows — all green against the live stack, fully offline (mocks).
- [ ] `make e2e` boots the stack, seeds, and runs the suite in one command.
- [ ] CI extended: stack-backed integration + e2e jobs + Playwright (both web apps) — all green.
- [ ] Async steps poll with bounded deadlines (no flaky sleeps); existing unit/integration/Playwright suites still pass.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Async flakiness | Med | High | bounded polling helpers, deterministic mocks, generous deadlines |
| CI wall-clock/cost (stack + browsers) | Med | Med | cache go/pnpm, chromium-only, parallel jobs, timeouts |
| Mock dedup surprises (same person) | Med | Med | seed distinct positions / assert the merge explicitly |
| 6a auth/rate-limit interferes | Low | Med | e2e runs with AUTH_PROVIDER=mock; stay under rate cap |
| Playwright self-start in CI | Med | Med | add `webServer` to configs; health-gate |

## Notes
- Go-over-HTTP for the cross-system flow (not browser) keeps the system-level assertions deterministic; Playwright continues to own the UI-level coverage.
- This is the regression safety net for S7/S8 (go-live) — every prior sprint's contract is exercised once here.
