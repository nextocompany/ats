# Plan: Sprint 6a — Security Hardening

## Summary
Production-grade security hardening across the stack: **security headers + CSP** on both Next.js apps and the Go/Fiber API, **rate limiting** on the public + auth surfaces, a **real Azure AD (Entra ID) auth seam** behind config (mock-default, replacing the dev-only `MockJWT`), and a **secret/dependency audit** (gosec, govulncheck, pnpm audit) plus a PDPA data-handling review. No behavioural change in dev (mocks stay default); the seams light up in production.

## User Story
As the **platform owner shipping to production**, I want **the app hardened against common web attacks and wired for real SSO**, so that **candidate PII and the HR console are protected and we pass a pre-go-live security review**.

## Problem → Solution
**Current state:** No security headers anywhere; no CSP; no rate limiting; auth is `middleware.MockJWT` (injects a fixed `super_admin` only when `ENV=development`, a no-op otherwise — so non-dev has **no** auth); a `.env` with dev creds is committed; no security scanning in CI.
**Desired state:** Helmet-style headers + CSP on API and both web apps; per-IP rate limits on `/api/v1/public/*` and auth; a config-gated **Azure AD JWT verifier** that populates the same `c.Locals(UserContextKey)` the handlers already read (mock dev user stays the default); `gosec`/`govulncheck`/`pnpm audit` in CI; `.env` removed from the repo.

## Metadata
- **Complexity**: Large (cross-cutting: API middleware + auth seam + 2 Next configs + CI; ~16 files)
- **Source PRD**: Nexto PRP v1.0 — Sprint 6 (security); roadmap §20 (S6–7)
- **Decisions locked**: Azure AD verifier is **mock-default-behind-config** (mirrors peoplesoft/ai/line/notify); the auth middleware keeps the `UserContextKey`→`DevUser` contract so **no handler changes**; headers via Fiber `helmet` + explicit CSP; rate limit via Fiber `limiter`
- **Estimated Files**: ~16
- **Dependents**: none (independent of 6b/6c)

---

## UX Design
Internal/security change — no user-facing UX. (One operator-visible effect: in production, requests without a valid Entra token get 401.)

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| API responses | no security headers | HSTS, X-Frame-Options, nosniff, Referrer-Policy, Permissions-Policy, CSP | Fiber helmet + CSP |
| Web apps | no headers/CSP | same header set via Next `headers()` | both :3000 + :3001 |
| Public apply/status | unlimited | per-IP rate limit (429 on abuse) | Fiber limiter |
| Auth (prod) | none (MockJWT no-op) | Entra JWT validated → `DevUser`-shaped locals | mock dev user still default |
| Secrets | `.env` committed | removed; `.env.example` only | rotate JWT_SECRET |

---

