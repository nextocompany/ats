# Implementation Report: Sprint 4c — Career Portal UI

## Summary
Built the public **Career Portal** as a separate Next.js app (`career-portal/`, runs on
**:3001**) for Thai job-seekers on mobile/LINE. Surfaces: `/jobs` (open positions),
`/jobs/[id]` (detail + Apply CTA), `/jobs/[id]/apply` (multi-step: PDPA consent → details →
resume upload + mock LINE login → status token), and `/status` (check by token). Warm,
mobile-first, trust-building direction distinct from the HR console. The backend consent
change (Task 1) was committed in the prior session (`4d32e0d`) and re-verified here.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium–Large | Medium (deterministic mirror of the HR app saved time) |
| Confidence | n/a | High — all 5 validation levels green |
| Files Changed | ~22 frontend + 2 backend | 25 frontend source files (42 incl. config); backend already done |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Backend — record PDPA consent on apply | ✅ Complete | Committed prior session (`4d32e0d`); re-verified 400/201 gate + 6 rows |
| 2 | Scaffold app + warm tokens | ✅ Complete | Hand-mirrored frontend setup (no interactive create-next-app); lean deps |
| 3 | API client + types + hooks + mock LINE | ✅ Complete | Envelope client w/ multipart `postForm`; `lib/line.ts` stub seam |
| 4 | Jobs list | ✅ Complete | `JobCard`, loading skeleton, empty + error states |
| 5 | Job detail + apply entry | ✅ Complete | Detail + prominent "สมัครงาน" CTA |
| 6 | Multi-step apply | ✅ Complete | Consent→details→resume; client file validation; token success screen |
| 7 | Status page | ✅ Complete | `?token=` prefill (Suspense-wrapped); friendly Thai labels; not-found |
| 8 | CORS + docs + e2e | ✅ Complete | CORS `:3001` already set; Playwright (3 viewports) + README + .env.example |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `tsc --noEmit` clean; `eslint` clean |
| Unit Tests | ✅ Pass | 3 `buildApplyForm` tests (multipart field contract) |
| Build | ✅ Pass | `next build` green; 5 routes |
| Integration | ✅ Pass | 18 Playwright tests across 320/375/768 against live stack |
| Edge Cases | ✅ Pass | empty/error states, consent gate (400 + client block), file type/size, unknown token, 320px no-overflow |

## Files Changed
- 25 source files created under `career-portal/` (app pages, components, lib, ui primitives, e2e)
- Plus config: `package.json`, `tsconfig.json`, `next.config.ts`, `postcss.config.mjs`,
  `eslint.config.mjs`, `components.json`, `playwright.config.ts`, `.gitignore`,
  `.env.example`, `.env.local`, `README.md`
- Backend (Task 1, prior commit `4d32e0d`): `internal/public/handler.go`, `cmd/api/main.go`,
  `pkg/config/config.go`

## Deviations from Plan
- **Scaffolding method**: used a deterministic hand-mirror of the proven `frontend/` setup
  instead of interactive `pnpm create next-app` / `shadcn init`. WHY: avoids interactive
  prompts in a non-interactive session and guarantees the exact Next 16 / Tailwind v4 /
  base-nova config already validated in S4b.
- **Leaner deps**: dropped `recharts`, `sonner`, `next-themes` (dashboard-only). WHY: the
  portal is a mobile microsite (perf budget); errors shown inline, no charts/toasts needed.
- **Playwright browser**: all 3 projects pinned to chromium (only browser cached on this
  machine) via explicit mobile viewports rather than the WebKit `iPhone SE` device profile.

## Issues Encountered
- **base-ui Button has no `asChild`** (known gotcha) — `<Button asChild><Link/></Button>`
  failed typecheck. Resolved by using `buttonVariants()` on `<Link>`, matching prior sprints.
- **`useSearchParams` prerender** — wrapped the status page content in `<Suspense>`.
- **WebKit not installed** — first Playwright run failed only on the `iPhone SE` (WebKit)
  project; switched all projects to chromium viewports.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `e2e/portal.spec.ts` | 3 × 3 viewports | jobs list, status not-found, full apply→token→status flow + screenshots |
| `e2e/apply-form.spec.ts` | 3 × 3 viewports | `buildApplyForm` multipart field contract |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Create PR via `/prp-pr` → squash-merge to `main` (like S0–S4b)
