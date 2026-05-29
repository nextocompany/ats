# Plan: Sprint 4c — Career Portal UI (Next.js, public)

## Summary
Build the public **Career Portal** as a separate Next.js app (`career-portal/`) for Thai job-seekers, mostly opening it inside LINE on mobile. Surfaces: a **job listing** (open positions), a **job detail + multi-step apply** flow (PDPA consent → details → resume upload, mock LINE auth), and a **status page** (check by token). Direction: warm, **mobile-first, trust-building** single-column flow. Includes a small backend change so public `/apply` records **PDPA consent** (F13) per candidate. Validated with Playwright at mobile breakpoints + `next build`.

## User Story
As a **job-seeker (on my phone, often via LINE)**, I want **to browse open positions, give PDPA consent, and apply with my resume in a few taps, then check my status by a link**, so that **applying is fast and I trust where my data goes** — feeding the same intake→score→assign pipeline.

## Problem → Solution
**Current state:** The `/public/*` API exists (Sprint 3) and the HR side is built (4a/4b), but candidates have no UI — applications only arrive via authenticated intake / curl.
**Desired state:** `career-portal/` app: `/jobs` (list) → `/jobs/[id]` (detail + Apply) → multi-step apply (consent + form + resume, mock LINE login) → returns a **status token** → `/status` (check). PDPA consent is captured and recorded against the candidate.

## Metadata
- **Complexity**: Medium–Large (second Next.js app, smaller than the dashboard; ~22 files + 1 small backend change)
- **Source PRD**: PRP v1.0 — Sprint 4 (W9–10), F14 Career Portal frontend; F13 consent capture
- **Decisions locked**: **Mobile-first friendly/trust** direction; **separate `career-portal/` app**; mock LINE auth (LIFF deploy-time later); public `/apply` records PDPA consent
- **Estimated Files**: ~22 frontend + 2 backend edits

---

## UX Design

### Direction: Mobile-first, friendly, trust-building
Single-column, large tap targets, generous spacing, warm accent, clear progress in the apply flow, reassuring PDPA + status messaging. Optimized for the LINE in-app browser on small screens (primary breakpoints 320/375; scales up to tablet/desktop).

