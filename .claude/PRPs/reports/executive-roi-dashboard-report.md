# Implementation Report: Executive ROI & Performance Dashboard

## Summary
Replaced the old executive dashboard (budget-dependent tabs) with a Recruitment ROI & Performance dashboard: ROI/cost-per-hire (admin cost-config driven), volume/response/hire funnel, time-to-hire (apply→offer-accept), and success cases by branch/region/position. Built on `feat/executive-roi-dashboard` (3 commits, not pushed). Executed by a fresh-context implementation agent against the self-contained plan.

## Assessment vs Reality
| Metric | Plan | Actual |
|---|---|---|
| Complexity | Large | Large — accurate |
| Files | ~13 | 22 (2035 +/60 −) |
| Confidence | 8/10 | green single-pass; 3 advisor-driven correctness fixes folded in |

## Tasks (all complete)
1. Migration **000049** `executive_cost_config` (single-row bool PK + CHECK, seeded)
2. Cost-config repo + `GET` (executive.view) / `PUT` (settings.admin, non-negative)
3. Live aggregations `roi.go` — ROI math, funnel+response, TTH avg+median, success-by-dimension; per-metric windows (hires/TTH on `hired_at`, volume on `created_at`); NULLIF guards
4. Deterministic mock ROIView + service interface extension + wiring
5. Frontend `/executive` rebuilt (ExecFilters/RoiBand/FunnelVolume/TimeToHirePanel/SuccessByDimension/CostConfigDialog), hand-rolled CSS bars, URL state, bilingual i18n (124/124 parity)
6. Old tabs retired from render (components kept on disk), same `executive.view` gate

## Validation (all PASS)
go build / vet / `go test ./internal/executive/...` (unit) / integration on PG16 (ROI math, funnel, TTH median, branch reconciliation incl NULL-store, dimension scoping, region grouping, cost-config round-trip + negative rejection) / tsc / eslint on changed files / migration up+down reversible (local DB v49).
Note: full-repo `pnpm lint` shows 2 PRE-EXISTING errors in untouched files (LocaleSwitcher, AppHeader, applications/page) — not from this work.

## Deviations (deliberate)
- `updated_by` = email string (no FK) — mirrors system_settings; SSO OIDs may lack a users row.
- Added `traditional_time_to_hire_days` config col so "vacancy cost avoided" has an honest formula (degrades to 0 unset).
- Filter bar = period + dimension switch only (backend supports per-value scoping params); `all` headline already shows all-up.
- Fix commit: branch breakdown LEFT JOIN + "Unassigned" bucket so Σ success.hires == headline (was INNER JOIN dropping NULL-store hires); mock rows distribute headline hires.

## Tests written
`roi_integration_test.go` (7 integration), `roi_mock_test.go` (2 unit).

## Next Steps
- [ ] `/code-review` on the branch (user's stated preference before PR/deploy)
- [ ] PR → deploy: migration 000049 + api + dashboard (worker/career-portal untouched; no new env)
- [ ] Browser UAT; flip `EXECUTIVE_PROVIDER=real` for live aggregations (stays `mock` for demo)
