# Code Review: Sprint 5a — Re-engagement + Notifier seam (pre-PR)

**Reviewed**: 2026-05-30
**Branch**: `feat/sprint-5a-reengagement-notifier`
**Reviewer**: go-reviewer agent + maintainer triage
**Decision**: APPROVE (legitimate findings fixed; overstated ones documented)

## Summary
Independent Go review surfaced 12 findings. Seven were valid correctness/schema/test
improvements and were fixed in this PR. Two CRITICAL/HIGH "auth" findings were assessed
as the codebase-wide auth posture (not a 5a defect) and fail-closed in prod; documented
rather than changed. Static analysis, race tests, and integration all pass.

## Findings & Disposition

| ID | Sev | Finding | Disposition |
|---|---|---|---|
| H3 | HIGH | LINE push response body not drained before close (conn-pool leak under retries) | **Fixed** — `io.Copy(io.Discard, resp.Body)` in `notify/rest.go` |
| H4 | HIGH | `reengagement_contacts.position_id` missing FK | **Fixed** — `REFERENCES positions(id) ON DELETE CASCADE` |
| H1 | HIGH | Test fake nil-map guard ordered after read | **Fixed** — guard moved before read |
| M4 | MED | RecordContact error not wrapped with candidate id | **Fixed** — wrapped with `%w` + candidate id |
| M5 | MED | No handler test (auth gate untested) | **Fixed** — `handler_test.go`: 201 / 403 role / 403 no-user / 400 bad-uuid |
| L2 | LOW | `channel` column lacks CHECK | **Fixed** — `CHECK (channel IN ('line','email'))` |
| L3 | LOW | `pickChannel` returns `(ChannelLINE,"")` for no channel | **Fixed** — returns `("","")` |
| C1/H2 | CRIT/HIGH | reengage endpoint reads dev user; no real auth | **Documented** — same posture as every authed endpoint (S4a/b `scopeFrom`); real Azure AD SSO is a project-wide deferral. MockJWT off in prod ⇒ **fails closed (403)**. Handler adds role-gating the other endpoints lack. `auditor` excluded intentionally (read-only role, re-engage is a write). |
| M3 | MED | No `asynq.Unique` ⇒ duplicate webhook enqueues do a redundant pass | **Won't fix** — DB suppression already guarantees no double-send; `Unique` would make the *manual* re-trigger 500 within the TTL. Accepted. |
| M1 | MED | `FullName` interpolated into message body | **Won't fix** — plain-text LINE channel does not interpret markup; safe today. Noted for future email/HTML. |
| M2 | MED | `ctx` discarded in `Trigger.OnVacancyOpened` | **N/A** — param already named `_`; asynq `Enqueue` is synchronous and takes no context. |
| L1 | LOW | real notifier returns value not pointer | **N/A** — matches `peoplesoft.newRESTClient` (also value return). |

## Validation (post-fix)

| Check | Result |
|---|---|
| `go vet ./...` | Pass |
| `golangci-lint run` | Pass (0 issues) |
| `go test -race` (notify, reengage, peoplesoft, pkg) | Pass |
| `go test -tags integration ./internal/reengage/...` | Pass |
| Migration round-trip + FK/CHECK verified | Pass |

## Files Reviewed
13 created + 8 updated (incl. the 4 review fixes: `notify/rest.go`, `migrations/000006_*.up.sql`,
`reengage/service.go`, `reengage/service_test.go`, `reengage/handler_test.go` added).