### After (flow)
```
/jobs               /jobs/[id]                apply (multi-step)            /status
┌──────────────┐    ┌──────────────┐    ① Consent (PDPA)  ┐               ┌──────────────┐
│ ตำแหน่งงาน     │ →  │ Cashier       │ →  ② Your details   │ → status token│ กรอก token →  │
│ • Cashier  ▸ │    │ Staff · 3 เปิด │    ③ Upload resume   │   shown +copy │ สถานะ: scored │
│ • Forklift ▸ │    │ [ สมัครงาน ]  │    [LINE login → ส่ง] ┘               └──────────────┘
└──────────────┘    └──────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Browse jobs | none | `/jobs` public list (open positions) | F14 |
| Apply | curl / MS Form | in-portal multi-step (consent→details→resume), mock LINE | F14 + F13 |
| Consent | not captured for public | required step, recorded to PDPA | F13 |
| Status | none | `/status` by opaque token | F14 |

---

## Mandatory Reading (the contract + patterns to reuse)
| Priority | File | Why |
|---|---|---|
| P0 | `backend/internal/public/handler.go` | `/public/positions`, `/positions/:id`, `/apply` (multipart fields + `X-LINE-IdToken` + `status_token`), `/status/:token` shapes |
| P0 | `frontend/lib/api.ts` | envelope-aware client pattern to copy into `career-portal/lib/api.ts` |
| P0 | `frontend/lib/queries.ts` | TanStack Query hook pattern |
| P1 | `frontend/app/globals.css` | token approach (define a warmer palette here) |
| P1 | `backend/internal/pdpa/pdpa.go` | `Repo.Record(Consent, ip)` — wire into public apply |
| P1 | `backend/internal/applications/service.go` | `IntakeResult.CandidateID` — available after intake to record consent |
| P1 | `~/.claude/rules/ecc/web/*` | design-quality, performance, a11y, testing |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| LIFF (later) | developers.line.biz/liff | real in-LINE auth needs a LIFF ID + `@line/liff`; Sprint 4c stubs login, structured so LIFF drops in. |
| Next multipart upload | fetch FormData | apply posts `FormData` (resume file + fields) to `/public/apply`; no JSON. |
| Mobile viewport/a11y | web rules | `<meta viewport>`, ≥44px tap targets, focus-visible, contrast AA. |

### Research Notes
```
KEY_INSIGHT: public apply does not record PDPA consent yet.
APPLIES_TO: backend.
GOTCHA: extend public.Apply to accept consent_given/consent_version; after intake (which returns CandidateID), call pdpa.Record. Inject a ConsentRecorder into public.NewHandler. Gate submission client-side on the consent checkbox too.

KEY_INSIGHT: dev LINE auth is a stub.
APPLIES_TO: apply flow.
GOTCHA: the backend mock verifier accepts any non-empty X-LINE-IdToken. The portal "Login with LINE" sets a stub token (dev). Real LIFF (window.liff.getIDToken) is a deploy-time swap behind the same lib/line.ts seam.

KEY_INSIGHT: status token is opaque and the only candidate-facing handle.
APPLIES_TO: status page.
GOTCHA: after apply, show the token + a copyable status link (/status?token=...). The candidate has no login to "their" applications beyond the token.
```

---

## Patterns to Mirror
### MOBILE_TOKENS (warm, candidate-facing — distinct from the HR console)
```css
:root {
  --color-bg: oklch(99% 0.01 95);      /* warm off-white */
  --color-text: oklch(25% 0.02 60);
  --color-brand: oklch(64% 0.16 150);  /* friendly green (LINE-adjacent) */
  --tap-min: 44px;                      /* min touch target */
  --radius: 0.875rem;
}
```
### API_CLIENT / QUERY / TYPES
Copy the envelope-aware client + TanStack hooks from `frontend/`. Public app needs no auth header; apply sends `X-LINE-IdToken`.

### MULTISTEP_FORM
Client component with `step` state (1 consent → 2 details → 3 resume); progress indicator; back/next; submit posts FormData; disable next until step valid.

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/internal/public/handler.go` | UPDATE | accept `consent_given`/`consent_version`; record consent after intake |
| `backend/cmd/api/main.go` | UPDATE | inject pdpa recorder into `public.NewHandler` |
| `backend/internal/public/handler.go` (interface) | UPDATE | add a small `ConsentRecorder` interface (avoids importing pdpa concretely if cleaner) |
| `career-portal/` (scaffold) | CREATE | Next.js app (TS, App Router, Tailwind, shadcn minimal) |
| `career-portal/app/layout.tsx`, `globals.css`, `providers.tsx` | CREATE | mobile-first shell + tokens + QueryClient + viewport meta |
| `career-portal/app/page.tsx` | CREATE | redirect → `/jobs` (or simple landing) |
| `career-portal/app/jobs/page.tsx` | CREATE | open positions list |
| `career-portal/app/jobs/[id]/page.tsx` | CREATE | job detail + Apply CTA |
| `career-portal/app/jobs/[id]/apply/page.tsx` | CREATE | multi-step apply (consent→details→resume) |
| `career-portal/app/status/page.tsx` | CREATE | token entry + status display |
| `career-portal/lib/{api.ts,types.ts,queries.ts,line.ts}` | CREATE | client, types, hooks, mock LINE auth seam |
| `career-portal/components/{JobCard,ApplyStepper,ConsentStep,LineLoginButton,StatusCard}.tsx` | CREATE | portal UI |
| `career-portal/components/ui/*` | CREATE | minimal shadcn (button, input, card, label, checkbox) |
| `career-portal/e2e/portal.spec.ts`, `playwright.config.ts` | CREATE | mobile-viewport e2e + screenshots |
| `career-portal/.env.example`, README | CREATE | `NEXT_PUBLIC_API_URL` |
| backend CORS origin | UPDATE | add the portal origin (e.g. :3001) to `CORS_ALLOW_ORIGINS` default |

