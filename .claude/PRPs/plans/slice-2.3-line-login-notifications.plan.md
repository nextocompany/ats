# Plan: LINE Login + Notifications (Sprint 8 slice 2.3)

## Summary
The LINE and Notify seams are built and mock-defaulted, but three things block real use:
(1) the frontend LINE Login is a **stub** (`career-portal/lib/line.ts` returns `dev-line-id-token`);
(2) the backend verifies the LINE id-token at apply time but **discards the returned LINE user id**
(`internal/public/handler.go` → `if _, err := h.verifier.Verify(...)`), so we never persist the one
value real LINE push needs; and (3) notifications only fire on **re-engagement**, and that path
addresses LINE by `phone` (wrong — Messaging API needs a LINE user id). This slice flips LINE Login
and Notify to real, **persists `line_user_id`** at apply, **fixes the notify recipient**, adds
**status-change notifications** (shortlisted / interview / hired), and writes the LINE/Notify runbook.

## User Story
As a **candidate**, I want to sign in with my real LINE account when I apply, and receive a LINE
message when my application advances (shortlisted / interview / hired) or when a new matching job
opens — so I stay informed without checking the status page. As **HR / the business**, we want real
LINE Login (LIFF id-token verified server-side) and real LINE push delivery in staging, with a
runbook, so slice 2.3 can pass Phase-2 exit.

## Problem → Solution
- **Stub LINE Login** → enable LIFF in the portal (`@line/liff`, `NEXT_PUBLIC_LIFF_ID`) + flip
  `LINE_PROVIDER=real` (+ `LINE_CHANNEL_ID`). The backend `realVerifier` already exists.
- **LINE user id thrown away** → capture `Verify()`’s `LineUser.Subject`, thread it through
  `IntakeInput` → `candidates.line_user_id` (new column).
- **Notify only on re-engagement, addressed by phone** → fix `pickChannel` to prefer the stored
  `line_user_id`; add a best-effort notify hook on status change (single PATCH + bulk); flip
  `NOTIFY_PROVIDER=real` (+ `NOTIFY_LINE_TOKEN`). Email stays a stub (documented).

## Metadata
- **Complexity**: L (frontend LIFF + 1 migration + backend persist/notify wiring + runbook) — **3 phases**
- **Source PRD**: `.claude/PRPs/plans/sprint-8-go-live-roadmap.md` (slice 2.3, Phase 2)
- **PRD Phase**: Phase 2 — Provision + Flip Real Seams → 2.3 LINE Login + Notifications
- **Estimated Files**: ~14 (3 frontend, 1 migration, ~7 backend edited, 1 backend test, 1 runbook, config)
- **Decision (locked)**: notify is **best-effort, non-blocking** everywhere (mirrors re-engagement + the AI-Search indexer hook) — a notify failure never fails an apply or a status change.
- **Mock-default invariant**: `LINE_PROVIDER=mock` + `NOTIFY_PROVIDER=mock` (CI/local) → zero external calls, no creds. New code paths must no-op/log under mock.
- **Scope guard**: real **LINE push** only. Real **email/SMTP is NOT wired** in this slice (stub returns an error; `NOTIFY_EMAIL_FROM` is read but not fail-fast) — documented, not built.

---

## UX Design
- **Career portal apply, Step 3** (`components/ApplyStepper.tsx` + `LineLoginButton.tsx`): unchanged
  layout. With LIFF on, “เข้าสู่ระบบด้วย LINE” opens the **real** LINE login (inside the LINE in-app
  browser it’s seamless; in a normal browser it redirects to LINE). On success the button shows
  “เชื่อมต่อ LINE แล้ว ✓” exactly as today — the only change is the token is real.
- **Candidate-facing notifications**: a LINE text message (no new UI). Status page (`/status`) remains
  the pull-based source of truth; notifications are an additive push.
- **HR dashboard**: no UI change. Status-change buttons already exist; they simply also enqueue a
  candidate notification now.

---

## Mandatory Reading

