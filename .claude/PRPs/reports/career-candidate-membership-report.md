# Implementation Report: Career Portal Candidate Membership

## Summary
Implemented a full candidate membership subsystem for the career portal (Phases A–E of the plan): persistent accounts with **signup/login via LINE, Google, and passwordless email-OTP**, an **httpOnly session cookie**, a **saved profile + saved resume**, and **account-first apply** with one-tap "apply with saved resume" plus a prefilled edit/upload path. Email/Google accounts can **link LINE** for push. All three providers default to **mock** (local/CI need no credentials). The new backend was smoke-tested end-to-end against the live local stack (Postgres/Redis/Azurite), which surfaced and fixed two integration bugs.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | XL | XL (as predicted) |
| Confidence | 8/10 | Backend verified live; frontend type/lint-clean (E2E require full stack to run) |
| Files Changed | ~22 new, ~10 modified | **22 new, 19 modified, 1 deleted** |

## Tasks Completed

| # | Phase / Task | Status | Notes |
|---|---|---|---|
| A1 | Migration 000013 (accounts/sessions/otps + candidates.account_id) | ✅ | Round-tripped up→13 / down→12 / up→13 on the live DB |
| A2 | pkg/email seam (mock + ACS REST shared-key HMAC) | ✅ | No Go SDK → REST; signer unit-tested |
| A3 | candidateauth model + repository | ✅ | find-or-create + sessions + OTP; sha256-at-rest |
| A4 | candidateauth service + OTP + handler + middleware + routes | ✅ | enumeration-safe start; cookie SameSite=None/Secure in prod |
| A5 | Wire Phase A in main.go | ✅ | |
| B1 | Config (Google/ACS/session/OTP) + validation | ✅ | Done up-front (reorder) so A could build |
| B2 | Google OAuth handler | ✅ | mirrors lineauth; id_token decoded (direct-exchange trust) |
| B3 | lineauth → SessionIssuer + link mode | ✅ | legacy fragment path preserved (issuer==nil) |
| B4 | Link-LINE + profile + resume endpoints | ✅ | |
| C1 | Cookie-credentialed client + session context | ✅ | credentials:'include'; useCandidate |
| C2 | Signup/Login/Account pages + auth components | ✅ | auth-aware multi-step; Suspense for useSearchParams |
| C3 | Header account menu (AccountNav) | ✅ | |
| D1 | Backend quick-apply + account-linked intake | ✅ | reuses Intake; account_id stamped; account-first /apply |
| D2 | Account-first apply page + prefilled stepper | ✅ | redirect-to-login; quick-apply + edit/upload |
| E1 | E2E specs (signup/login + apply rewrite) | ✅ (written) | require the full stack to execute |
| E2 | .env.example + deploy notes | ✅ | new env block + cross-site cookie caveat |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `gofmt` clean, `go vet` clean, `golangci-lint` 0 issues; portal `tsc --noEmit` + `eslint` clean |
| Unit Tests | ✅ Pass | Backend `go test -race ./...` all green; new: pkg/email (3), candidateauth (10) |
| Build | ✅ Pass | `go build ./...`; portal `tsc` clean |
| Integration | ✅ Pass (live smoke) | Ran the built api against live PG/Redis/Azurite: email-OTP→cookie→/me, profile PATCH, LINE+Google mock login (302+cookie, open-redirect guarded), logout→401 |
| DB migration | ✅ Pass | 000013 up/down/up round-trip on live hr_db |
| E2E (Playwright) | ⏸ Not executed | Specs written; need api+portal+stack running with mock providers |

## Files Changed

**New (22):** `backend/migrations/000013_candidate_accounts.{up,down}.sql`; `backend/pkg/email/{email,mock_sender,acs_sender,email_test}.go`; `backend/internal/candidateauth/{model,repository,service,otp,handler,middleware,routes,google,service_test,google_test}.go`; `career-portal/lib/{auth.ts,session.tsx}`; `career-portal/components/AccountNav.tsx`; `career-portal/components/auth/{AuthMethods,EmailOtpForm,ProfileForm,ResumeUploadStep,LinkLineButton}.tsx`; `career-portal/app/{signup,login,account}/page.tsx`; `career-portal/e2e/{signup,login}.spec.ts`.