## Mandatory Reading (the contract + patterns to reuse)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/cmd/api/main.go` | 108-123 | `fiber.New` + `app.Use(cors…, RequestLogger, MockJWT)` — exact middleware block + order to extend |
| P0 | `backend/internal/middleware/mock_jwt.go` | all | `UserContextKey`, `DevUser`, `MockJWT(enabled)` — the contract the new auth middleware MUST preserve |
| P0 | `backend/internal/auth/line.go` | 24-45 | **THE seam pattern**: `Verifier` iface + `NewVerifier(cfg)` mock-default factory — mirror for Azure AD |
| P0 | `backend/pkg/config/config.go` | 49-53, 75-126, 162-167 | mock-default config fields, `getenv`, `ProviderReal`, `UsesReal*()` predicates |
| P0 | `backend/internal/public/routes.go` | 1-12 | the public endpoints to rate-limit |
| P1 | `backend/internal/middleware/logging.go` | all | existing middleware shape (`RequestLogger`, sets `X-Request-ID`) to mirror for a headers middleware |
| P1 | `backend/internal/rbac/scope.go` | 16-25 | `rbac.New(role, storeID, subregion)` — the fields the Entra token must map to (roles/groups → role, store/subregion claims) |
| P1 | `frontend/next.config.ts` + `career-portal/next.config.ts` | all | empty configs — add `async headers()` |
| P1 | `frontend/middleware.ts` | all | existing Next middleware (session gate) — CSP nonce option lives here if pursued |
| P1 | `~/.claude/rules/ecc/web/security.md` | all | the exact header + CSP block to apply |
| P2 | `.github/workflows/ci.yml` | all | Go CI to extend with gosec/govulncheck; add pnpm audit |
| P2 | `.gitignore` + `.env` | — | `.env` is committed despite being ignored — remove from tracking |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Fiber helmet | docs.gofiber.io/api/middleware/helmet | ships with fiber v2; sets X-Frame-Options/X-Content-Type-Options/Referrer-Policy/HSTS/Permissions-Policy + `ContentSecurityPolicy` field |
| Fiber limiter | docs.gofiber.io/api/middleware/limiter | `limiter.New(limiter.Config{Max, Expiration, KeyGenerator, LimitReached})`; in-memory store (fine for single instance; note Redis store for multi) |
| Azure AD / Entra ID token validation | learn.microsoft.com/entra/identity-platform/access-tokens | validate JWT against tenant OIDC discovery + JWKS; check `aud`, `iss`, `exp`; roles via `roles`/`groups` claim. Use `github.com/coreos/go-oidc/v3/oidc` (pulls existing `x/oauth2`). |
| Next headers | nextjs.org/docs/app/api-reference/config/next-config-js/headers | `async headers()` returns per-path header arrays; applies to all routes |
| govulncheck | go.dev/blog/govulncheck | `go install golang.org/x/vuln/cmd/govulncheck@latest; govulncheck ./...` — reports known CVEs in deps |

### Research Notes
```
KEY_INSIGHT: MockJWT is a no-op outside dev → production currently has NO auth.
APPLIES_TO: auth middleware.
GOTCHA: do NOT remove MockJWT. Add a sibling real verifier; the middleware chooses by config. Both must set c.Locals(UserContextKey, middleware.DevUser{...}) so the 6 handlers that read it (search/applications/profiles/reengage/reports/users) need ZERO changes. The Entra token claims map to DevUser{ID,Email,Role,StoreID,Subregion}.

KEY_INSIGHT: .env is tracked despite being in .gitignore.
APPLIES_TO: secret hygiene.
GOTCHA: `git rm --cached .env` (keep the local file), confirm only .env.example is tracked, and document JWT_SECRET rotation. Do not print secret values.

KEY_INSIGHT: Fiber v2.52.13 already vendors helmet + limiter.
APPLIES_TO: headers + rate limit.
GOTCHA: no new Go dep for those two — just import gofiber/fiber/v2/middleware/{helmet,limiter}. Azure AD verification DOES need a new dep (go-oidc/v3).

KEY_INSIGHT: CSP on Next without nonces forces 'unsafe-inline' for styles.
APPLIES_TO: web CSP.
GOTCHA: header-based CSP (in next.config headers()) cannot use per-request nonces. Allow 'self' + the API origin in connect-src + 'unsafe-inline' for style-src only (documented). Nonce-based CSP via middleware is a noted future enhancement, not this sprint.
```

---

## Patterns to Mirror

### AUTH_SEAM (mirror auth/line.go:29-45 + mock_jwt.go)
```go
// internal/middleware/auth.go (new) — config-gated; preserves the DevUser locals contract.
func Auth(cfg *config.Config) fiber.Handler {
	if cfg.UsesRealAuth() {
		v := newEntraVerifier(cfg) // validates Entra JWT (OIDC discovery + JWKS)
		return func(c *fiber.Ctx) error {
			claims, err := v.Verify(c.UserContext(), bearer(c))
			if err != nil {
				return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
			}
			c.Locals(UserContextKey, DevUser{ID: claims.OID, Email: claims.Email, Role: claims.Role, StoreID: claims.StoreID, Subregion: claims.Subregion})
			return c.Next()
		}
	}
	return MockJWT(cfg.IsDevelopment()) // unchanged dev path
}
```

