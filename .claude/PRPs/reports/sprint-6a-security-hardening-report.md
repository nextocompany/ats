# Implementation Report: Sprint 6a — Security Hardening

## Summary
Hardened the stack: helmet security headers + baseline CSP on the Go/Fiber API; CSP + security headers on both Next apps; per-IP rate limiting on `/api/v1/public/*`; a config-gated **Azure AD (Entra) auth seam** that preserves the `DevUser`/`UserContextKey` contract (mock dev super_admin stays default, zero handler changes); `gosec`/`govulncheck`/`pnpm audit` in CI + `make security`; and `docs/SECURITY.md` (PDPA + rotation runbook). Verified live: headers present, rate limit returns 429 past cap, `/health` exempt.

## Assessment vs Reality
| Metric | Predicted | Actual |
|---|---|---|
| Complexity | Large | Large |
| Confidence | 8/10 | High — builds/tests/lint green, live headers+limit verified |
| Files Changed | ~16 | 13 (5 created, 8 updated) + deps |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Config — auth provider | ✅ | `AUTH_PROVIDER`+`AZURE_AD_TENANT_ID`/`CLIENT_ID`, `UsesRealAuth()`, JWT_SECRET warn |
| 2 | Entra verifier | ✅ | go-oidc discovery+JWKS; neutral `Identity` (no import cycle); claim-map test |
| 3 | Auth middleware | ✅ | `Auth(ctx,cfg)` → real verifier or MockJWT; bypass list (/health, /public, /ps) |
| 4 | Wire helmet+limiter+auth | ✅ | helmet after CORS; path-scoped limiter on /api/v1/public; `MockJWT`→`Auth` |
| 5 | Next security headers | ✅ | `headers()` + CSP (connect-src=API origin) in both next.config.ts |
| 6 | CI scans + docs + secrets | ✅ | CI `security` job (govulncheck/gosec/pnpm audit), `make security`, SECURITY.md |

## Validation
| Level | Status | Notes |
|---|---|---|
| Static | ✅ | `go vet` clean; `golangci-lint` 0 issues |
| Unit | ✅ | auth (claim map), middleware (dev injects super_admin, bearer parse), config |
| Build | ✅ | `go build ./...` (api/worker/scheduler) + both `pnpm build` |
| Live | ✅ | headers on API; 30/min then 429; `/health` 200 unthrottled |

## Files Changed
Created: `internal/auth/entra.go`(+test), `internal/middleware/auth.go`(+test), `docs/SECURITY.md`.
Updated: `pkg/config/config.go`, `cmd/api/main.go`, `frontend/next.config.ts`, `career-portal/next.config.ts`, `.github/workflows/ci.yml`, `Makefile`, `go.mod`/`go.sum` (+go-oidc/v3, go-jose/v4, msgp via limiter).

## Deviations from Plan
- **`.env` already untracked** — `git ls-files` shows no `.env`; the `git rm --cached` step was unnecessary (the explore agent's "committed" claim was wrong). Confirmed only `.env.example` is tracked.
- **No separate `security_headers.go`** — helmet config inlined in `cmd/api/main.go` (simpler).
- **HR-auth bypass list added** (not in the plan's middleware sketch): `/health`, `/api/v1/public/*` (LINE-authed), `/api/v1/ps/*` (PS machine webhooks) must NOT require an Entra HR token. Documented; PS webhook auth flagged as a follow-up in SECURITY.md.
- **HSTS** only emitted over HTTPS by helmet (absent on local HTTP) — expected.

## Issues Encountered
- `limiter` needed `tinylib/msgp` go.sum entry → `go get .../limiter@v2.52.13` + `go mod tidy`.

## Tests Written
`internal/auth/entra_test.go` (claim mapping, first-role/empty-role), `internal/middleware/auth_test.go` (dev path super_admin, bearer extraction).

## Next Steps
- [ ] Code review · [ ] PR (wait for user to merge)
