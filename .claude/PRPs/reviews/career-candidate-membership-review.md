# Code Review: Career Portal Candidate Membership (local, uncommitted)

**Reviewed**: 2026-06-13
**Branch**: feat/career-candidate-membership → main
**Scope**: 22 new, 19 modified, 1 deleted (backend Go + career-portal Next.js + migration)
**Decision**: ✅ **APPROVE with comments** — both HIGH findings fixed during review; MEDIUM/LOW are follow-ups.

## Summary
Auth/PII-sensitive feature reviewed adversarially. Logic, types, and patterns are sound and match the codebase. Two real security gaps were found and **fixed in this pass** (CSRF on cookie-authed multipart endpoints; fabricated PDPA consent for members). Remaining items are hardening recommendations, not blockers.

## Findings

### CRITICAL
None.

### HIGH (both FIXED in this review)
- **H1 — CSRF on cookie-authed state-changing endpoints.** Session cookie is `SameSite=None` in prod (portal↔api are cross-site under the `azurecontainerapps.io` public suffix). JSON endpoints are protected by CORS preflight, but **multipart** endpoints (`POST /api/v1/public/apply` account-first, `POST /api/v1/public/auth/resume`) are "simple requests" with no preflight → a malicious site could overwrite a victim's saved resume or submit applications using their cookie.
  **Fix:** added `candidateauth.EnforceOrigin(allowlist)` (rejects non-safe-method requests whose `Origin` is not allowlisted) mounted on `/api/v1/public/auth` and `/api/v1/public/apply`. Verified live: evil Origin → 403, allowed Origin → 200, GET (OAuth) passes.
- **H2 — Fabricated PDPA consent for members.** `finalizeApplication` recorded `ConsentGiven: true` unconditionally; an OAuth member who never completed the consent step could apply (account-first) and we'd record consent they never gave — a PDPA-compliance defect.
  **Fix:** `Apply` (member path) and `QuickApply` now require `acct.PDPAConsent == true` (else 400) and record using the account's real `PDPAVersion`. Verified the gate is in place.

### MEDIUM (recommended follow-ups — not fixed)
- **M1 — OTP brute-force throttling.** A 6-digit code (1M space) is bounded only by the per-IP public rate limiter (30/min) + 10-min TTL (~300 guesses/code window ≈ 0.03%). The `email_otps.attempts` column is only bumped on a *matching* code, so it never counts failures. **Recommend:** count failed verifies per email and invalidate the challenge after N (e.g., 5), independent of IP.
- **M2 — No cleanup of expired rows.** `email_otps` and `candidate_sessions` grow unbounded (consumed/expired rows are never purged). **Recommend:** a periodic sweep (reuse the retention scheduler) deleting expired/consumed rows.
- **M3 — `resume_blob_url` stores a blob KEY, not a URL.** Intentional (so `Download` works) and documented in code, but the name is misleading. **Recommend:** rename to `resume_blob_key` in a later migration, or add a column comment.

### LOW
- **L1 — Google id_token `aud`/`iss` not validated.** Acceptable because the token is taken directly from Google's token endpoint over TLS in the server-to-server exchange (documented trust). Add `aud == client_id` validation if the flow ever accepts client-supplied id_tokens.
- **L2 — Cross-provider email unification invariant.** `findOrCreateBySub` merges onto an existing email account when the provider email matches. This is safe *only because* LINE login requests scope `openid profile` (no email → no LINE-driven merge) and Google merges only when `email_verified`. Worth a regression test pinning the LINE scope so a future change can't open an account-takeover path.
- **L3 — gosec G101 (not run here).** Var/field names like `channelSecret`, `tokenHash`, `ACSEmailAccessKey`, `GoogleClientSecret` may trip gosec's hardcoded-credential heuristic in CI. Rename or `//nosec G101` if CI flags them (per project lesson, prefer renaming).
- **L4 — Legacy guest `/apply` (LINE id-token) still reachable.** The UI is account-first, but the endpoint still accepts a LINE id-token identity as a fallback. Intentional during transition; consider removing once membership is fully rolled out.

## Validation Results

| Check | Result |
|---|---|
| Type check (`go vet`, portal `tsc --noEmit`) | ✅ Pass |
| Lint (`golangci-lint`, `eslint`) | ✅ Pass (0 issues) |
| Tests (`go test -race ./...`, portal unit) | ✅ Pass (incl. new EnforceOrigin test) |
| Build (`go build ./...`, `pnpm build`) | ✅ Pass |
| DB migration round-trip | ✅ Pass |
| Live smoke (email-OTP, OAuth, CSRF guard, consent gate) | ✅ Pass |

## Files Reviewed
All 42 changed paths (backend `candidateauth`/`email`/`public`/`lineauth`/`config`/`candidates`/`applications`/`main.go` + migration; portal `lib`/`components`/`app` auth surfaces + e2e). Focus on the auth/PII/session/OAuth/apply paths.

## Decision Rationale
Zero CRITICAL; both HIGH fixed and verified; remaining MEDIUM/LOW are hardening follow-ups suitable for backlog. Recommend addressing **M1 (OTP throttle)** and **M2 (row cleanup)** soon after merge, and verifying CSRF + ACS signing + cross-site cookie on prod (deferred per plan).
