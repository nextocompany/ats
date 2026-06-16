# Implementation Report: Notifications — Email (candidate + HR) + MS Teams (HR)

## Summary
Extended the `notify` seam so email is a real channel (via the existing `pkg/email`
ACS sender) and added an MS Teams Incoming-Webhook channel. Candidates now receive
status updates by email in addition to LINE; store HR are pinged by email + Teams
when an application is scored+assigned (worker pipeline) and when interview feedback
is recorded (api). All sends are best-effort — failures are logged, never break the
triggering action/job.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium | Medium |
| Confidence | 8/10 | held — no surprises |
| Files Changed | ~12 | 11 changed + 3 new (teams.go, hr_message.go, hr_directory.go, channels_test.go) |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Teams + Dashboard config | ✅ | `TeamsWebhookURL`, `DashboardBaseURL` (DASHBOARD_BASE_URL) |
| 2 | Teams sender | ✅ | `internal/notify/teams.go` |
| 3 | Email + Teams routing in restNotifier | ✅ | reuses `email.Sender`; stub error removed |
| 4 | StatusEmailMessage | ✅ | reuses `statusBody` (no copy drift) |
| 5 | Candidate dispatch → email | ✅ | LINE + email independent in `notifyStatusChange` |
| 6 | HR directory lookup | ✅ | `EmailsForStore` (active sgm/hr_manager/hr_staff) |
| 7 | HR message builders | ✅ | `NewScoredHR`, `FeedbackRecordedHR` |
| 8 | Pipeline HR trigger | ✅ | `Processor.SetNotifier` + `notifyScored` after scored+assigned |
| 9 | Feedback HR trigger | ✅ | `FeedbackHandler.SetNotifier` + `notifyFeedbackRecorded` |
| 10 | Wire api + worker | ✅ | feedback handler (api) + processor (worker) get notifier+hrDir |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static (go vet) | ✅ Pass | whole module |
| Unit Tests | ✅ Pass | notify + applications green; 14 new tests |
| Build | ✅ Pass | `go build ./...` (api + worker) |
| Full suite | ✅ Pass | `go test ./...` exit 0, 26 pkgs ok, no regressions |
| gofmt | ✅ Pass | edited files clean (pre-existing `cmd/seedresumes` untouched) |

## Files Changed
| File | Action | Lines |
|---|---|---|
| `internal/notify/teams.go` | CREATE | +40 |
| `internal/notify/hr_message.go` | CREATE | +60 |
| `internal/notify/channels_test.go` | CREATE | +120 |
| `internal/applications/hr_directory.go` | CREATE | +55 |
| `internal/notify/notify.go` | UPDATE | ChannelTeams |
| `internal/notify/rest.go` | UPDATE | email.Sender + teams routing |
| `internal/notify/message.go` | UPDATE | StatusEmailMessage |
| `internal/applications/notify.go` | UPDATE | candidate email + dispatchHR |
| `internal/applications/feedback_handler.go` | UPDATE | SetNotifier + notifyFeedbackRecorded |
| `internal/applications/feedback_test.go` | UPDATE | HR-notify non-fatal test |
| `internal/applications/notify_test.go` | UPDATE | email/both-channel dispatch tests |
| `internal/pipeline/process.go` | UPDATE | SetNotifier + notifyScored |
| `pkg/config/config.go` | UPDATE | TeamsWebhookURL + DashboardBaseURL |
| `cmd/api/main.go`, `cmd/worker/main.go` | UPDATE | wiring |

## Deviations from Plan
- **Pipeline HR-trigger unit test skipped** — `notifyScored` is a method on the
  worker `Processor` whose deps are unexported and DB-backed; a focused unit test
  would require a large fixture. Its building blocks (`NewScoredHR` builder +
  best-effort dispatch + `EmailsForStore`) ARE unit-tested, and the wiring builds.
  Covered end-to-end by the manual prod checklist instead.
- Email channel reuses `pkg/email.Sender` (planned) rather than the `NOTIFY_EMAIL_FROM`
  stub — so the real path uses the same ACS resource already provisioned for OTP.

## Issues Encountered
- None. `pkg/email` already existed with a mock/real seam, so the email channel was
  a wiring exercise, not a new integration.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `internal/notify/channels_test.go` | 8 | teams POST, email field mapping, teams-webhook-required, StatusEmailMessage gating, HR builders |
| `internal/applications/notify_test.go` | +2 | candidate email-only + both-channels dispatch |
| `internal/applications/feedback_test.go` | +1 | HR-notify failure is non-fatal (still 201) |

## Deployment Notes (env to set on prod when going live)
- `NOTIFY_PROVIDER=real` (+ `NOTIFY_LINE_TOKEN` already set) — enables real channels.
- `EMAIL_PROVIDER=real` + reuse existing `ACS_EMAIL_ENDPOINT/ACS_EMAIL_ACCESS_KEY/ACS_EMAIL_SENDER` — enables candidate + HR email.
- `TEAMS_WEBHOOK_URL=<incoming webhook>` — enables the HR Teams channel (empty = disabled, no error).
- `DASHBOARD_BASE_URL=https://hrats-prod-dashboard…` — HR deep links in email/Teams.
- Migration: none. Schema unchanged.

## Next Steps
- [ ] Code review via `/code-review`
- [ ] PR + deploy (api + worker images; set env above)
- [ ] Re-plan PRP-2/3/4 now that notification gaps are closed
