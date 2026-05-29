# Plan: Sprint 4b — HR Dashboard UI (Next.js)

## Summary
Build the **HR Dashboard** as a Next.js 15 (App Router) app in a **light operations console** direction, consuming the Sprint 4a API. Surfaces: a ranked, filterable applications **inbox**; an application **detail** view with the resume (native PDF via signed URL) beside an **AI summary/score** panel + status actions (incl. hire→PeopleSoft); **bulk actions**; **candidate profile + timeline**; and an **analytics** overview (funnel / KPI / sources via Recharts). Dev auth is mocked (the Go API already trusts a mock super_admin). Validation is Playwright + visual screenshots at the standard breakpoints.

## User Story
As an **HR reviewer**, I want **a fast, dense dashboard to triage the ranked inbox, open a candidate's resume next to the AI summary, action candidates (single or bulk), and see recruitment analytics**, so that **screening drops from ~10 min to ~2 min per CV (the project's headline goal)**.

## Problem → Solution
**Current state (post-4a):** A complete, tested HR API exists but there is no UI — HR can't see the inbox, resumes, or analytics.
**Desired state:** `frontend/` Next.js app: login (mock) → dashboard overview → ranked inbox (filter/paginate/select) → detail (resume + AI panel + actions) → candidate profile/timeline → analytics. Responsive, accessible, intentional (not a template).

## Metadata
- **Complexity**: Large (first frontend app; ~30 files)
- **Source PRD**: PRP v1.0 — Sprint 4 (W9–10), HR Dashboard UI (F05/F08/F10 frontend; F13 PDPA view)
- **Decisions locked**: **HR Dashboard only** (Career Portal UI = Sprint 4c); resume viewer = **native/iframe + AI panel**; mock NextAuth dev auth; TanStack Query; shadcn/ui + Tailwind + Recharts; separate `frontend/` app
- **Estimated Files**: ~30
- **Validation shift**: Playwright + visual (breakpoints 320/768/1024/1440), a11y, reduced-motion — not curl

---

## UX Design

### Direction: Light Operations Console (Swiss / data-first)
Dense, information-first, used all day. Strong type hierarchy, restrained neutral palette, **one semantic accent** mapped to AI score (e.g. score color ramp), table/inbox-centric, intentional hover/focus/active states. Explicitly avoids the banned generic dark-dashboard / uniform-card-grid template.

