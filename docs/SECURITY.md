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
  `/api/v1/ps/*` (PeopleSoft machine webhooks — authenticated separately by HMAC, below).
- **PeopleSoft webhook auth (Sprint 7)**: the state-changing PS POSTs (`/api/v1/ps/vacancy-opened`,
  `/vacancy-closed`, `/sync-hired`) require `X-PS-Signature: <hex HMAC-SHA256(PS_WEBHOOK_SECRET, raw-body)>`,
  verified constant-time (`hmac.Equal`); a missing/invalid signature returns 401. `GET /api/v1/ps/health` stays
  open as a probe. Gated by the mock-default seam: enforced when `PS_WEBHOOK_SECRET` is set (mandatory and
  fail-fast-validated when `PS_PROVIDER=real`); dev/CI (`mock`, no secret) leave the group open so tests stay green.
- **Follow-up (optional hardening)**: replay protection (timestamp tolerance / nonce) on the PS webhooks — the
  current HMAC + TLS posture does not detect a replayed, validly-signed request.

## Rate limiting
- Per-IP limiter on `/api/v1/public/*` (apply/positions/status) — the public abuse surface — at
  `RATE_LIMIT_PUBLIC_MAX` req/min (default 30). **Redis-backed** (Sprint 7), so the window is shared
  across api replicas instead of counted per process; keys live under `ratelimit:*`. The limiter **fails
  open** on a Redis outage (availability over strict limiting for a public endpoint) and never touches
  non-rate-limit keys (`Reset` is scoped to `ratelimit:*`, never `FLUSHDB`).
- **Trusted-proxy client IP (Sprint 8)**: the Fiber app runs with `EnableTrustedProxyCheck` +
  `ProxyHeader: X-Forwarded-For` and a `TRUSTED_PROXIES` allowlist (IPs/CIDRs). When the request comes from
  a trusted proxy (set `TRUSTED_PROXIES` to the prod ingress/LB CIDR), `c.IP()` is the real client; an empty
  allowlist (dev/CI) trusts no proxy and uses the direct peer. `X-Forwarded-For` is **never** honoured from a
  non-allowlisted source (it is client-spoofable), so an attacker cannot mint a fresh rate-limit bucket per
  request.

## Secrets
- No secrets are committed; only `.env.example` is tracked (`.env` is gitignored and untracked).
- Required at startup: `DB_URL`, `REDIS_URL`, `AZURE_BLOB_CONNECTION_STRING`.
- **Prod fail-fast guards (Sprint 8)**: when `ENV != development`, startup **fails** if `JWT_SECRET` is empty
  or `CORS_ALLOW_ORIGINS` still contains `localhost`/`127.0.0.1`. Provider flags are value-validated always
  (`AI_PROVIDER`/`AI_SEARCH_PROVIDER` ∈ `mock|azure`; `AUTH_PROVIDER`/`PS_PROVIDER`/`LINE_PROVIDER`/
  `NOTIFY_PROVIDER` ∈ `mock|real`) so a typo (e.g. `AI_PROVIDER=real`) fails fast instead of silently
  falling back to mock.
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
- **Retention**: candidate PII (name/phone/email/id_card/address/DOB, resume blobs) is anonymized in place
  ≤ 1 year after intake by a daily scheduled sweep (`retention:sweep`, Sprint 7). Rows are de-identified,
  not deleted, to preserve referential integrity + aggregate analytics; resume blobs are removed from
  storage and consent-ledger IPs nulled. The sweep is gated behind `RETENTION_SWEEP_ENABLED` (off by
  default) and skips candidates still in an active pipeline (`pending`/`parsed`/`scored`) as well as
  hired candidates (`hired`), whose records are retained in the ATS beyond the window for HR/PeopleSoft
  reconciliation. Each anonymization writes a `retention_anonymize` audit log entry.