### CONFIG_PREDICATE (mirror config.go:162)
```go
func (c *Config) UsesRealAuth() bool { return c.AuthProvider == ProviderReal }
```

### HEADERS_MIDDLEWARE (Fiber helmet + CSP)
```go
app.Use(helmet.New(helmet.Config{
	XFrameOptions:         "DENY",
	ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
	ReferrerPolicy:        "strict-origin-when-cross-origin",
	HSTSMaxAge:            31536000,
	HSTSIncludeSubdomains: true,
	PermissionPolicy:      "camera=(), microphone=(), geolocation=()",
}))
```

### RATE_LIMIT (Fiber limiter on the public group)
```go
publicLimiter := limiter.New(limiter.Config{
	Max: 30, Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
	LimitReached: func(c *fiber.Ctx) error { return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded") },
})
// apply to the public group only (apply/status are the abuse surface)
```

### NEXT_HEADERS (both next.config.ts)
```ts
const securityHeaders = [
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
  { key: "Content-Security-Policy", value: CSP }, // 'self' + NEXT_PUBLIC_API_URL in connect-src
];
const nextConfig: NextConfig = { async headers() { return [{ source: "/(.*)", headers: securityHeaders }]; } };
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/pkg/config/config.go` | UPDATE | `AUTH_PROVIDER` + `AZURE_AD_TENANT_ID`/`AZURE_AD_CLIENT_ID`(+audience) + `UsesRealAuth()` + validation |
| `backend/internal/auth/entra.go` | CREATE | Entra verifier: OIDC discovery + JWKS, validates aud/iss/exp, maps claims→role/store/subregion |
| `backend/internal/auth/entra_test.go` | CREATE | claim-mapping + (table) validation unit tests (no live Entra) |
| `backend/internal/middleware/auth.go` | CREATE | `Auth(cfg)` middleware — real verifier or MockJWT, preserving the `DevUser` locals contract |
| `backend/internal/middleware/auth_test.go` | CREATE | dev path injects super_admin; real path 401 on bad/missing token (httptest) |
| `backend/internal/middleware/security_headers.go` | CREATE (or inline helmet) | helmet config wrapper if cleaner than inline |
| `backend/cmd/api/main.go` | UPDATE | add helmet + public limiter; swap `MockJWT(...)` → `middleware.Auth(cfg)` |
| `backend/internal/public/routes.go` | UPDATE | accept + apply the rate limiter to the public group |
| `backend/go.mod`/`go.sum` | UPDATE | add `github.com/coreos/go-oidc/v3` |
| `frontend/next.config.ts` | UPDATE | `async headers()` security headers + CSP |
| `career-portal/next.config.ts` | UPDATE | same header set (CSP connect-src includes the API origin) |
| `.github/workflows/ci.yml` | UPDATE | add `gosec`, `govulncheck`, and `pnpm audit` (frontend + career-portal) steps |
| `Makefile` | UPDATE | `security` target: gosec + govulncheck |
| `.env` | DELETE (untrack) | `git rm --cached .env`; keep `.env.example` |
| `backend/pkg/config/config.go` (validation) | UPDATE | warn/fail if `JWT_SECRET` empty when `ENV!=development` |
| `docs/SECURITY.md` | CREATE | PDPA data-handling review + header/CSP rationale + secret-rotation runbook |

## NOT Building (later / out of scope)
- **Nonce-based CSP** via Next middleware (header-based CSP this sprint; nonce is a documented follow-up).
- **Real Entra app registration / tenant setup** — seam + validation only; mock stays default.
- **Redis-backed distributed rate limiter** (in-memory limiter is fine for the current single-API-instance dev/stage; noted for multi-instance prod).
- **WAF / DDoS / bot management** (infra/CDN concern).
- **Field-level encryption / new PDPA tooling** — 6a reviews + documents handling; the consent capture already exists (5a/F13).
- **Frontend login replacement** (the dashboard session-cookie dev login stays; real SSO wiring is deploy-time once the API verifier is live).

---

## Step-by-Step Tasks