### After (inbox + detail)
```
┌───────────────────────────────────────────────┐
│ HR ATS   Inbox  Candidates  Analytics   [user] │  ← semantic <header><nav>
├───────────┬───────────────────────────────────┤
│ Filters   │  Ranked Inbox            [bulk bar]│
│ status ▾  │  ◻ Score  Name      Position  Store│
│ score ▭   │  ◻  92    สมชาย…     Cashier  CM   │  ← score badge (accent ramp)
│ store ▾   │  ◻  86    ...                       │
│           │  …                         ‹ 1 2 › │  ← pagination (Meta)
└───────────┴───────────────────────────────────┘
detail: ┌ resume (iframe, signed URL) ┊ AI summary + breakdown + actions ┐
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| See inbox | none | ranked table, filter, paginate, multi-select | F05/F10 |
| Review CV | none | PDF (native) + AI summary/score side-by-side | core 2-min goal |
| Action | API only | status dropdown + bulk bar (approve/reject/shortlist) | writes via 4a |
| Hire | curl | "Hire" action → PATCH status → PS sync (4a) | |
| Analytics | none | KPI cards + funnel + sources charts | F08 |

---

## Mandatory Reading (the API contract — 4a handlers)
| Priority | File | Why |
|---|---|---|
| P0 | `backend/internal/applications/dashboard_handler.go` | `GET /applications` (filters + Meta), `POST /applications/bulk`, `GET /applications/:id/resume` shapes |
| P0 | `backend/internal/applications/handler.go` | `GET /applications/:id`, `PATCH /applications/:id/status` (status set incl. hired) |
| P0 | `backend/internal/profiles/handler.go` | `GET /candidates`, `/candidates/:id` (candidate+applications), `/:id/timeline` |
| P0 | `backend/internal/reports/handler.go` | `/reports/funnel|kpi|sources` payloads |
| P0 | `backend/pkg/httpx/response.go` | envelope `{success,data,error,meta}` + `Meta{total,page,limit}` — the client unwraps this |
| P1 | `backend/internal/users/handler.go` | `GET /users/me` (role/store/subregion) |
| P1 | `backend/internal/pdpa/handler.go` | consent GET/POST for the PDPA panel |
| P1 | `~/.claude/rules/ecc/web/*` | design-quality, performance, testing, security (CSP), coding-style |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Next.js 15 App Router | nextjs.org/docs | Server Components by default; mark interactive views `"use client"`; route handlers/BFF optional. |
| shadcn/ui | ui.shadcn.com | `npx shadcn@latest init` then add `table`, `button`, `badge`, `select`, `dialog`, `dropdown-menu`, `card`, `checkbox`, `sonner`. |
| TanStack Query | tanstack.com/query | server-state cache; `QueryClientProvider` in a client root; query keys per filter; mutations for bulk/status with invalidation. |
| Recharts | recharts.org | `BarChart`/`FunnelChart`-style for funnel + sources; responsive container. |
| NextAuth (mock) | authjs.dev | Credentials provider with a single dev user for local gating; real Azure AD (`@auth/...`) is a later wire-up. |

### Research Notes
```
KEY_INSIGHT: dev auth is essentially free.
APPLIES_TO: API calls from the browser.
GOTCHA: the Go API's MockJWT trusts a super_admin in ENV=development regardless of token, so dev API calls need no bearer. NextAuth here only gates the UI + drives the login screen. Real Azure AD token forwarding is a later sprint.

KEY_INSIGHT: CORS is required.
APPLIES_TO: backend.
GOTCHA: browser at :3000 → API at :8080 is cross-origin. Add Fiber CORS middleware (allow http://localhost:3000, credentials) — a small backend change in this sprint.

KEY_INSIGHT: resume via signed URL.
APPLIES_TO: detail view.
GOTCHA: call GET /applications/:id/resume to get a short-lived SAS URL, then render it in <iframe>/<object>. Don't proxy the bytes through Next.

KEY_INSIGHT: envelope unwrap + Meta.
APPLIES_TO: api client.
GOTCHA: every response is {success,data,error,meta}; the client returns data and surfaces error; list hooks read meta for pagination.
```

---

## Patterns to Mirror (establish for the frontend; mirror web rules)
### DESIGN_TOKENS (CSS custom properties — coding-style/web)
```css
:root {
  --color-surface: oklch(99% 0 0);
  --color-text: oklch(20% 0 0);
  --color-muted: oklch(55% 0 0);
  --color-accent: oklch(62% 0.17 250);     /* single semantic accent */
  --score-high: oklch(65% 0.17 150);
  --score-mid:  oklch(75% 0.15 85);
  --score-low:  oklch(63% 0.20 25);
  --text-xs:.75rem; --text-sm:.875rem; --text-base:1rem; --text-2xl:1.5rem;
  --space-1:.25rem; --space-2:.5rem; --space-4:1rem; --space-6:1.5rem;
  --radius:.5rem; --duration:150ms; --ease:cubic-bezier(.16,1,.3,1);
}
```
### API_CLIENT (envelope-aware, typed)
```ts
// lib/api.ts — unwrap {success,data,error,meta}; throw ApiError on !success.
async function apiGet<T>(path: string): Promise<{ data: T; meta?: Meta }> { ... }
```
### COMPONENT_NAMING / STATE (patterns/web)
- Components PascalCase; hooks `useX`; CSS kebab-case.
- Server state via TanStack Query (no duplicating into client stores); URL holds filters/pagination/active tab.
- Container/presentational split; keyboard/ARIA/focus in headless layer.

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/middleware/cors.go` + wire in `cmd/api/main.go` | CREATE/UPDATE | Fiber CORS allowing the dev origin |
| `frontend/package.json`, `next.config.ts`, `tsconfig.json`, `tailwind.config.ts`, `postcss` | CREATE | scaffold |
| `frontend/app/layout.tsx`, `globals.css` (tokens), `providers.tsx` | CREATE | shell + QueryClient + theme tokens |
| `frontend/app/(auth)/login/page.tsx` + `middleware.ts` | CREATE | mock login + route gate |
| `frontend/lib/{api.ts,types.ts,queries.ts,auth.ts}` | CREATE | typed envelope client, TanStack hooks, NextAuth mock |
| `frontend/app/dashboard/page.tsx` | CREATE | overview (KPI cards + funnel) |
| `frontend/app/applications/page.tsx` | CREATE | ranked inbox (filters/paginate/select) |
| `frontend/app/applications/[id]/page.tsx` | CREATE | detail: resume + AI panel + actions |
| `frontend/app/candidates/page.tsx`, `candidates/[id]/page.tsx` | CREATE | candidate list + profile/timeline |
| `frontend/app/analytics/page.tsx` | CREATE | funnel + sources + KPI charts |
| `frontend/components/ui/*` | CREATE | shadcn primitives (table/button/badge/select/dialog/checkbox/card/dropdown/sonner) |
| `frontend/components/inbox/{InboxTable,Filters,ScoreBadge,Pagination}.tsx` | CREATE | inbox composition |
| `frontend/components/bulk/BulkActionBar.tsx` | CREATE | selection + bulk mutations |
| `frontend/components/resume/{ResumeViewer,AiSummaryPanel}.tsx` | CREATE | iframe PDF + AI panel |
| `frontend/components/analytics/{FunnelChart,SourcesChart,KpiCards}.tsx` | CREATE | Recharts widgets |
| `frontend/components/shell/{AppHeader,SideFilters}.tsx` | CREATE | semantic layout |
| `frontend/e2e/*.spec.ts`, `playwright.config.ts` | CREATE | e2e + visual + a11y |
| `frontend/.env.example`, `README` update | CREATE/UPDATE | `NEXT_PUBLIC_API_URL`, run docs |

