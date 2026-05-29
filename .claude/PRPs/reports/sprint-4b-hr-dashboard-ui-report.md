# Implementation Report: Sprint 4b — HR Dashboard UI (Next.js)

## Summary
Built the **HR Dashboard** as a Next.js 16 (App Router) app in a **light operations console** direction, consuming the Sprint 4a API. Delivers a ranked, filterable, paginated applications **inbox** with multi-select **bulk actions**; an application **detail** view with the resume (native PDF via signed URL) beside an **AI summary/score** panel + status actions (incl. hire→PeopleSoft); **candidate profile + timeline**; and an **analytics** overview (KPI cards + funnel + sources via Recharts). Dev auth is a lightweight session cookie (the Go API trusts a mock super_admin). Validated with Playwright (5 specs) + visual screenshots at 320/768/1024/1440 + `next build`.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 7/10 | Single-pass; 3 build/type fixups (Suspense, Button asChild, Select null) |
| Files Changed | ~30 | 52 frontend files (incl. shadcn primitives) + 2 backend |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Backend CORS | ✅ | Fiber cors middleware; `CORS_ALLOW_ORIGINS` config; preflight 204 verified |
| 2 | Scaffold (Next 16 + Tailwind 4 + shadcn + deps) | ✅ | TanStack Query, Recharts, lucide; design tokens + Noto Sans Thai |
| 3 | API client + types + Query hooks | ✅ | envelope-aware client; typed hooks + mutations |
| 4 | Mock auth + middleware gate + login | ✅ | cookie session (NextAuth/Azure AD deferred) |
| 5 | App shell + layout | ✅ | semantic header/nav, skip-link, responsive |
| 6 | Ranked inbox | ✅ | filters (status/min_score) in URL, pagination (Meta), score badges, multi-select |
| 7 | Bulk action bar | ✅ | shortlist/interview/reject → /bulk, toast, invalidate |
| 8 | Application detail | ✅ | resume iframe (signed URL) + AI panel + status actions incl. hire |
| 9 | Candidate list + profile/timeline | ✅ | detail composes applications + activity timeline |
| 10 | Analytics overview | ✅ | KPI cards + funnel + sources (Recharts) |
| 11 | E2E + visual + build | ✅ | 5 Playwright specs green; screenshots at all breakpoints |
| 12 | Docs + env | ✅ | `.env.example`, README |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static (lint + tsc) | ✅ | `pnpm lint` clean; `tsc --noEmit` clean |
| Build | ✅ | `pnpm build` — 9 routes compiled |
| Backend (CORS) | ✅ | preflight 204 + `Access-Control-Allow-Origin` header |
| E2E (Playwright) | ✅ | 5/5 pass: login gate, inbox+responsive, detail (resume+AI panel), analytics, candidates |
| Visual | ✅ | screenshots 320/768/1024/1440 — inbox + detail + analytics rendered (verified) |

### Visual evidence
Inbox renders the light operations console: green AI-score badges (86), status/gate columns, filters, semantic nav, pagination. Analytics shows KPI cards (Applied 8 / Passed 7 / Onboarded 1 / Waiting 5) + funnel + sources charts.

## Files Changed
52 frontend files (app routes under `(app)`/`(auth)`, `lib/` client+hooks+auth+types, `components/` shell/inbox/bulk/resume/analytics + shadcn ui, `middleware.ts`, Playwright). 2 backend: `cmd/api/main.go` (CORS), `pkg/config/config.go` (`CORSAllowOrigins`).

## Deviations from Plan
1. **Next.js 16** (create-next-app@latest), not 15 — App Router/React 19 identical; no functional impact.
2. **Lightweight cookie auth** instead of NextAuth v5 — avoids Auth.js beta churn; the API trusts the dev mock anyway. Real Azure AD/NextAuth is a later wire-up. (Plan allowed mock; this is simpler.)
3. **Route group `(app)`** for shared chrome; feature pages live there (plan listed flat paths).
4. **`useSearchParams` Suspense wrappers** on inbox + candidates (Next prerender requirement).
5. **Recharts `isAnimationActive={false}`** — deterministic rendering + reduced-motion friendly.
6. shadcn Button (base-ui primitive) has no `asChild` → used `buttonVariants()` on a Link.

## Issues Encountered (all resolved)
- `useSearchParams` build error → Suspense boundaries.
- Select `onValueChange` typed `string | null` → coerced.
- Button `asChild` unsupported by this primitive → `buttonVariants()`.
- Running api image predated CORS (`up -d` not `--build`) → rebuilt; preflight then 204.

## Tests Written
| Test File | Tests | Area |
|---|---|---|
| `frontend/e2e/dashboard.spec.ts` | 5 | login gate; inbox + responsive screenshots; inbox→detail; analytics charts; candidates list |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Open PR (branch `feat/sprint-4b-hr-dashboard-ui`)
- [ ] **Sprint 4c**: public Career Portal UI (Next.js) on `/public/*` — reuses this app's token/client/design patterns.
- [ ] Later: real Azure AD SSO (NextAuth/Entra) replacing the dev cookie; PWA; per-role scoped views.
