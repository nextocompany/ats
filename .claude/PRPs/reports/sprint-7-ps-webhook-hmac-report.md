# Implementation Report: Sprint 7 — PeopleSoft Webhook HMAC Authentication

## Summary
Added HMAC-SHA256 shared-secret authentication to the three state-changing PeopleSoft webhook POSTs (`/api/v1/ps/vacancy-opened`, `/vacancy-closed`, `/sync-hired`) via a new Fiber middleware mounted in `peoplesoft.RegisterRoutes`. Requests must carry `X-PS-Signature: hex(HMAC-SHA256(PS_WEBHOOK_SECRET, raw-body))`, verified constant-time with `hmac.Equal`; missing/invalid → 401. `GET /api/v1/ps/health` stays open. Gated by the mock-default seam: enforced when `PS_WEBHOOK_SECRET` is set (mandatory + fail-fast when `PS_PROVIDER=real`); dev/CI leave the group open so existing tests stay green. Closes the `docs/SECURITY.md` PS follow-up.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium | Medium |
| Confidence | 9/10 | Held — single pass, zero failures, one trivial path deviation |
| Files Changed | ~9 | 9 (2 created, 7 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Add `PSWebhookSecret` config + real-mode fail-fast | ✅ Complete | new `if c.PSWebhookSecret == ""` inside `UsesRealPeopleSoft()` block |
| 2 | config_test for the requirement | ✅ Complete | +2 tests (`RequiresWebhookSecret`, `WithWebhookSecret`) + `setRealPSIBCreds` helper |
| 3 | Create `VerifyHMAC` middleware | ✅ Complete | `webhook_auth.go`; `hmac.Equal`, no secret logging |
| 4 | Table-driven middleware tests | ✅ Complete | `webhook_auth_test.go`; valid/wrong-secret/tampered/missing/guards-all/health-open |
| 5 | Mount middleware in routes.go | ✅ Complete | health registered before guard; `secret==""` → open |
| 6 | Fix existing webhook_test harness | ✅ Complete | `RegisterRoutes(app, h, "")`; existing tests green (BodyParser regression guard) |
| 7 | Wire secret in main.go | ✅ Complete | passes `cfg.PSWebhookSecret` |
| 8 | `.env.example` + `docs/SECURITY.md` | ✅ Complete | env var + signature contract; follow-up flipped to resolved |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis (vet) | ✅ Pass | exit 0 |
| Lint (golangci-lint v2.11.3) | ✅ Pass | `0 issues.` |
| Unit Tests | ✅ Pass | 15 pkgs ok; +6 HMAC + 2 config tests; existing PS tests unchanged-green |
| gosec | ✅ Pass | exit 0 (no new findings; secret never logged) |
| Build | ✅ Pass | `go build ./...` exit 0 |
| govulncheck | ✅ N/A | no dependency changes from green `main` |

## Files Changed

| File | Action | Lines |
|---|---|---|
| `backend/internal/peoplesoft/webhook_auth.go` | CREATED | +38 |
| `backend/internal/peoplesoft/webhook_auth_test.go` | CREATED | +110 |
| `backend/internal/peoplesoft/routes.go` | UPDATED | +9 / −3 |
| `backend/pkg/config/config.go` | UPDATED | +8 |
| `backend/pkg/config/config_test.go` | UPDATED | +34 |
| `backend/internal/peoplesoft/webhook_test.go` | UPDATED | +1 / −1 |
| `backend/cmd/api/main.go` | UPDATED | +1 / −1 |
| `.env.example` | UPDATED | +3 |
| `docs/SECURITY.md` | UPDATED | +8 / −3 |

## Deviations from Plan

1. **`.env.example` lives at the repo root, not `backend/.env.example`.** The plan listed `backend/.env.example`; the actual tracked example is at the repo root (`/.env.example`, the only env example — backend has none). Added `PS_WEBHOOK_SECRET` + the signature-contract comment there. No functional impact.

## Issues Encountered

- **SECURITY.md edit required a tool Read first** — I had only viewed it via `sed`/`grep`, so the first `Edit` was rejected ("File has not been read yet"). Read the section via the Read tool, then the edit applied. No code impact.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `internal/peoplesoft/webhook_auth_test.go` | 8 | valid sig → 200 (proves BodyParser intact post-`c.Body()`); uppercase-hex accepted; non-hex/wrong-secret/tampered/missing → 401; all 3 POSTs guarded; `/health` open |
| `pkg/config/config_test.go` | +2 | real PS without `PS_WEBHOOK_SECRET` → error; with it → loads + field set |

## Next Steps
- [ ] Code review via `/code-review` (focus: middleware ordering, constant-time compare, no secret leakage)
- [ ] Create PR via `/prp-pr` (branch `feat/s7-ps-webhook-hmac`)
- [ ] After merge: the `docs/SECURITY.md` PS follow-up is resolved; optional replay-protection hardening remains noted
