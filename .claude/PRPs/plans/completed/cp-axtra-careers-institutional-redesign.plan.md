# Plan: CP Axtra Careers — Institutional-Minimal Redesign (from scratch)

## Summary
A clean-slate redesign of the public **career-portal** for CP Axtra — a ~150B THB/yr SET-listed retail
conglomerate (Makro + Lotus's). Direction: **institutional, minimal, modern, restrained big-corporate**
— the McKinsey / J.P. Morgan / HSBC / GIC / SCBX register. Near-monochrome canvas (navy ink on white)
with **CP Axtra blue `#0B47B8` as the single semantic accent**; one neutral type superfamily; strict
grid + generous whitespace; near-zero motion. **All prior "AXTRA Dots / Editorial Bold" work is scrapped
and NOT referenced** — this branches fresh off `main`.

## User Story
As a **prospective CP Axtra employee (Thai job-seeker)**, I want **a calm, premium, trustworthy careers
site that reflects a major listed company**, so that **I take the employer seriously and apply with
confidence** — and as **CP Axtra**, the careers site protects and elevates the corporate image.

## Problem → Solution
- **Now (on `main`):** a generic, shadcn-template-flavoured career portal (centered hero, uniform card
  grids, weak hierarchy, blue+yellow used decoratively). Reads like a template, not a 150B-THB listed
  conglomerate. (The separate AXTRA-Dots branch over-corrected into flashy/decorative — explicitly
  rejected.)
- **Then:** an institutional-minimal employer brand — quiet oversized headlines in a neutral
  superfamily, real-people photography slots, a single disciplined blue accent on near-monochrome,
  plain-number proof of scale, and an embedded, filterable, on-brand job list. Confidence through
  subtraction.

## Metadata
- **Complexity**: **XL** (full presentation rebuild of one app; data layer reused)
- **Source PRD**: N/A (free-form via `/prp-plan`; direction confirmed with user)
- **PRD Phase**: N/A
- **Estimated Files**: ~18–24 in `career-portal/` (globals + layout + new design-system primitives +
  every page/section component); **0 backend, 0 data-layer changes**
- **Confirmed decisions (do not relitigate)**:
  - Scope = **career-portal ONLY** (admin untouched).
  - Palette = **CP Axtra blue `#0B47B8` as the lone institutional accent** on navy-ink/white neutrals;
    **yellow + the dot motif are retired** from this surface; no gradients/particles/halftone/flourish.
  - Start **fresh from `main`** — do not inherit or reference the AXTRA-Dots branch.

## The new design system — "CP Axtra Institutional"
Define in `career-portal/app/globals.css` (oklch tokens), replacing the current token set:
- **Surfaces**: `--surface` white `oklch(100% 0 0)`, `--surface-muted` near-white `oklch(98.5% 0.003 250)`,
  `--line` hairline `oklch(91% 0.006 255)`.
- **Ink**: `--ink` navy-near-black `oklch(22% 0.03 264)`, `--ink-muted` `oklch(46% 0.02 260)`.
- **Accent (the only brand color)**: `--accent` CP Axtra blue `oklch(46% 0.18 264)` (#0B47B8),
  `--accent-ink` on-accent white, `--accent-soft` `oklch(96% 0.03 258)` for the rare tint (active/hover).
  **No `--gold`, no `--dot-*`.**
- **Type scale (discipline, not personality)**: `--text-display` `clamp(2.4rem,1.4rem+3.6vw,4.25rem)`,
  `--text-h2` `clamp(1.6rem,1.1rem+1.8vw,2.5rem)`, `--text-lead`, `--text-body`, `--text-caption`.
- **Rhythm**: `--space-section` `clamp(4.5rem,3rem+6vw,9rem)`, container `max-width: 1200px`, 12-col grid.
- **Motion**: `--ease-out` cubic-bezier(0.16,1,0.3,1), `--dur` 400ms; ONE `.reveal` (opacity+translateY)
  gated by `prefers-reduced-motion`. Nothing else animates.
- **Typography**: **Anuphan** (display/headings, loopless, modern) + **IBM Plex Sans Thai Looped**
  (body/UI, looped for Thai reading comfort) via `next/font/google` — both Thai+Latin, Cadson-Demak/Plex
  lineage, equal Thai/Latin weight, `display: swap`. Map `--font-heading`/`--font-sans`.

## Pages to rebuild (career-portal, full surface)
| Route | Institutional-minimal treatment |
|---|---|
| `/` | Quiet oversized headline + one primary CTA + a real-photo hero slot; a plain-number **scale band** (revenue/stores/people/awards as large quiet figures); modular alternating image+text "why join / growth / care / ESG" sections; an embedded **featured roles** strip; a restrained closing CTA. No decoration. |
| `/jobs` | **Embedded, on-brand job browse** (the biggest modern win): card list + **left-rail filters** + **live result count** + **removable filter tags**, filters mirrored to URL search-params. Generous whitespace, flat cards. |
| `/jobs/[id]` | Generous single-column role detail; clear hierarchy; supportive content; **sticky Apply CTA** (mobile + desktop). |
| `/jobs/[id]/apply` | Calm, low-friction multi-step apply; quiet progress; reassurance. Keep the flow working. |
| `/signup`, `/login` | Minimal institutional auth — one neutral panel, restrained; LINE/Google/email-OTP presented plainly. |
| `/status` | Calm status read; clear states. |
| `/account` | Clean profile/resume/linked-accounts; institutional. |
| `/interview` | Quiet, focused conversational screening UI. |
| Header / Footer | Slim institutional chrome: a real **CP Axtra wordmark** (text-based, no dot mark), TH/EN affordance, restrained nav, generous footer with scale/credentials. |

---

## UX Design

### Before (on `main`)
```
┌──────────────────────────────────────────┐
│ [N] ร่วมงานกับเรา        nav   nav   ○     │
│                                            │
│   ร่วมเติบโต            ┌──────────┐        │
│   ไปกับเรา              │ skeleton │  ← centered, generic
│   [ดูตำแหน่งงาน][สถานะ]  │  card    │        │
│                        └──────────┘        │
│  ▢ value  ▢ value  ▢ value  ▢ value  ← uniform grid
└──────────────────────────────────────────┘
```

### After (institutional-minimal)
```
┌──────────────────────────────────────────┐
│ CP AXTRA  Careers      งาน  ชีวิต  EN ▾    │  ← slim chrome
│                                            │
│  เติบโตไปกับองค์กร          ┌─────────────┐ │
│  ค้าปลีกชั้นนำของไทย         │ real staff  │ │  ← quiet big headline
│  [ ดูตำแหน่งงาน → ]          │  photo      │ │     + ONE blue CTA
│                            └─────────────┘ │
│  ──────────────────────────────────────   │
│  150,000        2,600+        50,000+      │  ← plain-number scale band
│  ล้านบาท/ปี      สาขา          พนักงาน        │
└──────────────────────────────────────────┘
JOBS:  ┌ filters ┐ ┌─ 48 ตำแหน่ง ──────────┐
       │ ค้นหา    │ │ [✕ ระดับเริ่มต้น]        │  ← live count + removable tags
       │ ระดับ ☑  │ │ ▢ role card            │
       │ ...      │ │ ▢ role card            │
       └─────────┘ └────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Hero | centered headline + decorative panel | quiet big headline + 1 CTA + real-photo slot | confidence via restraint |
| Color | blue + yellow decorative | **blue accent only**, semantic | yellow/dots retired |
| Jobs | uniform card grid, link-out | embedded list + left-rail filters + live count + URL tags | the SCB pattern; biggest modern win |
| Type | Noto Sans Thai + Inter (main) | Anuphan + IBM Plex Sans Thai Looped | neutral institutional superfamily |
| Motion | reveal flourishes | single subtle `.reveal`, reduced-motion | near-zero |
| Proof | none | plain-number scale band + quiet credentials (SET/ESG/HR-Asia) | institutional trust |

---

## Mandatory Reading (all on `main` — the clean baseline to rebuild)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `career-portal/app/globals.css` | all | Token set to REPLACE; keep the `@theme inline` mapping mechanics + reduced-motion block |
| P0 | `career-portal/app/layout.tsx` | 1–49 | Font wiring to replace (Noto+Inter → Anuphan + IBM Plex Sans Thai Looped); metadata/PWA/viewport keep |
| P0 | `career-portal/lib/queries.ts` | 1–99 | **Reuse unchanged** — `usePublicPositions`, `usePublicPosition`, apply/status/interview hooks |
| P0 | `career-portal/lib/types.ts` | 1–105 | **Reuse unchanged** — `PublicPosition {id,title_th,title_en,level,open_count}`, `PositionDetail {id,title_th,title_en,level}`, ApplyInput, Account, etc. (defines what filters/fields are actually available) |
| P0 | `career-portal/lib/api.ts` | 1–75 | **Reuse unchanged** — envelope client, `credentials:'include'`, BASE from `NEXT_PUBLIC_API_URL` |
| P1 | `career-portal/app/page.tsx`, `components/landing/{Hero,LandingSections,FeaturedJobs}.tsx` | all | Landing structure to replace (server page + client `FeaturedJobs` island) |
| P1 | `career-portal/components/JobCard.tsx`, `app/jobs/page.tsx`, `app/jobs/[id]/page.tsx` | all | Jobs funnel to rebuild |
| P1 | `career-portal/components/{SiteHeader,SiteFooter,Container,PortalShell,AccountNav}.tsx` | all | Chrome + shells to rebuild institutionally |
| P1 | `career-portal/components/ui/{button,card,input,label,checkbox,skeleton}.tsx` | all | Base UI primitives — **reuse/retune**, do not rewrite the primitive API |
| P1 | `career-portal/app/{signup,login,status,account,interview}/page.tsx` + `components/{ApplyStepper,ConsentStep,ResumeUploadStep,InterviewChat,StatusCard,auth/*}.tsx` | all | Secondary surfaces to rebuild minimally |
| P1 | `career-portal/next.config.ts` | all | **GOTCHA**: dev CSP lacks `'unsafe-eval'` → `next dev --webpack` renders but never hydrates. Re-apply the dev-only CSP relax (see Task 0.2) |
| P2 | `career-portal/package.json` | all | Stack: Next 16, React 19, Tailwind 4, Base UI `@base-ui/react`, TanStack Query, lucide-react, serwist PWA; dev port 3001 (`--webpack`) |
| P2 | `career-portal/e2e/*.spec.ts` | all | Keep selectors/accessible names sane so e2e (apply-form/login/signup/interview/pwa/portal) still pass |

## External Documentation
No live-doc lookup needed at build time — the direction + references + fonts are researched and locked
below. `next/font/google` exposes `Anuphan` and `IBM_Plex_Sans_Thai_Looped`.

### Research basis (institutional-minimal references — what to emulate)
| Topic | Source | Takeaway |
|---|---|---|
| Restraint + one-accent | HSBC, J.P. Morgan, GIC, BlackRock careers | single brand accent reserved for CTAs/links/active; near-monochrome canvas |
| Quiet authority | McKinsey careers | scale-discipline typography, vast margins, almost no color |
| On-brand job list | SCB careers (`careers.scb.co.th/en/jobs`) | card list + left-rail filters + live count + removable tags — the modern pattern |
| Real-people premium lever | CP Group, GIC, SCBX, Siemens | candid named-employee photography beats abstract graphics/stock |
| Thai institutional type | IBM Plex Sans Thai Looped + Anuphan (Cadson Demak) | neutral, corporate, Thai+Latin parity; looped body / loopless display |
| Avoid | Sarabun (gov-doc), Prompt/Kanit (campaign), dot motifs, multi-color chrome | the "dated Thai portal" tells |

---

## Patterns to Mirror (from the codebase — keep these mechanics)

### DATA_HOOK (reuse verbatim — do NOT change the network layer)
```tsx
// SOURCE: career-portal/lib/queries.ts:16-29
export function usePublicPositions() {
  return useQuery({ queryKey: ["public-positions"],
    queryFn: () => api.get<PublicPosition[]>("/api/v1/public/positions").then((r) => r.data) });
}
export function usePublicPosition(id: string) {
  return useQuery({ queryKey: ["public-position", id],
    queryFn: () => api.get<PositionDetail>(`/api/v1/public/positions/${id}`).then((r) => r.data), enabled: !!id });
}
```
**Data reality (drives filter scope):** a position exposes only `title_th`, `title_en`, `level`,
`open_count`. So the jobs filters that are honestly buildable today = **free-text search (title)** +
**level facet** (entry/experienced/senior/management) + open-count sort. Store-brand / province /
function facets are NOT in the public API → out of scope here, listed as a backend dependency.

### BASE_UI_USAGE (both reused + new components — never asChild)
```tsx
// SOURCE: career-portal/components/ui/button.tsx + Base UI docs
<Dialog.Trigger render={<Button variant="outline">…</Button>} />   // render=, NOT asChild
```

### URL_STATE (new — filters as shareable search-params)
```tsx
// Mirror Next App Router pattern: useSearchParams + router.replace(`/jobs?${params}`)
// filters: q (search), level (repeatable). Parse on load, write on change. Shareable + back-button safe.
```

### TOKEN_THEME (replace values, keep the @theme inline mechanism)
```css
/* SOURCE: career-portal/app/globals.css (current @theme inline + :root) — keep the wiring, swap the
   palette to ink/white + single --accent; delete --gold and --dot-* and the dot utility classes. */
```

### FONT_WIRING (replace the families)
```tsx
// SOURCE: career-portal/app/layout.tsx:8-14 (pattern) — swap Noto+Inter for:
const display = Anuphan({ variable: "--font-display", subsets: ["thai","latin"], weight:["400","500","600","700"], display:"swap" });
const body = IBM_Plex_Sans_Thai_Looped({ variable: "--font-body", subsets:["thai","latin"], weight:["300","400","500","600"], display:"swap" });
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `career-portal/app/globals.css` | REPLACE | New institutional token set; delete dot/gold tokens + dot utility classes |
| `career-portal/app/layout.tsx` | UPDATE | Fonts → Anuphan + IBM Plex Sans Thai Looped; keep metadata/PWA/viewport |
| `career-portal/next.config.ts` | UPDATE | Re-apply dev-only CSP `'unsafe-eval'` + localhost connect-src (Task 0.2) |
| `career-portal/components/ds/*` | CREATE | New design-system primitives: `SectionHeading`, `StatBand`, `MediaBlock` (image+text), `Eyebrow` (plain), `Container` (retune) |
| `career-portal/components/SiteHeader.tsx`, `SiteFooter.tsx` | REPLACE | Slim institutional chrome + text wordmark |
| `career-portal/app/page.tsx`, `components/landing/*` | REPLACE | Institutional landing (hero, scale band, media sections, featured roles, CTA) |
| `career-portal/components/JobCard.tsx` | REPLACE | Flat institutional role card |
| `career-portal/app/jobs/page.tsx` | REPLACE | Embedded list + left-rail filters + live count + removable tags + URL params |
| `career-portal/app/jobs/[id]/page.tsx` | REPLACE | Single-column role detail + sticky apply |
| `career-portal/app/jobs/[id]/apply/page.tsx` + `components/ApplyStepper.tsx` etc. | UPDATE | Calm minimal apply; keep flow/hooks |
| `career-portal/app/{login,signup,status,account,interview}/page.tsx` + related components | UPDATE | Minimal institutional rebuild; keep auth/data hooks |
| `career-portal/components/ui/{button,input,card,...}.tsx` | UPDATE | Retune tokens/radius/shadow to institutional (flat); keep the primitive API |

## NOT Building
- ❌ ANY AXTRA-Dots artifact — no DotField, StaticDotPoster, BrandMark dot-lattice, EditorialShell,
  FormPanel, halftone/`.dot-*`/`.dot-ink` classes, gold/cream/sand tokens. (Scrapped; not referenced.)
- ❌ Admin/`frontend/` changes (separate scope, untouched).
- ❌ Backend / API / data-layer / migration changes. Filters limited to fields the public API exposes.
- ❌ New job facets (store brand, province, function) — require backend fields; documented as dependency.
- ❌ Real licensed photography sourcing — the build ships tasteful **image SLOTS** + placeholder
  treatment; supplying real Makro/Lotus's staff photos is a CONTENT dependency for the client.
- ❌ Dark mode (careers is light-only, institutional).
- ❌ Decorative motion, gradients, particle/canvas effects.

---

## Step-by-Step Tasks

### Phase 0 — Foundation (branch, infra, design system)
**Task 0.1: Fresh branch off main**
- ACTION: from `main`, `git checkout -b feat/career-portal-institutional`. Do NOT branch off the AXTRA branch.
- VALIDATE: `git log --oneline -1` shows `main` tip; `career-portal/components/dots/` does not exist.

**Task 0.2: Re-apply dev-CSP fix (infra gotcha)**
- ACTION: in `career-portal/next.config.ts`, gate `script-src` to add `'unsafe-eval'` and widen
  `connect-src` to `http://localhost:* ws://localhost:*` **only when `NODE_ENV==='development'`**; prod stays strict.
- GOTCHA: without it, `next dev --webpack` pages render but never hydrate (jobs list stuck on skeletons).
- VALIDATE: after Task 0.4, `/jobs` hydrates and fetches (no CSP `unsafe-eval` console error).

**Task 0.3: Fonts + tokens**
- ACTION: `layout.tsx` → Anuphan (`--font-display`) + IBM Plex Sans Thai Looped (`--font-body`).
  `globals.css` → institutional token set (ink/white/`--accent` only); delete dot/gold tokens + dot
  utility classes; add `.reveal` (reduced-motion-gated) + base h1–h4 heading rule (Thai-safe line-height).
- MIRROR: FONT_WIRING, TOKEN_THEME.
- VALIDATE: `pnpm exec tsc --noEmit`; a probe page shows new type + no leftover `--dot`/`--gold` refs
  (`grep -rE "dot-|--gold|halftone|DotField" career-portal/app career-portal/components` → none).

**Task 0.4: Local dev + mock for QA**
- ACTION: run `career-portal` dev on a free port (3001/8080 are taken by unrelated `ananta-*` docker →
  use 3030) with a tiny zero-dep mock API (CP Axtra positions, CORS+envelope) on :8090, pointed via
  `NEXT_PUBLIC_API_URL`. (Recreate the simple mock; it is design-agnostic infra.)
- VALIDATE: `/jobs` lists positions; `curl :3030/ → 200`.

**Task 0.5: Design-system primitives**
- ACTION: build `components/ds/{Container, SectionHeading, Eyebrow, StatBand, MediaBlock}.tsx` + retune
  `components/ui/*` to flat institutional (squared-ish radius, hairline borders, no decorative shadow).
- MIRROR: BASE_UI_USAGE.
- VALIDATE: tsc clean; primitives render in isolation.

### Phase 1 — Chrome + Landing (the corporate-image surface)
**Task 1.1: SiteHeader + SiteFooter** — slim institutional chrome, text wordmark "CP AXTRA · Careers",
restrained nav, generous footer with scale/credentials. VALIDATE: renders 1440/375, sticky, a11y nav.

**Task 1.2: Landing** — `app/page.tsx` + `components/landing/*`: quiet oversized hero + ONE blue CTA +
real-photo hero slot; **StatBand** (plain numbers); 3–4 **MediaBlock** sections (why-join / growth /
care / ESG) alternating image+text; embedded **FeaturedRoles** (live `usePublicPositions().slice`);
restrained closing CTA. MIRROR: DATA_HOOK. VALIDATE: looks like a listed-company employer brand at
1440/768/375; no decoration; reduced-motion safe.

### Phase 2 — Jobs funnel (the biggest modern win)
**Task 2.1: `/jobs` embedded browse** — left-rail filters (search `q` + `level` facet) + live count +
removable filter tags; filters ↔ URL search-params; flat role cards. MIRROR: DATA_HOOK, URL_STATE.
GOTCHA: only `level` + title are filterable (data reality); don't invent facets. VALIDATE: filter →
count + tags + URL update; back button restores; empty/loading/error states styled.

**Task 2.2: `/jobs/[id]`** — single-column role detail, strong hierarchy, supportive content, sticky
Apply (mobile+desktop). MIRROR: DATA_HOOK. VALIDATE: renders for a real id; apply CTA reachable.

### Phase 3 — Secondary surfaces (minimal institutional)
**Task 3.1: Apply funnel** (`/jobs/[id]/apply` + ApplyStepper/Consent/ResumeUpload) — calm multi-step,
quiet progress; keep hooks/flow. VALIDATE: flow renders; tsc clean.
**Task 3.2: Auth** (`/login`, `/signup` + `components/auth/*`) — one neutral panel, plain method buttons.
**Task 3.3: `/status`, `/account`, `/interview`** — calm reads/forms/chat; keep data hooks.
VALIDATE per page: renders light, no overflow at 375, accessible.

### Phase 4 — QA + sign-off
**Task 4.1**: `tsc --noEmit` + `eslint` clean; `next build --webpack` green (all routes).
**Task 4.2**: Screenshot all 9 pages at 1440/768/375; confirm institutional-minimal (no decoration, one
accent, real-photo slots, embedded filterable jobs). Optionally a light GAN-style eval (target restrained-
premium, NOT flashy). **Task 4.3**: run `pnpm exec playwright test` (needs mock/backend) — keep green.

---

## Testing Strategy
### Unit / Type
| Test | Expected |
|---|---|
| `pnpm exec tsc --noEmit` | 0 errors |
| Filter param parse/serialize (`/jobs`) | URL ↔ state round-trips; repeatable `level` handled |
| Thai heading metrics (Anuphan display) | no clip; equal Thai/Latin weight |

### Edge Cases Checklist
- [ ] Empty jobs / filtered-to-zero (styled empty state)
- [ ] Loading + error states (skeleton/retry)
- [ ] 375px no horizontal overflow; tap targets ≥44px
- [ ] reduced-motion (only `.reveal`, neutralized)
- [ ] Missing photography → placeholder treatment holds
- [ ] TH/EN content parity; long Thai titles wrap

### Validation Commands
```bash
# types + lint
cd /Users/nex/Documents/SourceCode/ats/career-portal && pnpm exec tsc --noEmit && pnpm exec eslint .
# production build (catches SSR/font/CSP issues dev hides)
cd /Users/nex/Documents/SourceCode/ats/career-portal && pnpm exec next build --webpack
# dev (ports 3001/8080 occupied by ananta docker → use 3030 + mock :8090)
cd /Users/nex/Documents/SourceCode/ats/career-portal && NEXT_PUBLIC_API_URL=http://localhost:8090 pnpm exec next dev -p 3030 --webpack
# e2e (needs data)
cd /Users/nex/Documents/SourceCode/ats/career-portal && pnpm exec playwright test
# guard: no AXTRA remnants
grep -rE "dot-ink|DotField|StaticDotPoster|EditorialShell|--gold|halftone|--dot-" career-portal/app career-portal/components || echo "clean ✓"
```
EXPECT: tsc 0 errors; build green; `/jobs` hydrates + filters; AXTRA-remnant grep returns clean.

### Manual Validation
- [ ] Reads as a serious listed-company employer brand (McKinsey/JPM/HSBC/GIC/SCBX register)
- [ ] One blue accent only; no yellow/dots/gradients/particles
- [ ] Embedded filterable jobs with live count + removable tags + shareable URL
- [ ] Real-photo slots present (placeholder until assets supplied)
- [ ] Flawless mobile; near-zero motion

## Acceptance Criteria
- [ ] Fresh branch off `main`; zero AXTRA-Dots code/classes/tokens remain (grep clean)
- [ ] Institutional token set + Anuphan/IBM Plex Sans Thai Looped wired
- [ ] All 9 surfaces rebuilt minimal-institutional; data/auth flows intact
- [ ] `/jobs` embedded filters (search + level) with live count + URL tags
- [ ] tsc + eslint clean; `next build` green; e2e green
- [ ] Single blue accent discipline; real-photo slots; no decoration

## Completion Checklist
- [ ] Base UI `render=` (no asChild); tokens-only (no hardcoded colors)
- [ ] No data/auth/query/backend changes
- [ ] Filters honor data reality (search + level only); facet gaps documented
- [ ] reduced-motion honored; tap ≥44px; semantic landmarks + focus-visible
- [ ] Files <800 lines; organized by surface
- [ ] dev-CSP fix re-applied; PWA/manifest/offline intact
- [ ] Self-contained — no further searching needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| No real photography → looks empty (photography is the #1 premium lever) | High | High | Design elegant image SLOTS + a restrained placeholder (mono blue tint + caption); flag as client content dependency; lean on typography+whitespace+numbers so it holds without photos |
| Filters over-promise vs data (only level/title available) | High | Med | Scope to search + level; document store/province/function as backend dependency; don't render dead facets |
| "Minimal" drifts into "empty/boring" | Med | High | Whitespace + scale-contrast + strict grid + plain-number proof carry it (Swiss discipline), per research; verify against named refs in QA |
| Dev hydration blocked by CSP again | Med | High | Re-apply dev-CSP fix in Task 0.2 before any dev QA |
| e2e selectors break on rebuilt markup | Med | Med | Keep accessible names/roles; run playwright after each phase |
| Scope creep into admin / backend | Med | Med | NOT Building list; career-portal presentation only |
| Local backend absent (8080 taken) | Low | Low | Mock API on :8090 (design-agnostic infra), dev on :3030 |

## Notes
- **Clean slate**: branch off `main`; the AXTRA-Dots branches (`feat/career-portal-axtra-redesign`,
  `feat/admin-axtra-parity`) are abandoned for the careers surface per the user ("ลบของเดิม คิดใหม่หมด").
  They can be deleted later; not referenced here.
- **The one big lever** is real-employee photography (CP Group/GIC/SCBX move). The build is structured to
  showcase it; supplying assets is the highest-impact follow-up.
- **The one big modern win** is the embedded, filterable, on-brand job list (the SCB pattern) — most
  Thai/SEA peers punt to a raw ATS; owning it is the differentiator.
- Reuse, untouched: `lib/{queries,api,types,session,auth,line,utils}.ts`, Base UI primitive APIs, serwist
  PWA, manifest/offline.

> Next step: `/prp-implement .claude/PRPs/plans/cp-axtra-careers-institutional-redesign.plan.md`
