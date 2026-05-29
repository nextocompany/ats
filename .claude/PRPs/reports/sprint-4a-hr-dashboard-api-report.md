# Implementation Report: Sprint 4a — HR Dashboard API

## Summary
Built the Go HR-facing API the dashboard renders: a ranked, filterable, role-scoped applications inbox; candidate list/detail/timeline; resume signed (SAS) URLs; bulk actions; analytics (funnel/KPI/sources); PDPA consent; and `users/me`. Role scoping is centralized in `internal/rbac` and unit-tested across roles. All curl/test-validatable; verified end-to-end on the live stack. The Next.js HR Dashboard + Career Portal UIs (Sprint 4b) build on this API.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | Single-pass; only test-signature fixups |
| Files Changed | ~22 | 17 created + 8 modified |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Auth scope (rbac) + DevUser fields | ✅ | central role→SQL scoping; unit-tested per role |
| 2 | Blob signed URL | ✅ | `SignedURL` + `SignedURLForStored` (SAS; works on Azurite) |
| 3 | Activity log (F16) | ✅ | Writer/Reader over activity_logs |
| 4 | Applications list/filter/rank | ✅ | dynamic positional-arg SQL; ai_score desc NULLS LAST |
| 5 | Dashboard handler (List/Bulk/Resume) | ✅ | separate DashboardHandler (intake Handler untouched) |
| 6 | Candidates list/detail/timeline | ✅ | `profiles` package composes candidates+applications (avoids import cycle) |
| 7 | Reports (funnel/kpi/sources) | ✅ | single-pass FILTER aggregations; conversion guarded |
| 8 | PDPA consent | ✅ | transactional insert + candidate snapshot |
| 9 | users/me | ✅ | auth identity from context |
| 10 | Wire api + docs | ✅ | routes registered; README dashboard section |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ | `go vet` (+integration tag) clean; `golangci-lint` 0 issues |
| Unit Tests (`-race`) | ✅ | rbac scope matrix (per role), applications, etc. |
| Build | ✅ | `go build` + `docker compose build` |
| Integration (`-tags integration`) | ✅ | applications.List (filter/rank/paginate/store-scope) + reports (funnel/sources) |
| Edge Cases | ✅ | store-scoped user without store → matches nothing; bulk cap; resume 404 |

### End-to-end evidence (live stack)
```
users/me → super_admin
GET /applications?status=scored&min_score=50 → meta{total,page,limit}, scores ranked desc
GET /applications/:id/resume → signed SAS URL (sig= present), ttl 900s
POST /applications/bulk {shortlisted} → updated:1
GET /candidates/:id → candidate + applications; /timeline → [bulk_action, view_resume]
GET /reports/funnel → {applied,passed_ai,reviewed,hired}; /sources → career_portal conversion 0.5
POST+GET /pdpa/consent → recorded + retrievable
```

## Files Changed
17 created, 8 modified. New: `internal/rbac`, `internal/activity`, `internal/profiles`, `internal/reports`, `internal/pdpa`, `internal/users`, `applications/list.go`, `applications/dashboard_handler.go`, `candidates/list.go` + tests. Modified: blob (SignedURL), middleware (DevUser fields), applications/candidates repos (+List/Timeline/ListByCandidate), api main.

## Deviations from Plan
1. **Candidate read handlers live in a new `profiles` package** (not `candidates`). WHY: `applications` already imports `candidates`; a candidate-detail handler needing applications would create a cycle. `profiles` composes both; `candidates` stays data-only.
2. **Dashboard endpoints use a separate `DashboardHandler`** rather than extending the intake `Handler`. WHY: avoids re-touching the intake handler/tests and keeps responsibilities clean.
3. **No new migration** — reused `activity_logs` + `pdpa_consents` from migration 000001 (plan implied possible; none needed).
4. **Activity log records action+entity (user_id null for now)** — the mock user is always super_admin; per-user attribution lands with real Azure AD.
5. **`/reports/kpi` implemented but reports scope is global** (not yet role-scoped) — funnel/sources are global aggregates for the POC; per-scope analytics is a fast follow.

## Issues Encountered
- Growing `applications.Repository`/`candidates.Repository` interfaces (+List/ListByCandidate/Timeline) — implementations added; no external test breakage this time (dashboard handler is separate).

## Tests Written
| Test File | Tests | Area |
|---|---|---|
| `internal/rbac/scope_test.go` | 2 | Kind() per role; ApplicationsClause (admin/subregion/store/no-store) |
| `internal/applications/list_integration_test.go` (tag) | 2 | filter+rank+paginate; store scope |
| `internal/reports/reports_integration_test.go` (tag) | 1 | funnel + source conversion |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Open PR (branch `feat/sprint-4a-hr-dashboard-api`)
- [ ] **Sprint 4b**: Next.js HR Dashboard + Career Portal UIs on this API — light operations console; validation shifts to Playwright/visual.
