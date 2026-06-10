# Generator State — Iteration 001 (new GAN-v2 loop)

## What Was Built (this iteration)
Elevated the existing "Ledger" CP Axtra design system on the highest-impact surfaces.
CI preserved throughout: deep blue #0B47B8, warm yellow #FFC02E, white, dot motif, globals tokens.

### Shell / chrome
- **SideNav**: faint brand dot-dither atmosphere on the navy spine; brass crown keyline;
  active nav item is now a lifted inset "pill" (inset + drop shadow) with a sliding brass
  rail (translate+opacity, compositor-friendly); signature `.dot-rule` closes the nav block.
- **AppHeader**: now sticky w/ backdrop blur; live ticking clock (date + HH:MM) and a
  pulsing brand "Live" dot (animate-ping, motion-reduce safe); cleaner breadcrumb.
- **BrandMark**: monogram gains depth (inset + colored drop shadow) and a brass corner-dot
  signature; ring color adapts to dark spine vs light mobile bar.

### Overview
- Editorial hero masthead: brass-underlined eyebrow, larger display title, `.dot-cluster`
  atmosphere top-right, elevated CTA.
- "Where to act": open-count brass chip; quick-action icon tiles invert to brand fill on hover.
- Pass-through card upgraded to a filled brand panel with brass left keyline + radial glow and
  a brass scaleX gauge — matches the hero KPI's premium accent language.

### Login
- Layered atmosphere (dual radial glows + dot dither, brass crown line) instead of a flat panel;
  depth on the monogram with brass corner-dot.
- Larger display headline; a tabular **proof ticker** (42 stores / AI / 1 source of truth).
- Brass-underlined eyebrow on the sign-in side; firmer card.

### Inbox
- **Active-filter chips**: URL state (status, min_score) shown as removable brand chips with
  per-chip clear + "Clear all"; hover inverts to brand fill.
- Empty state redesigned: `.dot-rule` signature, stronger hierarchy, contextual "Clear filters".

### Tokens
- Fixed `--font-sans` / `--font-heading` / `--font-mono` to resolve to the real Geist + Noto
  Thai families (were self-referential), so `font-sans`/`font-heading` utilities pair correctly.

## Known Issues / Next
- Detail pages (application, candidate), Candidates, Search, Analytics still on the prior
  (strong) treatment — to push further in next iterations per evaluator feedback.

## Dev Server
- URL: http://localhost:3000
- Status: running (verified 200 on /login)
- Command: pnpm dev
- Build: green (pnpm build)
