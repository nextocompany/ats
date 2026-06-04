# Code Review: Playwright Browser E2E in CI (local, branch `feat/s7-playwright-ci`)

**Reviewed**: 2026-06-04
**Mode**: Local (uncommitted changes vs `main`) — config/CI only, lighter review
**Decision**: APPROVE

## Summary
CI/config-only change wiring the existing Playwright specs to run. No app logic, no new deps. Both suites were executed locally via the exact CI flow (prod build + `CI=1` + self-starting webServer) and pass (portal 30/30, dashboard 5/5). Review focused on CI-job correctness and ordering.

## Findings

### CRITICAL / HIGH
None.

### MEDIUM
None.

### LOW
1. **Duplicate stack boot** — the `playwright` job boots+seeds the Go stack independently of the `e2e` job (~1-2 min). Intentional trade-off for job independence/parallelism (documented). No change.
2. **No Playwright browser cache** — `playwright install chromium` downloads each run. Acceptable; a `~/.cache/ms-playwright` cache is a future optimization. No change.
3. **Adds CI time/cost** — a new job with two prod builds + browser tests. This is the intended cost of front-end regression coverage. No change.

## Correctness checks (all pass)
- **Step ordering** — Boot stack → Wait health → Migrate+seed precede the `pnpm build` steps. Correct: the portal `/jobs` route is dynamic and the build/runtime needs the API at `localhost:8080`. ✅
- **API URL** — default `NEXT_PUBLIC_API_URL` (`http://localhost:8080`) matches the compose-exposed API; no env injected, nothing hardcoded. ✅
- **Prod build exercises SW** — `pnpm start` (not `pnpm dev`) in CI generates the Serwist service worker; verified the SW test actually ran (not skipped) locally → `30 passed`. ✅
- **Port isolation** — apps run sequentially; portal binds 3001, dashboard 3000; `reuseExistingServer:false` in CI on clean runners. ✅
- **No DB race** — Playwright specs don't `TRUNCATE`; the apply flow creates its own candidate/application per project, so cross-project parallelism is safe (unlike the Go integration suite). ✅
- **Node 20 globals** — `apply-form.spec` uses `File`/`FormData`; both are Node globals (File ≥20, FormData ≥18) → CI's Node 20 is fine. ✅
- **Artifact hygiene** — `e2e/__screens__/` and `test-results/` are gitignored (`git check-ignore` confirms); local run screenshots won't be committed. Upload-artifact uses `if: failure()` + `if-no-files-found: ignore`. ✅
- **pnpm idiom** — `corepack prepare pnpm@9.15.9 --activate` + `pnpm install --frozen-lockfile` mirrors the existing `security` job. ✅
- **YAML** — no tabs; jobs parse. ✅

## Validation Results

| Check | Result |
|---|---|
| Config parse (`playwright test --list`) | Pass — portal 30, dashboard 5 |
| Local CI-mirror — career-portal | Pass — 30/30 (incl. prod-build SW test) |
| Local CI-mirror — dashboard | Pass — 5/5 |
| YAML sanity | Pass |
| Artifact gitignore | Pass |

## Files Reviewed
- `frontend/playwright.config.ts` (Modified)
- `career-portal/playwright.config.ts` (Modified)
- `frontend/package.json` / `career-portal/package.json` (Modified)
- `.github/workflows/ci.yml` (Modified — new `playwright` job)

> The definitive validation is the `playwright` job going green on the PR (clean runner, Node 20). Local proof (Node 25) already passes.
