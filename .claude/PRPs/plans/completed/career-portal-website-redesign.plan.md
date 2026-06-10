# Plan: Career Portal — Proper Website Redesign (Clean-Luxury, Responsive)

## Summary
Elevate the candidate-facing career portal from a minimal mobile-only single-column app into a proper, responsive marketing-grade website in a **clean-luxury** direction: a real landing/home page (hero, value props, why-join, featured roles, CTA, full footer), a responsive shell (desktop nav + footer, fluid 320→1920), and a visual overhaul of the existing jobs/detail/apply/status pages. All existing functionality, accessible names, test ids, and the apply/status backend contract are preserved.

## User Story
As a **Thai job-seeker (often in the LINE in-app browser, sometimes on desktop)**, I want **a polished, trustworthy company career site that looks intentional on any screen**, so that **I feel confident the employer is real and professional, and applying is effortless on my phone or laptop**.

## Problem → Solution
**Current state:** the portal is `max-w-screen-sm` (~640px) single-column everywhere (`PortalShell`), so on tablet/desktop it's a narrow sparse strip that "looks like an error / unfinished." `/` just `redirect("/jobs")` — there is no landing page, no brand story, no desktop navigation/footer. The warm-green friendly tokens are decent but the layout reads as a utility form, not a website.
**Desired state:** a responsive clean-luxury site — a landing page that sells working at the company, a job grid that scales to multi-column, a richer job-detail layout, refined apply/status flows, a real header nav + footer, all fluid across 320/375/768/1024/1440/1920 while keeping the mobile/LINE experience first-class.

## Metadata
- **Complexity**: Large
- **Source PRD**: N/A (free-form; follows the Sprint-8 demo readiness thread)
- **PRD Phase**: N/A (standalone frontend slice)
- **Estimated Files**: ~16 (5 new components/pages, ~11 updated)

---

## Design Direction: Clean Luxury (light)

Chosen direction (per ECC design-quality rules — a specific direction, not "clean minimal"). Light luxury: disciplined high-contrast palette, dramatic scale contrast, generous editorial whitespace/rhythm, hairline borders + restrained soft depth, refined micro-interactions, a single brand accent used sparingly. Assets: tasteful placeholders (gradient/atmosphere panels, geometric/illustrative blocks, abstract imagery) + sample Thai copy — no external brand photos.

### Palette (overhaul `:root` in `app/globals.css`, OKLCH)
| Token | Value | Role |
|---|---|---|
| `--background` | `oklch(98.5% 0.004 95)` | ivory near-white |
| `--foreground` | `oklch(20% 0.012 160)` | deep ink (high contrast) |
| `--primary` | `oklch(27% 0.03 160)` | near-black ink CTA (luxury monochrome) |
| `--primary-foreground` | `oklch(98% 0.01 150)` | |
| `--accent` | `oklch(46% 0.10 158)` | refined deep emerald (brand, used sparingly) |
| `--accent-foreground` | `oklch(98% 0.01 150)` | |
| `--brand-soft` | `oklch(95% 0.03 158)` | tinted surface for hero/atmosphere |
| `--gold` (new) | `oklch(72% 0.10 85)` | brass/gold hairline detail, very sparing |
| `--card` | `oklch(100% 0 0)` | crisp white surface |
| `--muted` / `--muted-foreground` | `oklch(96% 0.006 95)` / `oklch(45% 0.015 160)` | |
| `--border` | `oklch(89% 0.008 150)` | hairline |
| `--ring` | `oklch(46% 0.10 158)` | |
| `--radius` | `0.75rem` | slightly tighter than current 0.875 for a sharper luxe feel |

Add scale tokens (mirror ECC web tokens): `--text-hero: clamp(2.5rem, 1.2rem + 5.5vw, 5rem)`, `--text-display: clamp(1.9rem, 1.2rem + 3vw, 3rem)`, `--space-section: clamp(4rem, 3rem + 5vw, 9rem)`, `--container: 1200px`, `--ease-out: cubic-bezier(0.16,1,0.3,1)`.