| Priority | File | Why |
|---|---|---|
| P0 | `backend/internal/auth/line.go` | `Verifier` seam: `mockVerifier` vs `realVerifier` (calls `https://api.line.me/oauth2/v2.1/verify`), `LineUser{Subject,Name,Email}`, `NewVerifier(cfg)` gate `UsesRealLINE()`. **Real impl already done** — we only need to *use* `Subject`. |
| P0 | `backend/internal/public/handler.go` (Apply ~74–160) | **The discard bug**: `if _, err := h.verifier.Verify(...)`. Change to capture the `LineUser` and pass `Subject` into `IntakeInput`. |
| P0 | `backend/internal/applications/service.go` (IntakeInput ~26, Intake ~70–110) | Add `LineUserID` to `IntakeInput`; set it on candidate create. This is the persistence path. |
| P0 | `backend/internal/notify/{notify,mock,rest}.go` | `Notifier` interface, `Message{Channel,Recipient,Subject,Body}`, `NewNotifier(cfg)`; real `rest.go` LINE push `POST https://api.line.me/v2/bot/message/push`; email = stub error. Channels `ChannelLINE`/`ChannelEmail`. |
| P0 | `backend/internal/reengage/service.go` (pickChannel ~81–91) | **Recipient bug**: uses `phone` as the LINE handle. Fix to prefer `line_user_id`. The message template + portal-link pattern to mirror for status-change messages. |
| P0 | `backend/internal/applications/handler.go` (UpdateStatus ~147–175) | Single status PATCH — add a best-effort notify hook after `SetStatus` succeeds. |
| P0 | `backend/internal/applications/dashboard_handler.go` (Bulk ~125–168) | Bulk status change — same notify hook per updated id. Mirror the existing best-effort `indexer.Index` call already there. |
| P1 | `backend/pkg/config/config.go` | `LINE_PROVIDER`/`LINE_CHANNEL_ID`, `NOTIFY_PROVIDER`/`NOTIFY_LINE_TOKEN`/`NOTIFY_EMAIL_FROM`, `PORTAL_BASE_URL`; the `Validate` allow-list + fail-fast (real LINE requires channel id; real notify requires token). Already complete — confirm only. |
| P1 | `backend/internal/candidates/repository.go` (UpdateProfileFields ~83, SetCanonical ~131) | Column-setter pattern + the create path to add `line_user_id`. |
| P1 | `career-portal/lib/line.ts` + `components/LineLoginButton.tsx` + `lib/queries.ts` (`X-LINE-IdToken`) | Stub → LIFF. `isLiffConfigured()` already gates on `NEXT_PUBLIC_LIFF_ID`. Token already sent as `X-LINE-IdToken`. |
| P2 | `backend/internal/reengage/service.go` (full) + `internal/peoplesoft/webhook.go` | Existing notify call-site to keep consistent; confirms re-engagement now benefits from the recipient fix. |
| P2 | `docs/azure-search-provisioning.md` | Runbook format to mirror for `docs/line-notify-provisioning.md`. |
| P2 | `backend/migrations/000001_init_schema.up.sql` (candidates) | Column conventions for the new migration. |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Verify LIFF id token | developers.line.biz/en/reference/line-login/#verify-id-token | `POST https://api.line.me/oauth2/v2.1/verify` form `id_token`,`client_id=<channel id>` → returns `sub` (LINE user id), `name`, `email` (email only if scope granted). **Already implemented in `line.go`.** |
| LIFF init / login / getIDToken | developers.line.biz/en/reference/liff/ | `liff.init({liffId})` → `liff.isLoggedIn()` → `liff.login()` → `liff.getIDToken()`. In the LINE in-app browser the user is already logged in. id-token aud = LIFF channel id → must equal backend `LINE_CHANNEL_ID`. |
| LINE Messaging push | developers.line.biz/en/reference/messaging-api/#send-push-message | `POST https://api.line.me/v2/bot/message/push` Bearer `<channel access token>`, body `{"to":<userId>,"messages":[{"type":"text","text":...}]}`. **`to` MUST be a LINE user id** (the `sub` from id-token) — not phone/email. Push requires the user has **added the Messaging-API bot as a friend**. |
| LIFF vs Messaging channels | developers.line.biz | Login/LIFF and Messaging API are **separate channels**. `sub` is **per-provider**, so the LIFF login channel and the Messaging bot must be under the **same LINE provider** for the `to` id to be valid. (Critical provisioning note.) |

```
KEY_INSIGHT: The single value that unblocks real LINE push is LineUser.Subject, which the Apply
handler currently throws away. Persisting it (candidates.line_user_id) is the spine of this slice.
APPLIES_TO: Phase A (Tasks A2–A4)
GOTCHA: `sub` is stable only within one LINE provider. If the LIFF login channel and the Messaging
push bot live under DIFFERENT providers, the stored id won't be pushable. Provision both under ONE
provider (runbook).

KEY_INSIGHT: Real push needs the candidate to have added the bot as a friend. A candidate who logs
in via LIFF but never friends the bot has a valid `sub` but push returns an error.
APPLIES_TO: runbook + best-effort error handling (never fail the flow)
GOTCHA: rest.go already returns an error on status>=300 — keep notify best-effort so a "not a friend"
push error is logged, not fatal.

KEY_INSIGHT: Notify is additive + best-effort. The status page stays the source of truth.
APPLIES_TO: status-change hooks (Phase B) — mirror the existing best-effort indexer.Index in Bulk.
```

