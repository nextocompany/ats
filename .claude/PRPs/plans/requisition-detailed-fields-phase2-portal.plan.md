# Plan: Requisition Detailed Fields - Phase 2 (Career Portal Surfacing)

## Summary
Surface the canonical job description (Responsibilities + Qualifications + Benefits) on the public career portal job detail page. The portal stays **position-keyed** (one card per position, aggregating open vacancies): no funnel rebuild. The detail page currently shows only title/level with hardcoded generic copy; this phase replaces that with the linked position's real `responsibilities` + `qualifications` + `benefits` text, falling back to the existing generic copy when a position has none.

> Depends on Phase 1: the `positions.benefits` column and the widened `positions.FindByID` (loading benefits) are added in Phase 1. `positions.responsibilities`/`qualifications` already existed. Phase 1 fills the requisition + adds the position-level benefits source; Phase 2 publishes the position-level JD to candidates.

## User Story
As a candidate browsing the career portal,
I want to read the real responsibilities, qualifications, and benefits for a role,
So that I understand the job before applying instead of seeing generic placeholder copy.

## Problem to Solution
Portal job detail shows only title + level + hardcoded "about/offer/steps" copy. After: portal job detail shows the position's real Responsibilities + Qualifications + Benefits (with graceful fallback to the existing copy).

## Metadata
- **Complexity**: Small
- **Source PRD**: N/A
- **PRD Phase**: Phase 2 of the requisition-detailed-fields request
- **Estimated Files**: ~5 (1 backend, 1 career-portal types, 1 career-portal component, 2 i18n)
- **Migration**: none

---

## UX Design

### Before
```
Job detail /jobs/[id]
┌──────────────────────────────┐
│ <level>                       │
│ <title_th>  <title_en>        │
│ About this role (generic copy)│
│ What we offer (generic copy)  │
│ How it works (generic steps)  │
│            [ Apply ]          │
└──────────────────────────────┘
```

### After
```
Job detail /jobs/[id]
┌──────────────────────────────┐
│ <level>                       │
│ <title_th>  <title_en>        │
│ Responsibilities  (real text) │  from positions.responsibilities
│ Qualifications    (real text) │  from positions.qualifications
│ Benefits          (real text) │  from positions.benefits
│ What we offer (generic copy)  │  unchanged
│ How it works (generic steps)  │  unchanged
│            [ Apply ]          │
└──────────────────────────────┘
(if position has no JD text, keep the generic "about" copy as today)
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| `GET /api/v1/public/positions/:id` | returns `id,title_th,title_en,level` | also returns `responsibilities, qualifications, benefits` | FindByID loads them (benefits added in Phase 1); just widen the projection |
| Job detail body | hardcoded `t("about")` | render real responsibilities + qualifications + benefits; fallback to `t("about")` if all empty | preserves "no fabricated requirements" safety when data is missing |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/public/handler.go` | 130-143 | `GetPosition` - the projection that currently DROPS responsibilities/qualifications |
| P0 | `backend/internal/positions/model.go` | 21-31, 63-86 | `Position` struct + `FindByID` (already SELECTs both JD fields) |
| P0 | `career-portal/app/jobs/[id]/JobDetailClient.tsx` | 16-95 | Detail body; the "intentionally generic" marker (17-20); where to render JD |
| P0 | `career-portal/lib/types.ts` | 87-92 | `PositionDetail` to extend |
| P1 | `career-portal/lib/queries.ts` | 89-95 | `usePublicPosition(id)` (no change, just confirms shape) |
| P1 | `career-portal/app/jobs/[id]/page.tsx` | 15-23 | Server wrapper fetch for OG metadata (no change needed) |
| P1 | `career-portal/messages/*.json` (`jobs` namespace) | - | Add `responsibilities`/`qualifications` section headings |

## External Documentation
No external research needed.

---

## Patterns to Mirror

### BACKEND public projection (widen this)
```go
// SOURCE: backend/internal/public/handler.go:130-143 (GetPosition)
p, err := h.pos.FindByID(ctx, id)   // loads Responsibilities + Qualifications + Benefits (benefits added in Phase 1)
// CURRENT (drops JD):
return httpx.OK(c, fiber.Map{
  "id": p.ID, "title_th": p.TitleTH, "title_en": p.TitleEN, "level": p.Level,
})
// AFTER: add "responsibilities": p.Responsibilities, "qualifications": p.Qualifications, "benefits": p.Benefits
```

### CAREER-PORTAL detail render (hardcoded copy to augment)
```tsx
// SOURCE: career-portal/app/jobs/[id]/JobDetailClient.tsx:59-95
// Currently renders t("about"), t.raw("offer"), t.raw("steps").
// Add Responsibilities + Qualifications sections ABOVE "offer", driven by data,
// with fallback to t("about") when both fields are empty.
```

### CAREER-PORTAL types
```ts
// SOURCE: career-portal/lib/types.ts:87-92
export interface PositionDetail {
  id: string; title_th: string; title_en: string; level: string;
  // ADD:
  responsibilities: string; qualifications: string; benefits: string;
}
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/public/handler.go` | UPDATE | Add `responsibilities`/`qualifications` to GetPosition response |
| `backend/internal/public/handler_test.go` | UPDATE | Assert new fields in response (if test exists) |
| `career-portal/lib/types.ts` | UPDATE | Extend `PositionDetail` |
| `career-portal/app/jobs/[id]/JobDetailClient.tsx` | UPDATE | Render JD with fallback |
| `career-portal/messages/en.json` + `th.json` | UPDATE | Section headings under `jobs` namespace |