### Task 1: Config — auth provider + Entra settings
- **ACTION**: Add `AuthProvider` (`getenv("AUTH_PROVIDER","mock")`), `AzureADTenantID`, `AzureADClientID` (audience); `UsesRealAuth()`; validate tenant+client present when real; warn when `JWT_SECRET==""` && `!IsDevelopment()`.
- **MIRROR**: `config.go:49-53,162` (LINE seam fields + predicate).
- **VALIDATE**: `go build ./pkg/config`; default → `UsesRealAuth()==false`.

### Task 2: Entra verifier
- **ACTION**: `internal/auth/entra.go` — `newEntraVerifier(cfg)` builds an `oidc.Provider` from `https://login.microsoftonline.com/<tenant>/v2.0`, verifies the bearer ID/access token (aud=client id, iss, exp), extracts claims → `{OID,Email,Role,StoreID,Subregion}`. Role from `roles` claim (fallback group map); store/subregion from custom claims.
- **MIRROR**: `auth/line.go` real verifier shape; `ai/azure_parser.go` for the http client style.
- **GOTCHA**: construct the OIDC provider only inside `UsesRealAuth()` so no network/JWKS fetch in dev/CI. Claims map must produce a valid `rbac` role string (super_admin/regional_director/operation_director/sgm/hr_manager/hr_staff/auditor).
- **VALIDATE**: `go test ./internal/auth` — claim→DevUser mapping table (parse a hand-crafted claims struct; no live IdP).

### Task 3: Auth middleware (seam)
- **ACTION**: `internal/middleware/auth.go` — `Auth(cfg)` returns the real-verifier handler (401 on missing/invalid bearer) when `UsesRealAuth()`, else `MockJWT(cfg.IsDevelopment())`. Both set `c.Locals(UserContextKey, DevUser{...})`.
- **MIRROR**: AUTH_SEAM above; `mock_jwt.go` contract.
- **GOTCHA**: keep `MockJWT` exported/unchanged (referenced in tests). Bearer parse from `Authorization: Bearer <jwt>`.
- **VALIDATE**: `go test ./internal/middleware` — dev path → super_admin locals; real path with no token → 401 (httptest fiber app).

### Task 4: Wire middleware + headers + rate limit in api main
- **ACTION**: In `cmd/api/main.go`: add `app.Use(helmet.New(...))` (after CORS), build a public limiter, replace `app.Use(middleware.MockJWT(cfg.IsDevelopment()))` with `app.Use(middleware.Auth(cfg))`. Pass the limiter into `public.RegisterRoutes` and apply it to the public group.
- **MIRROR**: HEADERS_MIDDLEWARE, RATE_LIMIT; existing `app.Use` order (CORS → headers → logger → auth).
- **GOTCHA**: `/health` must stay unauthenticated and unthrottled (registered before auth, or skip in Auth for `/health`). Keep CORS first so preflight still works.
- **VALIDATE**: `go build ./cmd/api`; live: response carries `X-Frame-Options: DENY` etc.; hammering `/api/v1/public/positions` >30/min → 429; `/health` still 200.

### Task 5: Next.js security headers (both apps)
- **ACTION**: Add `async headers()` to `frontend/next.config.ts` and `career-portal/next.config.ts` with the shared header set; CSP `connect-src 'self' <NEXT_PUBLIC_API_URL>`, `style-src 'self' 'unsafe-inline'`, `img-src 'self' data:`, `frame-ancestors 'none'`.
- **MIRROR**: NEXT_HEADERS; `~/.claude/rules/ecc/web/security.md` block.
- **GOTCHA**: career-portal CSP must allow the API origin (`http://localhost:8080` dev) in `connect-src` or fetches break; read it from `process.env.NEXT_PUBLIC_API_URL` at config eval.
- **VALIDATE**: `pnpm build` both; `curl -I` a served page shows the headers; portal apply still reaches the API (no CSP block).

