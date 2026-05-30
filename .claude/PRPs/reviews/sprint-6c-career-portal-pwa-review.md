# Code Review: Sprint 6c — Career Portal PWA (local, uncommitted)

**Reviewed**: 2026-05-30
**Branch**: `feat/sprint-6c-career-portal-pwa`
**Scope**: career-portal PWA + cross-app CSP fix (career-portal + frontend)
**Decision**: APPROVE with comments

## Summary
Clean, well-scoped PWA implementation (Serwist manifest/SW/offline/install prompt)
plus a deliberate, user-approved CSP relaxation to make Next prod builds hydrate.
No CRITICAL or HIGH issues. One MEDIUM worth recording (the `'unsafe-inline'`
trade-off) and a couple of LOW notes.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
1. **`script-src 'self' 'unsafe-inline'` weakens XSS defense-in-depth**
   (`career-portal/next.config.ts`, `frontend/next.config.ts`).
   This is the user-approved, documented trade-off: per-request nonces can't be
   applied to Next's statically-prerendered pages (Next emits `nonce=undefined`),
   so inline hydration scripts require `'unsafe-inline'`. Risk is bounded — both
   apps render via React (auto-escaping), don't inject user HTML into scripts, and
   the portal sanitizes inputs. **Mitigation/follow-up (not S6):** tighten via
   script hashes, or nonce + forced dynamic rendering if the static/CDN cost is
   acceptable. Recorded in README + report.

### LOW
1. **`new URL(apiOrigin)` in `app/sw.ts:19`** throws if `NEXT_PUBLIC_API_URL` is
   malformed (SW init fails). Acceptable: value is build-inlined and already
   exercised by `lib/api.ts`; a bad value would fail the whole app earlier.
2. **`/api/v1/public/status/:token` GETs are cached (NetworkFirst).** Intended
   (warm offline status). The token is in the URL path so the cache key is
   per-token — no cross-token leakage. Apply POST is structurally excluded (matcher
   requires `GET`). Good.
3. **InstallPrompt `handleInstall`/`handleDismiss` declared in render body** — fine
   for React; no memoization needed here.

## Notes (positive)
- SW caches **only** public GETs; apply POST (PII + LINE id-token) never cached/replayed — matches the plan's security intent.
- a11y: install prompt has `role="region"` + labels, dismiss button `aria-label`; offline page uses semantic `<h1>` + PortalShell. Reduced-motion handled globally in `globals.css`.
- No secrets, no `console.log`, no `dangerouslySetInnerHTML` in new source.
- Generated `public/sw.js` correctly gitignored + eslint-ignored.
- Portal `middleware.ts` cleanly removed (CSP back to header form); dashboard `middleware.ts` restored to its original auth-only state (no scope creep).

## Validation Results

| Check | career-portal | frontend (dashboard) |
|---|---|---|
| Type check (`tsc --noEmit`) | Pass | Pass |
| Lint (`eslint`) | Pass | Pass |
| Build | Pass (`--webpack`) | Pass (turbopack) |
| Tests (Playwright) | Pass — **30/30** (PWA+portal+apply) | Smoke pass (0 CSP violations, hydrates, auth gate) |
| Prod CSP | 0 violations, SW registers, `/status` hydrates | 0 violations, `/login` hydrates |

## Files Reviewed
- `career-portal/app/sw.ts` (Added)
- `career-portal/app/manifest.ts` (Added)
- `career-portal/app/offline/page.tsx` (Added)
- `career-portal/components/InstallPrompt.tsx` (Added)
- `career-portal/e2e/pwa.spec.ts` (Added)
- `career-portal/app/jobs/page.tsx` (Modified — mount InstallPrompt)
- `career-portal/app/layout.tsx` (Modified — PWA metadata)
- `career-portal/next.config.ts` (Modified — withSerwist + CSP)
- `career-portal/eslint.config.mjs`, `.gitignore`, `package.json`, `README.md` (Modified)
- `career-portal/public/*` (Added — 5 binary icons, not code-reviewed)
- `frontend/next.config.ts` (Modified — script-src relaxed)

## Decision
**APPROVE with comments.** No blocking issues. The MEDIUM CSP item is an accepted,
documented trade-off with a clear hardening path; safe to merge per the sprint cadence.
