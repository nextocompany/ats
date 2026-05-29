# Plan: Sprint 4a — HR Dashboard API (Read / Analytics / Bulk / PDPA)

## Summary
Build the Go HR-facing API the dashboard needs: a **ranked, filterable, role-scoped applications inbox**, candidate list/detail/timeline, resume **signed URLs**, **bulk actions**, **analytics** (funnel / KPI / sources), **PDPA consent**, and `users/me`. All curl/test-validatable. The Next.js HR Dashboard + Career Portal UIs (Sprint 4b) build on this complete, tested API.

## User Story
As an **HR reviewer**, I want **a ranked inbox scoped to my stores, candidate profiles with history, secure resume access, bulk actions, and recruitment analytics**, so that **I can screen and action candidates efficiently — the data layer the dashboard renders**.

## Problem → Solution
**Current state (post-Sprint 3):** The platform ingests, scores, assigns, and syncs candidates, but the only read paths are `GET /applications/:id` and the public API. There's no inbox list, no filtering, no role scoping, no analytics, no resume URL, no bulk action, no PDPA endpoints — the dashboard has nothing to call.
**Desired state:** `GET /api/v1/applications?status=&min_score=&store_id=&page=` returns a ranked, paginated, role-scoped inbox; candidates have detail + timeline; resumes return time-limited signed URLs; HR can bulk-update; analytics endpoints power the charts; PDPA consent is recorded and queryable.

## Metadata
- **Complexity**: Large (read/analytics/bulk/PDPA across new + existing packages; ~22 files)
- **Source PRD**: PRP v1.0 — Sprint 4 (W9–10), API portion (F05/F08/F10/F13/F16 backend)
- **Decisions locked**: **Backend-first** (UIs = Sprint 4b); UI direction for 4b = **light operations console** (Swiss/data-first)
- **Estimated Files**: ~22

---

## UX Design
**N/A — API.** Powers the Sprint 4b UI. (4b direction recorded in Notes.)

### Interaction Changes
| Touchpoint | Before (S3) | After (S4a) | Notes |
|---|---|---|---|
| HR inbox | none | `GET /applications` ranked+filtered+paginated+role-scoped | centerpiece |
| Candidate profile | none | `GET /candidates/:id` (+applications) + `/timeline` | F16 history |
| Resume access | raw blob url | `GET /applications/:id/resume` → time-limited signed URL | secure |
| Bulk action | none | `POST /applications/bulk` (status on N) | F05 |
| Analytics | none | `/reports/funnel`,`/kpi`,`/sources` | F08/F10 |
| PDPA | consent at intake only | `POST /pdpa/consent`, `GET /pdpa/consent/:candidate_id` | F13 |

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/pkg/httpx/response.go` | 9–36 | `Envelope[T]` + `Meta{Total,Page,Limit}` — list responses use Meta for pagination |
| P0 | `backend/internal/middleware/mock_jwt.go` | 5–30 | `UserContextKey`/`DevUser` — extend with StoreID/Subregion for role scoping |
| P0 | `backend/internal/applications/repository.go` | 13–48 | add `List(filter)`; mirror pgx query style |
| P0 | `backend/internal/applications/handler.go` | all | mirror handler + validation; add List/Bulk/Resume |
| P0 | `backend/cmd/api/main.go` | 116–140 | wire new repos/handlers + routes |
| P1 | `backend/internal/candidates/repository.go` | 13–58 | add `List`, `Timeline`, detail join |
| P1 | `backend/pkg/blob/blob.go` | 58–80 | add `SignedURL` (azblob SAS) next to Upload/Download |
| P1 | `backend/internal/applications/model.go` | all | status set, fields the inbox returns |
| P2 | `backend/migrations/000001_init_schema.up.sql` | activity_logs, pdpa_consents | tables for timeline + PDPA |
| P2 | `.claude/PRPs/plans/completed/sprint-3-*.plan.md` | conventions | mirror conventions |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| azblob SAS | `azblob/.../blob.Client.GetSASURL` | generate a read-only, time-limited SAS for a blob; works with the Azurite dev account key. |
| pgx dynamic filters | jackc/pgx | build WHERE with positional args; avoid string interpolation (no SQL injection). |
| keyset/offset pagination | — | offset+limit acceptable for POC; return `Meta{Total,Page,Limit}`. |

### Research Notes
```
KEY_INSIGHT: role scoping is a query concern, not per-row filtering.
APPLIES_TO: applications/candidates List.
GOTCHA: derive a Scope{Role, StoreID, Subregion} from the auth user. super_admin → no filter; regional/operation director → WHERE subregion=; hr_manager/hr_staff → WHERE assigned_store_id= (applications) / store via subregion (candidates). Build the WHERE in Go with positional args; unit-test the builder for each role.

