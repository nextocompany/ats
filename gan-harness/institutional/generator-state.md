# Generator State — CP Axtra Careers, Institutional-Minimal Rebuild

## Design system built ("CP Axtra Institutional")
- **Foundation (pre-existing, built ON, not changed):** `app/globals.css` institutional token set
  (white canvas, navy ink, ONE accent = CP Axtra blue `#0B47B8`, hairline `--line`, `--accent-soft`,
  scale-discipline type ramp `--text-display/h2/h3/lead/stat`, `--space-section`, `--container` 1200px,
  the single `.reveal` motion + reduced-motion guard, `.num` tabular numerals). Fonts in `app/layout.tsx`:
  **Anuphan** (display) + **IBM Plex Sans Thai Looped** (body). dev-CSP fix already in `next.config.ts`.
- **New DS primitives** (`components/ds/`): `Container` (max-width var + generous gutters, `narrow` form
  column, `as` element), `Eyebrow` (plain uppercase tracked label, muted/accent/invert tones — no dots),
  `SectionHeading` (eyebrow + Anuphan heading + lead, h1/h2, invert tone), `StatBand` (row of large `.num`
  figures, hairline-divided, invert tone), `ImageSlot` (elegant photo placeholder: aspect box + hairline +
  centered Thai caption + quiet image glyph — looks intentional, swap for `next/image` when assets land),
  `MediaBlock` (alternating image-slot + text editorial section). Barrel `components/ds/index.ts`.
- **`components/ui/*` retuned to flat institutional:** button (solid-blue primary, hairline outline/ghost,
  no lift/decorative shadow, focus ring + offset, taller h-10/h-11/h-12 sizes, ≥44px `tap`), card (hairline
  border instead of ring/shadow, surface-muted footer), input/checkbox (hairline, calm blue 2px focus ring,
  no dark-mode noise), skeleton (surface-muted). Base UI primitive APIs kept (`render=`, never `asChild`).
- **Shared helpers:** `lib/levels.ts` (level taxonomy + Thai labels — single source of truth),
  `lib/useJobFilters.ts` (URL-mirrored filter state hook).

## Each surface
- **Chrome** — `Wordmark.tsx` (text "CP AXTRA · Careers", hairline divider, invert variant, no dot mark).
  `SiteHeader` slim sticky hairline + restrained nav + `AccountNav` (quiet ink avatar). `SiteFooter`
  generous: wordmark + positioning line + quiet credentials ("จดทะเบียนใน SET · HR Asia Best Companies to
  Work for") + two nav columns + PDPA row. `AccountNav` re-styled neutral.
- **Landing** (`app/page.tsx` + `components/landing/*`): quiet oversized Anuphan hero headline
  ("เติบโตไปกับองค์กรค้าปลีกชั้นนำของไทย") + ONE blue CTA + plain status link + hero `ImageSlot`; `StatBand`
  (150,000 ล้านบาท/ปี · 2,600+ สาขา · 50,000+ พนักงาน · SET — illustrative, noted as such); 4 alternating
  `MediaBlock` sections (เส้นทางอาชีพ / สวัสดิการ / วัฒนธรรม / ESG); live `FeaturedJobs` strip (top 6,
  loading/empty states); closing CTA band on solid navy ink (single CTA). No gradients/dots/yellow.
- **Jobs** (`app/jobs/page.tsx` + `components/jobs/*`): the modern win. Left-rail `JobFilters` (search bound
  to `q` + level checkboxes with per-level counts) + main column with live "พบ N ตำแหน่ง" count, removable
  `FilterTags` chips, flat `JobCard` grid. Filters mirrored to URL search-params via `useJobFilters`
  (shareable + back-button safe). Styled loading/error/empty (filtered-to-zero) states. Rail is sticky on
  desktop, stacks above results on mobile. Honors data reality: search + level only (no invented facets).
- **Role detail** (`app/jobs/[id]/page.tsx`): generous single-column, quiet hero (level eyebrow + Anuphan
  title + EN), hairline-divided "สิ่งที่เรามอบให้" + numbered apply steps, sticky Apply card (mobile+desktop).
- **Apply** (`apply/page.tsx` + `ApplyStepper`/`ConsentStep`/`ResumeUploadStep`): calm card panel, review →
  edit modes, quick-apply, PDPA consent (accent-soft selected state), success token + copy link. Flows/hooks
  untouched.
- **Auth** (`login`/`signup` + `components/auth/*`): neutral institutional header + one bordered card panel;
  plain LINE/Google/email method buttons (provider brand colors kept intentionally on the LINE/Google CTAs).
- **Status / Account / Interview**: institutional headers (eyebrow + h2), hairline cards; `StatusCard` tone
  map re-mapped to secondary/primary/accent-soft; `InterviewChat` flat card + blue user bubbles; account
  sections wrapped in bordered panels.

## Data / network layer
- Untouched: `lib/{queries,api,types,session,auth,line,utils}.ts`. All apply/status/interview/auth flows
  preserved. `useJobFilters` + `lib/levels` are new client-only presentation helpers (no network).

## Validation
- `pnpm exec tsc --noEmit` → 0 errors. `pnpm exec eslint app components lib` → 0 problems.
- Purge grep (dot-/--gold/--brand-soft/--text-hero/DotField/halftone/bg-gold) → clean.
- Smoke (dev :3030, mock :8090): `/ /jobs /jobs?level=… /jobs/p01 /jobs/p01/apply /login /signup /status
  /account /interview` → all 200. Hero/StatBand/ImageSlot/jobs-filters/wordmark/footer-credentials render.

## Known issues / deferred / risks
- **Photography is the #1 premium lever and is still placeholder** — `ImageSlot`s are intentional, captioned
  reserved slots; supplying real Makro/Lotus's staff photos is a CONTENT dependency. Swap `ImageSlot` inner
  for `next/image` when assets arrive.
- Scale figures in `StatBand` are illustrative (noted in the note line), not audited disclosures — confirm
  real numbers with the client before any external publish.
- Job facets limited to title-search + level (the only fields the public API exposes). Store-brand /
  province / function facets remain a backend dependency, deliberately not rendered.
- `PositionDetail` has no `open_count`, so the detail-page apply card omits the count figure (list cards keep
  it). Not a regression — the field simply isn't in the detail projection.

## Dev server
- URL: http://localhost:3030 (mock API http://localhost:8090) — running, hot-reloads. Do NOT restart.