---

## Patterns to Mirror

### NOTIFY_BEST_EFFORT (status-change hook; mirror Bulk's indexer call)
```go
// after SetStatus succeeds, never fail the request on a notify error
if cand, err := h.cands.FindByID(ctx, app.CandidateID); err == nil {
    msg := notify.StatusMessage(cand, app, target, cfg.PortalBaseURL) // builds channel+recipient+body
    if msg.Recipient != "" {
        if err := h.notifier.Send(ctx, msg); err != nil {
            log.Warn().Err(err).Str("application", id.String()).Msg("status notify failed (non-fatal)")
        }
    }
}
```

### RECIPIENT_FIX (reengage.pickChannel → prefer line_user_id)
```go
func pickChannel(t Target) (channel, recipient string) {
    if t.LineUserID != "" { return notify.ChannelLINE, t.LineUserID } // real push handle
    if t.Email != ""      { return notify.ChannelEmail, t.Email }      // email = stub for now
    return "", ""                                                       // no reachable channel → skip
}
```

### LIFF_REAL (career-portal/lib/line.ts — replace the stub body)
```ts
import liff from "@line/liff";
export async function getIdToken(): Promise<string> {
  if (!isLiffConfigured()) return DEV_STUB_TOKEN;        // keep mock when LIFF id unset (CI/local)
  await liff.init({ liffId: process.env.NEXT_PUBLIC_LIFF_ID! });
  if (!liff.isLoggedIn()) liff.login();
  return liff.getIDToken() ?? "";
}
```

### CONFIG_GATE (already present — confirm, don't rebuild)
`UsesRealLINE()` → `LINE_PROVIDER=="real"` (requires `LINE_CHANNEL_ID`); `NewNotifier` switches on
`NOTIFY_PROVIDER` (real requires `NOTIFY_LINE_TOKEN`). Mock-default everywhere else.

---

## Files to Change

**New**
- `backend/migrations/0000XX_candidate_line_user_id.up.sql` / `.down.sql` — add `candidates.line_user_id TEXT`
- `backend/internal/notify/message.go` — `StatusMessage(...)` + `ReengageMessage(...)` builders (Thai copy, portal links)
- `docs/line-notify-provisioning.md` — runbook
- `backend/internal/notify/message_test.go` — message builder unit tests

**Edited — backend**
- `internal/public/handler.go` — capture `LineUser`, pass `Subject` to Intake
- `internal/applications/service.go` — `IntakeInput.LineUserID` + set on candidate create
- `internal/candidates/repository.go` (+ `model.go`) — persist/read `line_user_id`
- `internal/applications/handler.go` — notify hook on single status change; inject `Notifier`+`candidates` dep
- `internal/applications/dashboard_handler.go` — notify hook in Bulk; inject deps
- `internal/reengage/service.go` — `Target.LineUserID` + `pickChannel` fix; use `ReengageMessage`
- `cmd/api/main.go` — wire `NewNotifier(cfg)` + candidates repo into the application handlers
- `pkg/config/config.go` — confirm only (no change expected)

**Edited — frontend**
- `career-portal/lib/line.ts` — real LIFF body (gated by `NEXT_PUBLIC_LIFF_ID`)
- `career-portal/package.json` — add `@line/liff`
- `career-portal/.env.example` (or docs) — document `NEXT_PUBLIC_LIFF_ID`

## NOT Building
- Real email/SMTP delivery (stub stays; documented).
- A "friend the bot" onboarding flow / rich messages (flex/templates) — plain text only.
- Notifications on apply-received or on parse/score completion (only status-change + re-engagement).
- HR-facing notification preferences / opt-out UI.
- Backfilling `line_user_id` for the 14 demo candidates (they have no real LINE id; demo push stays mock).

---

## Step-by-Step Tasks

### Phase A — Real LINE Login + persist `line_user_id`

#### Task A1: Migration — `candidates.line_user_id`
- **ACTION**: Add nullable `line_user_id TEXT` to `candidates` (+ down drop).
- **GOTCHA**: nullable; no unique constraint (one LINE id could legitimately reapply; dedup already handled separately).
- **VALIDATE**: `migrate up` then `\d candidates` shows the column.

#### Task A2: Persist through Intake
- **ACTION**: Add `LineUserID string` to `IntakeInput`; set it on the `candidates.Create`/profile path.
- **MIRROR**: existing `UpdateProfileFields` column-setter; include in the create insert.
- **VALIDATE**: `go build ./internal/applications/ ./internal/candidates/`.

