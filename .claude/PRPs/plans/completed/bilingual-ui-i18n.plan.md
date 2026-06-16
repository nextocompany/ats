# Plan: Bilingual UI (i18n TH/EN with switcher)

## Summary
Make both frontends fully bilingual (Thai/English) with a runtime language switcher,
using **next-intl** on the Next.js App Router. Career-portal (public) uses
locale-prefixed routing with `as-needed` prefix (Thai default = no prefix, so existing
notification deep links keep working; English = `/en/...`). The dashboard (private)
uses cookie-based locale (no route restructuring). UI chrome is translated;
AI/LLM-generated content and candidate notifications stay as-is (data, not chrome).

## User Story
As a **Thai or English-speaking user** (candidate on the career portal, HR on the
dashboard), I want to switch the interface language, so that I can use the system in
my preferred language.

## Problem → Solution
No i18n framework today: career-portal is hardcoded Thai, dashboard hardcoded
English, text inline in ~100 components. → next-intl message catalogs + a switcher;
strings extracted surface-by-surface; Thai is the default for both apps.

## Metadata
- **Complexity**: Large (mechanical string extraction across ~100 components)
- **Source PRD**: `.claude/PRPs/plans/delivery-scope-roadmap.md` (PRP-4)
- **PRD Phase**: PRP-4 (P1–P2) — independent of PRP-1/2/3, can run in parallel
- **Estimated Files**: framework setup ~12 + message catalogs + per-surface edits across ~100 tsx (phased)

---

## Key decision (confirm at implement time)
| Choice | Recommendation | Why |
|---|---|---|
| Library | **next-intl** | The App-Router-native i18n standard; SSR + client components |
| Career-portal routing | **locale-prefixed, `localePrefix: 'as-needed'`, default `th`** | Public site → SEO + shareable `/en` links; `as-needed` keeps `${PORTAL_BASE_URL}/interview` & `/status` (Thai, no prefix) working so backend notification deep-links don't break |
| Dashboard routing | **cookie-based locale (no URL prefix)** | Private, already cookie-gated by middleware; avoids restructuring 13 routes |
| Default locale | **th** (both apps) | Primary audience is Thai; English is the toggle |
| AI/LLM + notification copy | **NOT translated** (stays Thai) | It's data, generated server-side per the Thai prompts; out of scope |

> If the stakeholder wants English as the dashboard default, or URL-prefix on the
> dashboard too, that flips here — everything else in the plan stays.

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `frontend/app/layout.tsx` | all | Root layout + Providers/AuthProvider wrap point (dashboard) |
| P0 | `career-portal/app/layout.tsx` | all | Root layout + metadata (portal); becomes `[locale]/layout.tsx` |
| P0 | `frontend/middleware.ts` | all | Existing auth-gate middleware — must compose with next-intl cookie locale |
| P0 | `frontend/components/shell/nav-config.tsx` | all | Nav labels (hardcoded EN) — first dashboard surface to translate + where the switcher sits |
| P1 | `career-portal/app/page.tsx`, `jobs/*`, `apply/*`, `login`, `signup`, `status`, `interview`, `account` | — | The 10 portal surfaces to translate (candidate-facing, highest value) |
| P1 | `frontend/app/(app)/**` + `components/**` | — | The 13 dashboard routes + components to translate |
| P1 | `backend/internal/notify/message.go` + `interview_message.go` | — | Confirm deep links are `${portal}/interview` `/status` (must remain valid under `as-needed`) |
| P2 | both `app/providers.tsx` | — | Where to nest `NextIntlClientProvider` |
| P2 | both `package.json` | — | Add `next-intl` |
| P2 | both `next.config.*` | — | Add the next-intl plugin |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| next-intl App Router setup | next-intl.dev/docs/getting-started/app-router | `i18n/request.ts` config, `NextIntlClientProvider`, `useTranslations` (client) + `getTranslations` (server) |
| localePrefix as-needed | next-intl.dev/docs/routing | Default locale has no path prefix → existing `/interview`,`/status` URLs resolve to Thai unchanged |
| Cookie locale (no routing) | next-intl docs | Without the routing/middleware integration, read locale from a cookie in `i18n/request.ts` — fits the dashboard |
| Composing middleware | next-intl + custom auth | Run the auth check, then return next-intl's response (dashboard keeps its `/login` gate) |

> GOTCHA: The portal PWA manifest + service worker + OAuth/LINE callback URLs must
> keep working. Callbacks are backend API routes (unaffected). The manifest
> `start_url`/`scope` should stay `/` (Thai default, no prefix). Test the installed
> PWA after the move.

---

