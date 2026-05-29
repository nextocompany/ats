# Security (Sprint 6a)

## Headers & CSP
- **API (Go/Fiber)**: `helmet` middleware sets `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: strict-origin-when-cross-origin`, `Strict-Transport-Security` (1y, includeSubDomains),
  `Permissions-Policy: camera=(), microphone=(), geolocation=()`, and a baseline CSP (`default-src 'self'`).
- **Web apps (dashboard :3000, career-portal :3001)**: same header set via Next `headers()` in each
  `next.config.ts`. CSP `connect-src` includes `NEXT_PUBLIC_API_URL` (the Go API) — required or fetches break.
  `style-src` allows `'unsafe-inline'` (no per-request nonce without middleware — **follow-up**: nonce-based CSP).
  Portal CSP also allows `worker-src`/`manifest-src 'self'` for the Sprint 6c PWA.

## Authentication
- HR API auth is a **mock-default seam** (`middleware.Auth`): dev injects a fixed `super_admin` (`MockJWT`);
  `AUTH_PROVIDER=real` validates an **Azure AD (Entra) JWT** (OIDC discovery + JWKS, checks `aud`/`iss`/`exp`)
  and maps claims → the same `DevUser` locals every handler reads (no handler changes).
- Auth gates the **HR console only**. Bypassed paths: `/health`, `/api/v1/public/*` (LINE-authed candidate API),
  `/api/v1/ps/*` (PeopleSoft machine webhooks).
- **Follow-up**: PeopleSoft webhooks (`/api/v1/ps/*`) are currently unauthenticated machine endpoints — add an
  HMAC/shared-secret check before production exposure.

## Rate limiting
- Per-IP limiter on `/api/v1/public/*` (apply/positions/status) — the public abuse surface — at
  `publicRateMax` req/min (in-memory). **Follow-up**: Redis-backed limiter for multi-instance deployments.

## Secrets
- No secrets are committed; only `.env.example` is tracked (`.env` is gitignored and untracked).
- Required at startup: `DB_URL`, `REDIS_URL`, `AZURE_BLOB_CONNECTION_STRING`. `JWT_SECRET` should be set in
  non-dev (a warning is logged when empty outside `ENV=development`).
- Integration secrets (Azure AI/Search, PeopleSoft, LINE, Notify, Entra) are required only when their
  provider is `real`/`azure`; everything defaults to mock.
- **Rotation runbook**: rotate `JWT_SECRET` and any real integration credentials at the secret manager,
  redeploy, and invalidate old values. Dev `.env` values were never production secrets.

## Dependency / SAST scanning
- CI `security` job: `govulncheck ./...`, `gosec -exclude-generated ./...`, and `pnpm audit --audit-level=high`
  for both web apps. Locally: `make security`.

## PDPA data-handling review
- Candidate consent is captured + recorded on public apply (F13, Sprint 5a `pdpa_consents`).
- PII (name/phone/email/id_card, resume blobs) is stored in Postgres + Blob; resume access is via short-lived
  signed URLs (15 min). Status is exposed only via an opaque random token (no enumerable IDs).
- Re-engagement suppresses repeat contact; report exports are delivered via short-lived signed links.
- **Retention** (documented to candidates): ≤ 1 year, then deletion/anonymisation — operationalising the
  retention sweep is a future task.
