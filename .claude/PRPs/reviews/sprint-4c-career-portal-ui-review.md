# Code Review: Sprint 4c — Career Portal UI (local, pre-PR)

**Reviewed**: 2026-05-30
**Branch**: `feat/sprint-4c-career-portal-ui`
**Scope**: 37 new files under `career-portal/` (backend Task 1 reviewed/merged-ready in `4d32e0d`)
**Decision**: APPROVE (one MEDIUM fixed during commit)

## Summary
A self-contained, mobile-first Next.js portal that mirrors the proven S4b frontend stack.
Clean static analysis, no security issues, all five validation levels green. One
onboarding nit (`.env.example` is gitignored) addressed by force-adding the file.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
- **`.env.example` ignored by `.gitignore` `.env*` pattern** (`career-portal/.gitignore`).
  The README instructs `cp .env.example .env.local`, but the example file would not ship.
  The sibling `frontend/` app has the same gap (its `.env.example` is also untracked).
  **Fix applied**: force-add `career-portal/.env.example` (non-secret — only the public
  `NEXT_PUBLIC_API_URL`) so the documented setup works on a fresh clone.

### LOW
- `lib/line.ts#getIdToken` is `async` yet returns a constant stub. Intentional and
  documented — keeps the signature stable for the LIFF drop-in. No change.
- `app/status/page.tsx` reads the token from the URL and queries immediately; opaque
  24-byte token is the intended candidate handle (matches the API). Acceptable.
- `ApplyStepper.tsx` is 252 lines — the largest file, well under the 800 limit and a single
  cohesive flow with small internal handlers. No split needed.

## Category Checklist
- **Correctness**: file-validation state transitions, consent/LINE gating, and status
  prefill all handle empty/invalid paths. Verified end-to-end via Playwright. ✓
- **Type Safety**: no `any`, no unsafe casts; explicit types on lib APIs. ✓
- **Pattern Compliance**: mirrors frontend envelope client, TanStack hooks, base-nova ui,
  CSS tokens; PascalCase components / camelCase hooks. ✓
- **Security**: no secrets; public-by-design API; client file gate mirrors server (415/413);
  server enforces consent (400) + LINE verify; no `dangerouslySetInnerHTML`/XSS. ✓
- **Performance**: lean deps (dropped recharts/sonner/next-themes); skeletons; static
  jobs/status routes; compositor-friendly transitions only. ✓
- **Completeness**: e2e (3 flows × 3 viewports) + unit (buildApplyForm) + README + CORS. ✓
- **Maintainability**: no console.log/TODO/FIXME; named constants for limits/versions. ✓

## Validation Results

| Check | Result |
|---|---|
| Type check (`tsc --noEmit`) | Pass |
| Lint (`eslint`) | Pass |
| Tests (Playwright, 18) | Pass |
| Build (`next build`) | Pass |
| Backend vet (Task 1) | Pass |
| Consent gate (curl 400/201) + DB row | Pass (6 `career_portal` rows) |

## Files Reviewed
37 added files under `career-portal/` — app pages (6), components (7 + 6 ui), lib (5),
e2e (2), config (11). Generated `next-env.d.ts` and `*.tsbuildinfo` correctly gitignored.
