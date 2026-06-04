# Plan: Playwright Browser E2E in CI

## Summary
The two Next.js apps already ship Playwright specs (`frontend/e2e/`, `career-portal/e2e/`) and `@playwright/test`, but nothing runs them — there's no `test:e2e` script, the configs assume a server is already up, and CI's `e2e` job only runs Go tests. This adds a `webServer` block to each `playwright.config.ts` so Playwright self-starts the Next app, a `test:e2e` script to each app, and a dedicated `playwright` CI job that boots the Go stack + seeds, then builds and runs both browser suites with failure artifacts uploaded.

## User Story
As a **developer changing the dashboard or career-portal UI**, I want **the existing Playwright specs to run automatically in CI against a real build + live backend**, so that **front-end regressions (broken inbox, apply flow, PWA manifest, responsive breakpoints) are caught on every PR instead of only when someone remembers to run them locally**.

## Problem → Solution
**Current state:** `frontend/playwright.config.ts` and `career-portal/playwright.config.ts` exist with good specs, but: (1) neither has a `webServer`, so they require a manually-started Next server; (2) neither `package.json` has a `test:e2e` script; (3) `.github/workflows/ci.yml`'s `e2e` job runs only Go integration + Go cross-system tests — the Playwright specs never execute in CI. Front-end regressions ship unguarded.
**Desired state:** `playwright test` self-starts the app (prod build in CI, `next dev` locally), and a `playwright` CI job boots the same Go stack the `e2e` job uses, seeds it, builds each Next app, runs its Playwright suite, and uploads screenshots/traces on failure.

## Metadata
- **Complexity**: Medium
- **Source PRD**: N/A (free-form — final Sprint 7 slice from session `2026-06-03-s7-ps-hmac`)
- **PRD Phase**: N/A (standalone)
- **Estimated Files**: 5 (0 new source, 5 updated: 2 configs, 2 package.json, 1 workflow)

---

## UX Design
N/A — CI/test-infra change. No product UX. Developer-facing outcome: `pnpm test:e2e` works locally with zero manual server setup, and PRs get a `playwright` check.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| `pnpm test:e2e` | does not exist | runs Playwright, self-starting the app | new script |
| Local run | must `next dev` + stack first | webServer auto-starts `pnpm dev` (reuses a running one) | `reuseExistingServer: !CI` |
| CI | Playwright specs never run | new `playwright` job runs both suites on a prod build + live stack | parallel to `e2e` |
| CI failure | n/a | screenshots + traces uploaded as an artifact | debuggable |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 (critical) | `.github/workflows/ci.yml` | 61-89 (`e2e` job) + 108-121 (pnpm-in-`security`) | Exact stack-boot/migrate/seed steps to mirror, and the established `corepack prepare pnpm@9.15.9` + `pnpm install --frozen-lockfile` idiom |
| P0 (critical) | `frontend/playwright.config.ts` | 1-15 | Where to add `webServer`; baseURL/`E2E_BASE_URL` + chromium project already set |
| P0 (critical) | `career-portal/playwright.config.ts` | 1-25 | Same; note 3 mobile projects + the SW-needs-prod-build comment |
| P1 (important) | `frontend/package.json` | scripts + devDeps | `dev`/`build`/`start` scripts; `@playwright/test ^1.60.0` present; add `test:e2e` |
| P1 (important) | `career-portal/package.json` | scripts + devDeps | `dev`/`build`/`start` use `-p 3001`/`--webpack`; serwist needs the prod build |
| P1 (important) | `career-portal/e2e/pwa.spec.ts` | 1-90 | SW test self-skips unless `/sw.js` is served → requires `pnpm build && pnpm start` in CI |
| P2 (reference) | `frontend/e2e/dashboard.spec.ts` | 1-60 | Needs the Go API up + `hr_session=dev` cookie; data-dependent tests self-skip on empty |
| P2 (reference) | `career-portal/e2e/portal.spec.ts` | 1-70 | Apply/status flow needs seeded open positions (`make seed`) + the mock LINE stub |
| P2 (reference) | `frontend/lib/api.ts` / `career-portal/lib/api.ts` | 6 | API base = `NEXT_PUBLIC_API_URL ?? http://localhost:8080` → default matches the stack; no env needed in CI |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Playwright `webServer` | Playwright Test config | `webServer.command` is auto-run before tests; Playwright waits for `url`; `reuseExistingServer` skips startup if something already serves it. Multiple servers allowed but here one per config (one port each). |
| `webServer` + CI build | Playwright docs | Keep `command: "pnpm start"` (serve a prebuilt `.next`) and run `pnpm build` as a separate CI step, so build time isn't bound by the webServer `timeout` and build errors surface independently. |
| GitHub Actions `CI` env | Actions | `CI=true` is set automatically on runners → drives `process.env.CI` branches in the config (prod `pnpm start` vs local `pnpm dev`). |
| `playwright install --with-deps` | Playwright CLI | Installs the browser **and** its OS deps via apt on ubuntu runners; we only need `chromium`. |