KEY_INSIGHT: ranked inbox ordering.
APPLIES_TO: GET /applications default sort.
GOTCHA: default ORDER BY ai_score DESC NULLS LAST, created_at DESC — pending/unscored sink below scored. Filterable by status/min_score/store_id/source_channel/date.

KEY_INSIGHT: resume signed URL must be short-lived.
APPLIES_TO: /applications/:id/resume.
GOTCHA: SAS TTL ~15 min, read-only; log the access to activity_logs (F16 export/view audit).

KEY_INSIGHT: timeline = activity_logs (+ derived status events).
APPLIES_TO: F16.
GOTCHA: introduce a minimal activity_logs writer used by status/bulk/resume actions; timeline reads it newest-first.
```

---

## Patterns to Mirror
### LIST_RESPONSE (envelope + Meta)
```go
// SOURCE: pkg/httpx/response.go:9-36 — list endpoints return data + Meta.
return c.Status(fiber.StatusOK).JSON(httpx.Envelope[[]Item]{Success: true, Data: items, Meta: &httpx.Meta{Total: total, Page: page, Limit: limit}})
```
### AUTH_SCOPE (read user from context)
```go
// SOURCE: middleware/mock_jwt.go:6-30
u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
scope := rbac.From(u) // Role/StoreID/Subregion
```
### REPOSITORY / ERROR / LOGGING
Mirror S1–S3: pgxpool + wrapped errors; `fiber.NewError` for 4xx; structured zerolog; immutable filter structs.

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/middleware/mock_jwt.go` | UPDATE | add `StoreID *int`, `Subregion string` to DevUser |
| `backend/internal/rbac/scope.go` (+test) | CREATE | `Scope` from user; WHERE-clause builder; unit-tested per role |
| `backend/internal/applications/list.go` (+repo methods) | CREATE/UPDATE | `ListFilter` + `List(ctx, filter, scope) ([]Application, total, err)` ranked |
| `backend/internal/applications/handler.go`+`routes.go` | UPDATE | `List`, `Bulk`, `Resume` handlers + routes |
| `backend/internal/applications/bulk.go` | CREATE | bulk status update over N ids (validated) |
| `backend/internal/candidates/repository.go`+`model.go` | UPDATE | `List`, `GetDetail` (candidate+applications), `Timeline` |
| `backend/internal/candidates/handler.go`+`routes.go` | CREATE | candidate list/detail/timeline |
| `backend/internal/reports/{repo,handler,routes}.go` (+test) | CREATE | funnel / kpi / sources aggregations |
| `backend/internal/pdpa/{repo,handler,routes}.go` | CREATE | consent record + check |
| `backend/internal/activity/{log,repo}.go` | CREATE | minimal activity_logs writer + reader (F16) |
| `backend/internal/users/handler.go`+`routes.go` | CREATE | `GET /users/me` |
| `backend/pkg/blob/blob.go` | UPDATE | `SignedURL(name, ttl)` via SAS |
| `backend/cmd/api/main.go` | UPDATE | wire repos/handlers + register routes; pass blob+activity into handlers |
| tests | CREATE | rbac scope matrix, list filter (integration), bulk, reports aggregation, pdpa, signed-url |

## NOT Building (4b / later)
- **Next.js HR Dashboard + Career Portal UIs** — Sprint 4b (light operations console).
- **Candidate profile edit** (`PUT /candidates/:id`), **manual merge endpoint**, `/reports/stores`, `/reports/weekly`, full `/users` CRUD.
- **Notifications send** (LINE) — Sprint 5 (bulk "notify" action stubs a notifications row only).
- **Semantic search** (F12), **re-engagement** (F06), **PWA** (F17).
- **Real Azure AD SSO** — mock JWT still backs authenticated routes (now with store/subregion for scoping).

---

## Step-by-Step Tasks