## NOT Building (4c / later)
- **Career Portal UI** (public app) — Sprint 4c (consumes `/public/*`).
- **Real Azure AD SSO** token forwarding — mock NextAuth + API mock JWT for now.
- **PWA / offline / install** (F17 beyond responsive), **semantic search UI** (F12), **user admin CRUD UI**, **profile edit / manual merge UI**.
- **LINE notifications** (S5). PDPA shown read-only on the profile (+ a consent toggle that calls 4a).
- New backend endpoints — UI consumes 4a as-is (only CORS is added).

---

## Step-by-Step Tasks

### Task 1: Backend CORS
- **ACTION**: Add Fiber CORS middleware (allow `http://localhost:3000`, common methods/headers, credentials) in `cmd/api/main.go` before routes.
- **GOTCHA**: dev-origin only; tighten for prod via env. Don't break existing routes.
- **VALIDATE**: `curl -I -X OPTIONS` preflight returns the ACAO header; backend tests still pass.

### Task 2: Scaffold Next.js + Tailwind + shadcn + deps + tokens
- **ACTION**: `pnpm create next-app frontend` (TS, App Router, Tailwind, ESLint, src off); `npx shadcn@latest init`; add deps: `@tanstack/react-query`, `recharts`, `next-auth`, `lucide-react`, shadcn components. Define design tokens in `globals.css`; set font pairing (one display + one text) with `next/font`.
- **MIRROR**: DESIGN_TOKENS; performance rules (font-display swap, ≤2 families).
- **GOTCHA**: pin Next 15 / React 19; ensure Tailwind v4 config matches scaffold; keep landing JS budget lean.
- **VALIDATE**: `pnpm --filter frontend build` succeeds; `pnpm dev` serves.

### Task 3: API client + types + Query provider
- **ACTION**: `lib/types.ts` (Application, Candidate, Funnel, KPI, Source, Meta, envelope); `lib/api.ts` (envelope-aware `apiGet/apiPost/apiPatch` against `NEXT_PUBLIC_API_URL`, throws `ApiError`); `lib/queries.ts` (TanStack hooks: `useApplications(filter)`, `useApplication(id)`, `useResumeUrl(id)`, `useCandidate(id)`, `useReports`, mutations `useBulk`, `useSetStatus`); `app/providers.tsx` (QueryClientProvider).
- **MIRROR**: API_CLIENT; STATE (server state in Query; filters in URL).
- **GOTCHA**: unwrap `data`; surface `error`; query keys include filter for cache correctness.
- **VALIDATE**: typecheck passes; a hook fetches `/users/me` on the shell.

