# Brief — HR Dashboard full redesign (UX/UI), keep CP Axtra CI

Redesign **every page** of the HR recruitment dashboard (`frontend/`) — both UX and
UI — to a more polished, modern, award-worthy product. This is a visual + interaction
overhaul, NOT a feature rewrite: keep all existing data, routes, and functionality.

## HARD CONSTRAINT — preserve CP Axtra corporate identity
The brand/CI is locked. Do NOT change the palette or brand language:
- **Brand / primary / CTA:** deep CP Axtra blue `#0B47B8` (`--brand` / `--primary`)
- **Accent / emphasis:** warm yellow `#FFC02E` (`--brass`)
- **Surfaces:** bright near-white / white; **text:** navy ink
- **Signature motif:** the multicolour **dot pattern** (`.dot-cluster`, `.dot-rule`, dot tokens)
- Keep using the design tokens in `frontend/app/globals.css` (`--brand`, `--brass`,
  `--dot-*`, sidebar tokens). You MAY refine token *values* slightly for harmony, add
  NEW tokens (spacing/elevation/typography scale), and add new component styles — but
  the blue+yellow+white+dots identity must remain unmistakably CP Axtra.

## Pages to redesign (all of `frontend/app/(app)/` + login)
1. **Overview / dashboard** (`dashboard/page.tsx`) — command center: hero metrics, "where to act".
2. **Inbox** (ranked applications) — `applications/page.tsx`.
3. **Application detail** — `applications/[id]/page.tsx` (resume viewer + AI panel).
4. **Candidates** — `candidates/page.tsx`.
5. **Candidate detail** — `candidates/[id]/page.tsx`.
6. **Search** — `search/page.tsx`.
7. **Analytics** — `analytics/page.tsx` (funnel, KPI, sources, scheduled exports).
8. **Login** — `app/(auth)/login/page.tsx` (the brand front door).
9. **Shell/chrome** — sidebar, app header, page headers, mobile bar (`components/shell/*`).

## Design goals (push for excellence)
- **Clear hierarchy** through real scale contrast; intentional spacing rhythm (not uniform padding).
- **Depth / layering** — surfaces, subtle elevation, overlap, the dot motif as atmosphere.
- **Typography with character** — a deliberate type scale + pairing; tabular figures for data.
- **Designed states** — hover/focus/active/empty/loading all feel intentional.
- **Motion that clarifies** — compositor-friendly (transform/opacity), respects reduced-motion.
- **Data-viz as part of the system** — funnel, KPIs, score badges, charts share one language.
- **Editorial / bento composition** where it elevates the page — break the uniform-card grid.
- Responsive: 320 / 768 / 1024 / 1440. Accessible: AA contrast, keyboard, focus rings.

## Constraints / guardrails
- Next.js 16 App Router, Tailwind v4, shadcn. Reuse/extend existing components where sensible.
- Keep data fetching (React Query hooks in `lib/queries.ts`) and routes unchanged.
- Do NOT touch auth flow (`lib/auth.ts`), the resume iframe (no `sandbox`), or CSP in `next.config.ts`.
- Must build clean (`pnpm build`) and keep TypeScript green.
- Light theme is the product default; keep it bright and corporate-warm.

## What "done" looks like
A dashboard that reads as a believable, premium product screenshot for a national Thai
retail group — unmistakably CP Axtra (blue/yellow/dots), strong hierarchy, refined
typography, designed interactions, and a cohesive data-viz language across all pages.
