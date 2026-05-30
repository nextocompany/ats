# Career Portal

Public, mobile-first Career Portal for job-seekers (mostly opening it inside LINE).
Built with Next.js (App Router) + Tailwind v4 + shadcn (base-nova). Consumes the
Go backend's public Career API (`/api/v1/public/*`). Runs on **port 3001** — the HR
dashboard owns 3000.

## Flow

`/jobs` (open positions) → `/jobs/[id]` (detail + Apply) → `/jobs/[id]/apply`
(multi-step: PDPA consent → details → resume upload + mock LINE login) → returns an
opaque **status token** → `/status` (check status by token).

## Environment

```bash
cp .env.example .env.local   # NEXT_PUBLIC_API_URL=http://localhost:8080
```

| Var | Default | Purpose |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Go API base URL |
| `NEXT_PUBLIC_LIFF_ID` | _(unset)_ | set to enable real LINE LIFF auth (see `lib/line.ts`) |

## Develop

```bash
pnpm install
pnpm dev            # http://localhost:3001
```

The backend stack must be up for live data:

```bash
# from repo root
make up && make migrate-up && make seed   # seeds open positions for /public/positions
```

## LINE auth

`lib/line.ts` is the single seam. In dev it returns a stub id-token (the backend mock
verifier accepts any non-empty `X-LINE-IdToken`). For production, set `NEXT_PUBLIC_LIFF_ID`
and swap `getIdToken()` to `liff.getIDToken()` — no caller changes needed.

## Validate

```bash
pnpm lint
pnpm exec tsc --noEmit
pnpm build
```

## E2E (Playwright, mobile viewports)

```bash
pnpm exec playwright install chromium
# with the stack + portal up:
pnpm dev &                         # or: pnpm build && pnpm start
pnpm exec playwright test          # mobile-320 / mobile-375 / tablet-768
```

Screenshots are written to `e2e/__screens__/` (gitignored). The apply flow test
skips gracefully if no open positions are seeded.

## PWA (Sprint 6c)

The portal is an installable, offline-capable PWA via **Serwist** (`@serwist/next`):

- `app/manifest.ts` → `/manifest.webmanifest` (name, icons, `theme_color`, `display: standalone`, `start_url: /jobs`).
- `app/sw.ts` → compiled to `public/sw.js` (gitignored). Precaches the app shell,
  `NetworkFirst` for `GET /api/v1/public/*` (positions/detail/status — the apply
  POST is never cached), and a document fallback to `/offline`.
- `public/` holds the brand icons (`icon-192`, `icon-512`, `icon-maskable-512`,
  `apple-touch-icon`, `favicon.ico`) — committed PNGs.
- `components/InstallPrompt.tsx` offers a dismissible "เพิ่มลงหน้าจอหลัก" affordance
  (Android/desktop Chrome; iOS degrades to Safari "Add to Home Screen").

The service worker is **disabled in `next dev`** (so HMR isn't cached). To exercise
the SW, manifest, and offline behavior, run a production build:

```bash
pnpm build && pnpm start            # http://localhost:3001
curl -s http://localhost:3001/manifest.webmanifest | python3 -m json.tool
pnpm exec playwright test pwa.spec.ts   # SW assertion only runs against prod
```

### Lighthouse (PWA category)

```bash
pnpm build && pnpm start &
pnpm dlx lighthouse http://localhost:3001/jobs \
  --only-categories=pwa --preset=desktop --view
```

Installability needs: a valid manifest, a registered service worker, a 200 for
`start_url` offline, maskable icons, and a theme/viewport — all wired above.
Regenerate the icons (white "N" on brand green) only if the brand mark changes.

### CSP note (Sprint 6a / 6c)

The header CSP (`next.config.ts`) allows the PWA via `worker-src 'self'` +
`manifest-src 'self'`. In Sprint 6c the `script-src` was relaxed to
`'self' 'unsafe-inline'`: a strict `script-src 'self'` blocks Next's own inline
hydration/streaming scripts, so production builds render but never hydrate. A
per-request nonce can't fix this for statically-prerendered pages (Next emits
`nonce=undefined` for them), so `'unsafe-inline'` is used — Next's inline scripts
are framework-generated and same-origin. Tightening this further (hashes, or
nonce + forced dynamic rendering) is a possible future hardening.