### Task 4: Mock auth + route gate + login
- **ACTION**: NextAuth Credentials provider with a single dev HR user; `middleware.ts` redirects unauthenticated → `/login`; `login/page.tsx` "Sign in as HR (dev)".
- **GOTCHA**: dev-only; document real Azure AD as later. API calls need no bearer in dev (API mock JWT).
- **VALIDATE**: visiting `/applications` unauthenticated → `/login`; sign-in → dashboard.

### Task 5: App shell + layout
- **ACTION**: `AppHeader` (semantic `<header><nav aria-label>`), nav (Inbox/Candidates/Analytics), user menu (`/users/me`); responsive layout; skip-link; focus-visible styles.
- **MIRROR**: semantic-HTML-first; a11y rules.
- **VALIDATE**: keyboard nav reaches all controls; header responsive at 320–1440.

### Task 6: Ranked Inbox
- **ACTION**: `applications/page.tsx` + `InboxTable`, `Filters` (status/min_score/store/source), `ScoreBadge` (accent ramp), `Pagination` (Meta). Filters/page in URL search params; `useApplications`. Row → detail; row checkbox → selection state.
- **MIRROR**: URL-as-state; container/presentational; intentional hover/focus/active.
- **GOTCHA**: empty state; loading skeletons; debounce score input; clamp page.
- **VALIDATE**: e2e: filter status=scored → only scored; pagination changes page; screenshot all breakpoints.

### Task 7: Bulk action bar
- **ACTION**: `BulkActionBar` appears when ≥1 selected; actions approve(→shortlisted)/reject/shortlist → `useBulk` (POST /applications/bulk) → toast + invalidate inbox.
- **GOTCHA**: cap selection messaging; optimistic update + rollback on error; clear selection after.
- **VALIDATE**: e2e: select 2 → reject → rows update; toast shown.