```
KEY_INSIGHT: docker-compose has NO frontend services (only postgres/redis/azurite/api/worker/scheduler).
APPLIES_TO: the whole approach — Playwright MUST start the Next apps; CI only gets the Go stack from compose.
GOTCHA: the dashboard/portal specs ALSO need the Go API + seed; the new job must boot+migrate+seed the stack like the e2e job, not just start the Next server.

KEY_INSIGHT: career-portal's service worker only generates on a prod build (`next dev` disables Serwist).
APPLIES_TO: CI build step + webServer command.
GOTCHA: run `pnpm build && pnpm start` (prod) in CI so pwa.spec's SW assertion actually exercises; otherwise it self-skips.

KEY_INSIGHT: NEXT_PUBLIC_* is baked at build time.
APPLIES_TO: the build step.
GOTCHA: the default (http://localhost:8080) already matches the stack, so no env is required; do NOT hardcode a different API URL into the build.
```

---

## Patterns to Mirror

### CI_STACK_BOOT (from the e2e job)
```yaml
# SOURCE: .github/workflows/ci.yml:69-76
- name: Boot stack
  run: docker compose up -d --build
- name: Wait for API health
  run: for i in $(seq 1 60); do curl -sf http://localhost:8080/health && break; sleep 2; done
- name: Migrate + seed
  run: |
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
    make migrate-up && make seed
```

### CI_PNPM_SETUP (from the security job)
```yaml
# SOURCE: .github/workflows/ci.yml:108-114
- name: pnpm audit (frontend)
  working-directory: frontend
  run: |
    corepack prepare pnpm@9.15.9 --activate
    pnpm install --frozen-lockfile
```

