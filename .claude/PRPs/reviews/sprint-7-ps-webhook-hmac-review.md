# Code Review: Sprint 7 — PeopleSoft Webhook HMAC Authentication

**Reviewed**: 2026-06-03
**Mode**: Local (uncommitted changes; no PR yet)
**Branch**: `feat/s7-ps-webhook-hmac` → `main`
**Decision**: ✅ **APPROVE with comments** (1 MEDIUM found **and fixed during review**)

## Summary
Security-focused review of a webhook-authentication change. The implementation correctly uses constant-time HMAC comparison, gates via the established mock-default seam, fails fast in real mode, and never logs the secret. One interop robustness issue (hex case-sensitivity) was found and fixed in-review with a regression test. No CRITICAL/HIGH; all validation green.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM

- **[M1] Signature compare was hex-case-sensitive — FIXED in review.** Original code compared hex *strings* (`hmac.Equal([]byte(want), []byte(got))`) where `want` is lowercase from `hex.EncodeToString`. A signer emitting **uppercase** hex would be rejected despite a correct MAC — fails *closed* (not a vulnerability) but is exactly the "signature encoding mismatch" the plan flagged as the top integration risk. **Fix applied**: decode `got` via `hex.DecodeString` and compare **raw MAC bytes** with `hmac.Equal` (accepts upper/lower case, validates hex format, still constant-time). Added `TestVerifyHMAC_AcceptsUppercaseHex` and `TestVerifyHMAC_NonHexSignature`. Re-verified green.

### LOW

- **[L1] Replay protection intentionally out of scope** — a captured, validly-signed request can be replayed; HMAC+TLS doesn't detect it. Documented as optional hardening in `docs/SECURITY.md`. Acceptable for this slice.
- **[L2] Invalid-but-present signature hashes the body before rejecting** — an attacker sending a junk (but hex) signature forces an HMAC over the body (≤ Fiber's 4 MB BodyLimit) before the 401. Negligible (SHA-256 is cheap, bounded); the missing-header case short-circuits before any read. No action.
- **[L3] `.env.example` path** — plan said `backend/.env.example`; the real tracked file is repo-root `.env.example`. Already noted in the report; informational.

## Detailed Notes by File

| File | Assessment |
|---|---|
| `internal/peoplesoft/webhook_auth.go` | ✔️ `hmac.Equal` constant-time; secret never logged (only path); `c.Body()` buffered (BodyParser intact — proven by handler tests). Post-fix: decodes hex + compares raw bytes. |
| `internal/peoplesoft/routes.go` | ✔️ `/health` registered before the guard (stays open); `ps.Use(VerifyHMAC)` only when `secret != ""`; app-level Auth already bypasses `/api/v1/ps`, so HMAC is the sole guard — correct. |
| `pkg/config/config.go` | ✔️ Secret via `os.Getenv` (no default); fail-fast in the existing `UsesRealPeopleSoft()` block, mirroring the IB-creds check. |
| `pkg/config/config_test.go` | ✔️ Covers real-without-secret (error) and real-with-secret (loads); `t.Setenv` auto-cleanup. |
| `internal/peoplesoft/webhook_test.go` | ✔️ Harness updated to `RegisterRoutes(app, h, "")`; existing tests stay green = regression guard for the open-when-mock path + BodyParser. |
| `cmd/api/main.go` | ✔️ Passes `cfg.PSWebhookSecret`; real-mode validation guarantees non-empty. |
| `.env.example` / `docs/SECURITY.md` | ✔️ Env var + signature contract documented; no secret committed; follow-up flipped to resolved with replay noted. |

## Validation Results

| Check | Result |
|---|---|
| Vet | ✅ Pass |
| Lint (golangci-lint v2.11.3) | ✅ Pass (`0 issues.`) |
| Tests | ✅ Pass (15 pkgs; 8 HMAC + 2 config tests, incl. new uppercase/non-hex) |
| gosec | ✅ Pass (exit 0; no secret leakage) |
| Build | ✅ Pass |

## Files Reviewed
- `backend/internal/peoplesoft/webhook_auth.go` — Added
- `backend/internal/peoplesoft/webhook_auth_test.go` — Added
- `backend/internal/peoplesoft/routes.go` — Modified
- `backend/internal/peoplesoft/webhook_test.go` — Modified
- `backend/pkg/config/config.go` — Modified
- `backend/pkg/config/config_test.go` — Modified
- `backend/cmd/api/main.go` — Modified
- `.env.example` — Modified
- `docs/SECURITY.md` — Modified

## Decision Rationale
Zero CRITICAL/HIGH; the one MEDIUM was fixed in-review with tests; all validation green. The security fundamentals (constant-time compare, no secret leakage, fail-fast real-mode, open-probe carve-out) are correct. **Approve** — safe to commit and open a PR.
