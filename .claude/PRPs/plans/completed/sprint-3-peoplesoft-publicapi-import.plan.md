# Plan: Sprint 3 — PeopleSoft Integration + Public Career API + Reference-Data Import

## Summary
Connect the platform to PeopleSoft (bi-directional, §9), expose the public Career Portal **API** (no UI), and import real reference data. Direction A: PS webhooks create/update vacancies (replacing Sprint 2's seeded demos) and map PS position codes → internal positions. Direction B: marking an application `hired` pushes the candidate to PS (Integration Broker REST, with a CSV-to-Blob fallback). The PS client and LINE auth follow the established **mock-default-behind-config** seam. A CSV importer loads the real Storelist (169 stores) + Master JD (~70 positions).

## User Story
As **PeopleSoft (HRIS) and external candidates**, I want **vacancies to flow into the platform automatically, hired candidates to flow back to PeopleSoft, and candidates to browse open positions and apply/check status via a public API**, so that **recruiting is driven by real requisitions and the platform is the single intake funnel**.

## Problem → Solution
**Current state (post-Sprint 2):** Vacancies are hand-seeded; nothing syncs to/from PeopleSoft; the only intake path is the authenticated `POST /applications`; reference data is synthetic.
**Desired state:** PS `vacancy-opened/closed` webhooks keep `vacancies` live and mapped to positions; setting an application `hired` pushes it to PS (or CSV fallback); `/api/v1/public/*` lets candidates list positions, apply (mock LINE auth, reusing the intake pipeline), and check status by opaque token; an importer ingests the real store/JD files.

## Metadata
- **Complexity**: Large (PS bi-directional + public API + importer; ~26 files)
- **Source PRD**: PRP v1.0 — Sprint 3 (W7–8, "PeopleSoft Integration + Career Portal + Data Migration", 35 MD)
- **Covers**: §9 (PeopleSoft A+B), F14 backend (public Career API), §17 reference-data import
- **Decisions locked**: **Backend-first** (Next.js Career Portal UI deferred to Sprint 4); **real reference-data importer** (not historical candidate migration); PS + LINE clients **mock by default**, real behind config
- **Estimated Files**: ~26

---

## UX Design
**N/A — backend/API + integration.** The public API is consumable by Power Automate, the future Next.js portal (Sprint 4), or curl. No UI in this sprint.

### Interaction Changes
| Touchpoint | Before (S2) | After (S3) | Notes |
|---|---|---|---|
| Vacancies | hand-seeded | created/closed by PS webhooks | `vacancies.ps_vacancy_id` upsert |
| Hired candidate | nowhere | pushed to PS (or CSV fallback) | Direction B |
| Public browse/apply | none | `/api/v1/public/positions`, `/apply`, `/status/:token` | mock LINE auth |
| Reference data | synthetic seed | importer from real CSVs | replaces `make seed` for prod-like data |

---

## Mandatory Reading (existing code to extend)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/ai/factory.go` | 8–15 | The mock/real provider seam to mirror for PS + LINE clients |
| P0 | `backend/internal/applications/service.go` | 26–62 | `IntakeInput`/`Intake` — public `/apply` reuses this verbatim |
| P0 | `backend/cmd/api/main.go` | 85–120 | Where to build PS/LINE clients + register `/ps` and `/public` routes + add a `peoplesoft` health checker |
| P0 | `backend/internal/applications/repository.go` | 13–23, 87–95 | Add `SetHired`, `SetPublicToken`, `FindByPublicToken`; `SetStatus` exists |
| P0 | `backend/internal/vacancies/model.go` | 25–67 | Add `Upsert` (by ps_vacancy_id) + `SetStatusByPSID` |
| P1 | `backend/internal/positions/model.go` | 32–60 | Add `FindByPSCode` for code→position mapping |
| P1 | `backend/internal/httpx`/`pkg/httpx/response.go` | 16–36 | Envelope for public + ps responses |
| P1 | `backend/internal/applications/handler.go` | all | mirror handler/validation style for public + status |
| P1 | `backend/pkg/config/config.go` | 26–82 | add PS_* / LINE_* + provider toggles + conditional fail-fast |
| P2 | `backend/internal/stores/subregion.go` | all | importer derives subregion + centroid from province |
| P2 | `.claude/PRPs/plans/completed/sprint-2-scoring-branch-dedup.plan.md` | conventions | mirror conventions |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| PS Integration Broker REST | PRP §9 | OAuth2 client credentials; `create_applicant` payload; 3 retries, exp backoff, 30s timeout; on failure log + CSV fallback to Blob. |
| OAuth2 client credentials (Go) | `golang.org/x/oauth2/clientcredentials` | token source from PS_IB token URL + client id/secret; reuse the http client. |
| LINE Login id token verify | developers.line.biz | verify id_token at `https://api.line.me/oauth2/v2.1/verify` (or JWKS); mock accepts a stub subject in dev. |
| Go encoding/csv | stdlib | importer reads Storelist.csv / Master JD CSV; header-mapped columns. |

### Research Notes
```
KEY_INSIGHT: position_code → position_id mapping is required for vacancy-opened.
APPLIES_TO: migration + positions repo.
GOTCHA: add positions.ps_position_code (UNIQUE, nullable). vacancy-opened looks it up; if unknown, store the vacancy with position_id NULL and flag for admin mapping (do not drop the event).

KEY_INSIGHT: public status must not expose the application UUID.
APPLIES_TO: /public/apply + /public/status/:token.
GOTCHA: generate an opaque applications.public_token (crypto/rand, URL-safe) at apply time; /status looks up by token and returns a minimal projection (status, position title, store) — never internal scoring fields.

KEY_INSIGHT: hired push trigger.
APPLIES_TO: Direction B.
GOTCHA: PATCH /applications/:id/status to "hired" sets hired_at and triggers peoplesoft.SyncHired; success sets ps_synced_at. PS unavailable → CSV row to a Blob container + ps_synced_at left null + activity log; never block the status change on PS.

KEY_INSIGHT: idempotent vacancy upsert.
APPLIES_TO: webhooks (PS may redeliver).
GOTCHA: upsert by ps_vacancy_id (ON CONFLICT DO UPDATE); vacancy-closed sets status='filled'/'cancelled' by ps_vacancy_id.
```

---

## Patterns to Mirror

### PROVIDER_SEAM (PS + LINE, like ai/factory)
```go
// SOURCE: backend/internal/ai/factory.go:8 — mock default, real behind config.
func NewPSClient(cfg *config.Config) Client { if cfg.UsesRealPeopleSoft() { return newRESTClient(cfg) }; return mockClient{} }
```

### INTAKE_REUSE (public apply)
```go
// SOURCE: backend/internal/applications/service.go:62 — public apply maps to IntakeInput with source_channel="career_portal".
res, err := intakeSvc.Intake(ctx, applications.IntakeInput{ SourceChannel: "career_portal", ... })
```

### RESPONSE_ENVELOPE / ERROR_HANDLING / REPOSITORY / LOGGING
Mirror Sprint 1/2: `httpx.OK/Fail`, `fiber.NewError` for 4xx, pgxpool repos with wrapped errors, structured zerolog.

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000005_ps_public.up/.down.sql` | CREATE | `positions.ps_position_code` UNIQUE; `applications.public_token` UNIQUE + index; `vacancies` already has ps_vacancy_id UNIQUE |
| `backend/pkg/config/config.go` | UPDATE | PS_* (IB base/token URL, client id/secret), LINE_* , `PS_PROVIDER`/`LINE_PROVIDER` toggles + conditional fail-fast |
| `backend/internal/peoplesoft/{client,model}.go` | CREATE | `Client` interface (`SyncHired`), payload structs |
| `backend/internal/peoplesoft/mock.go` | CREATE | records calls in memory/log (default) |
| `backend/internal/peoplesoft/rest.go` | CREATE | OAuth2 client-credentials REST + retry/backoff |
| `backend/internal/peoplesoft/csv_fallback.go` | CREATE | write applicant CSV row to Blob when REST unavailable |
| `backend/internal/peoplesoft/service.go` | CREATE | SyncHired orchestration (REST → fallback → ps_synced_at/activity log) |
| `backend/internal/peoplesoft/webhook.go` | CREATE | vacancy-opened/closed handlers; health |
| `backend/internal/peoplesoft/routes.go` | CREATE | mount `/api/v1/ps/*` |
| `backend/internal/auth/line.go` | CREATE | LINE auth verifier interface + mock + real |
| `backend/internal/public/{handler,routes}.go` | CREATE | `/public/positions`, `/positions/:id`, `/apply`, `/status/:token` |
| `backend/internal/vacancies/model.go` | UPDATE | `Upsert(ctx, v)`, `SetStatusByPSID`, `ListOpenPositions` |
| `backend/internal/positions/model.go` | UPDATE | `FindByPSCode`, `ListActive` (for public list) |
| `backend/internal/applications/repository.go` & `model.go` | UPDATE | `SetHired`, `SetPublicToken`, `FindByPublicToken`, `StatusHired`, public_token field |
| `backend/internal/applications/handler.go`+`routes.go` | UPDATE | `PATCH /applications/:id/status` (triggers PS sync on hired) |
| `backend/cmd/api/main.go` | UPDATE | build PS + LINE clients; register `/ps` + `/public`; PS health checker; inject PS service into status handler |
| `backend/cmd/importref/main.go` | CREATE | CSV importer (stores + positions), subregion/centroid from province map |
| `scripts/stores.sample.csv`, `scripts/positions.sample.csv` | CREATE | sample real-format CSVs + `make import` |
| `Makefile` | UPDATE | `make import DIR=...` |
| tests | CREATE | webhook upsert, hired sync (mock+fallback), public apply/status, importer parse, OAuth2 token (mock) |

## NOT Building (later sprints)
- **Next.js Career Portal UI / LIFF** — Sprint 4 (alongside HR Dashboard).
- **LINE *notifications*** (push templates) — Sprint 5; vacancy-opened writes a pending `notifications` row only.
- **HR Dashboard, analytics, search, PDPA UI, re-engagement.**
- **Historical candidate migration** (PRP §18 — separate project).
- **Real Azure AD SSO** — mock JWT still covers authenticated routes.

---

## Step-by-Step Tasks

### Task 1: Migration 000005
- **ACTION**: `positions.ps_position_code VARCHAR(50) UNIQUE`; `applications.public_token VARCHAR(64) UNIQUE` + index. Down drops them.
- **MIRROR**: MIGRATION_NAMING; additive.
- **VALIDATE**: `make migrate-up`/`down 1` round-trip.

### Task 2: Config extension (PS + LINE)
- **ACTION**: Add `PSProvider` (default `mock`), `PSIBBaseURL`, `PSIBTokenURL`, `PSIBClientID`, `PSIBClientSecret`, `PSCSVFallbackContainer`; `LINEProvider` (default `mock`), `LINEChannelID`. Conditional fail-fast when `PS_PROVIDER=real` / `LINE_PROVIDER=real`.
- **MIRROR**: `config.go:62-82` Azure conditional validation; `UsesRealPeopleSoft()`, `UsesRealLINE()`.
- **VALIDATE**: config tests for both toggles.

### Task 3: Vacancies repo — upsert + status + open positions
- **ACTION**: `Upsert(ctx, Vacancy)` (ON CONFLICT (ps_vacancy_id) DO UPDATE), `SetStatusByPSID(ctx, psID, status)`, `ListOpenPositionIDs(ctx)`.
- **MIRROR**: vacancies repo style.
- **GOTCHA**: upsert must be idempotent on PS redelivery.
- **VALIDATE**: integration test: open then re-open (no dup), close sets status.

### Task 4: Positions repo — code lookup + active list
- **ACTION**: `FindByPSCode(ctx, code)`, `ListActive(ctx)` (and a public projection).
- **VALIDATE**: integration test with seeded ps_position_code.

### Task 5: PeopleSoft client (mock + REST + fallback)
- **ACTION**: `Client` interface `SyncHired(ctx, Applicant) (Result, error)`; `mockClient` records + returns ok; `restClient` OAuth2 client-credentials + retry(3, backoff) + 30s timeout; `csvFallback` writes a row to Blob.
- **MIRROR**: PROVIDER_SEAM; Sprint 1/2 REST style.
- **GOTCHA**: never panic on missing PS config in mock mode; factory only builds REST when selected.
- **VALIDATE**: unit test mock; rest is compile-only (no live PS).

### Task 6: PeopleSoft service (SyncHired orchestration)
- **ACTION**: `Service.SyncHired(ctx, applicationID)`: load application+candidate, build payload, call client; on success set `ps_synced_at`; on failure → CSV fallback to Blob + activity log, leave ps_synced_at null. Return outcome.
- **GOTCHA**: PS failure must NOT fail the hire — log + fallback.
- **VALIDATE**: integration test: mock success sets ps_synced_at; forced failure writes CSV + leaves null.

### Task 7: PeopleSoft webhooks + health + routes
- **ACTION**: `POST /api/v1/ps/vacancy-opened` (map ps position_code→position_id; `vacancies.Upsert` status open; write pending HR `notifications` row), `POST /api/v1/ps/vacancy-closed` (`SetStatusByPSID`), `GET /api/v1/ps/health`, `POST /api/v1/ps/sync-hired` (manual trigger by application_id).
- **MIRROR**: RESPONSE_ENVELOPE, ERROR_HANDLING; validate payload at boundary.
- **GOTCHA**: unknown position_code → store vacancy with null position_id + log (don't 500/drop).
- **VALIDATE**: handler tests (valid open → 200 + vacancy upserted; bad payload → 400); idempotent re-delivery.

### Task 8: LINE auth (mock + real)
- **ACTION**: `internal/auth/line.go`: `Verifier.Verify(ctx, idToken) (LineUser, error)`; mock accepts a non-empty stub token → fixed user; real verifies via LINE endpoint.
- **MIRROR**: PROVIDER_SEAM + mock_jwt gating.
- **VALIDATE**: unit test mock accept/reject.

### Task 9: Public Career API
- **ACTION**: `internal/public/{handler,routes}.go`: `GET /api/v1/public/positions` (active positions with ≥1 open vacancy), `GET /api/v1/public/positions/:id` (public projection), `POST /api/v1/public/apply` (LINE-verify → reuse `intakeSvc.Intake` with source_channel=career_portal → generate+store public_token → return token), `GET /api/v1/public/status/:token` (minimal projection).
- **MIRROR**: INTAKE_REUSE; never expose internal scoring on public endpoints.
- **GOTCHA**: public routes bypass MockJWT (they're unauthenticated / LINE-authed); enforce file validation as in the authed intake.
- **VALIDATE**: handler tests: apply happy (mock LINE) → 201 + token; status by token → minimal fields; bad token → 404.

### Task 10: Application status PATCH + hired → PS
- **ACTION**: `PATCH /api/v1/applications/:id/status`; on `hired` set hired_at + call `peoplesoft.Service.SyncHired`.
- **MIRROR**: applications handler.
- **GOTCHA**: validate target status against allowed transitions; PS sync async-safe (log, don't block response on PS latency beyond timeout).
- **VALIDATE**: integration test: set hired → ps_synced_at set (mock) or CSV fallback on failure.

### Task 11: Reference-data importer
- **ACTION**: `cmd/importref/main.go`: read `stores.csv` (Store_No, Store_Name, Format_Type, Subregion, Province, lat, lng) and `positions.csv` (title_th, title_en, level, ps_position_code, min_education_level, min_experience_months, keywords). Upsert stores (derive subregion/centroid from province map when missing) + positions. Sample CSVs + `make import`.
- **GOTCHA**: idempotent upserts (ON CONFLICT); validate rows, skip+log malformed; this replaces synthetic seed for prod-like data.
- **VALIDATE**: run against sample CSVs → rows present; re-run → no duplicates.

### Task 12: Wire api + docs
- **ACTION**: `cmd/api/main.go` builds PS client/service + LINE verifier; registers `/ps` + `/public` routes; adds `peoplesoft` health checker; injects PS service into the status handler. README updates (PS webhooks, public API, `make import`).
- **VALIDATE**: `/health` includes `peoplesoft`; e2e below.

---

## Testing Strategy
### Unit / Integration
| Test | Input | Expected | Edge? |
|---|---|---|---|
| config PS real requires creds | PS_PROVIDER=real, no creds | error | Yes |
| vacancy upsert idempotent | same ps_vacancy_id twice | one row, updated | Yes |
| vacancy-opened unknown code | unmapped position_code | stored w/ null position_id, logged | Yes |
| vacancy-closed | ps_vacancy_id | status filled/cancelled | No |
| hired sync mock | application_id | ps_synced_at set | No |
| hired sync PS-down | forced failure | CSV in Blob, ps_synced_at null, hire still succeeds | Yes |
| LINE mock verify | stub token / empty | user / error | Yes |
| public apply | LINE token + file | 201 + public_token, application created (career_portal) | No |
| public status | valid/invalid token | minimal projection / 404 | Yes |
| public positions | seeded open vacancy | lists only positions with open vacancies | No |
| importer | sample CSVs | stores+positions upserted; re-run no dups | Yes |

### Edge Cases Checklist
- [ ] PS webhook redelivery (idempotent upsert)
- [ ] Unknown ps_position_code (don't drop event)
- [ ] PS REST timeout/unavailable → CSV fallback, hire unaffected
- [ ] Public token not guessable (crypto/rand) and not the UUID
- [ ] Public endpoints never leak scoring/internal fields
- [ ] Importer malformed row → skip + log, continue
- [ ] Migration 000005 round-trip

---

## Validation Commands
### Static / Unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Integration
```bash
make up && make migrate-up
cd backend && go test -tags integration ./... -count=1
```
### Reference import
```bash
make import DIR=scripts            # loads stores.sample.csv + positions.sample.csv
```
### End-to-end (the Sprint 3 gate)
```bash
make up && make migrate-up && make import DIR=scripts
# 1) PS opens a vacancy
curl -s -X POST localhost:8080/api/v1/ps/vacancy-opened \
  -d '{"ps_vacancy_id":"V-2026-001","store_id":1,"position_code":"CASHIER","headcount":1,"opened_date":"2026-06-01"}'
# 2) public sees it
curl -s localhost:8080/api/v1/public/positions
# 3) public applies (mock LINE) → token
curl -s -F resume=@cv.pdf;type=application/pdf -F position_id=<id> -F full_name=... \
  -H 'X-LINE-IdToken: dev-stub' localhost:8080/api/v1/public/apply
# 4) status by token
curl -s localhost:8080/api/v1/public/status/<token>
# 5) hire → PS push (mock) sets ps_synced_at
curl -s -X PATCH localhost:8080/api/v1/applications/<id>/status -d '{"status":"hired"}'
curl -s localhost:8080/api/v1/ps/health
```
EXPECT: vacancy upserted + visible publicly; apply returns opaque token; status shows minimal projection; hire sets `ps_synced_at` (mock) or writes CSV fallback.

---

## Acceptance Criteria
- [ ] PS `vacancy-opened/closed` upsert vacancies idempotently and map position codes (unknown codes don't drop events).
- [ ] Marking `hired` pushes to PS (mock) setting `ps_synced_at`, or writes a CSV fallback to Blob on failure — the hire never fails on PS.
- [ ] Public API: list open positions, apply (mock LINE, reusing the intake pipeline) returning an opaque token, status by token (no internal fields leaked).
- [ ] Reference-data importer loads real-format CSVs idempotently and derives subregion/centroid.
- [ ] PS + LINE selectable via config; mock default needs no PS/LINE creds.
- [ ] All validation levels pass incl. `-race`; coverage ≥80% on new tested packages.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| No live PS to test against | High | Med | mock default + REST compile-only; CSV fallback path tested |
| PS redelivery duplicates vacancies | Med | Med | upsert by ps_vacancy_id |
| Public token guessable / PII leak | Low | High | crypto/rand token; minimal public projection; never the UUID |
| Hire blocked by PS latency/outage | Med | High | timeout + CSV fallback + activity log; status change independent of PS |
| Real CSV columns differ from sample | Med | Med | header-mapped importer; document expected columns; skip+log malformed |

## Notes
- PS + LINE complete the trio of external integrations behind the same mock-default seam (AI in S1/S2, now PS + LINE) — CI never needs external creds.
- Vacancy sync **replaces** Sprint 2's seeded demos in prod-like runs; `make seed` remains for quick local demos.
- The Next.js Career Portal UI (and LIFF) consumes these `/public/*` endpoints in Sprint 4.
