# Implementation Report: Candidate Status State Machine + Interview Scheduling

## Summary
Replaced the free-form application-status changes with a server-enforced state machine (transitions.go), added human-interview scheduling with a Microsoft Graph + Teams calendar integration (mock/real seam), a mandatory internal rejection reason, and a "Hire → offer" stage. The frontend now shows only the actions each status permits and collects a schedule / reason via dialogs.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large (~20 files) | Large — 28 code files |
| Confidence | 8/10 | Implemented single-pass; all validation green |
| Files Changed | ~20 | 28 (excl. plan/report) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000019 (rejection_reason + interview_appointments) | ✅ Complete | |
| 2 | Status constants + transition map + tests | ✅ Complete | |
| 3 | Repository: SetRejection + appointment CRUD + reason in FindByID | ✅ Complete | |
| 4 | UpdateStatus handler enforces machine + reason + offer | ✅ Complete | Removed dead `hired`/SyncHired branch (see Deviations) |
| 5 | Interview pkg drives status (invite→ai_interview, complete→ai_interviewed) | ✅ Complete | + ErrNotScreened guard, 3 new tests |
| 6 | Calendar package (mock + Graph + tests) | ✅ Complete | onlineMeetings-first pattern |
| 7 | Config GRAPH_* + validation + UsesRealGraph | ✅ Complete | |
| 8 | Interview-schedule endpoint | ✅ Complete | online-without-email → 400 |
| 9 | Bulk re-gate (shortlist + reject-with-reason only) | ✅ Complete | per-id state-machine gate |
| 10 | Wire calendar + schedule handler in main.go | ✅ Complete | |
| 11 | FE statusMachine.ts mirror + types | ✅ Complete | |
| 12 | FE dialogs + AiSummaryPanel gating | ✅ Complete | Deviated — see below |
| 13 | FE labels/tones + bulk bar + queries | ✅ Complete | bulk reject uses window.prompt for reason |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `gofmt` clean, `go vet ./...` clean, `tsc --noEmit` clean, eslint 0 errors (1 pre-existing warning) |
| Unit Tests | ✅ Pass | transitions (table-driven), calendar (httptest), interview (3 new state tests) |
| Build | ✅ Pass | `go build ./...`; `next build` 14 routes |
| Integration | N/A | not run locally (needs DB + Graph creds) — covered by handler/service tests |
| Edge Cases | ✅ Pass | scored-can't-shortlist, reject-needs-reason, online-no-email, onsite-skips-Teams, no-clobber on completion |

## Deviations from Plan
- **Label primitive**: the plan assumed a `ui/label` component; the dashboard has none. Used a local `Label` span inside each dialog (mirrors `UserManagement.tsx`'s `Field` pattern). WHY: avoid adding a new shared primitive for two dialogs.
- **Bulk reject reason UX**: used `window.prompt` for the shared reason rather than a dedicated dialog. WHY: keeps the bulk bar lightweight; a full dialog is a follow-up if needed.
- **Dead `hired` branch**: the old `UpdateStatus` PeopleSoft-sync-on-hired branch was removed (funnel now ends at `offer`). The `HiredSyncer` dependency + `SetHired` repo method are retained for the future offer-accepted step. WHY: matches the planned PS-sync deferral.
- **Graph event time zone**: events are sent in UTC (`timeZone:"UTC"`) instead of `"SE Asia Standard Time"`. WHY: avoids a tzdata dependency in the Alpine container; calendar clients localize regardless. Documented in graph.go.

## Files Changed
28 code files: +1,918 / −90 (see `git diff --stat main`). New: `transitions.go`(+test), `schedule_handler.go`, `internal/calendar/*` (4), migration 000019 (2), `lib/statusMachine.ts`, `ScheduleInterviewDialog.tsx`, `RejectDialog.tsx`.

## Issues Encountered
- Removing `allowedStatuses` broke the Bulk handler (referenced it) → resolved by doing Task 9 immediately.
- Adding `SetStatus` to the interview `appReader` interface required updating `stubApps` + setting test apps to `scored` so existing Invite tests still pass.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/applications/transitions_test.go` | 18 cases + 4 | Every allowed/denied transition + RequiresSchedule/Reason |
| `internal/calendar/graph_test.go` | 3 | online mint+book, onsite skip, mock join URL |
| `internal/interview/service_test.go` (added) | 3 | invite sets ai_interview, not-screened rejected, completion → ai_interviewed |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr`
- [ ] Operational (for real Graph): app registration perms Calendars.ReadWrite + OnlineMeetings.ReadWrite (admin consent) + Teams Application Access Policy on the service mailbox + set GRAPH_* env