## NOT Building (out of scope, Phase 2)
- Showing requisition-level-only fields (other, salary, priority, reason, store) to candidates: those stay internal (agreed tradeoff: portal is position-keyed). NOTE: Benefits IS shown, sourced from `positions.benefits` (position-level), not from the per-requisition benefits copy.
- Changing the portal from position-keyed to vacancy/store-keyed (the large alternative we explicitly declined).
- Any change to the apply flow / intake (still `position_id`-keyed).
- AI screening integration.

---

## Step-by-Step Tasks

### Task 1: Widen public GetPosition projection
- **ACTION**: Add the three JD fields to the response map in `backend/internal/public/handler.go:140-142`.
- **IMPLEMENT**: add `"responsibilities": p.Responsibilities, "qualifications": p.Qualifications, "benefits": p.Benefits` to the `fiber.Map`.
- **MIRROR**: the existing map at handler.go:140-142.
- **GOTCHA**: This is a PUBLIC endpoint, exposing responsibilities/qualifications/benefits is intended here (they are role-generic JD, not PII). Do NOT expose any requisition-level or store-level fields on this endpoint. Requires Phase 1's `Position.Benefits` field to exist.
- **VALIDATE**: `go test ./internal/public/...`; `curl /api/v1/public/positions/<id>` shows the three fields.

### Task 2: Extend career-portal PositionDetail type
- **ACTION**: Add `responsibilities: string; qualifications: string; benefits: string` to `PositionDetail` in `career-portal/lib/types.ts:87-92`.
- **MIRROR**: existing interface.
- **VALIDATE**: typecheck in career-portal (confirm the package manager/script from `package.json`).

### Task 3: Render JD on the detail page with fallback
- **ACTION**: Update `career-portal/app/jobs/[id]/JobDetailClient.tsx` to render Responsibilities + Qualifications + Benefits.
- **IMPLEMENT**:
  - Compute `hasJd = !!(position.responsibilities?.trim() || position.qualifications?.trim() || position.benefits?.trim())`.
  - When `hasJd`: render a "Responsibilities" section showing `position.responsibilities`, a "Qualifications" section showing `position.qualifications`, and a "Benefits" section showing `position.benefits` (each rendered only if its own field is non-empty). Preserve line breaks (`whitespace-pre-line`).
  - When `!hasJd`: keep the current generic `t("about")` block (back-compat for positions with no JD text).
  - Keep the existing "offer"/"steps" sections unchanged.
  - Remove/relax the "intentionally generic" comment (17-20) note since real data is now shown when present.
- **MIRROR**: the section markup already in JobDetailClient.tsx:59-95.
- **GOTCHA**: JD text may contain newlines/bullet-ish lines, render with `whitespace-pre-line`, never `dangerouslySetInnerHTML` (text is plain, keep it escaped). Empty-string vs null both count as "no JD". Render each section conditionally so an empty benefits field does not leave a dangling heading.
- **VALIDATE**: typecheck; a position with JD shows real text, one without shows the generic copy.

### Task 4: i18n section headings
- **ACTION**: Add `responsibilities`, `qualifications`, and `benefits` headings under the `jobs` namespace in both `career-portal/messages/en.json` and `th.json`.
- **IMPLEMENT**: e.g. TH `"responsibilities": "หน้าที่ความรับผิดชอบ"`, `"qualifications": "คุณสมบัติผู้สมัคร"`, `"benefits": "สวัสดิการ"`; EN equivalents.
- **MIRROR**: existing `jobs` keys (`about`, `offer`, `steps`).
- **GOTCHA**: add to BOTH locale files with identical keys.
- **VALIDATE**: build; both locales render headings.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| public GetPosition includes JD | position with responsibilities/qualifications/benefits | response has all three fields | no |
| public GetPosition empty JD | position with empty JD | fields present as `""` | yes |

### Edge Cases Checklist
- [ ] Position with no JD text → portal falls back to generic copy (no empty headings)
- [ ] JD text with newlines → rendered with preserved line breaks
- [ ] JD text is escaped (no HTML injection)
- [ ] Position not found → 404 unchanged

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./internal/public/...
cd career-portal && pnpm tsc --noEmit
```
EXPECT: clean

### Unit Tests
```bash
cd backend && go test ./internal/public/...
```
EXPECT: pass

### Manual Validation
- [ ] Open `/jobs/<id>` for a position that has responsibilities/qualifications → real JD shows
- [ ] Open `/jobs/<id>` for a position without JD → generic copy shows (no blank sections)
- [ ] Apply flow still works (unchanged, position-keyed)

---

## Acceptance Criteria
- [ ] Candidates see real Responsibilities + Qualifications + Benefits on the job detail page when available
- [ ] Graceful fallback to existing generic copy when a position has no JD
- [ ] No requisition-level/store-level data leaked to the portal
- [ ] Apply funnel unchanged

## Completion Checklist
- [ ] Public endpoint exposes only role-generic JD (no PII, no internal fields)
- [ ] Text rendered safely (escaped, `whitespace-pre-line`)
- [ ] i18n keys in BOTH locale files
- [ ] No funnel/intake changes

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Position JD empty for many roles → blank-looking page | Medium | Low | Fallback to existing generic copy when JD absent |
| Accidentally exposing internal fields on public endpoint | Low | Medium | Only add responsibilities/qualifications; review the fiber.Map diff |

## Notes
- Portal remains position-keyed by design decision (chosen over a vacancy/store-keyed rebuild). The JD shown is the **position catalog's** canonical text - the same source Phase 1 prefills requisitions from - so dashboard and portal stay consistent.
- Deploy after dev: api (public handler) + career-portal. No migration. No worker/scheduler change.
- HARD RULES: no em dash; deploy after dev; NO Co-Authored-By.