### Task 8: Application detail (resume + AI panel + actions)
- **ACTION**: `[id]/page.tsx` two-pane: `ResumeViewer` (fetch `/resume` signed URL → `<iframe>`), `AiSummaryPanel` (score + breakdown bars + Thai strengths + red flags + assigned store + dedup state); status actions (dropdown: shortlist/interview/reject/**hire**) → `useSetStatus` (PATCH). Hire surfaces PS-sync result.
- **MIRROR**: native viewer decision; compositor-friendly transitions only.
- **GOTCHA**: handle no-resume (404) gracefully; signed URL expiry note; sandbox the iframe.
- **VALIDATE**: e2e: open detail → iframe has signed src; change status → persisted; screenshot.

### Task 9: Candidate list + profile/timeline
- **ACTION**: `candidates/page.tsx` (list, paginate) + `candidates/[id]/page.tsx` (profile: candidate fields, applications list, **timeline** from `/timeline`, PDPA consent read + toggle calling 4a `/pdpa/consent`).
- **VALIDATE**: e2e: profile shows applications + timeline events; consent toggle persists.

### Task 10: Analytics overview
- **ACTION**: `analytics/page.tsx` + `KpiCards` (/reports/kpi), `FunnelChart` (/reports/funnel), `SourcesChart` (/reports/sources, conversion). Also surface KPI cards on `dashboard/page.tsx`.
- **MIRROR**: data-viz as part of the design system (not an afterthought); responsive containers.
- **VALIDATE**: charts render with API data; screenshot; no layout shift (CLS).

### Task 11: E2E + visual + a11y + build
- **ACTION**: `playwright.config.ts` (webServer: next dev + assumes API+stack up); specs: login→inbox→detail→bulk→candidate→analytics happy path; visual screenshots at 320/375/768/1024/1440; an automated a11y check (axe) on key pages; reduced-motion assertion.
- **VALIDATE**: Playwright green; `next build` clean; Lighthouse/CWV sanity on the inbox.

### Task 12: Docs + env
- **ACTION**: `frontend/.env.example` (`NEXT_PUBLIC_API_URL=http://localhost:8080`); README: how to run the stack + API + `pnpm --filter frontend dev`; note mock auth + real Azure AD later.

---

## Testing Strategy (web)
### Priority order (per web testing rules)
1. **Visual regression** — Playwright screenshots at 320/768/1024/1440 for inbox, detail, analytics (light theme).
2. **Accessibility** — axe automated checks; keyboard nav; focus-visible; reduced-motion; color contrast (AA) for score badges.
3. **E2E flows** — login → ranked inbox (filter/paginate) → detail (resume + status change) → bulk → candidate profile/timeline → analytics.
4. **Unit** — `lib/api.ts` envelope unwrap + error; `ScoreBadge` ramp; filter→query-key mapping.

### E2E shape
```ts
test('inbox loads ranked and opens detail', async ({ page }) => {
  await signInAsHR(page);
  await page.goto('/applications?status=scored');
  await expect(page.getByRole('row')).toHaveCount(/* >1 */);
  await page.getByRole('row').nth(1).click();
  await expect(page.locator('iframe[title="resume"]')).toBeVisible();
});
```

### Edge Cases Checklist
- [ ] Empty inbox / no candidates
- [ ] Application with no resume (404 → friendly message)
- [ ] API error (envelope error surfaced as toast, not crash)
- [ ] Long Thai names / RTL-free wrapping; overflow at 320px
- [ ] Reduced-motion honored
- [ ] Score badge contrast AA at all ramp colors

---

## Validation Commands
### Static
```bash
cd frontend && pnpm lint && pnpm exec tsc --noEmit
```
### Build
```bash
cd frontend && pnpm build
```
### E2E + Visual + a11y (stack + API must be up)
```bash
make up && make migrate-up && make seed          # backend deps + data
# (submit a few applications so the inbox is populated — see backend README)
cd frontend && pnpm exec playwright install --with-deps chromium
pnpm exec playwright test                         # flows + screenshots at breakpoints + axe
```
EXPECT: Playwright green; screenshots captured at 320/768/1024/1440; axe finds no serious violations; `next build` clean.

### Manual
- [ ] Sign in → inbox ranked by score; filter status/min_score works.
- [ ] Open detail → resume renders (signed URL) beside AI summary; change status persists.
- [ ] Select rows → bulk reject → inbox updates.
- [ ] Candidate profile shows applications + timeline; PDPA toggle persists.
- [ ] Analytics charts reflect API data.

---

## Acceptance Criteria
- [ ] HR Dashboard app builds (`pnpm build`) and runs against the 4a API (CORS enabled).
- [ ] Ranked inbox: filter + paginate + multi-select + score badges, server state via TanStack Query, filters in URL.
- [ ] Detail: native PDF (signed URL) + AI summary/score panel; status actions incl. hire persist via the API.
- [ ] Bulk action bar updates N applications with feedback.
- [ ] Candidate profile + timeline; PDPA consent visible/toggleable.
- [ ] Analytics: funnel + KPI + sources (Recharts).
- [ ] Playwright e2e green; screenshots at all breakpoints; axe no serious issues; reduced-motion honored.
- [ ] Looks like an intentional light operations console — not a default template (design checklist passes).

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Next 15 / React 19 / Tailwind v4 scaffold churn | Med | Med | use create-next-app + shadcn defaults; pin versions; build early in Task 2 |
| Playwright browser download in sandbox | Med | Med | `playwright install chromium`; if blocked, fall back to `next build` + component/unit + manual screenshots, and note |
| CORS/auth friction in dev | Med | Low | API mock JWT needs no bearer; permissive dev CORS |
| iframe PDF cross-origin (SAS URL) | Low | Med | Azurite/Azure SAS is a direct GET; sandbox iframe; fallback download link |
| Template-look (violates design rules) | Med | High | committed token system + dense Swiss layout + score accent; design checklist in acceptance |
| Scope creep (two apps) | — | — | Career Portal explicitly deferred to 4c |

## Notes
- This is the first frontend; it establishes the token system, API client, and Query/auth patterns the Career Portal (4c) will reuse.
- The dev mock user is super_admin → the inbox shows all; role-scoped views are exercised by the 4a unit tests, and will be real with Azure AD.
- Performance budgets (web/performance.md): landing JS < 150kb gz, CWV targets; lazy-load charts; preload only critical font weight.