#### Task A3: Stop discarding the LINE user id (Apply)
- **ACTION**: `lu, err := h.verifier.Verify(...)`; on success pass `lu.Subject` into `IntakeInput.LineUserID`.
- **GOTCHA**: mock verifier returns `Subject="U-dev-"+token` — fine for local; real returns the true `sub`.
- **VALIDATE**: unit (httptest) — apply with a token → candidate row has `line_user_id` set.

#### Task A4: Frontend LIFF (real, gated)
- **ACTION**: `pnpm add @line/liff`; replace `getIdToken()` body with `LIFF_REAL` (keep stub when `NEXT_PUBLIC_LIFF_ID` unset).
- **GOTCHA**: `@line/liff` is browser-only — keep the dynamic `import`/guard so SSR/build doesn’t execute it; `isLiffConfigured()` already guards.
- **VALIDATE**: `pnpm build` (career-portal) green with and without `NEXT_PUBLIC_LIFF_ID`.

### Phase B — Notifications (recipient fix + status-change + flip)

#### Task B1: Message builders (`internal/notify/message.go`)
- **ACTION**: `StatusMessage(cand, app, status, portalURL) Message` and `ReengageMessage(...)` — Thai copy per status (shortlisted/interview/hired; skip others), channel+recipient via the stored `line_user_id`.
- **VALIDATE**: unit — each status → expected subject/body; non-notifiable status → empty Recipient (skipped).

#### Task B2: Recipient fix in re-engagement
- **ACTION**: add `LineUserID` to `reengage.Target` + query; `pickChannel` prefers it; use `ReengageMessage`.
- **VALIDATE**: existing reengage tests pass; new case: target with `line_user_id` → ChannelLINE+id.

#### Task B3: Status-change notify hooks
- **ACTION**: inject `Notifier` + `candidates.Repository` into `Handler` (PATCH `UpdateStatus`) and `DashboardHandler` (`Bulk`); add the `NOTIFY_BEST_EFFORT` hook after `SetStatus`.
- **MIRROR**: the best-effort `indexer.Index` already in `Bulk`.
- **GOTCHA**: best-effort — log + continue on error; do not change the HTTP response on notify failure.
- **VALIDATE**: unit — `hired` → notifier.Send called with ChannelLINE + candidate’s `line_user_id`; notify error → 200 still returned.

#### Task B4: Wire deps in `cmd/api/main.go`
- **ACTION**: build `NewNotifier(cfg)` once; pass it + candidates repo into both handlers.
- **VALIDATE**: `go build ./...`; mock provider → no external calls.

### Phase C — Provision, flip, validate

#### Task C1: Runbook (`docs/line-notify-provisioning.md`)
- **ACTION**: step-by-step: create LINE provider; **LINE Login channel** (+ LIFF app → `NEXT_PUBLIC_LIFF_ID`, `LINE_CHANNEL_ID`); **Messaging API channel under the SAME provider** (→ `NOTIFY_LINE_TOKEN`); friend-the-bot note; env table; `NOTIFY_EMAIL_FROM` read-but-not-fail-fast caveat; rollback = flip flags to mock.
- **VALIDATE**: doc lists every env + the same-provider gotcha + friend requirement.

#### Task C2: Flip in staging + smoke
- **ACTION**: set `LINE_PROVIDER=real`+`LINE_CHANNEL_ID`, `NOTIFY_PROVIDER=real`+`NOTIFY_LINE_TOKEN`, `NEXT_PUBLIC_LIFF_ID` (dashboard/portal build-arg), `PORTAL_BASE_URL`. Rebuild career-portal + roll api/worker.
- **VALIDATE**: see Manual Validation.

---

## Testing Strategy

### Unit / Component Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Apply persists line id | apply w/ token (mock verifier) | candidate.line_user_id == verifier Subject | — |
| StatusMessage | status=hired, cand w/ line id | ChannelLINE, recipient=line id, Thai body | — |
| StatusMessage skip | status=parsed/scored | Recipient=="" (no push) | skip path |
| pickChannel prefers LINE | target w/ line id + email | ChannelLINE + line id | recipient fix |
| pickChannel fallback | target w/ email only | ChannelEmail + email | fallback |
| pickChannel none | no line id / email | "" (skipped) | empty |
| status notify best-effort | notifier returns error | handler still 200, logged | failure mode |
| mock invariant | NOTIFY_PROVIDER=mock | zero HTTP, log only | CI safe |
| real notifier (httptest) | Send LINE | POST /v2/bot/message/push, Bearer, `to`+text | — |