## Patterns to Mirror
### MESSAGE_CATALOG (namespaced by surface)
```jsonc
// messages/th.json
{ "nav": { "inbox": "กล่องใบสมัคร", "candidates": "ผู้สมัคร" },
  "bulk": { "title": "อัปโหลด CV จำนวนมาก", "submit": "อัปโหลด" } }
// messages/en.json — same keys, English values
```

### CLIENT_USAGE
```tsx
"use client";
import { useTranslations } from "next-intl";
const t = useTranslations("bulk");
<h1>{t("title")}</h1>
```

### SERVER_USAGE
```tsx
import { getTranslations } from "next-intl/server";
const t = await getTranslations("nav");
```

### SWITCHER (persist locale)
```tsx
// portal: <Link href={pathname} locale="en"> / useRouter().replace with locale
// dashboard: set NEXT_LOCALE cookie + router.refresh()
```

---

## Files to Change
### Framework setup (both apps)
| File | Action |
|---|---|
| `*/package.json` | UPDATE — add `next-intl` |
| `*/next.config.ts` | UPDATE — wrap with `createNextIntlPlugin()` |
| `*/i18n/request.ts` | CREATE — locale resolution (portal: from route; dashboard: from cookie) |
| `*/messages/th.json`, `*/messages/en.json` | CREATE — message catalogs |
| `*/components/LocaleSwitcher.tsx` | CREATE — TH/EN toggle |

### Career-portal (locale-prefixed)
| File | Action |
|---|---|
| `career-portal/app/[locale]/layout.tsx` | CREATE/MOVE — wrap `NextIntlClientProvider`, set `<html lang>` from locale |
| `career-portal/app/[locale]/**` | MOVE — the 10 routes under `[locale]/` |
| `career-portal/middleware.ts` | CREATE — next-intl routing middleware (`as-needed`, default th) |
| each portal `page.tsx`/component | UPDATE — replace hardcoded Thai with `t(...)` |

### Dashboard (cookie locale)
| File | Action |
|---|---|
| `frontend/app/providers.tsx` (or layout) | UPDATE — nest `NextIntlClientProvider` (locale from cookie) |
| `frontend/middleware.ts` | UPDATE — keep auth gate; set/honor `NEXT_LOCALE` cookie |
| `frontend/components/shell/{nav-config,SideNav,MobileBar,Header}.tsx` | UPDATE — labels via `t` + mount switcher |
| each dashboard route/component | UPDATE — replace hardcoded English with `t(...)` |