### Task 1: Auth scope foundation
- **ACTION**: Add `StoreID *int`, `Subregion string` to `middleware.DevUser`; create `internal/rbac/scope.go` with `Scope{Role,StoreID,Subregion}`, `From(DevUser) Scope`, and `Applications(scope) (clause string, args []any)` / `Candidates(...)`.
- **MIRROR**: AUTH_SCOPE.
- **GOTCHA**: super_admin → empty clause. Mock dev user stays super_admin (sees all); scoping is exercised by unit tests with other roles.
- **VALIDATE**: `rbac` unit test: super_admin (no filter), operation_director (subregion), hr_staff (store).

### Task 2: Blob signed URL
- **ACTION**: `blob.SignedURL(ctx, name, ttl) (string, error)` via `GetSASURL` (read-only).
- **GOTCHA**: works with Azurite dev key; clamp ttl (default 15m).
- **VALIDATE**: integration — generated URL downloads the blob.

### Task 3: Activity log (F16)
- **ACTION**: `internal/activity` — `Writer.Record(ctx, userID, action, entityType, entityID, old, new)` + `List(ctx, entityType, entityID)`. Insert into `activity_logs`.
- **GOTCHA**: best-effort — a logging failure must not fail the user action (log + continue).
- **VALIDATE**: integration: record then list returns it.

### Task 4: Applications list + filter + rank
- **ACTION**: `ListFilter{Status, MinScore, StoreID, SourceChannel, From, To, Page, Limit}`; repo `List(ctx, filter, scope) ([]Application, int, error)` with `ORDER BY ai_score DESC NULLS LAST, created_at DESC` + scope WHERE + COUNT for Meta.
- **MIRROR**: REPOSITORY; positional args only.
- **GOTCHA**: clamp Limit (e.g. ≤100, default 20); validate status enum.
- **VALIDATE**: integration: seed mixed apps → filter by status/min_score returns correct, ranked, paginated set.

### Task 5: Applications handlers (List/Bulk/Resume) + routes
- **ACTION**: `GET /applications` (parse query → filter+scope → List → Envelope+Meta); `POST /applications/bulk` `{ids:[],action:"status"|"reject",value}` (validate, update each, activity-log, return counts); `GET /applications/:id/resume` (load app → `blob.SignedURL(raw_file key)` → activity-log view → return url).
- **MIRROR**: LIST_RESPONSE, ERROR_HANDLING.
- **GOTCHA**: bulk caps N (≤100); resume 404 if no raw file; derive blob key from raw_file_blob_url or stored key.
- **VALIDATE**: handler/integration: list 200 + Meta; bulk updates N; resume returns a URL.

### Task 6: Candidates list / detail / timeline
- **ACTION**: candidates repo `List(ctx, filter, scope)`, `GetDetail(ctx, id)` (candidate + its applications), `Timeline(ctx, id)` (from activity + status); handler `GET /candidates`, `GET /candidates/:id`, `GET /candidates/:id/timeline`.
- **GOTCHA**: detail joins applications; timeline newest-first.
- **VALIDATE**: integration: detail returns candidate+apps; timeline returns recorded events.

### Task 7: Reports / analytics
- **ACTION**: `internal/reports` repo: `Funnel` (counts by stage: applied→passed AI→reviewed→interview→hired), `KPI` (applied/passed/onboarded/waiting), `Sources` (by source_channel: count + conversion to hired); handler `GET /reports/funnel|kpi|sources` (role-scoped).
- **GOTCHA**: single-pass SQL aggregations (GROUP BY); guard divide-by-zero in conversion.
- **VALIDATE**: integration: seed apps across statuses/sources → funnel/sources numbers correct.

### Task 8: PDPA consent
- **ACTION**: `internal/pdpa` repo+handler: `POST /pdpa/consent` (insert `pdpa_consents` + set candidates.pdpa_consent/at/version), `GET /pdpa/consent/:candidate_id` (latest consent state).
- **GOTCHA**: validate candidate exists; record source_channel + ip.
- **VALIDATE**: integration: record consent → GET reflects it.

### Task 9: users/me
- **ACTION**: `GET /api/v1/users/me` → returns the auth user projection (id/email/role/store/subregion) from context.
- **VALIDATE**: e2e: returns the mock super_admin.

### Task 10: Wire + docs
- **ACTION**: `cmd/api/main.go` builds activity writer + reports/pdpa/users/candidates handlers; registers routes; passes blob + activity into applications handler. README: dashboard API section + role-scoping note.
- **VALIDATE**: `/health` unchanged; e2e below.

