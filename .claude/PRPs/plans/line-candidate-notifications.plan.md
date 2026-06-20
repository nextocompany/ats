# Plan: LINE Candidate Notifications — close lifecycle coverage gaps (UAT #11)

## Summary
The LINE Messaging-API push seam is **already fully built** (`notify` package: real push in `rest.go`, mock default, status/onboarding/AI-interview message builders, inline best-effort dispatch). UAT #11 is therefore **not new infrastructure** — it is **coverage + activation**: add the two missing candidate-facing notifications (a *human interview scheduled* message carrying the real date/time/place/Teams-link, and an *onboarding-documents requested* CTA on hire), and document the operator steps + LINE-provider gotchas to flip `NOTIFY_PROVIDER=real` safely on prod.

## User Story
As a **job candidate**, I want a **LINE message the moment my interview is booked (with the exact date, time, and place/online link) and a clear prompt to upload my onboarding documents once I'm hired**, so that **I never miss a step and don't have to keep checking the portal**.

## Problem → Solution
**Current:** when HR books a real interview, the candidate only gets the *generic* status message "คุณได้รับเชิญเข้าสัมภาษณ์ ทีมงานจะติดต่อเพื่อนัดหมาย" — no date, no time, no location, no Teams link. When hired, they get "ทีม HR จะติดต่อเรื่องการเริ่มงาน" — no prompt to upload onboarding docs. LINE push also runs as `mock` on prod (logs only).
**Desired:** the interview-scheduled push carries the concrete appointment (Thai-formatted Bangkok date/time, onsite location *or* online join link, round number); the hired push directs the candidate to `/account` to upload required documents; `NOTIFY_PROVIDER=real` is verified live so these actually deliver over LINE + email.