### Edge Cases Checklist
- [ ] Candidate logs in via LIFF but never friended the bot → push 4xx → logged, flow unaffected
- [ ] `LINE_PROVIDER=mock` / `NOTIFY_PROVIDER=mock` → no external calls anywhere (CI)
- [ ] `NEXT_PUBLIC_LIFF_ID` unset → portal still builds + uses stub token
- [ ] Candidate with empty `line_user_id` (legacy/demo) → status change sends nothing (no crash)
- [ ] Non-notifiable status (parsed/scored/rejected*) → no push (*confirm copy: rejected optional)
- [ ] Bulk status change of N → N best-effort pushes, partial failures don’t abort the batch
- [ ] id-token aud ≠ `LINE_CHANNEL_ID` → verify 4xx → apply 401 (correct)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./internal/notify/ ./internal/applications/ ./internal/reengage/ ./internal/public/ ./cmd/api/
```
EXPECT: clean

### Unit Tests
```bash
cd backend && go test ./internal/notify/ ./internal/applications/ ./internal/reengage/ ./internal/public/ -count=1
```
EXPECT: pass (no LINE creds — httptest + mock)

### Migration
```bash
cd backend && migrate -path migrations -database "$DB_URL" up   # then \d candidates shows line_user_id
```

### Frontend build (both modes)
```bash
cd career-portal && pnpm build                                   # stub mode
NEXT_PUBLIC_LIFF_ID=test pnpm build                              # LIFF mode (no runtime call at build)
```

### Full Suite (no regressions)
```bash
cd backend && go test ./... -count=1
```

### Manual Validation (staging)
- [ ] Apply from the **LINE in-app browser** → LIFF login succeeds, application created, `line_user_id` populated in DB
- [ ] Apply from a normal browser → LINE redirect login works
- [ ] Friend the Messaging bot, move that application to **shortlisted/interview/hired** → real LINE message received
- [ ] Trigger a re-engagement (vacancy open) for a candidate with a `line_user_id` → real LINE message received
- [ ] Email path: confirm it logs/stubs (not delivered) — documented behavior
- [ ] Flip flags back to mock → no external calls (rollback verified)

---

## Acceptance Criteria
- [ ] Real LINE Login via LIFF; server verifies the id-token (`LINE_PROVIDER=real`)
- [ ] `candidates.line_user_id` persisted at apply from the verified `Subject`
- [ ] Real LINE push (`NOTIFY_PROVIDER=real`) delivered on status change (shortlisted/interview/hired) and re-engagement, addressed by `line_user_id`
- [ ] All notify is best-effort: no apply or status-change request fails due to a notify error
- [ ] Mock-default fully preserved (CI/local: zero external calls, no creds)
- [ ] `docs/line-notify-provisioning.md` covers channels (same provider), LIFF, friend requirement, env, rollback, email caveat
- [ ] `go build`/`vet`/unit/full suite green; career-portal builds in both modes

## Completion Checklist
- [ ] Phase A merged (login real + persist) — flag still mock-safe
- [ ] Phase B merged (notify recipient + status hooks) — flag still mock-safe
- [ ] Phase C: runbook + staging smoke green
- [ ] Deploy note: rebuild **career-portal** with `NEXT_PUBLIC_LIFF_ID`; roll **api + worker** (notify + persist live in both). Mirror the [[score-explainability-live]] deploy-together lesson.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LIFF login channel and Messaging bot under different providers → `sub` not pushable | Medium | High | Runbook: provision both under ONE provider; validate one real push in staging before sign-off |
| Candidate hasn’t friended the bot → push fails | High | Low | Best-effort (logged, non-fatal); status page remains source of truth; consider add-friend CTA later |
| Demo/legacy candidates have no `line_user_id` → silent no-send | High (by design) | Low | Expected; skip when empty; note in runbook |
| Email expected but only stubbed | Medium | Medium | Explicitly out of scope + documented; `NOTIFY_EMAIL_FROM` not fail-fast |
| id-token aud mismatch (LIFF id ≠ `LINE_CHANNEL_ID`) | Medium | Medium | Runbook asserts they match; verify returns 4xx → apply 401, surfaced early |

## Notes
- Build directly on the verified seam state from [[score-explainability-live]] and the existing
  re-engagement notifier (`sprint-5a-reengagement-notifier`).
- After ship, update memory: LINE/Notify live, the same-provider + friend gotchas, and the
  `line_user_id` persistence. Remaining go-live after this: PeopleSoft live (2.6) + Phase 3/4.
```