## NOT Building
- No translation of **AI/LLM output** (scores' strengths/summaries, fit analysis) — generated Thai server-side.
- No translation of **candidate notifications** (LINE/email copy) — server-side, stays Thai.
- No translation of **dynamic data** (position titles, store names — already have title_th/title_en where relevant).
- No machine-translation pipeline — catalogs are human-authored.
- No RTL / third language.
- No URL-prefix on the dashboard (cookie only).

---

## Step-by-Step Tasks

### Task 1: Install + configure next-intl (career-portal)
- **ACTION**: Add `next-intl`; wrap `next.config.ts` with the plugin; add `i18n/request.ts` + `i18n/routing.ts` (`locales: ['th','en']`, `defaultLocale: 'th'`, `localePrefix: 'as-needed'`); add `messages/{th,en}.json` (start minimal).
- **MIRROR**: next-intl App Router docs.
- **GOTCHA**: `as-needed` is mandatory so `/interview` & `/status` stay Thai (deep links). Verify the manifest `start_url` stays `/`.
- **VALIDATE**: `pnpm exec next build` (portal builds with empty catalogs).

### Task 2: Portal routing + layout move
- **ACTION**: Move `app/*` routes under `app/[locale]/`; add `app/[locale]/layout.tsx` wrapping `NextIntlClientProvider` and setting `<html lang={locale}>`; add `middleware.ts` (next-intl routing).
- **GOTCHA**: Keep `manifest.webmanifest`, icons, and any root files at `app/` root (not under `[locale]`). Preserve the existing `providers.tsx` nesting.
- **VALIDATE**: portal builds; `/`, `/jobs`, `/en/jobs` all resolve; `/status`, `/interview` still work (Thai).

### Task 3: Portal switcher + string extraction (by surface)
- **ACTION**: Add `LocaleSwitcher`; extract hardcoded Thai → `t(...)` across the 10 surfaces, adding keys to both catalogs (English authored alongside). Order: landing → jobs/list → job detail → apply → login/signup → status → interview → account → offline.
- **MIRROR**: CLIENT_USAGE / SERVER_USAGE / SWITCHER.
- **GOTCHA**: `metadata` (title/description) must localize via `generateMetadata` per locale.
- **VALIDATE**: each surface renders TH + EN; no missing-key warnings.

### Task 4: Install + configure next-intl (dashboard, cookie mode)
- **ACTION**: Add `next-intl`; plugin in `next.config.ts`; `i18n/request.ts` reads locale from the `NEXT_LOCALE` cookie (default `th`); `messages/{th,en}.json`.
- **GOTCHA**: No routing integration — dashboard keeps its paths; locale comes from the cookie only.
- **VALIDATE**: dashboard builds.

### Task 5: Dashboard provider + middleware compose
- **ACTION**: Nest `NextIntlClientProvider` (locale + messages from the cookie) in `providers.tsx`/layout; update `middleware.ts` to keep the auth redirect AND default the `NEXT_LOCALE` cookie when absent.
- **GOTCHA**: The existing matcher excludes `/login` + assets — preserve it. Don't break the session redirect.
- **VALIDATE**: login gate still works; locale cookie respected.

### Task 6: Dashboard switcher + string extraction (by surface)
- **ACTION**: Add `LocaleSwitcher` to the shell header; extract hardcoded English → `t(...)` across nav + 13 routes + components. Order: nav/shell → login → inbox (applications) → application detail (incl. the PRP-1/feedback + PRP-2 bulk surfaces) → candidates → search → analytics → admin/members.
- **MIRROR**: CLIENT_USAGE / SWITCHER.
- **GOTCHA**: Some components already mix Thai (e.g. feedback panel, bulk page) — consolidate their strings into the catalogs too. Keep AI-output rendering untouched.
- **VALIDATE**: each route renders TH + EN.

### Task 7: Switcher persistence + lang attribute
- **ACTION**: Portal switcher swaps the locale segment; dashboard switcher writes `NEXT_LOCALE` + `router.refresh()`. Ensure `<html lang>` reflects the active locale in both.
- **VALIDATE**: refresh keeps the chosen language; `lang` attribute correct (a11y).

### Task 8: Tests
- **ACTION**: Unit-test the catalogs have **matching key sets** (no missing TH/EN keys) per app; a small Playwright/e2e check that toggling shows translated text on a key page (optional, if e2e infra exists).
- **IMPLEMENT**: A simple test that loads both JSON files and asserts identical key trees (catches drift).
- **VALIDATE**: `pnpm test` / `tsc`.

---

## Testing Strategy
### Automated
| Test | Expected |
|---|---|
| catalog key parity (th vs en) per app | identical key sets (no missing/extra) |
| tsc both apps | no type errors |
| next build both apps | clean |
| (optional) e2e: toggle locale on landing + inbox | text changes TH↔EN |

### Manual / Visual
- [ ] Portal: `/` and `/en` render Thai/English; switcher persists across navigation
- [ ] Portal deep links `${PORTAL_BASE_URL}/status`, `/interview#token=...` still load (Thai)
- [ ] Portal PWA still installs; manifest unaffected
- [ ] Dashboard: switcher toggles all chrome; refresh keeps locale (cookie)
- [ ] AI scores/strengths + notifications remain Thai regardless of UI locale
- [ ] `<html lang>` correct in both locales (screen-reader/a11y)
- [ ] No "missing message" console warnings on any surface

## Validation Commands
```bash
# per app
pnpm exec tsc --noEmit
pnpm exec next build
pnpm test   # catalog key-parity test
```
EXPECT: clean; key parity passes

## Acceptance Criteria
- [ ] Both apps switch TH↔EN at runtime via a visible switcher
- [ ] Portal locale in URL (`as-needed`); dashboard locale in cookie
- [ ] All UI chrome translated; catalogs key-parity clean
- [ ] Existing portal deep links + PWA unaffected
- [ ] AI/notification content unchanged (Thai)
- [ ] tsc + build green both apps

## Completion Checklist
- [ ] No hardcoded user-facing strings left on the migrated surfaces
- [ ] Default locale = th; English complete for migrated surfaces
- [ ] Switcher persists (URL segment / cookie)
- [ ] Deep links + PWA + auth gate verified
- [ ] Catalog parity test green

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Portal route move breaks deep links / PWA | Med | High | `localePrefix: 'as-needed'` (Thai = no prefix); explicit deep-link + PWA tests |
| Middleware conflict (auth + i18n) on dashboard | Med | Med | Cookie-only (no routing middleware); auth gate preserved + tested |
| Huge string surface → partial coverage | High | Med | Phase by surface (portal first); catalog-parity test flags gaps; track per-surface done |
| Translation quality / tone | Med | Low | Thai authored by team; English reviewed; keep terms consistent in catalog |
| Build-arg/env per locale (dashboard SSO) | Low | Med | Locale is cookie/runtime, independent of the Entra build-args — no deploy-arg change |

## Notes
- Sequence portal **then** dashboard — portal is candidate-facing (higher value) and
  exercises the harder routing path first.
- Catalogs are the single source of copy — future copy changes happen there, not in JSX.
- This is the last delivery-scope PRP; after it, all 7 items are covered.
```