### Type
Keep the **two existing families** (Noto Sans Thai primary/Thai, Inter Latin/numerals) — Thai-first content makes a 3rd display font risky. Achieve luxury via scale contrast (huge hero), tight heading tracking (`-0.02em`), weight contrast (600/700 headings vs 400 body), and generous line-height on body. (Optional stretch, not required: a Latin-only serif display for the hero accent word.)

### Responsive system
- Mobile (≤640): single column, current LINE-comfort, tap ≥44px.
- Tablet (768): 2-col job grid, wider hero.
- Desktop (1024/1440/1920): centered `--container` (max 1200px), desktop header nav, multi-col footer, hero with side atmosphere panel, job grid 2–3 col, job-detail 2-col (content + sticky apply card). No overflow at 320 or 1920.

---

## UX Design

### Before
```
/ ──redirect──▶ /jobs
┌───────────────[ 640px column, centered on any screen ]──────────────┐
│ N ร่วมงานกับเรา                                                       │
│ ตำแหน่งงานที่เปิดรับ                                                   │
│ [ row ] [ row ] [ row ] [ row ]                                      │
│ ตรวจสอบสถานะ                                                          │
│        ( vast empty space on desktop — looks broken )                │
└─────────────────────────────────────────────────────────────────────┘
```

### After
```
/  =  LANDING (full-width, responsive)
┌──────────────────────────────────────────────────────────────────────┐
│ [logo]   ตำแหน่งงาน · ตรวจสอบสถานะ            [ สมัครงาน ▸ ]  (desktop nav)│
├──────────────────────────────────────────────────────────────────────┤
│  HERO:  "ร่วมเติบโตไปกับเรา"  big display    │  [ atmosphere panel ]    │
│         subcopy + [ดูตำแหน่งงาน] [ตรวจสอบสถานะ]                          │
├──────────────────────────────────────────────────────────────────────┤
│  ทำไมต้องร่วมงานกับเรา  → 3–4 value cards (grid)                         │
│  ตำแหน่งงานแนะนำ        → featured job grid (live from API)              │
│  สมัครง่ายใน 3 ขั้นตอน   → steps band                                    │
│  CTA band: "พร้อมเริ่มต้นแล้วหรือยัง" [ดูตำแหน่งงานทั้งหมด]               │
├──────────────────────────────────────────────────────────────────────┤
│  FOOTER: brand · ลิงก์ · ติดต่อ · PDPA                                   │
└──────────────────────────────────────────────────────────────────────┘
/jobs, /jobs/[id], /apply, /status  =  same shell, responsive, luxe-styled
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| `/` | redirect → /jobs | full landing page | new home |
| Header | mobile bar only (logo) | responsive: logo + nav links + CTA on desktop, compact on mobile | preserve "ร่วมงานกับเรา" brand link |
| Jobs list | 1-col rows, capped 640 | responsive grid (1→2→3 col) in `--container` | keep `<ul><li><a>` structure (e2e) |
| Job detail | narrow, generic blurb | 2-col on desktop (role content + sticky apply card), section rhythm | API still only title/level — enrich with generic luxe copy |
| Apply / Status | narrow form | centered narrow form column (good for luxury) + polish | preserve all labels/ids |
| Footer | one PDPA line | multi-column luxe footer | keep PDPA line |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `career-portal/app/globals.css` | all | The `@theme inline` + `:root` token system to overhaul; `@layer base`; reduced-motion block to keep |
| P0 | `career-portal/components/PortalShell.tsx` | all | The shared shell to split into responsive `SiteHeader` + `SiteFooter` + container; preserve "ร่วมงานกับเรา" + PDPA footer line |
| P0 | `career-portal/app/jobs/page.tsx` | all | Jobs list; **keep `<ul><li><JobCard/>`** (e2e `ul li a`), heading "ตำแหน่งงานที่เปิดรับ", loading/error/empty states |
| P0 | `career-portal/components/JobCard.tsx` | all | Card to restyle for grid; stays an `<a>` (Link) |
| P0 | `career-portal/components/ApplyStepper.tsx` | all (252) | Apply wizard — **preserve** labels/ids: `id="status-token"`, "ส่งใบสมัครเรียบร้อยแล้ว", "ถัดไป", "ส่งใบสมัคร", /ชื่อ-นามสกุล/, /อัปโหลดเรซูเม่/, error text |
| P0 | `career-portal/components/ConsentStep.tsx` | all | "ความยินยอมในการใช้ข้อมูล" heading + checkbox role (e2e) |
| P0 | `career-portal/e2e/portal.spec.ts` + `pwa.spec.ts` | all | The exact accessible names/ids/structure the redesign must not break (see Preserve list) |
| P1 | `career-portal/app/jobs/[id]/page.tsx` | all | Detail layout to enrich (2-col desktop); "สมัครงาน" CTA link |
| P1 | `career-portal/app/jobs/[id]/apply/page.tsx` | all | Apply page shell wrapper |
| P1 | `career-portal/app/status/page.tsx` | all | "ตรวจสอบสถานะใบสมัคร", "รหัสติดตาม", "ตรวจสอบ", `/ไม่พบใบสมัคร/` |
| P1 | `career-portal/components/StatusCard.tsx` | all | Status result card to restyle |
| P1 | `career-portal/app/layout.tsx` | all | fonts + `viewport.themeColor` (update to new brand) |
| P1 | `career-portal/app/manifest.ts` | all | `theme_color`/`background_color` (update to new palette) + keep name |
| P1 | `career-portal/components/ui/button.tsx` | all | variants incl `size:"tap"`; reuse, do not fork |
| P2 | `career-portal/app/page.tsx` | all | the redirect to replace with the landing |
| P2 | `career-portal/app/offline/page.tsx` | all | "คุณกำลังออฟไลน์" — light luxe polish, keep heading |
| P2 | `career-portal/components/InstallPrompt.tsx`, `LineLoginButton.tsx` | all | keep behavior; restyle to match |
| P2 | `career-portal/lib/queries.ts`, `lib/types.ts` | all | `usePublicPositions()` for featured jobs; `PublicPosition`/`PositionDetail` shapes (title_th/title_en/level/open_count only) |

## External Documentation
No external research needed — established internal patterns (Next 16 App Router, Tailwind v4 `@theme`, shadcn/base-ui components, OKLCH tokens). Anti-template + responsive rules per ECC `web/design-quality.md`, `web/performance.md`, `web/coding-style.md`.

---

## Patterns to Mirror

### TOKENS (Tailwind v4 @theme + :root)
```css
/* SOURCE: career-portal/app/globals.css */
@theme inline { --color-primary: var(--primary); --radius-2xl: calc(var(--radius) * 1.8); ... }
:root { --primary: oklch(58% 0.15 150); --tap-min: 44px; --ease-out: cubic-bezier(0.16,1,0.3,1); }
@media (prefers-reduced-motion: reduce) { *,*::before,*::after { animation-duration:.01ms!important; transition-duration:.01ms!important } }
```
→ Overhaul `:root` values to the luxury palette; ADD scale tokens; KEEP the reduced-motion block + `@layer base`.

### CLIENT_PAGE + QUERY STATES
```tsx
// SOURCE: career-portal/app/jobs/page.tsx
"use client";
const { data: positions, isLoading, isError, refetch } = usePublicPositions();
// loading → <Skeleton/>, error → retry card, empty → empty card, data → <ul> of <JobCard/>
```
→ Reuse this exact state structure; restyle the markup; landing's featured-jobs reuses `usePublicPositions()` (show top N).

### TAPPABLE CARD (anchor)
```tsx
// SOURCE: career-portal/components/JobCard.tsx
<Link href={`/jobs/${position.id}`} className="group ... rounded-2xl bg-card p-4 ring-1 ring-foreground/10 hover:ring-primary/40 active:translate-y-px focus-visible:ring-3 focus-visible:ring-ring/50 ...">
```
→ Keep the focus-visible ring + active translate + group hover idiom; restyle to luxe (hairline border, refined hover, gold/emerald detail).

### BUTTON (reuse variants; do not fork)
```tsx
// SOURCE: career-portal/components/ui/button.tsx — size "tap" = h-12 ≥44px
<Link className={cn(buttonVariants({ size: "tap" }), "w-full")}>สมัครงาน</Link>
```

### SHELL → split
```tsx
// SOURCE: career-portal/components/PortalShell.tsx (sticky header + max-w-screen-sm main + PDPA footer)
```
→ Replace with `SiteHeader` (responsive nav) + a responsive `Container` + `SiteFooter`. Form pages (apply/status) opt into a narrower inner column; landing/jobs use full `--container`.

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `career-portal/app/globals.css` | UPDATE | luxury palette + scale tokens (keep reduced-motion/@layer base) |
| `career-portal/components/SiteHeader.tsx` | CREATE | responsive header nav (logo + links + CTA), replaces PortalShell header |
| `career-portal/components/SiteFooter.tsx` | CREATE | multi-column luxe footer (keep PDPA line) |
| `career-portal/components/Container.tsx` | CREATE | responsive width wrapper (`--container`, padding); `narrow` prop for forms |
| `career-portal/components/PortalShell.tsx` | UPDATE | recompose around SiteHeader/Container/SiteFooter; `backHref` + `narrow` props; preserve brand+PDPA strings |
| `career-portal/components/landing/Hero.tsx` | CREATE | hero (display heading, subcopy, dual CTA, atmosphere panel) |
| `career-portal/components/landing/LandingSections.tsx` | CREATE | value props, featured jobs (live), 3-step band, CTA band |
| `career-portal/app/page.tsx` | UPDATE | replace redirect with the landing page (SiteHeader + Hero + LandingSections + SiteFooter) |
| `career-portal/app/jobs/page.tsx` | UPDATE | responsive grid + section header; keep `<ul><li>` + states + heading |
| `career-portal/components/JobCard.tsx` | UPDATE | luxe card for grid |
| `career-portal/app/jobs/[id]/page.tsx` | UPDATE | 2-col desktop detail (content + sticky apply card), section rhythm |
| `career-portal/app/jobs/[id]/apply/page.tsx` | UPDATE | narrow centered column wrapper, luxe polish |
| `career-portal/components/ApplyStepper.tsx` | UPDATE | restyle ONLY; preserve all labels/ids/structure |
| `career-portal/components/ConsentStep.tsx` | UPDATE | restyle; keep heading + checkbox |
| `career-portal/components/StatusCard.tsx` | UPDATE | restyle |
| `career-portal/app/status/page.tsx` | UPDATE | narrow column + luxe polish; keep strings |
| `career-portal/app/offline/page.tsx` | UPDATE | luxe polish; keep "คุณกำลังออฟไลน์" |
| `career-portal/components/InstallPrompt.tsx`, `LineLoginButton.tsx` | UPDATE | restyle to match |
| `career-portal/app/layout.tsx` | UPDATE | `viewport.themeColor` → new brand color |
| `career-portal/app/manifest.ts` | UPDATE | `theme_color`/`background_color` → new palette (keep name/start_url) |
| `career-portal/e2e/pwa.spec.ts` | UPDATE | update the `theme_color` assertion to the new value (only this assertion) |

## NOT Building

- **Backend / API changes** — the public API returns only `title_th/title_en/level/open_count`; no new job-description fields. Detail page enriches with generic luxe copy, not new API data.
- **New routes beyond the landing** (no about/benefits/FAQ pages) — that's the "full marketing site" option, explicitly out of this slice.
- **Real brand photography** — placeholders/gradients/illustration + sample Thai copy only.
- **Dark mode** — light luxury only (the `dark` variant scaffolding stays but isn't designed).
- **A 3rd font / serif display** — keep Noto Sans Thai + Inter (Thai-first).
- **Changing apply/status logic, multipart contract, or `lib/queries`/`lib/api`** — visual/layout only.
- **The CSP dev-eval fix** — separate slice; this redesign is validated via prod build (as the demo/CI already do).

---

## Step-by-Step Tasks

### Task 1: Luxury design tokens
- **ACTION**: Overhaul `:root` in `app/globals.css` to the luxury palette; add scale tokens; keep `@theme inline` wiring (add `--color-gold`, radius unchanged keys), `@layer base`, and reduced-motion.
- **IMPLEMENT**: the palette + scale tokens from the Design Direction table; add `@theme inline { --color-gold: var(--gold); }`; ensure `--font-heading` maps to the sans.
- **MIRROR**: TOKENS.
- **GOTCHA**: Tailwind v4 reads `@theme inline` → utility classes; changing `:root` values is enough for existing `bg-primary`/`text-foreground` usages to re-skin. Don't remove token keys other components rely on (`--accent`, `--secondary`, `--destructive`, `--card`, `--brand-soft`).
- **VALIDATE**: `pnpm build` compiles; spot-check a page renders with new colors.

### Task 2: Responsive primitives — Container, SiteHeader, SiteFooter
- **ACTION**: Create `components/Container.tsx`, `components/SiteHeader.tsx`, `components/SiteFooter.tsx`.
- **IMPLEMENT**:
  - `Container`: `mx-auto w-full max-w-[var(--container)] px-4 sm:px-6 lg:px-8`; optional `narrow` → `max-w-xl` for forms.
  - `SiteHeader`: sticky, hairline border, backdrop blur; left = brand (`<Link href="/">` with logo mark + **"ร่วมงานกับเรา"**); desktop (`hidden md:flex`) nav links → "ตำแหน่งงาน" (/jobs), "ตรวจสอบสถานะ" (/status) + a `tap`/default CTA "สมัครงาน"→/jobs; mobile keeps it compact (brand + a single "ตำแหน่งงาน" link or a minimal menu). Optional `backHref` chevron (preserve current behavior for inner pages).
  - `SiteFooter`: multi-column on desktop (brand blurb · ลิงก์ [ตำแหน่งงาน, ตรวจสอบสถานะ] · ติดต่อ placeholder) collapsing to stacked on mobile; **keep the PDPA line** "ข้อมูลของคุณได้รับการคุ้มครองตาม พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล (PDPA)".
- **MIRROR**: SHELL split; focus-visible ring idiom from JobCard/PortalShell.
- **GOTCHA**: keep brand link text exactly "ร่วมงานกับเรา" (pwa offline test asserts the role link). Header must stay usable at 320px (no overflow) and on LINE in-app browser.
- **VALIDATE**: renders at 320 + 1440 without overflow.

### Task 3: Recompose PortalShell
- **ACTION**: Rewrite `PortalShell` to compose `SiteHeader` + `Container` + `SiteFooter`; props `backHref?`, `narrow?`, `className?`.
- **IMPLEMENT**: `min-h-dvh flex flex-col`; `<SiteHeader backHref={backHref}/>`; `<main><Container narrow={narrow}>{children}</Container></main>`; `<SiteFooter/>`.
- **GOTCHA**: every existing page imports `PortalShell` — keep the export + `backHref` working so jobs/detail/apply/status keep functioning. apply + status pass `narrow`.
- **VALIDATE**: all 4 inner pages still render + back nav works.

### Task 4: Landing page (`/`)
- **ACTION**: Replace `app/page.tsx` redirect with the landing.
- **IMPLEMENT**: `SiteHeader` + `Hero` + `LandingSections` + `SiteFooter`. `Hero` (`components/landing/Hero.tsx`): display heading using `--text-hero`, Thai subcopy, dual CTA ("ดูตำแหน่งงาน"→/jobs default button, "ตรวจสอบสถานะ"→/status ghost/link), an atmosphere panel (gradient/`--brand-soft` + geometric/illustrative SVG) beside it on desktop, stacked on mobile. `LandingSections`: (a) 3–4 value cards ("เติบโตในสายอาชีพ", "สวัสดิการที่ดูแลคุณ", "ทีมที่อบอุ่น", "สาขาทั่วประเทศ" — sample copy); (b) **featured jobs** — `"use client"` section using `usePublicPositions()` showing top ~6 as `JobCard`s in a grid with a "ดูทั้งหมด" link (reuse loading/empty states); (c) 3-step band ("เลือกตำแหน่ง → กรอกใบสมัคร → รอการติดต่อ"); (d) CTA band.
- **MIRROR**: CLIENT_PAGE + QUERY STATES (featured jobs); TAPPABLE CARD; BUTTON.
- **GOTCHA**: landing is largely static/server-renderable; isolate the live featured-jobs into a client sub-component so the rest can be server-rendered (perf). Keep within bundle budget (web/performance.md: microsite JS < 80kb where feasible — avoid heavy libs; pure CSS/SVG for atmosphere/motion).
- **VALIDATE**: `/` renders hero + sections + live featured jobs at all breakpoints.

### Task 5: Jobs list → responsive luxe grid
- **ACTION**: Update `app/jobs/page.tsx` + `JobCard.tsx`.
- **IMPLEMENT**: section header (display scale) + the `<ul>` becomes a responsive grid (`grid gap-4 sm:grid-cols-2 lg:grid-cols-3`) — **keep `<ul>`/`<li>`/`<JobCard>` (anchor)** so `ul li a` still matches. Restyle `JobCard` to luxe (white card, hairline border, refined hover lift, emerald/gold accent on the "เปิดรับ N อัตรา" chip, larger title). Keep loading/error/empty states (restyled).
- **MIRROR**: CLIENT_PAGE states; TAPPABLE CARD.
- **GOTCHA**: preserve h1 "ตำแหน่งงานที่เปิดรับ" (e2e heading) and the `<ul><li><a>` structure (apply-flow test clicks `ul li a`).
- **VALIDATE**: grid 1→2→3 col across breakpoints; e2e jobs test still green.

### Task 6: Job detail → 2-col desktop
- **ACTION**: Update `app/jobs/[id]/page.tsx`.
- **IMPLEMENT**: desktop 2-col (`lg:grid-cols-3`: content span-2 + sticky apply card span-1); content = title (display), level chip, sections ("เกี่ยวกับตำแหน่งนี้", "สิ่งที่เรามอบให้", "ขั้นตอนการสมัคร") using generic luxe Thai copy (API has no description); apply card = sticky on desktop, full-width CTA "สมัครงาน"→/apply, reassurance microcopy. Mobile = stacked, CTA pinned at column bottom (current behavior).
- **GOTCHA**: keep the "สมัครงาน" link (role link, e2e). Don't invent API fields — copy is generic. Keep loading/error states.
- **VALIDATE**: detail renders 2-col desktop / stacked mobile; apply CTA works.

### Task 7: Apply + Consent + LINE button restyle (preserve contract)
- **ACTION**: Update `apply/page.tsx` (narrow column), `ApplyStepper.tsx`, `ConsentStep.tsx`, `LineLoginButton.tsx` — **visual only**.
- **IMPLEMENT**: refined step indicator, luxe form fields (reuse `ui/input`,`ui/label`,`ui/checkbox`), card surfaces, spacing. Center in `narrow` container on desktop.
- **GOTCHA — PRESERVE EXACTLY** (e2e + backend): `id="status-token"`; headings "ความยินยอมในการใช้ข้อมูล", "ส่งใบสมัครเรียบร้อยแล้ว"; buttons "ถัดไป", "ส่งใบสมัคร", "เข้าสู่ระบบด้วย LINE"; text "เชื่อมต่อ LINE แล้ว"; labels matching `/ชื่อ-นามสกุล/`, `/อัปโหลดเรซูเม่/`, "รหัสติดตาม"; the checkbox role; the error text pattern. Do NOT touch `lib/queries` apply mutation or `buildApplyForm`.
- **VALIDATE**: `e2e/portal.spec.ts` apply flow passes unchanged.

### Task 8: Status + Offline + InstallPrompt restyle
- **ACTION**: Update `status/page.tsx`, `StatusCard.tsx`, `offline/page.tsx`, `InstallPrompt.tsx`.
- **IMPLEMENT**: narrow column, luxe token input + result card; offline page on-brand luxe; install prompt refined.
- **GOTCHA**: preserve "ตรวจสอบสถานะใบสมัคร", "รหัสติดตาม", "ตรวจสอบ", `/ไม่พบใบสมัคร/`, and "คุณกำลังออฟไลน์".
- **VALIDATE**: status + pwa offline e2e pass.

### Task 9: PWA color sync (manifest + viewport + test)
- **ACTION**: Update `app/manifest.ts` `theme_color`+`background_color`, `app/layout.tsx` `viewport.themeColor`, and the `pwa.spec.ts` `theme_color` assertion — all to the new brand color.
- **IMPLEMENT**: pick the new brand color = the emerald accent in hex (e.g. derive from `--accent` oklch(46% 0.10 158) ≈ a deep green hex) for `theme_color`; `background_color` = ivory (`--background` hex). Update `viewport.themeColor` to match. Update `pwa.spec.ts`: `expect(manifest.theme_color).toBe("<new hex>")`.
- **GOTCHA**: `pwa.spec.ts` currently asserts `theme_color === "#1f9d57"` and `name === "ร่วมงานกับเรา"` — keep the name, update only the color assertion. Convert OKLCH→hex carefully (use a fixed hex constant; document it).
- **VALIDATE**: `e2e/pwa.spec.ts` manifest test passes with the new color.

### Task 10: Responsive + a11y + perf sweep
- **ACTION**: Verify all pages at 320/375/768/1024/1440/1920; no overflow; tap targets ≥44px; focus-visible rings; contrast (luxury high-contrast helps); reduced-motion (global block already there — ensure new motion uses transform/opacity only).
- **MIRROR**: web/performance.md (compositor-only animation), web/testing.md breakpoints.
- **VALIDATE**: see Validation Commands (Playwright across the portal's 3 viewports + manual desktop check).

---

## Testing Strategy

The existing Playwright specs are the safety net — they must stay green (proving the redesign preserved behavior). No new unit tests (visual/layout change); visual-regression via Playwright screenshots (already captured to `e2e/__screens__/`).

### Verification matrix
| Check | Expectation |
|---|---|
| `e2e/portal.spec.ts` | jobs list, apply flow, status flow all pass (preserved names/ids) |
| `e2e/pwa.spec.ts` | manifest (new theme_color), icons, offline, SW pass |
| Landing `/` | hero + sections + live featured jobs render; CTAs route correctly |
| Responsive | no overflow at 320/375/768/1024/1440/1920; mobile/LINE first-class |
| a11y | tap ≥44px, focus rings, contrast AA, reduced-motion honored |

### Edge Cases Checklist
- [ ] Empty positions → landing featured + /jobs show empty states (not broken)
- [ ] API error → retry cards render in luxe style
- [ ] 320px smallest → no horizontal scroll on every page
- [ ] 1920px → content centered in `--container`, not stretched/sparse
- [ ] Long Thai job titles → truncate/wrap gracefully in grid cards
- [ ] LINE in-app browser (mobile) → header/CTA/forms usable

---

## Validation Commands

### Static + build
```bash
cd career-portal && pnpm install --frozen-lockfile && pnpm lint && pnpm build
```
EXPECT: lint clean; prod build succeeds (no CSP/eval concern in prod).

### E2E (prod build; mirrors CI `playwright` job)
```bash
make up && make migrate-up && make seed   # stack for apply/status data
cd career-portal && pnpm build && CI=1 pnpm test:e2e
```
EXPECT: all portal + pwa specs pass (3 mobile projects).

### Responsive/visual (manual + screenshots)
```bash
# screenshots already emitted to e2e/__screens__/ per project (320/375/768)
# desktop: open http://localhost:3001 and check 1024/1440/1920 in devtools
```
EXPECT: intentional layout at every breakpoint; review the captured screens.

### Manual Validation
- [ ] `/` landing: hero + value cards + live featured jobs + steps + CTA + footer, all responsive.
- [ ] `/jobs` grid scales 1→2→3 col; cards luxe; states styled.
- [ ] `/jobs/[id]` 2-col desktop / stacked mobile; apply CTA works.
- [ ] apply + status flows complete end-to-end (LINE stub) and look refined.
- [ ] Hard-refresh (SW) — fresh styles load; offline page on-brand.

## Acceptance Criteria
- [ ] `/` is a real responsive landing page (was a redirect).
- [ ] All pages are responsive 320→1920 with no overflow; mobile/LINE first-class.
- [ ] Clean-luxury direction applied via tokens (palette/scale/depth), not ad-hoc styles.
- [ ] All existing Playwright specs pass (apply/status/jobs/pwa) — behavior preserved.
- [ ] PWA theme/bg colors + viewport + pwa.spec updated consistently.
- [ ] `pnpm lint` + `pnpm build` clean.

## Completion Checklist
- [ ] Tokens drive the design (no hardcoded hex sprinkled in components)
- [ ] Reuses `ui/*` primitives + `buttonVariants` (no forks)
- [ ] Preserved every e2e accessible name / id / structure (see Task 7/8/9 gotchas)
- [ ] Animations compositor-only (transform/opacity); reduced-motion honored
- [ ] Two-font rule kept (Noto Sans Thai + Inter)
- [ ] No API/contract/`lib/queries` changes
- [ ] No new heavy dependencies (bundle budget)

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Redesign breaks an e2e accessible name/id | Medium | High | Explicit Preserve list in Tasks 7–9; run `pnpm test:e2e` after each page |
| `theme_color` change desyncs manifest/viewport/test | Medium | Medium | Task 9 updates all three together with one documented hex |
| Desktop layout stretches/looks sparse at 1920 | Low | Medium | `--container` max 1200px; centered; atmosphere panels fill hero |
| OKLCH→hex conversion for PWA color wrong | Low | Low | use a fixed documented hex constant; verify in pwa.spec |
| Scope creep into a full marketing site | Medium | Medium | NOT Building list; landing is one page only |
| Bundle bloat from animation libs | Low | Medium | CSS/SVG only; no GSAP/framer unless justified |
| Dev mode still CSP-broken during local dev | High | Low | validate via prod build (as demo/CI do); dev-CSP fix is a separate slice |

## Notes
- Direction confirmed 2026-06-08: **Landing + elevate existing**, **Clean luxury (light)**, **placeholders + sample Thai copy**. Responsive 320→1920 with mobile/LINE first-class is a hard requirement.
- The HR dashboard (`frontend/`) is intentionally a separate, neutral "operations" look — this redesign is **career-portal only**; do not touch `frontend/`.
- Validate via **prod build** (`pnpm build && pnpm start`) — `pnpm dev` is currently CSP-broken (webpack eval); that fix is a separate slice and not required here.
- Branch `feat/career-portal-redesign`, NO attribution, squash-merge; the CI `playwright` job is the gate. After implement → `/code-review` → `/prp-pr`.
```