## Metadata
- **Complexity**: Medium (3–6 files, ~250 lines incl. tests; reuses the existing seam end-to-end)
- **Source PRD**: N/A (UAT backlog item #11)
- **PRD Phase**: N/A — standalone
- **Estimated Files**: 3 code + 2 test + 1 ops doc

---

## UX Design

### Before
```
HR books interview (Tue 25 Jun, 14:00, online, Teams link created)
        │
        ▼
Candidate's LINE:  "คุณได้รับเชิญเข้าสัมภาษณ์ ทีมงานจะติดต่อเพื่อนัดหมายเร็ว ๆ นี้
                    ตรวจสอบสถานะได้ที่ <portal>/status"        ← no when/where/link

HR sets status = hired
        │
        ▼
Candidate's LINE:  "ยินดีด้วย! ... ทีม HR จะติดต่อเรื่องการเริ่มงาน"  ← no upload CTA
```

### After
```
HR books interview (Tue 25 Jun, 14:00, online, Teams link created)
        │
        ▼
Candidate's LINE:  "สวัสดีคุณ<ชื่อ> นัดสัมภาษณ์ (รอบ 1) ของคุณ
                    📅 25 มิถุนายน 2569 เวลา 14:00 น.
                    💻 สัมภาษณ์ออนไลน์ — เข้าร่วมที่ <join-url>
                    ดูรายละเอียดได้ที่ <portal>/status"
        + email (same copy) when the candidate has an address

HR sets status = hired
        │
        ▼
Candidate's LINE:  "ยินดีด้วย! ... กรุณาอัปโหลดเอกสารเริ่มงานของคุณที่ <portal>/account"
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Interview booked (`POST /applications/:id/interview-schedule`) | generic status push | concrete appointment push (date/time/mode/place/round) | onsite → location text; online → join URL |
| Each additional interview round | (same generic) | new push per round, labelled "รอบ N" | `Appointment.RoundNo` |
| Status → hired | "HR will contact you" | adds "upload your onboarding documents at /account" | enhance `statusBody("hired")` only |
| Channel activation | `NOTIFY_PROVIDER=mock` (log-only) | `=real` (LINE push + email live) | operator env + verify, no code default change |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/notify/message.go` | 1-135 | The exact builder pattern to mirror (primitives in, zero-Message skip, `statusBody` shared by LINE+email); **hired copy lives here** |
| P0 | `backend/internal/applications/notify.go` | 1-95 | `statusNotifyDeps` + `notifyStatusChange`/`notifyDocumentReviewed` — the dispatch wrapper to mirror for `notifyInterviewScheduled` |
| P0 | `backend/internal/applications/schedule_handler.go` | 83-181 | Call site: replace the generic `notifyStatusChange(StatusInterview)` (line 179); `saved` Appointment carries all fields |
| P1 | `backend/internal/notify/rest.go` | 16-83 | Real LINE push (already done) — confirms `Message.Body` plain text is what ships; nothing to change here |
| P1 | `backend/internal/notify/interview_message.go` | 1-30 | Closest builder analog (a single deep-link interview message) |
| P1 | `backend/internal/applications/model.go` | 91-109 | `Appointment` struct fields + `ModeOnsite`/`ModeOnline` constants |
| P1 | `backend/internal/letters/pdf.go` | 67-75 | `thaiMonths` + `thaiDate` (พ.ศ.) — the Thai-date pattern to mirror (do NOT import letters; copy the tiny helper into notify) |
| P2 | `backend/internal/notify/onboarding_message_test.go` | all | Builder test style to mirror for the new builders |
| P2 | `backend/cmd/api/main.go` | 383-401 | Confirms `scheduleHandler.SetNotifier(...)` is already wired — no main.go change needed |
| P2 | `backend/pkg/config/config.go` | 79-99, 284-287, 420-429 | `NotifyProvider`/`NotifyLINEToken`, `UsesRealNotify()`, the `NOTIFY_LINE_TOKEN` required-when-real guard |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| LINE Messaging API push | developers.line.biz/en/reference/messaging-api/#send-push-message | `POST https://api.line.me/v2/bot/message/push`, `Authorization: Bearer {channel access token}`, body `{to, messages:[{type:"text",text}]}` — **already implemented in `rest.go`** |
| userId ↔ provider linkage | LINE platform behavior (stable) | The LINE Login `sub` (stored as `candidates.line_user_id`) equals the Messaging-API `userId` **only when the Login channel and the Messaging-API channel are under the SAME LINE provider**. Cross-provider → push 400/“invalid to”. **Operator must verify** Login channel `2010375490` and bot `2010375394` share one provider |
| Friendship requirement | LINE platform behavior (stable) | Push to a user who has **not added the OA as a friend** returns HTTP 403. Login bot-prompt is already `aggressive` (`LINE_LOGIN_BOT_PROMPT`), which offers “add friend” at login — but it is optional for the user, so push must stay **best-effort** (already is) |
| Channel access token | developers.line.biz/en/docs/messaging-api/channel-access-tokens | `NOTIFY_LINE_TOKEN` must be the **Messaging-API channel** access token (long-lived or stateless), **not** the Login channel secret (`LINE_CHANNEL_SECRET`) |

> No web fetch required — the push transport is built and proven; these are established LINE-platform constraints captured as gotchas for the operator + builders.

---

## Patterns to Mirror

### MESSAGE_BUILDER (primitives in, zero-Message skip, shared body)
```go
// SOURCE: backend/internal/notify/message.go:15-49
func StatusMessage(lineUserID, fullName, status, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	body, ok := statusBody(fullName, status, portalBaseURL)
	if !ok {
		return Message{}
	}
	return Message{Channel: ChannelLINE, Recipient: lineUserID, Subject: "อัปเดตสถานะใบสมัคร", Body: body}
}
// ...email twin reuses the SAME body builder so LINE/email copy never drifts:
func StatusEmailMessage(emailAddr, fullName, status, portalBaseURL string) Message { ... ChannelEmail ... }
```

### DISPATCH_WRAPPER (best-effort, never returns error, no-op when unset)
```go
// SOURCE: backend/internal/applications/notify.go:26-52
func (d statusNotifyDeps) notifyStatusChange(ctx context.Context, apps Repository, appID uuid.UUID, status string) {
	if d.notifier == nil || d.cands == nil {
		return
	}
	app, err := apps.FindByID(ctx, appID)
	if err != nil { log.Warn().Err(err)... ; return }
	cand, err := d.cands.FindByID(ctx, app.CandidateID)
	if err != nil { log.Warn().Err(err)... ; return }
	if msg := notify.StatusMessage(cand.LineUserID, cand.FullName, status, d.portalBaseURL); msg.Recipient != "" {
		if err := d.notifier.Send(ctx, msg); err != nil { log.Warn().Err(err)...("line send failed (non-fatal)") }
	}
	if em := notify.StatusEmailMessage(cand.Email, cand.FullName, status, d.portalBaseURL); em.Recipient != "" {
		if err := d.notifier.Send(ctx, em); err != nil { log.Warn().Err(err)...("email send failed (non-fatal)") }
	}
}
```

### THAI_DATE (Buddhist-era; copy into notify, add explicit Bangkok tz)
```go
// SOURCE: backend/internal/letters/pdf.go:67-75
var thaiMonths = [...]string{"มกราคม","กุมภาพันธ์",...,"ธันวาคม"}
func thaiDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), thaiMonths[int(t.Month())-1], t.Year()+543)
}
```

### BUILDER_TEST (table-driven, asserts skip + copy)
```go
// SOURCE: backend/internal/notify/onboarding_message_test.go (style)
func TestDocumentReviewedMessage(t *testing.T) {
	m := DocumentReviewedMessage("U123", "สมชาย", "id_card", true, "", "https://p")
	if m.Recipient != "U123" || !strings.Contains(m.Body, "บัตรประชาชน") { t.Fatalf("...") }
	if got := DocumentReviewedMessage("", "x", "id_card", true, "", "p"); got.Recipient != "" {
		t.Fatal("empty line id must yield zero Message")
	}
}
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/notify/interview_message.go` | UPDATE | Add `InterviewScheduledMessage` + `InterviewScheduledEmailMessage` + shared `interviewScheduledBody` + local `thaiMonths`/`thaiDateTimeBangkok` helper |
| `backend/internal/notify/message.go` | UPDATE | Enhance `statusBody("hired")` copy to add the `/account` onboarding-upload CTA |
| `backend/internal/applications/notify.go` | UPDATE | Add `notifyInterviewScheduled(ctx, appID, appt)` dispatch wrapper (mirrors `notifyStatusChange`) |
| `backend/internal/applications/schedule_handler.go` | UPDATE | Replace line 179 generic `notifyStatusChange(StatusInterview)` with `notifyInterviewScheduled(..., saved)` |
| `backend/internal/notify/interview_message_test.go` | CREATE | Table tests for the new builders (round 1/N, onsite/online, empty-skip, Bangkok time) |
| `backend/internal/notify/message_test.go` | UPDATE | Assert the hired copy now contains the `/account` CTA |
| `docs/line-notifications-activation.md` | CREATE | Operator runbook: env to set, provider/friendship/token gotchas, smoke steps |

## NOT Building
- **No new notification infrastructure** — the seam, real LINE push, mock default, and inline best-effort dispatch already exist and are unchanged.
- **No asynq/queue** — notifications stay inline best-effort (matches every existing call site; a failure already never blocks the HR action).
- **No LINE Flex/rich/template/quick-reply messages** — plain text only, mirroring the current builders.
- **No offer-detail push** (salary/start-date over LINE) — the generic offer status push + the `/offers` portal page remain the channel for sensitive terms.
- **No HR-facing LINE** — HR uses Teams (`dispatchHR`); out of scope.
- **No DB migration / schema change** — all needed data (`Appointment`, `candidates.line_user_id/email/full_name`) already exists.
- **No reschedule-edit notification** — there is no appointment-edit endpoint; each new round already books via `Schedule` and will notify.
- **No change to the `mock` default** — `NOTIFY_PROVIDER` stays `mock` in code; going live is an operator env flip.

---

## Step-by-Step Tasks

### Task 1: Add the interview-scheduled message builders
- **ACTION**: In `backend/internal/notify/interview_message.go`, add `InterviewScheduledMessage(lineUserID, fullName string, roundNo int, scheduledAt time.Time, durationMin int, mode, locationText, onlineJoinURL, portalBaseURL string) Message`, its email twin `InterviewScheduledEmailMessage(emailAddr, ...same... )`, and a shared private `interviewScheduledBody(...)`.
- **IMPLEMENT**:
  - Guard: LINE builder returns `Message{}` when `lineUserID == ""`; email builder returns `Message{}` when `emailAddr == ""`.
  - Body (Thai): greeting (`"สวัสดีคุณ"+fullName` else `"สวัสดีค่ะ"`); a round label only when `roundNo > 1` (e.g. ` (รอบ N)`); the date+time via the new helper; mode line — `ModeOnline` → `"💻 สัมภาษณ์ออนไลน์ — เข้าร่วมที่ "+onlineJoinURL` (omit the join clause if `onlineJoinURL==""`), `ModeOnsite` → `"📍 สถานที่: "+locationText` (omit when empty); trailing `" ดูรายละเอียดได้ที่ "+portalBaseURL+"/status"`.
  - Add a local `thaiMonths` array + `thaiDateTimeBangkok(t time.Time) string` that converts to `Asia/Bangkok` first, then formats `"%d %s %d เวลา %02d:%02d น."` (day, thai month, year+543, hour, minute).
  - `Subject: "นัดหมายสัมภาษณ์"` for both.
- **MIRROR**: MESSAGE_BUILDER (`message.go:15-49`) + THAI_DATE (`letters/pdf.go:67-75`).
- **IMPORTS**: `fmt`, `time` (package `notify`).
- **GOTCHA**: `Appointment.ScheduledAt` is parsed from client RFC3339 and may carry UTC; **always `t.In(bangkok)` before reading `.Hour()/.Day()`** or the candidate sees a time 7h off. Use `time.LoadLocation("Asia/Bangkok")`; if it errors (no tzdata), fall back to a fixed `time.FixedZone("ICT", 7*3600)` so the build never depends on the OS tz database.
- **VALIDATE**: `cd backend && go build ./internal/notify/...`

### Task 2: Add the `notifyInterviewScheduled` dispatch wrapper
- **ACTION**: In `backend/internal/applications/notify.go`, add `func (d statusNotifyDeps) notifyInterviewScheduled(ctx context.Context, appID uuid.UUID, appt Appointment)`.
- **IMPLEMENT**: copy the shape of `notifyStatusChange`: no-op when `d.notifier==nil || d.cands==nil`; load `app` is unnecessary because the caller passes the saved `Appointment` — but we still need the candidate, and `appt.ApplicationID`/the app's `CandidateID`. Load `app, err := d?`… → simpler: take `cand *candidates.Candidate` is not available here, so load it: `app` via `apps`? The wrapper has no `apps` param in this signature. **Resolve by passing what we have**: change signature to `notifyInterviewScheduled(ctx, apps Repository, appID uuid.UUID, appt Appointment)` to match `notifyStatusChange` exactly; inside, `apps.FindByID(appID)` → `d.cands.FindByID(app.CandidateID)`. Then build + send the LINE message (`notify.InterviewScheduledMessage(cand.LineUserID, cand.FullName, appt.RoundNo, appt.ScheduledAt, appt.DurationMin, appt.Mode, appt.LocationText, appt.OnlineJoinURL, d.portalBaseURL)`) and the email twin (`cand.Email`), each guarded by `msg.Recipient != ""`, each `log.Warn` non-fatal on error.
- **MIRROR**: DISPATCH_WRAPPER (`notify.go:26-52`) — same logging strings style (`"interview notify: line send failed (non-fatal)"`).
- **IMPORTS**: already present in the file (`context`, `uuid`, `log`, `candidates`, `notify`).
- **GOTCHA**: keep it best-effort — never return an error; a notify failure must not roll back the booked appointment.
- **VALIDATE**: `cd backend && go build ./internal/applications/...`

### Task 3: Wire the specific notification into the schedule handler
- **ACTION**: In `backend/internal/applications/schedule_handler.go`, replace line 179 `h.notifyDeps.notifyStatusChange(c.UserContext(), h.apps, id, StatusInterview)` with `h.notifyDeps.notifyInterviewScheduled(c.UserContext(), h.apps, id, saved)`.
- **IMPLEMENT**: single-line swap; `saved` is the persisted `Appointment` returned by `CreateAppointment` (carries `RoundNo`, `OnlineJoinURL`, etc.). Leave the `SetStatus(StatusInterview)` call above untouched — only the *notification* changes from generic to specific.
- **MIRROR**: existing call-site convention (best-effort after the state write).
- **IMPORTS**: none new.
- **GOTCHA**: do **not** also call `notifyStatusChange(StatusInterview)` — that would double-message. The specific message supersedes the generic for this transition. (Other transitions still use `notifyStatusChange`.)
- **VALIDATE**: `cd backend && go build ./... && go vet ./internal/applications/...`

### Task 4: Enhance the hired copy with the onboarding-upload CTA
- **ACTION**: In `backend/internal/notify/message.go`, edit the `case "hired":` arm of `statusBody`.
- **IMPLEMENT**: change to direct the candidate to upload documents, e.g. `return greeting + fmt.Sprintf(" ยินดีด้วย! คุณได้รับการคัดเลือก กรุณาอัปโหลดเอกสารเริ่มงานของคุณที่ %s/account ทีม HR จะติดต่อกลับเรื่องวันเริ่มงาน", portalBaseURL), true`. Keep the function signature and the `(string, bool)` contract; this also updates the email copy automatically (shared body).
- **MIRROR**: existing `offer` arm which already builds an `/offers` deep link in the same style.
- **IMPORTS**: none new.
- **GOTCHA**: this is the single source of hired copy for **both** LINE and email — verify the `/account` route exists on the career-portal (it does: onboarding upload lives at `/account`, per `documentReviewedBody`).
- **VALIDATE**: `cd backend && go build ./internal/notify/...`

### Task 5: Tests
- **ACTION**: CREATE `backend/internal/notify/interview_message_test.go`; UPDATE `backend/internal/notify/message_test.go`.
- **IMPLEMENT**:
  - Round 1 online: body contains the join URL, the Thai month, `"เวลา"`, and **no** `"รอบ"` label; `Recipient=="U…"`.
  - Round 2 onsite: body contains `"รอบ 2"` and the location text, **no** join URL.
  - Empty `lineUserID` → `InterviewScheduledMessage(...).Recipient == ""`; empty email → email twin `Recipient == ""`.
  - Bangkok formatting: feed a UTC `time.Date(2026,6,25,7,0,0,0,time.UTC)` (= 14:00 ICT) and assert the body contains `"14:00"` and `"2569"`.
  - `message_test.go`: assert `statusBody("…","hired","https://p")` body contains `"/account"`.
- **MIRROR**: BUILDER_TEST (`onboarding_message_test.go`).
- **IMPORTS**: `strings`, `testing`, `time`.
- **GOTCHA**: assert on substrings, not the whole string, so copy tweaks don't break tests brittlely.
- **VALIDATE**: `cd backend && go test ./internal/notify/... ./internal/applications/...`

### Task 6: Operator activation runbook
- **ACTION**: CREATE `docs/line-notifications-activation.md`.
- **IMPLEMENT**: document (a) env to set on `hrats-prod-api`: `NOTIFY_PROVIDER=real`, `NOTIFY_LINE_TOKEN=<Messaging-API channel access token for bot 2010375394>` (ACA secret, **not** the Login channel secret); email already real (ACS); (b) **pre-flight gotchas** — Login channel `2010375490` and Messaging bot `2010375394` must share one LINE provider (else userId mismatch), candidate must have added the OA as friend (403 otherwise), token is the Messaging channel token; (c) smoke: book a test interview for a LINE-linked candidate, confirm the dated push arrives + the prod `[mock notify]`→ real transition in logs; (d) rollback = set `NOTIFY_PROVIDER=mock` (instant, no redeploy of image needed beyond env revision).
- **MIRROR**: the existing provisioning-doc style (e.g. `docs/career-membership-provisioning.md`).
- **GOTCHA**: setting `NOTIFY_PROVIDER=real` **without** `NOTIFY_LINE_TOKEN` makes `config.Load` fail fast (guard at `config.go:428`) → the api won't boot. Set both together.
- **VALIDATE**: doc only — no build step.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| InterviewScheduled online r1 | `U1, "สมชาย", 1, 25Jun14:00ICT, 60, online, "", "https://meet", "https://p"` | body has join URL, `"25 มิถุนายน 2569"`, `"14:00"`, no `"รอบ"`; Recipient `U1` | — |
| InterviewScheduled onsite r2 | `U1, "", 2, …, onsite, "สาขาลาดพร้าว", "", "https://p"` | body has `"รอบ 2"`, `"สาขาลาดพร้าว"`, no join URL | multi-round |
| Empty LINE id | `"", …` | `Message{}` (Recipient `""`) | skip |
| Empty email | email twin `""` | `Message{}` | skip |
| Bangkok offset | `time.UTC 07:00` | body contains `"14:00"` | tz correctness |
| Hired CTA | `statusBody(_, "hired", "https://p")` | contains `"/account"`, `ok==true` | copy regression |

### Edge Cases Checklist
- [x] Empty input (no LINE id / no email) → zero Message, skipped
- [x] Online with empty join URL (Graph mock failed) → message still sent, join clause omitted
- [x] Multi-round (RoundNo > 1) labelled
- [x] Network failure on push → logged `non-fatal`, appointment unaffected (best-effort wrapper)
- [x] Candidate not a friend of OA → push 403, logged non-fatal (no user-visible failure)
- [ ] Concurrent access — N/A (stateless builders)

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l internal/notify internal/applications && go vet ./...
```
EXPECT: no files listed by gofmt; vet clean

### Unit Tests
```bash
cd backend && go test -race ./internal/notify/... ./internal/applications/...
```
EXPECT: all pass (incl. new interview_message + updated message tests)

### Full Test Suite
```bash
cd backend && go test ./...
```
EXPECT: no regressions

### Build
```bash
cd backend && go build ./...
```
EXPECT: clean

### Manual Validation (post-deploy, operator)
- [ ] `NOTIFY_PROVIDER=real` + `NOTIFY_LINE_TOKEN` set on `hrats-prod-api`; api boots (`/health` 200)
- [ ] Book a real interview for a LINE-linked test candidate → dated push lands in LINE within seconds
- [ ] Onsite vs online: location text vs Teams join link rendered correctly
- [ ] Set that candidate to `hired` → push contains the `/account` upload link
- [ ] Logs show real push (no `[mock notify]`) and no 4xx from `api.line.me`

---

## Acceptance Criteria
- [ ] Interview-schedule push carries Bangkok date/time, mode, place/online link, and round label
- [ ] Hired push directs the candidate to `/account` to upload onboarding documents
- [ ] Both new messages send over LINE **and** email, best-effort, never blocking the HR action
- [ ] No double-messaging on the interview transition (generic status push replaced, not duplicated)
- [ ] All validation commands pass; no type/vet/lint errors; `go test ./...` green
- [ ] Activation runbook committed with the provider/friendship/token gotchas

## Completion Checklist
- [ ] Code follows the discovered builder + dispatch + Thai-date patterns
- [ ] Error handling matches codebase (`log.Warn().Err(...)` non-fatal, zero-Message skip)
- [ ] Logging follows `"<area> notify: <channel> send failed (non-fatal)"` convention
- [ ] Tests follow the table-driven `*_message_test.go` style
- [ ] No hardcoded base URLs (uses `cfg.PortalBaseURL` already threaded through `SetNotifier`)
- [ ] No new scope (no queue, no Flex, no migration)
- [ ] Self-contained — every call site, field, and constant captured above

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Login & Messaging channels are in **different LINE providers** → `line_user_id` ≠ push `userId` → every push 400 | Medium | High (feature silently no-ops over LINE) | Operator verifies single-provider linkage **before** flip; email twin still delivers; runbook Task 6 makes this the first pre-flight check |
| Candidate hasn't added the OA as friend → 403 | Medium | Low (per-user) | Best-effort + logged; aggressive login bot-prompt already nudges add-friend; email covers them |
| `ScheduledAt` stored as UTC → wrong displayed time | Medium | High (wrong interview time) | Explicit `Asia/Bangkok` conversion in the builder + a unit test asserting 07:00 UTC → "14:00" |
| `NOTIFY_PROVIDER=real` set without token → api fails to boot | Low | High (outage) | Documented as “set both together”; config guard fails fast at startup, not mid-request |
| Double-message if both generic + specific fire | Low | Low (annoyance) | Task 3 explicitly replaces (not adds) the generic call on the interview transition |

## Notes
- **The transport is already done** — `notify/rest.go sendLINE` ships real pushes today; this plan only adds copy + two builders + one wrapper + a one-line wire + the env flip. That is why complexity is Medium, not Large.
- LINE and email are **independent best-effort** channels (a candidate with only one is still reached) — preserve that in the new wrapper exactly as `notifyStatusChange` does.
- `cmd/api/main.go:384` already calls `scheduleHandler.SetNotifier(notifier, candidateRepo, cfg.PortalBaseURL)` — **no main.go change** is needed; the deps are live the moment `NOTIFY_PROVIDER=real`.
- Independent backlog sibling still pending after this: **#7 k6 concurrent load test** (staging, last).
```
