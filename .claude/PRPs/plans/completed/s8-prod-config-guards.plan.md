# Plan: Production Config Guards — Secrets Fail-Fast + Trusted-Proxy (S8 slice 1.4 + 1.7)

## Summary
Two small, surgical backend changes that harden the app for production without touching any feature logic: (1) **fail-fast** at startup on the production misconfigurations the audit flagged — empty `JWT_SECRET` outside dev, a localhost `CORS_ALLOW_ORIGINS` outside dev, and invalid provider-flag values (e.g. `AI_PROVIDER=real`, which silently falls back to mock); and (2) wire Fiber's **trusted-proxy** support so the public rate limiter keys on the real client IP behind the Azure Container Apps ingress instead of the ingress IP — resolving the HIGH follow-up from PR #18.

## User Story
As a **platform operator deploying to Azure Container Apps**, I want **the app to refuse to start on dangerous prod misconfigurations and to read the real client IP behind the ingress**, so that **a localhost CORS list, an empty JWT secret, a typo'd provider flag, or proxy-masked IPs can't silently ship to production**.

## Problem → Solution
**Current state:**
- `JWT_SECRET` empty outside dev only **logs a warning** (`config.go:198-200`) — prod can boot without it.
- `CORS_ALLOW_ORIGINS` defaults to `http://localhost:3000,http://localhost:3001` (`config.go:158`) with no prod guard — a forgotten override silently blocks the real frontends.
- Provider flags aren't value-checked: `AI_PROVIDER=real` (should be `azure`) or `AUTH_PROVIDER=azure` (should be `real`) **silently fall back to mock** — a dangerous, invisible misconfig.
- The rate limiter keys on `c.IP()` = the direct TCP peer (`cmd/api/main.go`), which behind a proxy/LB is the **ingress IP** → all clients share one bucket (#18 HIGH follow-up).

**Desired state:** `config.Load()` returns an error on these prod misconfigs (dev/CI unaffected by defaults), and Fiber is configured with `EnableTrustedProxyCheck` + a config-driven `TRUSTED_PROXIES` allowlist + `ProxyHeader: X-Forwarded-For`, so `c.IP()` is the real client when (and only when) the request comes from a trusted proxy.

## Metadata
- **Complexity**: Small-Medium
- **Source**: `.claude/PRPs/plans/sprint-8-go-live-roadmap.md` slices 1.4 (secrets/CORS/provider fail-fast) + 1.7 (trusted-proxy). Resolves #18 review HIGH.
- **Estimated Files**: 5 (0 new, 5 updated)

---

## UX Design
N/A — backend/infra change. No product UX. Operator-facing: clearer startup failures; correct per-client rate limiting in prod.

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Startup, prod, empty JWT | warn, boots | **fail-fast** error | dev unaffected |
| Startup, prod, localhost CORS | boots | **fail-fast** error | dev unaffected |
| Startup, `AI_PROVIDER=real` (typo) | silently mock | **fail-fast** error | always (defaults valid) |
| `c.IP()` behind ingress | ingress IP | real client IP (when `TRUSTED_PROXIES` set) | dev/CI: empty → direct peer (unchanged) |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/pkg/config/config.go` | 96-100 (provider consts), 156-201 (validation block + JWT warn + CORS default), `ReportRecipientList` splitter | Where every fail-fast lives; the warn to convert; the splitter to mirror for `TrustedProxyList` |
| P0 | `backend/cmd/api/main.go` | the `fiber.New(fiber.Config{...})` block + the `cors.New` block | Where to add the 3 proxy fields; CORS reads `cfg.CORSAllowOrigins` |
| P0 | `<fiber>/app.go` | 231-237 (`ProxyHeader`), 348-367 (`EnableTrustedProxyCheck`/`TrustedProxies` docs), 592-602 (CIDR via `net.ParseCIDR`) | Confirms field names, CIDR support, and the empty-list = "trust nobody" behaviour |
| P1 | `backend/pkg/config/config_test.go` | full | `t.Setenv` + AAA test idiom to mirror; existing provider/JWT tests |
| P1 | `.claude/PRPs/reviews/redis-rate-limiter-review.md` | HIGH finding | The exact trusted-proxy requirement + the "don't trust XFF without an allowlist (spoofable)" caveat |
| P2 | `docs/SECURITY.md` | Rate limiting section (the trusted-proxy follow-up note) | To flip the follow-up → implemented |

## External Documentation

```
KEY_INSIGHT: Fiber — EnableTrustedProxyCheck:true + empty TrustedProxies ⇒ trust NO proxy ⇒ c.IP() = direct peer (XFF ignored). (app.go:348-357, IsProxyTrusted)
APPLIES_TO: dev/CI default (TRUSTED_PROXIES="") preserves today's behaviour; no test churn.
GOTCHA: setting ProxyHeader WITHOUT EnableTrustedProxyCheck+allowlist trusts XFF from anyone (spoofable bypass). Always pair them.

KEY_INSIGHT: TrustedProxies accepts exact IPs AND CIDR ranges (app.go:595 net.ParseCIDR).
APPLIES_TO: prod value = the ACA ingress CIDR (determined during Phase 1 infra; injected via env, not hardcoded).
```

---

## Patterns to Mirror

### FAIL_FAST (existing)
```go
// SOURCE: backend/pkg/config/config.go:166-168
if c.BlobConnString == "" {
    return nil, fmt.Errorf("config: AZURE_BLOB_CONNECTION_STRING is required")
}
```

### LIST_SPLITTER (existing)
```go
// SOURCE: backend/pkg/config/config.go ReportRecipientList
func (c *Config) ReportRecipientList() []string {
    var out []string
    for _, r := range strings.Split(c.ReportRecipients, ",") {
        if t := strings.TrimSpace(r); t != "" { out = append(out, t) }
    }
    return out
}
```

### CONFIG_TEST (existing)
```go
// SOURCE: backend/pkg/config/config_test.go
t.Setenv("DB_URL", "postgres://localhost/db")
t.Setenv("REDIS_URL", "redis://localhost:6379")
t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
// ... Act: Load() ... Assert
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/pkg/config/config.go` | UPDATE | `TrustedProxies` field + `TRUSTED_PROXIES` default + `TrustedProxyList()`; provider-value validation; JWT fail-fast; CORS localhost guard |
| `backend/cmd/api/main.go` | UPDATE | `EnableTrustedProxyCheck` + `TrustedProxies` + `ProxyHeader` in `fiber.New` |
| `backend/pkg/config/config_test.go` | UPDATE | tests for each new guard + splitter |
| `.env.example` (repo ROOT) | UPDATE | document `TRUSTED_PROXIES` + a prod-config note |
| `docs/SECURITY.md` | UPDATE | trusted-proxy follow-up → implemented; note the new prod fail-fast guards |

## NOT Building

- **Blob Azurite guard** — that's roadmap slice 2.2; keep this slice to JWT/CORS/provider + proxy.
- **Secret-manager (Key Vault) integration** — slice 1.4's Key Vault wiring is ACA-infra work; this slice only hardens the in-process validation.
- **Hardcoding the ACA ingress CIDR** — it's env-injected (`TRUSTED_PROXIES`); the value is determined during Phase 1 infra setup.
- **Changing the rate-limiter logic** (`pkg/ratelimit`) — unchanged; only the Fiber app's IP resolution changes.
- **CORS guard in dev/CI** — only enforced when `!IsDevelopment()`.

---

## Step-by-Step Tasks

### Task 1: Config — `TrustedProxies` field + `TrustedProxyList()`
- **ACTION**: Edit `backend/pkg/config/config.go`.
- **IMPLEMENT**:
  - Struct (near `CORSAllowOrigins`):
    ```go
    // TrustedProxies is the comma-separated allowlist of proxy IPs/CIDRs (e.g. the
    // ACA ingress range) whose X-Forwarded-For is trusted for client-IP resolution.
    // Empty (dev/CI) ⇒ no proxy trusted ⇒ c.IP() is the direct peer.
    TrustedProxies string
    ```
  - In `Load` struct literal (near `CORSAllowOrigins`):
    ```go
    TrustedProxies: os.Getenv("TRUSTED_PROXIES"),
    ```
  - Add splitter (mirror `ReportRecipientList`):
    ```go
    // TrustedProxyList splits TRUSTED_PROXIES into trimmed, non-empty entries.
    func (c *Config) TrustedProxyList() []string {
        var out []string
        for _, p := range strings.Split(c.TrustedProxies, ",") {
            if t := strings.TrimSpace(p); t != "" { out = append(out, t) }
        }
        return out
    }
    ```
- **MIRROR**: LIST_SPLITTER.
- **GOTCHA**: default empty (no env) → `TrustedProxyList()` returns `nil`/empty → Fiber trusts no proxy → direct peer (current behaviour). No fail-fast on this var.
- **VALIDATE**: `go build ./pkg/config/...`.

### Task 2: Config — provider-value validation (always)
- **ACTION**: Add a value check for all 6 provider flags in `Load`, before the existing conditional blocks.
- **IMPLEMENT**:
  ```go
  // Catch typo'd provider flags (e.g. AI_PROVIDER=real) that would silently fall
  // back to mock. AI/Search use "azure"; Auth/PS/LINE/Notify use "real".
  for _, p := range []struct{ name, val string; allowed []string }{
      {"AI_PROVIDER", c.AIProvider, []string{"mock", AIProviderAzure}},
      {"AI_SEARCH_PROVIDER", c.AISearchProvider, []string{"mock", AIProviderAzure}},
      {"AUTH_PROVIDER", c.AuthProvider, []string{"mock", ProviderReal}},
      {"PS_PROVIDER", c.PSProvider, []string{"mock", ProviderReal}},
      {"LINE_PROVIDER", c.LINEProvider, []string{"mock", ProviderReal}},
      {"NOTIFY_PROVIDER", c.NotifyProvider, []string{"mock", ProviderReal}},
  } {
      if !isOneOf(p.val, p.allowed) {
          return nil, fmt.Errorf("config: %s must be one of %v, got %q", p.name, p.allowed, p.val)
      }
  }
  ```
  Add helper near `getenv`:
  ```go
  func isOneOf(v string, allowed []string) bool {
      for _, a := range allowed {
          if v == a { return true }
      }
      return false
  }
  ```
- **GOTCHA**: runs ALWAYS (dev too) — defaults are all `mock` (valid), so no churn. An anonymous-struct slice with a `[]string` field is fine in Go (the `;`-separated field list in the struct literal type is valid). If gofmt/vet dislikes the inline struct type, hoist to a named local type.
- **VALIDATE**: `go build ./pkg/config/... && go vet ./pkg/config/...`.

### Task 3: Config — JWT fail-fast + CORS localhost guard (non-dev only)
- **ACTION**: Replace the JWT warn; add the CORS guard. Keep both gated on `!IsDevelopment()`.
- **IMPLEMENT** (replace `config.go:198-200`):
  ```go
  if !c.IsDevelopment() {
      if c.JWTSecret == "" {
          return nil, fmt.Errorf("config: JWT_SECRET is required when ENV != development")
      }
      if strings.Contains(c.CORSAllowOrigins, "localhost") || strings.Contains(c.CORSAllowOrigins, "127.0.0.1") {
          return nil, fmt.Errorf("config: CORS_ALLOW_ORIGINS must be set to real origins (not localhost) when ENV != development")
      }
  }
  ```
- **GOTCHA**: this removes the `log.Warn` — check whether `log` (zerolog) is still imported/used elsewhere in config.go; if this was its only use, drop the import to avoid an unused-import build error. (It's also used at other spots? verify with grep during impl.)
- **VALIDATE**: `go build ./pkg/config/...`.

### Task 4: Wire trusted-proxy into the Fiber app
- **ACTION**: Edit `backend/cmd/api/main.go` `fiber.New(fiber.Config{...})`.
- **IMPLEMENT**:
  ```go
  app := fiber.New(fiber.Config{
      ErrorHandler:            httpx.ErrorHandler,
      DisableStartupMessage:   true,
      BodyLimit:               maxBodyBytes,
      EnableTrustedProxyCheck: true,
      TrustedProxies:          cfg.TrustedProxyList(),
      ProxyHeader:             fiber.HeaderXForwardedFor,
  })
  ```
- **MIRROR**: confirmed Fiber field names (app.go:237,362,367).
- **GOTCHA**: with `TrustedProxies` empty, `EnableTrustedProxyCheck: true` ⇒ no proxy trusted ⇒ `c.IP()` = direct peer — identical to today for dev/CI and the existing rate-limit integration test (which builds its own apps anyway). Only prod (with `TRUSTED_PROXIES` set to the ingress CIDR) changes behaviour.
- **VALIDATE**: `go build ./cmd/api/...`.

### Task 5: Config tests
- **ACTION**: Add to `backend/pkg/config/config_test.go`.
- **IMPLEMENT** (mirror CONFIG_TEST; helper to set the 3 always-required vars):
  - `TestLoad_NonDevRequiresJWT`: ENV=production, no JWT → error; with JWT + real CORS → ok.
  - `TestLoad_NonDevRejectsLocalhostCORS`: ENV=production, JWT set, default/localhost CORS → error; real origin → ok.
  - `TestLoad_InvalidProviderValue`: `AI_PROVIDER=real` → error; `AUTH_PROVIDER=azure` → error.
  - `TestLoad_TrustedProxyList`: `TRUSTED_PROXIES=" 10.0.0.0/8 , 100.64.0.1 "` → `["10.0.0.0/8","100.64.0.1"]`; unset → empty.
  - Sanity: existing `TestLoad_Defaults` still passes (dev default, providers mock, localhost CORS allowed).
- **GOTCHA**: set `ENV=production` to exercise non-dev guards; remember to also set JWT + real CORS in the provider-value test so it isn't masked by the JWT/CORS guards (or keep ENV=development there so only the always-on provider check fires).
- **VALIDATE**: `go test ./pkg/config/...`.

### Task 6: Env + docs
- **ACTION**: Update repo-root `.env.example` and `docs/SECURITY.md` (Read both via the Read tool first).
- **IMPLEMENT**:
  - `.env.example`:
    ```bash
    # Comma-separated proxy IPs/CIDRs whose X-Forwarded-For is trusted for client-IP
    # resolution (e.g. the prod ingress/LB range). Empty in dev ⇒ direct peer IP.
    # REQUIRED in prod for correct per-client rate limiting (see docs/SECURITY.md).
    TRUSTED_PROXIES=
    ```
    plus a one-line note near CORS/JWT that both must be set to real values when `ENV != development`.
  - `docs/SECURITY.md` Rate limiting: change the trusted-proxy "follow-up" note to "implemented — `TRUSTED_PROXIES` (set to the ingress CIDR in prod) makes `c.IP()` the real client; empty ⇒ direct peer." Add a line under a config/hardening note: prod startup now fails fast on empty `JWT_SECRET`, localhost `CORS_ALLOW_ORIGINS`, or invalid provider values when `ENV != development`.
- **GOTCHA**: Read tool before Edit (session lesson).
- **VALIDATE**: `grep -n TRUSTED_PROXIES .env.example`; SECURITY.md no longer calls trusted-proxy a follow-up.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge |
|---|---|---|---|
| NonDevRequiresJWT | ENV=production, JWT="" | error | prod guard |
| NonDevRequiresJWT (ok) | ENV=production, JWT set, real CORS | ok | |
| NonDevRejectsLocalhostCORS | ENV=production, CORS=localhost | error | prod guard |
| InvalidProviderValue | AI_PROVIDER=real / AUTH_PROVIDER=azure | error | typo catch |
| TrustedProxyList | "10.0.0.0/8, 1.2.3.4" / unset | parsed / empty | splitter |
| Defaults (regression) | ENV unset | ok, providers mock, localhost CORS allowed | dev unaffected |

### Edge Cases Checklist
- [x] dev default: empty JWT + localhost CORS allowed (IsDevelopment)
- [x] provider value check runs in dev too (defaults valid → no churn)
- [x] `TRUSTED_PROXIES` empty ⇒ Fiber trusts no proxy ⇒ direct peer (no test churn)
- [x] removing `log.Warn` doesn't orphan the zerolog import (verify/drop)
- [x] CIDR + exact IP both valid in `TRUSTED_PROXIES` (Fiber parses both)

## Validation Commands

### Static + unit
```bash
cd backend && go vet ./... && go build ./... && go test ./pkg/config/...
```
EXPECT: zero errors; config tests pass.

### Lint + security (CI mirror)
```bash
cd backend && golangci-lint run ./... && gosec -exclude-generated ./... && GOTOOLCHAIN=go1.26.4 govulncheck ./...
```
EXPECT: 0 issues / exit 0 / no vulns.

### Full integration (serialized, no regression)
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration -p 1 ./... -count=1
```
EXPECT: 18 ok / 0 FAIL. (Rate-limit integration unaffected: it builds its own apps; CI runs ENV=development so non-dev guards don't fire.)

### Manual
- [ ] `ENV=production go run ./cmd/api` with no JWT → exits with the JWT error (don't bind a port).
- [ ] Set `JWT_SECRET`, `CORS_ALLOW_ORIGINS=https://hr.example.com`, `TRUSTED_PROXIES=10.0.0.0/8`, `ENV=production` → boots; behind a proxy sending `X-Forwarded-For`, `c.IP()` is the client.

## Acceptance Criteria
- [ ] Prod (`ENV!=development`) fails fast on empty `JWT_SECRET` and localhost `CORS_ALLOW_ORIGINS`.
- [ ] Any invalid provider-flag value fails fast (always).
- [ ] Fiber resolves `c.IP()` via `X-Forwarded-For` only from `TRUSTED_PROXIES`; empty ⇒ direct peer.
- [ ] Dev/CI behaviour unchanged; full suite green; `docs/SECURITY.md` + `.env.example` updated.

## Completion Checklist
- [ ] Mirrors existing fail-fast + splitter patterns; errors wrapped `config: ...`
- [ ] No feature-logic changes; rate-limiter package untouched
- [ ] zerolog import cleaned if newly unused
- [ ] Trusted-proxy paired with `EnableTrustedProxyCheck` (no spoofable XFF)
- [ ] Self-contained — no further searching

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Removing `log.Warn` orphans zerolog import → build break | Medium | Low | grep usages during impl; drop import if unused |
| Non-dev guards trip a legitimate non-dev env (e.g. a localhost-bound internal env) | Low | Medium | guards only fire when `ENV!=development`; documented; staging uses real origins |
| `TRUSTED_PROXIES` set wrong in prod → wrong client IP or spoofable | Low | Medium | env-injected to the known ingress CIDR in Phase 1; doc warns against trusting XFF without the allowlist |
| Provider-value check breaks an undocumented value someone relied on | Very Low | Low | only `mock`/`azure`/`real` are valid anywhere; anything else was already silently-mock (a bug) |

## Notes
- Resolves the #18 review HIGH (`redis-rate-limiter-review.md`) and advances roadmap slices 1.4 + 1.7.
- The prod `TRUSTED_PROXIES` value (ACA ingress CIDR) is finalised during Phase 1 infra; this slice ships the mechanism with a safe empty default.
- Branch `feat/s8-prod-config-guards`, NO attribution, squash-merge; CI green (ENV=development in CI ⇒ non-dev guards dormant). After implement → `/code-review` → `/prp-pr`.
```