## NOT Building (later)
- **Real LIFF / LINE Login** (window.liff) — stubbed; deploy-time swap behind `lib/line.ts`.
- **Meta (Facebook) share / LINE OA** deep integration — a share link only.
- **Candidate account / saved applications** — token-only status (matches the API).
- **Real Azure AD** (that's the HR side), PWA, i18n toggle (Thai-first copy is fine).
- New scoring/pipeline behavior — apply reuses the existing intake→pipeline.

---

## Step-by-Step Tasks

### Task 1: Backend — record PDPA consent on public apply
- **ACTION**: Add a `ConsentRecorder` interface to `public` (`Record(ctx, candidateID, given bool, version, sourceChannel, ip string) error`); extend `Apply` to read `consent_given`/`consent_version` form fields and, after intake, record consent for `result.CandidateID`. Inject the pdpa repo (adapter) in `main.go`.
- **MIRROR**: existing handler validation + `pdpa.Repo.Record`.
- **GOTCHA**: require consent server-side (reject apply if `consent_given` != true) — PDPA is mandatory before storing data. Recording failure shouldn't lose the application (log; consent is also retryable), but a missing consent flag → 400.
- **VALIDATE**: curl apply with `consent_given=true` → application created + `pdpa_consents` row + candidate snapshot; without consent → 400.

### Task 2: Scaffold career-portal app + tokens
- **ACTION**: `pnpm create next-app career-portal` (TS, App Router, Tailwind); `shadcn init` + add button/input/card/label/checkbox; warm mobile-first tokens in `globals.css`; viewport meta; Noto Sans Thai as primary; QueryClient provider.
- **MIRROR**: MOBILE_TOKENS; performance (lean JS, font swap).
- **GOTCHA**: run on port 3001 (dashboard uses 3000); set `NEXT_PUBLIC_API_URL`.
- **VALIDATE**: `pnpm --filter career-portal build` succeeds.

### Task 3: API client + types + hooks + mock LINE
- **ACTION**: copy envelope client; `types.ts` (PublicPosition, ApplyResult{status_token}, Status{status,applied_at,position}); `queries.ts` (`usePublicPositions`, `usePublicPosition(id)`, `useApplyMutation`, `useStatus(token)`); `lib/line.ts` (`getIdToken()` → dev stub; structured for LIFF).
- **GOTCHA**: apply is multipart — the mutation builds FormData (not JSON); include `X-LINE-IdToken`.
- **VALIDATE**: typecheck; positions hook fetches `/public/positions`.

### Task 4: Jobs list
- **ACTION**: `jobs/page.tsx` + `JobCard` — list open positions (title_th, level, open_count) as large tappable cards; empty state; loading skeleton.
- **MIRROR**: mobile-first, ≥44px targets.
- **VALIDATE**: e2e (mobile viewport): list renders; tap → detail.

### Task 5: Job detail + apply entry
- **ACTION**: `jobs/[id]/page.tsx` — position detail + prominent "สมัครงาน" CTA → `/jobs/[id]/apply`.
- **VALIDATE**: detail renders title; CTA navigates.

### Task 6: Multi-step apply
- **ACTION**: `jobs/[id]/apply/page.tsx` + `ApplyStepper`, `ConsentStep`, `LineLoginButton`: step 1 PDPA consent (required checkbox + purpose/retention text), step 2 details (name required; phone/email/id_card/province), step 3 resume upload + LINE login + submit → `useApplyMutation` (FormData) → success screen with **status token** + copyable status link.
- **MIRROR**: MULTISTEP_FORM; validate file type/size client-side (pdf/docx/jpg/png ≤10MB) mirroring the API.
- **GOTCHA**: block submit until consent given + LINE token present; show API errors inline; handle the 400/415/413 envelope errors gracefully.
- **VALIDATE**: e2e: complete the flow with a stub token + sample PDF → token returned + shown.

### Task 7: Status page
- **ACTION**: `status/page.tsx` — token input (and `?token=` prefill) → `useStatus` → `StatusCard` (status, position, applied_at) with friendly Thai labels; not-found message.
- **VALIDATE**: e2e: enter the token from apply → status shows.

### Task 8: CORS + docs + e2e
- **ACTION**: add `http://localhost:3001` to backend `CORS_ALLOW_ORIGINS` default; `playwright.config.ts` (mobile device profile) + `portal.spec.ts`; `.env.example`; README run steps.
- **VALIDATE**: full Playwright run + screenshots at 320/375/768.

---

## Testing Strategy (web, mobile-first)
### Priority
1. **Visual** — screenshots at 320/375/768 for jobs, detail, each apply step, status (the primary in-LINE sizes).
2. **A11y** — axe on jobs + apply; tap-target size; labels on every input; focus-visible; contrast AA.
3. **E2E flows** — browse → detail → apply (consent→details→resume, stub LINE) → token → status.
4. **Unit** — api client envelope unwrap; apply FormData builder; consent-gating logic.

### E2E shape
```ts
test('apply flow returns a status token', async ({ page }) => {
  await page.goto('/jobs');
  await page.getByRole('link', { name: /cashier/i }).first().click();
  await page.getByRole('link', { name: /สมัครงาน|apply/i }).click();
  await page.getByRole('checkbox', { name: /consent|ยินยอม/i }).check();
  // …details + file + stub LINE login…
  await expect(page.getByText(/status token|รหัสติดตาม/i)).toBeVisible();
});
```

### Edge Cases Checklist
- [ ] No open positions → friendly empty state
- [ ] Consent not given → submit blocked (client) + 400 (server)
- [ ] Unsupported file / >10MB → inline error (mirrors API 415/413)
- [ ] Invalid/unknown status token → not-found message
- [ ] Small screen (320px) — no overflow, tap targets ≥44px
- [ ] Reduced motion honored

---

## Validation Commands
### Static + Build
```bash
cd career-portal && pnpm lint && pnpm exec tsc --noEmit && pnpm build
```
### Backend consent change
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### E2E (stack + API + portal up)
```bash
make up && make migrate-up && make seed     # ensure open vacancies exist for /public/positions
# (PS vacancy-opened or seed provides open positions)
cd career-portal && pnpm exec playwright install chromium && pnpm exec playwright test
```
EXPECT: jobs list shows seeded open positions; apply returns a token; status resolves; screenshots at mobile breakpoints; consent recorded server-side (verify pdpa_consents row).

### End-to-end (manual)
- [ ] `/jobs` lists open positions; tap → detail.
- [ ] Apply: consent → details → resume + LINE(stub) → token shown.
- [ ] `/status?token=…` shows the application status.
- [ ] DB: `pdpa_consents` has the new consent row.

---

## Acceptance Criteria
- [ ] `career-portal/` builds + runs against `/public/*`; mobile-first, not a template.
- [ ] Jobs list + detail render open positions from the API.
- [ ] Multi-step apply (consent→details→resume, mock LINE) submits and returns a status token.
- [ ] Public `/apply` **records PDPA consent**; consent is required (server + client).
- [ ] Status page resolves a token to a friendly status.
- [ ] Backend change: vet/lint/tests pass. Portal: lint/tsc/build clean; Playwright green; screenshots at 320/375/768; axe no serious issues.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LIFF stubbing diverges from real LINE | Med | Med | isolate behind `lib/line.ts`; backend mock verifier already mirrors this; real LIFF is a drop-in |
| Consent not legally captured | Med | High | mandatory server-side gate + recorded row; client checkbox; this sprint fixes the gap |
| Multipart apply from browser | Low | Med | FormData (no JSON); reuse the tested API; client-side type/size validation |
| Two frontends drift | Med | Low | copy the small client/token bits intentionally; document; a shared pkg is a later option |
| Mobile overflow / tap targets | Med | Med | 320px screenshots + axe in CI; ≥44px targets in tokens |

## Notes
- This completes F14 end-to-end: PS opens a vacancy → it appears on the portal → candidate applies → the same intake→OCR→parse→dedup→score→assign pipeline runs → HR sees it in the dashboard → hire syncs to PeopleSoft.
- Real LINE Login/LIFF + Meta share + Azure Static Web App deploy are deploy-time wiring (POC §19), not blocked by this sprint.
- After 4c, the remaining roadmap is S5 (re-engagement + report scheduler + Azure AI Search), S6–7 (PWA + E2E + security), S8 (UAT + go-live).
