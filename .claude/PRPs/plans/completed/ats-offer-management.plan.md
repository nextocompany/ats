# Plan: ATS Slice 3 — Offer Management (Module-3 item 3.6)

## Summary
After the 4-level approval chain advances an application to `offer`, HR (hr_manager/super_admin) composes an **offer package** (salary, start date, terms, optional expiry), **sends** it to the candidate, and the candidate — logged into the career-portal membership — **accepts or declines**. Accept → application `hired` + best-effort **PeopleSoft push** (the existing `psService.SyncHired` seam, auto-fired for the first time). Decline → application `rejected` with the decline reason. The offer's own lifecycle (`draft → sent → accepted/declined/expired`) lives on a new `offers` table; the application funnel reuses `offer → hired/rejected`.

## User Story
As **HR (hr_manager)** I want to send a structured offer a candidate can accept or decline online, so the hire is recorded, the candidate is notified, and an accepted hire flows to PeopleSoft — without manual re-keying.

## Problem → Solution
Today `offer` is a bare status with no payload, no candidate-facing step, and `psService.SyncHired` is wired but never auto-fired. → An `offers` table + HR compose/send endpoints + a membership-authenticated accept/decline path + auto PS push on accept.

## Metadata
- **Complexity**: Large (~24 files; migration 000023; backend offers domain/repo/2 handlers; dashboard panel; career-portal page)
- **Branch**: `feat/ats-offer-management` (stacked on `feat/ats-approval-workflow` / PR #82)
- **Estimated Files**: ~24

## Design Decisions (LOCKED — user answers + survey)
1. **Scope = all four**: compose/edit offer package · send + candidate accept/decline · offer state machine · PeopleSoft push on accept.
2. **Accept channel = career-portal membership login** (NOT the public read-only token). Candidate identity via `candidateauth` cookie session (`CandidateFromCtx`); responses scoped to the account that owns the application (`candidates.account_id`).
3. **Write gate (HR) = `hr_manager` + `super_admin`** (`offerWriteRoles` map, mirroring scorecard `taRecordRoles`).
4. **Offer lifecycle lives on the `offers` table** (`draft|sent|accepted|declined|expired`) — the "offer state machine". The application funnel is NOT bloated with new statuses: app stays `offer` while draft/sent; **accept → `hired`** (reusing the legacy-but-now-meaningful terminal + `hired_at`, which is exactly what `psService.SyncHired` expects); **decline → `rejected`** (reason persisted to `applications.rejection_reason`). These two app transitions are owned by the respond endpoint inside a transaction (like the approval decide owns `pending_approval`'s exits).
5. **One offer per application** (`offers.application_id UNIQUE`). Compose is an upsert-style create then PATCH-edit while `draft`; editable until sent.
6. **Expiry = stored + enforced at respond time** (no new worker this slice): `expires_at` optional; accept/decline rejected with 409 if past expiry, and a `sent` offer past `expires_at` reads as `expired`. (SLA-style sweep was already added in 3.5; not repeated here.)
7. **PS push is best-effort** (mock on prod): accept commits the hire regardless; `SyncHired` failure never fails the accept (matches `peoplesoft/service.go` which swallows + CSV-fallbacks).

### NOT building
- Offer letter / PDF generation (that is slice 3.3).
- Negotiation / counter-offer / multiple offers per application.
- Auto-expiry worker (enforced lazily at respond time).
- Public-token (no-login) acceptance — membership login only.
- Editing an offer after it is sent (must reject+recreate; out of scope v1 — note it).

---

## Data Model — migration 000023
```sql
-- 000023_offers.up.sql
-- Offer package for a hired-track application (Module-3 3.6). One offer per
-- application; its lifecycle is draft→sent→accepted/declined/expired. Accept flips
-- the application to 'hired' (+ PeopleSoft push); decline flips it to 'rejected'.
CREATE TABLE IF NOT EXISTS offers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'draft', -- draft | sent | accepted | declined | expired
    salary         NUMERIC(12,2),
    start_date     DATE,
    terms          TEXT,
    created_by     UUID REFERENCES users(id),
    sent_at        TIMESTAMPTZ,
    responded_at   TIMESTAMPTZ,
    expires_at     TIMESTAMPTZ,
    decline_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers (status);
```
```sql
-- 000023_offers.down.sql
DROP TABLE IF EXISTS offers;
```

---

## Patterns to Mirror (from prior slices — already in this codebase)
- **HR handler + per-role allowlist + narrow store interface + RegisterXxxRoutes**: `feedback_handler.go` / `approval_handler.go`. Gate via `u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)` + `offerWriteRoles[u.Role]` → 403; `ExistsInScope` for per-record auth; `httpx.OK/Created`; `fiber.NewError` for 4xx.
- **Repository tx + INSERT…RETURNING + `applications: %w` wrap + `pgx.ErrNoRows`→(nil,nil)**: `approval_repository.go`, `repository.go` (`CreateFeedback`, `FindAppointment`).
- **Account-scoped join chain** `candidate_accounts → candidates → applications`: `members/repository.go:271-274` (template for `ListOffersByAccount` with `WHERE c.account_id = $1`).
- **Candidate-auth gated route + `CandidateFromCtx`**: `candidateauth/routes.go` (per-route `gate := RequireCandidate(svc, cookieName)`), `candidateauth/middleware.go:63`. Register new gated routes in `candidateauth/routes.go` or in `main.go` like `QuickApply`.
- **Candidate notify seam**: `notify.statusBody` (`notify/message.go`) + `statusNotifyDeps.notifyStatusChange` (`applications/notify.go`). Add an `offer` case to `statusBody`; the send handler fires `notifyStatusChange(ctx, repo, appID, StatusOffer)`.
- **PS push**: `peoplesoft.Service.SyncHired(ctx, applicationID)` — inject as a narrow `hiredSyncer` interface (shape already at `applications/handler.go:29-31`).
- **Frontend dashboard panel**: `ApprovalPanel.tsx` / `Scorecards.tsx` (role-gated form + `mutate` + toasts + semantic tokens). **Use `mutate`, not `mutateAsync`** (the 3.5 review lesson).
- **Career-portal authed page**: `career-portal/app/account/page.tsx` (`useCandidate()` + redirect-to-login gate); `career-portal/lib/api.ts` (`credentials:"include"`); `lib/auth.ts` typed helpers; `lib/types.ts`; next-intl `useTranslations`.

---

## Files to Change

### Backend
| File | Action | Notes |
|---|---|---|
| `migrations/000023_offers.{up,down}.sql` | CREATE | offers table |
| `internal/applications/offer.go` | CREATE | domain: status consts, `offerWriteRoles`, `Offer`/`OfferInput`/`OfferResponseInput`/`OfferView` structs, validators, decision consts |
| `internal/applications/offer_repository.go` | CREATE | `CreateOffer`, `UpdateOffer`, `GetOfferByApplication`, `GetOfferByID`, `SendOffer`, `RespondOffer` (tx: offer + app status), `ListOffersByAccount` |
| `internal/applications/offer_handler.go` | CREATE | HR `OfferHandler`: Create / Update / Send / Get + `RegisterOfferRoutes`; role gate hr_manager+super_admin; `SetNotifier` (statusNotifyDeps reuse) |
| `internal/applications/offer_candidate_handler.go` | CREATE | candidate `OfferCandidateHandler`: ListMine / Respond; `RegisterCandidateOfferRoutes(app, h, gate)`; uses `candidateauth.CandidateFromCtx`; injects `hiredSyncer` for PS push on accept |
| `internal/applications/repository.go` | UPDATE | add the 7 offer methods to `Repository` interface |
| `internal/applications/transitions.go` | UPDATE | doc: `offer → hired` (accept, endpoint-owned) ; `offer → rejected` already present |
| `internal/notify/message.go` | UPDATE | add `offer` case to `statusBody` (candidate "ได้รับข้อเสนอ" copy) |
| `cmd/api/main.go` | UPDATE | construct + register both offer handlers (HR after authMW; candidate under `RequireCandidate`, origin-guarded) |

### Backend tests
| File | Action |
|---|---|
| `internal/applications/offer_test.go` | CREATE — HR gate (403 non-hr_manager), create-from-wrong-status 400, send validation, get; candidate respond gates (not-mine 404, not-sent 409, expired 409, accept→hired path, decline→rejected+reason) |

### Frontend — dashboard (`frontend/`)
| File | Action |
|---|---|
| `lib/types.ts` | UPDATE — `Offer`, `OfferInput`, `OfferStatus` |
| `lib/queries.ts` | UPDATE — `useOffer(appId)` (404→null), `useSaveOffer(appId)`, `useSendOffer(appId)` |
| `lib/roles.ts` | UPDATE — `OFFER_ROLES` + `canManageOffer` |
| `components/resume/OfferPanel.tsx` | CREATE — compose/edit form + Send (role-gated); shows status + candidate response |
| `app/(app)/applications/[id]/page.tsx` | UPDATE — render `<OfferPanel>` (only when status is offer/hired/rejected-with-offer) |
| `messages/en.json` + `th.json` | UPDATE — `offer.*` block (parity) |

### Frontend — career-portal (`career-portal/`)
| File | Action |
|---|---|
| `lib/types.ts` | UPDATE — `Offer`, `OfferResponseInput` |
| `lib/auth.ts` | UPDATE — `getMyOffers()`, `respondToOffer(id, decision, reason?)` |
| `lib/queries.ts` | UPDATE — offers query + respond mutation |
| `app/offers/page.tsx` | CREATE — session-gated; lists offers; accept/decline (decline needs reason) |
| `components/StatusCard.tsx` | UPDATE — add `offer`, `pending_approval`, `hired` already present |
| `messages/en.json` + `th.json` | UPDATE — `offers.*` block (parity) |

---

## Step-by-Step Tasks

1. **Migration 000023** — offers table (above). Mirror 000022 conventions. VALIDATE: visual vs 000020-22.
2. **offer.go** — consts `OfferDraft/Sent/Accepted/Declined/Expired`; `OfferDecisionAccept/Decline` + `validOfferDecision`; `offerWriteRoles = {hr_manager, super_admin}` + `canManageOffer(role)`; structs `Offer` (id, application_id, status, salary *float64, start_date *time.Time as date, terms, sent_at/responded_at/expires_at *time.Time, decline_reason, created_at), `OfferInput` (salary, start_date, terms, expires_at), `OfferResponseInput` (decision, reason), `OfferView` (Offer + candidate_name/position_title/store_id/app status for the candidate list); `ValidateOfferForSend` (salary>0, start_date set); `IsExpired(now)` helper. MIRROR feedback.go/approval.go. VALIDATE: `go build ./internal/applications/`.
3. **offer_repository.go** — methods + add to `Repository` interface:
   - `CreateOffer(ctx, applicationID, createdBy, in OfferInput) (Offer, error)` — INSERT (status draft) … RETURNING; unique-violation on application_id → return a sentinel `ErrOfferExists`.
   - `UpdateOffer(ctx, applicationID, in OfferInput) (Offer, error)` — UPDATE … WHERE application_id=$ AND status='draft' RETURNING; RowsAffected==0 → `ErrOfferNotEditable`.
   - `GetOfferByApplication(ctx, appID)`/`GetOfferByID(ctx, id)` — (nil,nil) on no row; lazy-expire: if status='sent' and expires_at<now, report status 'expired' in the returned struct (do not mutate DB on read — keep read pure; the respond tx enforces).
   - `SendOffer(ctx, appID) (Offer, error)` — UPDATE status='sent', sent_at=NOW() WHERE application_id=$ AND status='draft' RETURNING; RowsAffected==0 → `ErrOfferConflict`.
   - `RespondOffer(ctx, offerID, accountID, accept bool, reason string) (Offer, error)` — **tx**: SELECT offer JOIN application JOIN candidate FOR UPDATE, verify `candidates.account_id = $accountID` (else ErrOfferNotFound), status='sent', not expired (else ErrOfferConflict); on accept → offer status='accepted', responded_at=NOW(); `UPDATE applications SET status='hired', hired_at=NOW()`; on decline → offer status='declined', decline_reason, responded_at; `UPDATE applications SET status='rejected', rejection_reason=$`; commit; return.
   - `ListOffersByAccount(ctx, accountID) ([]OfferView, error)` — join chain `candidate_accounts→candidates→applications→offers WHERE c.account_id=$1 AND o.status IN ('sent','accepted','declined','expired')` + position title; ORDER BY o.created_at DESC.
   MIRROR approval_repository.go (tx, FOR UPDATE, sentinels, %w wrap). VALIDATE: `go build && go vet`.
4. **offer_handler.go (HR)** — `offerStore` narrow iface; `OfferHandler{apps, notify statusNotifyDeps}`; routes: `POST /applications/:id/offer` (create), `PATCH /applications/:id/offer` (update), `POST /applications/:id/offer/send` (send+notify), `GET /applications/:id/offer` (read). Create/Update/Send gate: `canManageOffer(u.Role)` (403), `ExistsInScope` (404). Create requires `app.Status==StatusOffer` (400). Send requires `ValidateOfferForSend` (400) then `SendOffer`; on success `h.notify.notifyStatusChange(ctx, repo, appID, StatusOffer)`. Map `ErrOfferExists`→409, `ErrOfferNotEditable`/`ErrOfferConflict`→409. MIRROR feedback_handler.go. VALIDATE: build+vet.
5. **offer_candidate_handler.go** — `OfferCandidateHandler{apps offerCandidateStore, hired hiredSyncer}`; `RegisterCandidateOfferRoutes(app, h, gate fiber.Handler)`: `GET /api/v1/public/auth/offers` (ListMine), `POST /api/v1/public/auth/offers/:id/respond` (Respond). Both read `acct := candidateauth.CandidateFromCtx(c)` (401 if nil). Respond: parse decision (accept|decline; decline needs reason→400), `RespondOffer(ctx, offerID, acct.ID, accept, reason)`; map `ErrOfferNotFound`→404, `ErrOfferConflict`→409; on accept, best-effort `h.hired.SyncHired(ctx, offer.ApplicationID)` (log on error, never fail). MIRROR candidateauth handler shape. GOTCHA: verify `applications` importing `candidateauth` does not cycle (build will catch; if it does, move this handler's identity read to accept an injected `accountFromCtx func(*fiber.Ctx) (uuid.UUID, bool)` set in main.go). VALIDATE: build+vet.
6. **transitions.go** — update the doc comment to describe `offer → hired` (accept, endpoint-owned, in tx) alongside the existing `offer → rejected`. No map change needed (hired set directly in the respond tx; reject already allowed). VALIDATE: build.
7. **notify/message.go** — add `case StatusOffer` (use the literal `"offer"`) to `statusBody`: subject/body "คุณได้รับข้อเสนอการจ้างงาน… ดูและตอบรับได้ที่ <portal>/offers". Keep `hired` copy as-is. VALIDATE: `go test ./internal/notify/`.
8. **cmd/api/main.go** — build `offerHandler := applications.NewOfferHandler(appRepo)` + `SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)`; `RegisterOfferRoutes(app, offerHandler)` (after authMW). Build `offerCandHandler := applications.NewOfferCandidateHandler(appRepo, psService)`; register under origin-guard + `candidateauth.RequireCandidate(caSvc, cfg.SessionCookieName)` gate, mirroring `QuickApply` (main.go:253). Ensure the `/api/v1/public/auth/offers*` prefix is origin-guarded. VALIDATE: `go build ./cmd/...`.
9. **Backend tests** (offer_test.go) — fakes for `offerStore` + `offerCandidateStore` + a fake `hiredSyncer` (records calls) + fiber harness (HR DevUser locals; candidate via a middleware that sets `candidateauth` locals — use `candidateauth.CandidateFromCtx`'s key by calling a tiny helper, or test the repository-facing logic). Cases per the test table. VALIDATE: `go test ./internal/applications/... -run Offer`.
10. **Dashboard**: types + roles + queries + `OfferPanel.tsx` + wire into detail page + i18n (both locales). OfferPanel: if status not in {offer,hired,rejected} → null; if no offer & status==offer & canManageOffer → compose form; if draft → edit + Send; if sent/accepted/declined → read-only status view. Use `mutate`. VALIDATE: `tsc`, `eslint`, parity, `next build`.
11. **Career-portal**: types + `lib/auth.ts` helpers + `lib/queries.ts` + `app/offers/page.tsx` (gated, accept/decline; decline reason required) + `StatusCard.tsx` add `offer` case + i18n `offers.*` both locales. VALIDATE: `tsc` (career-portal), parity, `next build` (career-portal).

---

## Validation Commands
```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
cd career-portal && pnpm exec tsc --noEmit && pnpm exec next build
node scripts/check-i18n-parity.mjs    # both apps, both locales
# migration (operator / when DB available): migrate up → down 1 → up  (schema 22→23)
```

## Acceptance Criteria
- [ ] HR (hr_manager/super_admin only) composes, edits (while draft), and sends an offer from a `offer`-status application
- [ ] Sending notifies the candidate (email/LINE; `offer` case added)
- [ ] Logged-in candidate sees their offer in career-portal and accepts/declines (decline requires reason)
- [ ] Accept → application `hired` + best-effort PeopleSoft `SyncHired` fired (mock on prod); decline → `rejected` with reason; both atomic
- [ ] Expired/already-responded offers reject the response with 409
- [ ] All validation green; i18n parity (both apps)

## Risks
| Risk | Mitigation |
|---|---|
| `applications` → `candidateauth` import cycle | Build catches immediately; fallback = inject `accountFromCtx` func from main.go |
| Multi-write atomicity (offer + app status) | single tx in `RespondOffer`, FOR UPDATE on the offer/app row |
| PS push fails on accept | best-effort: log + continue; hire already committed (matches peoplesoft/service.go) |
| account-scope leak (candidate responds to another's offer) | `RespondOffer` verifies `candidates.account_id = acct.ID` inside the tx → ErrOfferNotFound |
| salary as money | `NUMERIC(12,2)` in DB, `*float64` in Go, render with thousands sep in UI |

## Notes
- Stacked on PR #82 (approval). Deploy order when both merge: migrate 000022 then 000023, then roll api/worker/scheduler/dashboard/portal.
- First auto-firing of `psService.SyncHired` — verify the mock logs `[mock peoplesoft] create_applicant` on accept in dev.
- Next ATS slice after this: 3.3 Interview/Offer Letter (PDF), then 3.8 Onboarding.
