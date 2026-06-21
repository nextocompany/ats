# Plan: PDPA Compliance (Thailand PDPA B.E. 2562) - full-system

## Summary
Bring the ATS to comprehensive compliance with Thailand's Personal Data Protection Act (PDPA, พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล พ.ศ. 2562). The system already has strong foundations (consent ledger, a 365-day retention sweep, two anonymization engines, an audit log, account lifecycle). This plan **closes the erasure-completeness gaps** and adds the missing compliance pillars across **5 sequenced phases**: (1) complete + unified erasure, (2) consent v2 (versioning, withdrawal, unify stores), (3) DSAR self-service on the portal, (4) privacy notice pages, (5) audit hardening + ROPA + breach register + DPO console. Implement phase-by-phase (Phase 1 is the foundation and the highest compliance risk).

## User Story
As a **data subject (candidate)**, I want to see, export, correct, and delete my personal data and withdraw consent myself, and as the **data controller (CP Axtra HR / DPO)**, I want every PDPA obligation - lawful consent, complete erasure, retention limits, an audit trail, a processing record (ROPA), and breach handling - enforced in the system, so that we meet Thai PDPA and can demonstrate compliance to the PDPC.

## Problem → Solution
**Current**: consent captured + a retention sweep + admin anonymize exist, but erasure is **incomplete** (Azure Search index, onboarding sensitive docs, interview chat, fit/feedback/offers/letters, account+notes, audit/notification PII are never erased), consent is hardcoded `v1.0` across two un-unified stores, there is **no candidate self-service DSAR**, **no privacy policy page**, and **no ROPA / breach register / DPO** surface.
**Desired**: one authoritative erasure that cascades across every PII store (DB + blobs + search index), versioned consent with withdrawal, portal self-service for all PDPA rights, published privacy notices, and an admin PDPA console (ROPA, breach register, DSAR queue, DPO contact).

## Metadata
- **Complexity**: XL (5 phases; ~30-40 files; 5+ migrations) - implement phase by phase
- **Source PRD**: N/A (free-form, comprehensive request)
- **PRD Phase**: N/A
- **Estimated Files**: ~35 across 5 phases

---

## PDPA Requirement Map (the "why" behind each phase)

| PDPA obligation (section) | Where it lands |
|---|---|
| Consent: explicit, informed, withdrawable as easily as given (s.19) | Phase 2 + 3 |
| Right of access (s.30) + data portability (s.31) | Phase 3 (DSAR export) |
| Right to object / restrict processing (s.32, 34) | Phase 3 |
| Right to erasure / be forgotten (s.33) | Phase 1 (complete erasure) + Phase 3 (request) |
| Right to rectification (s.36) | Phase 3 |
| Retention limited to necessity | Already (365-day sweep) + Phase 1 completeness |
| Records of Processing Activities / ROPA (s.39) | Phase 5 |
| Breach notification to PDPC within 72h + subject if high risk (s.37(4)) | Phase 5 |
| DPO appointment for large-scale / sensitive processing (s.41) | Phase 5 |
| Cross-border transfer safeguards (s.28-29) | Phase 5 (ROPA documents the processors; deployment-config note) |
| Security measures (s.37(1)) | Already (hashed sessions/OTP, RBAC, scoped access) + Phase 5 audit |

Sensitive data note: the system stores Thai national ID (`id_card`) and onboarding scans incl. `health_check`, `house_registration`, `bank_book` (`onboarding_documents.doc_type`) - these are special-category under PDPA, which is why **DPO is required (s.41(2))** and why Phase 1 erasure completeness is critical.

---