### Task 6: Secret hygiene + CI security scans
- **ACTION**: `git rm --cached .env` (keep local), verify only `.env.example` tracked. Add CI steps: `gosec ./...`, `govulncheck ./...` (backend), `pnpm audit --audit-level=high` (frontend + career-portal). Add `make security`. Write `docs/SECURITY.md` (PDPA review + headers/CSP rationale + JWT_SECRET rotation runbook).
- **MIRROR**: `.github/workflows/ci.yml` existing Go steps; `Makefile` `lint`/`vet` target style.
- **GOTCHA**: pin tool versions; `pnpm audit` can be advisory (don't fail the build on transitive low/moderate — gate at `high`). `gosec` may flag the dev `.env`/test fixtures — scope to `./...` excluding generated.
- **VALIDATE**: `make security` runs clean (or with triaged, documented exceptions); CI workflow YAML valid; `git ls-files | grep -c '^\.env$'` == 0.

---

## Testing Strategy
### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Entra claim mapping | claims w/ roles+store+subregion | correct DevUser | — |
| Entra invalid aud/exp | bad claims | error | yes |
| Auth dev path | ENV=development | super_admin locals | — |
| Auth real path, no bearer | missing header | 401 | yes |
| config validation | AUTH_PROVIDER=real, no tenant | Load error | yes |

### Edge Cases Checklist
- [ ] `/health` reachable without auth + not rate-limited
- [ ] CORS preflight (OPTIONS) still succeeds with headers added
- [ ] Public limiter returns 429 (not 500) past the cap
- [ ] Real auth: expired/wrong-audience token → 401
- [ ] Portal/dashboard fetches not blocked by CSP `connect-src`
- [ ] Missing `JWT_SECRET` in prod → startup warning/fail

## Validation Commands
### Static + unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
### Security scans
```bash
cd backend && gosec ./... && govulncheck ./...
cd frontend && pnpm audit --audit-level=high
cd career-portal && pnpm audit --audit-level=high
```
### Build
```bash
cd backend && go build ./cmd/api ./cmd/worker ./cmd/scheduler
cd frontend && pnpm build && cd ../career-portal && pnpm build
```
### Live header/limit check (stack up)
```bash
make up && make migrate-up && make seed
curl -sI http://localhost:8080/api/v1/public/positions | grep -iE "x-frame-options|content-security|referrer|permissions-policy"
for i in $(seq 1 40); do curl -s -o /dev/null -w "%{http_code} " http://localhost:8080/api/v1/public/positions; done   # expect 429s after ~30
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health   # 200, not throttled
```

## Acceptance Criteria
- [ ] Security headers (X-Frame-Options/nosniff/Referrer-Policy/Permissions-Policy/HSTS/CSP) on API + both web apps.
- [ ] Per-IP rate limit on `/api/v1/public/*` (429 past cap); `/health` exempt.
- [ ] `middleware.Auth(cfg)` seam: mock dev user default; Entra JWT validated when `AUTH_PROVIDER=real`; **no handler changes**.
- [ ] `.env` untracked; only `.env.example` committed; `docs/SECURITY.md` (PDPA + rotation) added.
- [ ] CI runs gosec + govulncheck + pnpm audit; vet/lint/`go test -race`/builds all pass.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| CSP breaks portal/dashboard fetches | Med | High | include API origin in connect-src; live-verify apply flow; staged rollout |
| Auth seam diverges from real Entra | Med | High | strict aud/iss/exp checks; mock mirrors the DevUser shape; claim-map tests |
| Rate limit blocks legit bursts | Low | Med | generous cap (30/min/IP), 429 not 500, tune later |
| `.env` already leaked in history | Med | High | untrack now + document rotation of all dev creds; real secrets were never prod |
| New OIDC dep adds CVEs | Low | Med | govulncheck gates it; pin version |

## Notes
- The auth middleware preserving the `DevUser`/`UserContextKey` contract is the linchpin — it means S0–S5 handlers (and the existing role gates in reengage/reports/search) keep working untouched.
- Real frontend SSO (acquiring the Entra token in the browser) is deploy-time wiring once the API verifier is live; the dashboard's dev session cookie stays for local use.