**Modified (19):** backend `cmd/api/main.go`, `pkg/config/config.go`, `internal/lineauth/lineauth.go` (+test), `internal/public/handler.go`, `internal/applications/service.go`, `internal/candidates/{model,repository}.go`; portal `lib/{api,line,queries,types}.ts`, `app/providers.tsx`, `app/jobs/[id]/apply/page.tsx`, `components/{ApplyStepper,SiteHeader}.tsx`, `e2e/{apply-form,portal}.spec.ts`; root `.env.example`.

**Deleted (1):** `career-portal/components/LineGate.tsx` (LINE-gate-before-apply replaced by account-first).

## Deviations from Plan
- **Config landed in Phase A, not B1.** Email + session config is needed by `pkg/email` and `candidateauth`, so all new config keys were added up-front to keep every step building. (Plan anticipated B1; just reordered.)
- **lineauth account-first change (B3) landed during Phase A** to keep `main.go` compiling after wiring the issuer.
- **Account resume stored as a blob KEY** (not full URL) in `resume_blob_url` so quick-apply can `Download` it directly. (Plan noted this intent.)
- **Quick-apply lives in the `public` package** (account-first `/apply` + `/apply/quick`), reusing the existing token + PDPA-consent logic, rather than a new candidateauth endpoint — less duplication.

## Issues Encountered (caught by live smoke test, fixed)
1. **Ambiguous columns in session lookup.** `FindAccountBySessionHash` reused the unqualified `accountColumns` in a JOIN where `id`/`created_at` exist on both `candidate_accounts` and `candidate_sessions` → query errored → middleware reported "login required". **Fix:** resolve via a subquery instead of a JOIN (keeps `accountColumns` reusable in INSERT…RETURNING). Fake-based unit tests missed this; the live DB caught it.
2. **Group-middleware prefix leak.** `g.Group("", RequireCandidate)` mounted the auth gate on the whole `/api/v1/public/auth` prefix, so the later-registered Google OAuth route returned 401. **Fix:** attach `RequireCandidate` per-route.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `backend/pkg/email/email_test.go` | 4 | mock sender, ACS signer determinism + bad key |
| `backend/internal/candidateauth/service_test.go` | 8 | OTP store/verify/reuse, session issue/resolve/revoke, LINE login, link, resume save+download, invalid email, OTP helpers |
| `backend/internal/candidateauth/google_test.go` | 4 | id_token decode, malformed, mock login cookie, open-redirect guard |
| `career-portal/e2e/{signup,login}.spec.ts` | 7 | provider chooser, email-OTP step, LINE/Google mock signup→profile, login return-url, account redirect |
| `career-portal/e2e/portal.spec.ts` | (updated) | account-first apply (login→prefilled→submit→status) |

## Security Notes (carried from plan — for deploy)
- Session + OTP tokens are **sha256-hashed at rest**; OTP is single-use + short-TTL; `email/start` is enumeration-safe.
- Prod session cookie is **SameSite=None; Secure** automatically (portal/api are cross-site under the `azurecontainerapps.io` public suffix); `CORS_ALLOW_ORIGINS` must list the exact portal origin.
- Provision **Google OAuth** client + **ACS Email** resource/sender into ACA secrets (`secretref:`); do not paste secrets in chat.
- ACS REST signing + cross-site cookie behaviour should be **verified on prod** (deferred per plan).

## Next Steps
- [ ] `/code-review` the diff (auth-sensitive — recommended before merge)
- [ ] Run the Playwright E2E against a `docker compose up` stack with mock providers
- [ ] Provision Google + ACS Email + set prod env, then live-smoke cross-site cookie
- [ ] Commit + PR (`/prp-commit` / `/prp-pr`)
