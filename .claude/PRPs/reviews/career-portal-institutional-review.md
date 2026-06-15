# Code Review: career-portal institutional redesign (`4fc6e57`)

**Reviewed**: 2026-06-15
**Branch**: `feat/career-portal-institutional` → `main`
**Decision**: **APPROVE-with-comments** → all comments actioned in `ebd6676`

## Summary
Presentation-only institutional-minimal rebuild. Verified: **data/network layer untouched**
(`lib/{queries,api,types}` diff empty), **AXTRA token purge clean**, **dev-CSP `unsafe-eval`
correctly gated to development**, **filter URL-state logic correct** (no infinite-loop trap;
`useSearchParams` single source of truth, memoized O(n) filtering). No CRITICAL/HIGH.

## Findings & resolution
| Sev | Finding | Status |
|---|---|---|
| MEDIUM | `SiteFooter` "องค์กร" links all `href="/"` (deceptive duplicate destinations) | ✅ Fixed `ebd6676` — About → external corporate site; ESG/culture → `/#esg`,`/#culture` anchors |
| MEDIUM | `JobFilters` Checkbox redundant `aria-label` hides count from AT | ✅ Fixed — dropped aria-label (label names it); count labelled |
| LOW | `StatBand` key on `label` (fragile for reusable comp) | ✅ Fixed — key on `value+label` |
| LOW | `MediaBlock` points key on content string | ✅ Fixed — key on `index+content` |
| LOW | `JobsPage` `"use client"` could move to inner browse comp | Noted; left (works, build green) |

## What was well-done (kept)
Clean filter logic (no useEffect→router.replace loop), strict type safety (no `any`, `isLevel`
narrows URL input), Base UI `render=` (no asChild), immutable filter state, `<fieldset>/<legend>`
+ `aria-live` count, decorative SVGs `aria-hidden`, all files <800 lines / functions <50.

## Validation
| Check | Result |
|---|---|
| Type check (tsc) | ✅ Pass |
| Lint (eslint) | ✅ Pass |
| Build (next build) | ✅ Pass (12 routes) |
| Tests (e2e) | ⏸ Not run (needs backend/mock) |

## Files reviewed
42 files in `career-portal/` (commit 4fc6e57) + 5 follow-up fixes (ebd6676).
