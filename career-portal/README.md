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