## Mandatory Reading
| Priority | File | Why |
|---|---|---|
| P0 | `backend/internal/pdpa/retention.go` | `Sweep` + `anonymize` (lines 49, 163-198) - the erasure engine to extend into a unified `EraseSubject`; `piiBlobs` (131-158); eligibility (97-109) |
| P0 | `backend/internal/pdpa/pdpa.go` | `Record`/`Latest` consent (29-65) - extend for versioning + withdrawal |
| P0 | `backend/internal/members/lifecycle.go` | `Anonymize` (118-165) - the account-path erasure to unify with the candidate path |
| P0 | `backend/internal/search/indexer.go` | `azureIndexer.UpsertBatch` + `mergeOrUpload` (90-115) - ADD a `Delete` path (the #1 erasure gap) |
| P0 | `backend/internal/activity/activity.go` | `Writer`/`Record` (38-52), action constants (16-26) - extend for actor/IP + DSAR/consent/breach actions |
| P0 | `backend/migrations/000029_requisitions.up.sql` + `000001`/`000013`/`000025` | migration + seed style; PII table schemas. **Next migration = 000030** |
| P1 | `backend/internal/candidateauth/{service,repository,handler}.go` | portal identity + `SaveConsent`/`SetConsent`; session resolve `AND status='active'` - the DSAR self-service hooks here |
| P1 | `backend/internal/public/handler.go` | apply flow consent-mandatory (158-281) - consent version source |
| P1 | `backend/internal/members/{export.go,lifecycle_handler.go}` | CSV export + audit-with-actor (`auditWith`) pattern to mirror for DSAR export + actor logging |
| P1 | `backend/internal/requisitions/{model,repository,handler}.go` | the canonical CRUD pattern (this session) for new admin packages (breach register, DSAR queue) |
| P1 | `backend/internal/rbac/permissions.go` | add PDPA permission keys (`pdpa.admin`, `dsar.handle`, `breach.manage`) |
| P1 | `frontend/components/admin/RolesPermissions.tsx` + `app/(app)/members/page.tsx` | dashboard CRUD/console pattern for the PDPA admin console |
| P1 | `career-portal/components/ConsentStep.tsx` + `app/account/*` | portal consent + account surface for DSAR self-service |
| P1 | `backend/pkg/config/config.go` | retention env (166-172); add DPO contact + breach + DSAR config |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Thailand PDPA B.E. 2562 | Official Act (PDPC) | Rights ss.19,30-36; ROPA s.39; breach 72h s.37(4); DPO s.41; cross-border s.28-29 |
| Azure AI Search delete docs | learn.microsoft.com (Indexer REST) | `@search.action: "delete"` with the key field only - the missing index-erasure action |

No new libraries needed - all internal patterns (pgx, Fiber, asynq, next-intl, TanStack Query) already in use.

---

## Patterns to Mirror

### ERASURE ENGINE (extend, do not rewrite)
```go
// SOURCE: internal/pdpa/retention.go:163-198 anonymize() - one tx across candidates+applications+pdpa_consents
// Phase 1 adds the missing stores to this same tx + a post-commit blob/index cleanup:
//   - interview_sessions (redact conversation/summary), onboarding_documents (delete blobs+rows),
//     application_fit_analyses/interview_feedback/offers/letters(blob), notifications.payload,
//     candidate_accounts (+resume blob) + member_notes/tags, activity_logs PII minimization,
//   - Azure Search delete, all linked blobs.
```

### SEARCH DELETE (new path mirroring UpsertBatch)
```go
// SOURCE: internal/search/indexer.go:90-115 (UpsertBatch + indexAction)
// Add Delete(ctx, ids []string): same REST /docs/index POST, action "delete", body {candidate_id} only.
type indexAction struct{ Action string `json:"@search.action"`; ... }
// actions = append(actions, indexAction{Action: "delete", Doc: Doc{CandidateID: id}})
```

### AUDIT WITH ACTOR (mirror member path; generalize)
```go
// SOURCE: internal/members/lifecycle_handler.go:42-53 auditWith({by: actor})
// SOURCE: internal/activity/activity.go:52 Record(ctx, action, entityType, entityID, newValue)
// Phase 5: add actor_id + ip + user_agent COLUMNS to activity_logs (000001 already has ip_address/user_agent
// on the table - wire them through Record so every PDPA-relevant access carries actor+ip).
```

### CONSENT RECORD (extend for version + withdrawal)
```go
// SOURCE: internal/pdpa/pdpa.go:29-51 Record() - inserts pdpa_consents + updates candidates snapshot in one tx
// Phase 2: source consent_version from a registry (not hardcoded "1.0"); add Withdraw() that records a
// consent_given=false ledger row + flags the subject for erasure-eligibility.
```

### NEW ADMIN CRUD PACKAGE (breach register, DSAR queue)
```go
// SOURCE: internal/requisitions/{model,repository,handler}.go (this session) - the canonical
// Repository interface + pgRepository + Handler(gate→BodyParser→validate→httpx envelope→writeErr) + RegisterRoutes.
// RBAC-gate with new perms; mirror exactly.
```

### RETENTION CONFIG (extend)
```go
// SOURCE: pkg/config/config.go:166-172, 322-325 - RETENTION_SWEEP_ENABLED/RETENTION_DAYS(365)/CRON/BATCH
// Phase 1 reuses RETENTION_DAYS=365 (user-chosen); add PDPA_DPO_* + DSAR_* + BREACH_* config.
```

---

## NOT Building
- No change to the lawful basis model beyond consent (the system uses consent + legitimate-interest for recruitment; documenting basis is a ROPA item, not new enforcement logic).
- No automated PDPC breach API submission (no public API exists) - the breach register tracks the 72h obligation + generates the notification content; submission stays manual.
- No e-signature/identity-proofing for DSAR beyond the existing authenticated portal session (self-service is gated by the candidate's own login, per the chosen scope).
- No deletion of `users` (HR staff) PII via DSAR - staff data is employment data on a different basis; out of candidate-DSAR scope.
- No retention change - stays 365 days (user-chosen), hired excluded (existing policy).
- No cross-border re-architecture - Azure/LINE/Google processors stay; Phase 5 only DOCUMENTS them in ROPA + flags region as a deployment-config decision.

---

## PHASE 1 - Complete + unified erasure (FOUNDATION, do first)
Closes the 7 erasure gaps so "delete my data" and the retention sweep actually erase everything.

### Task 1.1: Azure AI Search delete path
- **ACTION**: Add `Delete(ctx, candidateIDs []string) error` to `internal/search` `Indexer` interface + `azureIndexer` + `noopIndexer`.
- **IMPLEMENT**: mirror `UpsertBatch`/`pushChunk` but action `"delete"`; body carries only the key field `candidate_id`. noopIndexer returns nil.
- **MIRROR**: `internal/search/indexer.go:90-115`.
- **GOTCHA**: index key field is `candidate_id` (search/index.go); send only the key for delete.
- **VALIDATE**: `go test ./internal/search/`; unit test builds the delete action JSON.

### Task 1.2: Unified `EraseSubject` engine
- **ACTION**: In `internal/pdpa`, refactor `anonymize` into an exported `EraseSubject(ctx, candidateID)` that erases ALL linked PII in one tx + post-commit external cleanup.
- **IMPLEMENT**: extend the existing tx to also: redact `interview_sessions` (conversation→`'[]'`, summary/strengths/concerns→NULL via application_id join), delete `onboarding_documents` rows (collect blob_urls first), redact `application_fit_analyses`/`interview_feedback` free-text, `offers` (terms/decline_reason), `letters` (collect blob_urls), null `notifications.payload`, and the linked `candidate_accounts` (call the members anonymize logic or inline the same redaction) + `member_notes`/`member_tags`. Post-commit: delete ALL collected blobs (existing 4 + onboarding + letters + account resume) via `BlobDeleter`, and call `search.Indexer.Delete`. Keep `pdpa_anonymized_at` marker + audit `retention_anonymize`/`dsar_erase`.
- **MIRROR**: `retention.go:163-198` (tx + piiBlobs + post-commit), `members/lifecycle.go:118-165` (account redaction).
- **GOTCHA**: collect every blob URL BEFORE redaction; account link is `candidates.account_id`; `full_name` is NOT NULL → sentinel `[ลบข้อมูลแล้ว]`. Make idempotent (re-run safe).
- **VALIDATE**: extend `retention_integration_test.go` - after erase, assert all listed tables/blobs/index cleared.

### Task 1.3: Wire EraseSubject into the retention sweep + members anonymize
- **ACTION**: `Sweep` calls `EraseSubject` (replaces the partial `anonymize`); the members admin anonymize path also calls it (or a shared core) so both routes fully erase + orchestrate the account↔candidate linkage.
- **MIRROR**: `retention.go:64-88` loop; `members/lifecycle_handler.go:156-192`.
- **GOTCHA**: keep the hired-exclusion + active-pipeline eligibility; keep best-effort blob/index failures logged-not-fatal.
- **VALIDATE**: `go test ./internal/pdpa/ ./internal/members/`; sweep still excludes hired.

### Task 1.4: migration 000030 - erasure-support indexes + activity actor columns prep
- **ACTION**: `000030_pdpa_erasure.up.sql` - add any missing indexes for the new erase joins (e.g. `interview_sessions(application_id)` exists; ensure onboarding/letters lookups are indexed) and a `dsar_erase` audit action is free-text so no schema change needed there.
- **VALIDATE**: `migrate up` on a scratch DB (operator); `go build ./...`.

---

## PHASE 2 - Consent v2 (versioning, withdrawal, unify stores)
### Task 2.1: Consent-version registry (migration 000031)
- **ACTION**: `consent_documents` table (version, locale, body/url, effective_at, is_current) seeded with the current notice as `v1.0`. Replace hardcoded `"1.0"` (frontend `ProfileForm.tsx:13`, backend `public/handler.go:162`) with a `GET /api/v1/pdpa/policy/current` lookup.
- **MIRROR**: requisitions migration + repo; `pdpa.go` Record.
- **GOTCHA**: keep accepting old-version consents as valid history; only NEW consents use current version.

### Task 2.2: Unify the two consent stores
- **ACTION**: make `pdpa_consents` (ledger, keyed `candidates.id`) the single source of truth; have the account path (`candidateauth SetConsent`) ALSO write a ledger row (link via `candidates.account_id`), so consent history is one queryable trail. Keep the snapshot columns as a denormalized cache.
- **MIRROR**: `pdpa.go:29-51` Record (ledger+snapshot in one tx).

### Task 2.3: Consent withdrawal endpoint + re-consent
- **ACTION**: `POST /api/v1/pdpa/consent/withdraw` (portal-authed) records a `consent_given=false` ledger row + flags the subject. Re-consent prompt when `current` version > the subject's latest accepted version.
- **GOTCHA**: withdrawal of consent for processing that has no other lawful basis → triggers Phase-1 erasure eligibility (with the hired/legal-hold exception). Audit `consent_withdraw`.
- **VALIDATE**: `go test ./internal/pdpa/`.

---

## PHASE 3 - DSAR self-service (career-portal)
Chosen scope: **self-service on the portal** (authenticated candidate at `/account`).
### Task 3.1: Subject data export (access s.30 + portability s.31)
- **ACTION**: `GET /api/v1/portal/me/export` (candidateauth-gated) returns the subject's COMPLETE record as machine-readable JSON (and a human PDF/print): account + candidates + applications + interview_sessions + onboarding_documents (metadata) + consent history + offers/letters (metadata). 
- **MIRROR**: `members/export.go` (CSV-safe, audited) but scoped to the caller's own `account_id`; envelope `httpx`.
- **GOTCHA**: scope STRICTLY to the authenticated account (never another subject); audit `dsar_export` with actor+ip.
- **VALIDATE**: `go test ./internal/candidateauth/`.

### Task 3.2: Rectification + erasure/withdraw requests (s.36, s.33)
- **ACTION**: portal `/account` UI: "ดาวน์โหลดข้อมูลของฉัน", "แก้ไขข้อมูล", "ถอนความยินยอม / ขอลบข้อมูล". Erasure request calls Phase-1 `EraseSubject` for the caller's own subject (immediate for self-service, subject to hired/legal-hold → if held, create a pending DSAR request for HR instead).
- **MIRROR**: career-portal `app/account/*` + `lib/queries.ts`; ConsentStep.
- **GOTCHA**: hired/legal-hold subjects cannot self-erase → route to a DSAR queue (Phase 5 console) with a clear message. All actions audited + bilingual TH/EN.
- **VALIDATE**: portal `tsc`/`next build`; i18n parity.

---

## PHASE 4 - Privacy notice + cookie/consent transparency
### Task 4.1: Privacy policy pages
- **ACTION**: `/privacy` (and `/pdpa`) pages in BOTH `career-portal` and `frontend`, rendering the current `consent_documents` version, bilingual; link from `SiteFooter` (replace the static text) + `ConsentStep`.
- **MIRROR**: existing static content pages; next-intl namespaces.
- **GOTCHA**: version-stamped + "last updated"; no em dash; both locales (parity).

### Task 4.2: Cookie transparency
- **ACTION**: a short cookie notice (the apps use only essential auth/locale cookies → a concise notice, not a heavy consent wall) + document cookies in the privacy page.
- **VALIDATE**: `next build` both apps; i18n parity.

---

## PHASE 5 - Audit hardening + ROPA + breach register + DPO console
### Task 5.1: Audit actor/IP at row level (migration 000032)
- **ACTION**: ensure every PDPA-relevant access/mutation writes `activity_logs` with `user_id`/actor, `ip_address`, `user_agent` (columns exist on the table; wire them through `activity.Record`). Add `view_resume`/`dsar_*`/`consent_*`/`breach_*` coverage.
- **MIRROR**: `activity/activity.go:52`; `members` auditWith.

### Task 5.2: ROPA (Records of Processing Activities)
- **ACTION**: a `docs/PDPA-ROPA.md` generated from the PII inventory (this plan's data map) + an admin read-only view; lists each processing activity, lawful basis, data categories, retention, and **processors/cross-border destinations** (Azure OpenAI/DocIntel/ACS, LINE, Google, MS Graph, PeopleSoft - region = deployment config).
- **GOTCHA**: cross-border (s.28) - document the Azure region from the endpoint env; flag if outside Thailand/adequate jurisdictions.

### Task 5.3: Breach register + 72h tracking (migration 000033)
- **ACTION**: `data_breaches` table + admin CRUD (record incident, severity, affected subjects, discovered_at, pdpc_notified_at, subjects_notified_at) with a 72h countdown indicator; generate the PDPC notification content.
- **MIRROR**: `internal/requisitions` CRUD + dashboard console.
- **GOTCHA**: 72h clock from `discovered_at` (s.37(4)); RBAC `breach.manage`.

### Task 5.4: DPO config + PDPA admin console
- **ACTION**: `PDPA_DPO_NAME/EMAIL/PHONE` config surfaced on privacy pages + a dashboard **PDPA console** (`/admin/pdpa` or a tab) consolidating: DSAR request queue, consent records lookup, retention status, breach register, ROPA link. RBAC `pdpa.admin`.
- **MIRROR**: requisitions/members console; RolesPermissions admin page.
- **VALIDATE**: full `go test ./...` + dashboard `tsc`/`eslint`/`next build` + i18n parity.

---

## Testing Strategy
Backend: extend `internal/pdpa/retention_integration_test.go` to assert **erasure completeness** (every table/blob/index in the PII map cleared after `EraseSubject`); unit tests for search Delete, consent withdrawal, DSAR export scoping (caller can only export own data), breach 72h logic. Frontend: tsc/eslint/next build + i18n parity for portal DSAR UI + privacy pages + admin console. No new test harness needed (mirror existing Go table-driven + integration tests).

### Edge Cases Checklist
- [ ] Erase a hired subject → blocked/queued (legal-hold), not silently erased
- [ ] Erase already-anonymized subject → idempotent no-op
- [ ] DSAR export scoped to caller only (cannot fetch another account_id)
- [ ] Search index delete when AI_SEARCH_PROVIDER=mock → noop, no error
- [ ] Consent withdrawal with no other lawful basis → erasure-eligible; hired → retained
- [ ] Blob delete failure → logged, not fatal; row still redacted
- [ ] Re-consent prompt only when current version > accepted version

## Validation Commands
```bash
cd backend && gofmt -l internal/pdpa internal/search && go vet ./... && go build ./... && go test ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
cd career-portal && pnpm exec tsc --noEmit && pnpm exec next build
node scripts/check-i18n-parity.mjs   # both apps in parity
# Migrations (operator): migrate up → schema advances per phase (000030..000033)
perl -CSD -ne 'print if /\x{2014}/' <changed files>   # expect zero U+2014 em dashes
```
EXPECT: all green; erasure integration test proves completeness.

## Acceptance Criteria
- [ ] One `EraseSubject` erases EVERY PII store in the inventory (DB + blobs + search index), proven by the integration test
- [ ] Retention sweep + admin anonymize both use it; hired excluded
- [ ] Consent versioned (registry), unified ledger, withdrawal endpoint
- [ ] Portal DSAR: export (access+portability), rectify, withdraw/erase - self-service, audited, bilingual
- [ ] `/privacy` pages live (both apps), versioned, linked
- [ ] Audit carries actor+ip; ROPA documented; breach register + 72h tracking; DPO contact published; PDPA admin console
- [ ] All validation green; 0 em dashes

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Incomplete erasure misses a PII store | Med | **High (compliance)** | Phase 1 integration test asserts EVERY table/blob/index in the inventory; this plan's PII map is the checklist |
| Self-service erasure deletes data under legal hold (hired/active) | Med | High | hired/active-pipeline exclusion enforced; held requests routed to HR DSAR queue, not auto-erased |
| Search/blob external delete fails silently | Med | Med | best-effort + logged + audited; a re-sweep retries; mock provider = noop |
| Cross-border transfer (Azure/LINE offshore) flagged in audit | High | Med (legal) | ROPA documents region from endpoint config; surface as a deployment/legal decision, not silent |
| XL scope shipped at once | High | Med | 5 phases, each independently shippable + reviewable; Phase 1 first |

## Notes
- **Implement phase-by-phase** via `/prp-implement` (this doc is the master plan). Recommend Phase 1 first (foundation + highest risk), then 2→5. Each phase = its own branch/PR/deploy. I can split each phase into its own plan file on request.
- Retention already = **365 days** (user-chosen; `RETENTION_DAYS` default), hired excluded - no change.
- DSAR = **portal self-service** (user-chosen); held/complex requests fall back to the Phase-5 HR/DPO console.
- Deploy footprint: Phases 1-2-5 touch api+worker+scheduler (+migrations) and the dashboard; Phases 3-4 touch career-portal + dashboard. No new external services.
- Confidence: high for Phase 1-3 (extends well-mapped internal code); Phase 5 breach/ROPA has a policy/legal component the org must also action outside code.