---

## Testing Strategy
### Unit / Integration
| Test | Input | Expected | Edge? |
|---|---|---|---|
| rbac super_admin | super_admin | empty WHERE | No |
| rbac operation_director | subregion set | subregion clause + arg | No |
| rbac hr_staff | store set | store clause + arg | Yes |
| list filter+rank | mixed apps | scored first, status filter applied, Meta.Total correct | No |
| list pagination | 25 apps, page 2 limit 10 | 10 rows, Total 25 | Yes |
| bulk status | 3 ids | all updated, count 3, activity logged | No |
| bulk too many | 101 ids | 400 | Yes |
| resume signed url | app w/ file | downloadable URL | No |
| resume no file | app w/o file | 404 | Yes |
| funnel/sources | seeded statuses | correct counts/conversion | No |
| pdpa record→check | consent | GET reflects consent | No |
| timeline | recorded actions | newest-first events | No |

### Edge Cases Checklist
- [ ] Empty inbox → empty list + Meta.Total 0
- [ ] Invalid filter value (bad status/score) → 400
- [ ] Limit clamp (>100)
- [ ] Non-admin scope returns only in-scope rows
- [ ] Bulk with unknown id → reported, others still applied
- [ ] Activity-log failure doesn't fail the action

---

## Validation Commands
### Static / Unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Integration
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./... -count=1
```
### End-to-end (the Sprint 4a gate)
```bash
make up && make migrate-up && make import DIR=scripts
# submit a couple applications (authed intake), let the pipeline score them, then:
curl -s 'localhost:8080/api/v1/applications?status=scored&min_score=50&page=1&limit=20'   # ranked + Meta
curl -s localhost:8080/api/v1/applications/<id>/resume                                    # → signed URL
curl -s -X POST localhost:8080/api/v1/applications/bulk -d '{"ids":["<id>"],"action":"status","value":"shortlisted"}'
curl -s localhost:8080/api/v1/candidates/<cid>            # candidate + applications
curl -s localhost:8080/api/v1/candidates/<cid>/timeline   # history
curl -s localhost:8080/api/v1/reports/funnel
curl -s localhost:8080/api/v1/reports/sources
curl -s -X POST localhost:8080/api/v1/pdpa/consent -d '{"candidate_id":"<cid>","consent_given":true,"consent_version":"1.0","source_channel":"career_portal"}'
curl -s localhost:8080/api/v1/users/me
```
EXPECT: ranked/paginated inbox with Meta; working signed URL; bulk updates; candidate detail+timeline; funnel/sources numbers; PDPA recorded; users/me returns the dev user.

---

## Acceptance Criteria
- [ ] `GET /applications` returns a ranked (score desc), filterable, paginated, role-scoped inbox with `Meta`.
- [ ] `GET /applications/:id/resume` returns a short-lived signed URL (access audited).
- [ ] `POST /applications/bulk` updates N applications and logs activity.
- [ ] Candidate detail (+applications) and timeline endpoints work.
- [ ] `/reports/funnel|kpi|sources` return correct aggregations.
- [ ] PDPA consent recorded + queryable; `users/me` returns the auth user.
- [ ] Role-scoping builder unit-tested across roles; all validation levels pass incl. `-race`.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Dynamic filter SQL injection | Low | High | positional args only; never interpolate user input |
| Role scoping leaks cross-store data | Med | High | central `rbac` builder, unit-tested per role; default to most-restrictive on unknown role |
| Azurite SAS differs from prod Azure | Med | Med | read-only SAS via SDK; document; signed-URL TTL short |
| Mock user is always super_admin | High | Low | scoping logic unit-tested with all roles; real Azure AD (S?) supplies role/store |
| Analytics queries slow at scale | Low | Med | indexed status/score/subregion (existing); POC volumes fine |

## Notes
- **Sprint 4b** builds the Next.js apps against this API in the **light operations console** direction: dense data-first inbox, strong type hierarchy, restrained palette + one semantic accent for AI score, shadcn/ui + Tailwind + Recharts + react-pdf, mock Azure AD (NextAuth credentials) / mock LINE in dev. Validation shifts to Playwright + visual checks per the web testing rules.
- This sprint completes the read/audit surface (F16 minimal activity log) the UI and compliance need.