### PLAYWRIGHT_CONFIG (current, to extend)
```ts
// SOURCE: frontend/playwright.config.ts:1-15
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  retries: 0,
  reporter: [["list"]],
  use: { baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000", trace: "on-first-retry", screenshot: "only-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `frontend/playwright.config.ts` | UPDATE | Add `webServer` (self-start) + CI retries |
| `career-portal/playwright.config.ts` | UPDATE | Add `webServer` (port 3001) + CI retries |
| `frontend/package.json` | UPDATE | Add `"test:e2e": "playwright test"` |
| `career-portal/package.json` | UPDATE | Add `"test:e2e": "playwright test"` |
| `.github/workflows/ci.yml` | UPDATE | New `playwright` job: boot stack → seed → build+run both suites → upload artifacts |

## NOT Building

- **New Playwright specs** — the existing specs are the coverage; this slice only makes them run. Writing more tests is separate.
- **Browser-download caching** in CI — `playwright install chromium` each run is acceptable; a cache is a later optimization.
- **Webkit/Firefox** — configs are chromium-only (the one browser cached/needed); keep it.
- **Visual-regression baselines / screenshot diffing** — specs capture screenshots for debugging, not pixel-diff assertions; no baseline store.
- **Adding the frontends to docker-compose** — Playwright's `webServer` is the lighter, standard way to serve them for tests.
- **Running Playwright inside the existing Go `e2e` job** — a dedicated job keeps Go vs browser failures independent and parallel.

---

## Step-by-Step Tasks

### Task 1: `webServer` for the dashboard config
- **ACTION**: Edit `frontend/playwright.config.ts`.
- **IMPLEMENT**: add to the `defineConfig({...})` object:
  ```ts
  retries: process.env.CI ? 1 : 0,
  // ...
  webServer: {
    command: process.env.CI ? "pnpm start" : "pnpm dev",
    url: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  ```
  (Replace the existing `retries: 0` with the CI-aware line; add the `webServer` block.)
- **MIRROR**: PLAYWRIGHT_CONFIG.
- **GOTCHA**: `pnpm start` serves a prebuilt `.next` — CI must run `pnpm build` first (Task 5). Locally `pnpm dev` auto-starts and `reuseExistingServer` reuses a running dev server.
- **VALIDATE**: `cd frontend && pnpm exec tsc --noEmit -p tsconfig.json` (config is TS) or `pnpm exec playwright test --list` (parses the config).

### Task 2: `webServer` for the career-portal config
- **ACTION**: Edit `career-portal/playwright.config.ts`.
- **IMPLEMENT**: same shape, port 3001:
  ```ts
  retries: process.env.CI ? 1 : 0,
  // ...
  webServer: {
    command: process.env.CI ? "pnpm start" : "pnpm dev",
    url: process.env.E2E_BASE_URL ?? "http://localhost:3001",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  ```
- **MIRROR**: PLAYWRIGHT_CONFIG + the portal's `-p 3001`/`--webpack` scripts.
- **GOTCHA**: `pnpm start` = `next start -p 3001` (prod) so the Serwist SW is generated → pwa.spec's SW test actually runs (it self-skips otherwise). `pnpm dev` = `next dev -p 3001 --webpack` locally.
- **VALIDATE**: `cd career-portal && pnpm exec playwright test --list`.

### Task 3: `test:e2e` script — dashboard
- **ACTION**: Edit `frontend/package.json` `scripts`.
- **IMPLEMENT**: add `"test:e2e": "playwright test"`.
- **GOTCHA**: keep valid JSON (comma placement); don't reorder existing scripts.
- **VALIDATE**: `cd frontend && pnpm run` lists `test:e2e`.

### Task 4: `test:e2e` script — career-portal
- **ACTION**: Edit `career-portal/package.json` `scripts`.
- **IMPLEMENT**: add `"test:e2e": "playwright test"`.
- **VALIDATE**: `cd career-portal && pnpm run` lists `test:e2e`.

### Task 5: `playwright` CI job
- **ACTION**: Add a job to `.github/workflows/ci.yml` (sibling of `e2e`).
- **IMPLEMENT**:
  ```yaml
  playwright:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: "1.26.4"
          cache-dependency-path: backend/go.sum
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
      - name: Boot stack
        run: docker compose up -d --build
      - name: Wait for API health
        run: for i in $(seq 1 60); do curl -sf http://localhost:8080/health && break; sleep 2; done
      - name: Migrate + seed
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          make migrate-up && make seed
      - name: Career portal — build + Playwright
        working-directory: career-portal
        run: |
          corepack prepare pnpm@9.15.9 --activate
          pnpm install --frozen-lockfile
          pnpm exec playwright install --with-deps chromium
          pnpm build
          pnpm test:e2e
      - name: Dashboard — build + Playwright
        working-directory: frontend
        run: |
          corepack prepare pnpm@9.15.9 --activate
          pnpm install --frozen-lockfile
          pnpm exec playwright install --with-deps chromium
          pnpm build
          pnpm test:e2e
      - name: Upload Playwright artifacts
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-artifacts
          path: |
            frontend/e2e/__screens__/
            career-portal/e2e/__screens__/
            frontend/test-results/
            career-portal/test-results/
          if-no-files-found: ignore
      - name: Stack logs on failure
        if: failure()
        run: docker compose logs --tail=200
  ```
- **MIRROR**: CI_STACK_BOOT, CI_PNPM_SETUP.
- **GOTCHA**:
  - Run apps **sequentially** (portal then dashboard) — each `webServer` binds its own port (3001 / 3000); sequential avoids any port/resource contention and keeps logs readable.
  - `make migrate-up`/`make seed` run from repo root (default working-directory) — the migrate binary lands in `$(go env GOPATH)/bin`, already on PATH in the runner as in the `e2e` job.
  - `playwright install --with-deps` needs apt (allowed on runners); only `chromium`.
  - `CI=true` is auto-set → configs pick `pnpm start`; the preceding `pnpm build` makes `.next` exist.
  - Do NOT set `NEXT_PUBLIC_API_URL` — default `http://localhost:8080` is the stack's API.
- **VALIDATE**: push branch; the `playwright` check must go green (see Validation Commands for the local dry-run).

---

## Testing Strategy

This slice's "tests" are the CI wiring itself plus the existing specs executing. No new unit tests.

### Verification matrix
| Check | Expectation |
|---|---|
| `pnpm test:e2e` (frontend, local, stack up) | Playwright starts `next dev`, dashboard specs pass/skip-on-empty |
| `pnpm test:e2e` (career-portal, local, stack up) | portal apply/status/jobs + pwa specs run; SW test skips under `dev` |
| CI `playwright` job | both suites run on a prod build against the seeded stack; green |
| CI on failure | `playwright-artifacts` uploaded with screenshots/traces |

### Edge Cases Checklist
- [x] No seeded data → data-dependent dashboard/portal specs `test.skip` gracefully (already coded)
- [x] SW only in prod build → pwa SW test self-skips under dev, runs under CI `pnpm start`
- [x] API URL → default matches stack; no env needed
- [x] Port isolation → portal :3001, dashboard :3000; run sequentially
- [x] `File`/`FormData` globals in `apply-form.spec` → Node 20 provides them

---

## Validation Commands

### Config parse (no stack needed)
```bash
cd frontend && pnpm install --frozen-lockfile && pnpm exec playwright test --list
cd career-portal && pnpm install --frozen-lockfile && pnpm exec playwright test --list
```
EXPECT: lists tests without config errors.

### Local full run (mirrors CI behaviour)
```bash
make up && make migrate-up && make seed
# dashboard
cd frontend && pnpm exec playwright install chromium && pnpm build && CI=1 pnpm test:e2e
# portal
cd career-portal && pnpm exec playwright install chromium && pnpm build && CI=1 pnpm test:e2e
make down
```
EXPECT: both suites pass (data-dependent specs may skip); screenshots written to `e2e/__screens__/`.

### YAML sanity
```bash
# optional, if actionlint is available
actionlint .github/workflows/ci.yml || true
```
EXPECT: no syntax errors. (Otherwise rely on the push triggering the job.)

### Manual Validation
- [ ] On the PR, the `playwright` check appears and turns green alongside `build-and-test`, `e2e`, `security`.
- [ ] Force a failure locally (e.g., break a heading assertion) → artifact contains the failure screenshot + trace.

## Acceptance Criteria
- [ ] Both `playwright.config.ts` self-start the app via `webServer`.
- [ ] Both apps expose `pnpm test:e2e`.
- [ ] A `playwright` CI job boots the seeded stack, builds each app, runs both suites, uploads artifacts on failure.
- [ ] The job is green on CI; no regression to existing jobs.

## Completion Checklist
- [ ] Mirrors the e2e job's boot/migrate/seed and the security job's pnpm idiom exactly
- [ ] No new dependencies (Playwright already present); no app-logic changes
- [ ] Prod build in CI (SW exercised); `next dev` locally
- [ ] Default API URL preserved (no hardcoded origin)
- [ ] Artifacts uploaded on failure for debuggability
- [ ] Self-contained — runnable without further searching

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Browser e2e flakiness reddens CI | Medium | Medium | `retries: 1` in CI + `trace: on-first-retry`; specs self-skip on missing data; `fullyParallel: false` |
| Job runtime long (2 builds + browser dl + stack) | Medium | Low | Sequential single job; acceptable; browser-cache is a future optimization |
| `pnpm build` fails in CI (type/lint) but not caught earlier | Low | Medium | Build is a discrete step → clear failure; surfaces real prod-build breakage (a feature, not a bug) |
| Port already bound / webServer timeout | Low | Medium | `reuseExistingServer:false` in CI on clean runners; `timeout:120s`; sequential apps |
| `make seed` lacks application rows → dashboard inbox empty | Medium | Low | Inbox-renders test needs no data; data-dependent tests self-skip; portal apply flow creates its own application |

## Notes
- **Why a separate `playwright` job, not folded into `e2e`:** keeps Go-test failures and browser failures independent (separate red checks, independent retry), and lets them run in parallel. The cost is a second stack boot (~1-2 min), worth the clarity.
- **Why `pnpm start` (prod) in CI, not `pnpm dev`:** exercises the real production build — Serwist SW generation, minification, RSC prod paths — which is what ships. `next dev` would mask SW + build-time issues.
- **Local DX:** `reuseExistingServer: !CI` means a dev with `pnpm dev` already running just has Playwright reuse it; otherwise Playwright starts dev for them. Zero manual server juggling.
- Session continuity: branch `feat/s7-playwright-ci`, NO commit attribution, squash-merge; merges on green. After implement → `/code-review` → `/prp-pr`. This is the last documented Sprint 7 slice.
```
