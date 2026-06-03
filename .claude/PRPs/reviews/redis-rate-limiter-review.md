# Code Review: Redis-Backed Rate Limiter (local, branch `feat/s7-redis-ratelimit`)

**Reviewed**: 2026-06-04
**Mode**: Local (uncommitted changes vs `main`)
**Decision**: APPROVE with comments (1 HIGH is pre-existing + topology-dependent — surfaced as a decision, not introduced by this diff)

## Summary
The new `pkg/ratelimit` adapter is small, correct, and faithfully implements `fiber.Storage` (verified against Fiber v2.52.13 `manager.go`). The cross-replica behaviour is proven by an integration test. One adjacent **HIGH** concern — `c.IP()` keying behind a proxy/LB — is pre-existing but becomes the limiting factor for this feature's production correctness; flagged for a deployment-topology decision rather than blindly "fixed" (a naive fix would introduce a spoofable bypass).

## Findings

### CRITICAL
None.

### HIGH
1. **Rate-limit key may be the load-balancer IP, not the client** (`backend/cmd/api/main.go:115-119` fiber.Config + limiter `KeyGenerator: c.IP()`) — **pre-existing; surfaced for decision, not auto-fixed.**
   The Fiber app sets no `EnableTrustedProxyCheck` / `ProxyHeader` / `TrustedProxies`, so `c.IP()` returns the direct TCP peer. In the multi-replica-behind-a-reverse-proxy/LB topology this very feature targets, that peer is the LB — so **every public client shares a single `ratelimit:<lb-ip>` bucket**, and 30 req/min would be exhausted globally (mass false-positive 429s), not per client. The Redis change itself is correct; this keying gap just means per-client limiting isn't actually achieved in prod until the proxy is trusted.
   - **Do NOT** simply set `ProxyHeader: "X-Forwarded-For"` — without trusted-proxy validation a client can spoof `X-Forwarded-For` to mint a fresh bucket per request (limiter bypass).
   - **Correct fix (needs ops input):** confirm the prod ingress/LB; if traffic transits a trusted proxy, set `EnableTrustedProxyCheck: true`, `TrustedProxies: [<LB CIDR>]`, and `ProxyHeader: fiber.HeaderXForwardedFor`. If the api is directly internet-facing, `c.IP()` is already correct and no change is needed.
   - **Recommendation:** ship the Redis store (strict improvement) and resolve this as an explicit follow-up once topology is confirmed. Tracked as an open question on the PR.

### MEDIUM
None.

### LOW
1. **Non-atomic cross-replica counting (soft limit)** (`pkg/ratelimit` + Fiber fixed window) — get→increment-in-Go→set is not atomic across replicas, so the global count can slightly overshoot under heavy concurrency. Documented in the plan/SECURITY.md; acceptable for abuse mitigation and far better than per-instance. No change.
2. **`Reset` bounded by `opTimeout` (3s)** (`store.go`) — on a very large keyspace the SCAN+DEL loop could exceed 3s and return a scan error. Reset is off the hot path and rarely called by the limiter; acceptable. No change.
3. **Fail-open is silent to clients on Redis outage** (`store.go` Get/Set) — by design (availability > strict limiting); logged at WARN. Documented. No change.

## Validation Results

| Check | Result |
|---|---|
| Build (`go build ./...`) | Pass |
| Vet (incl. `-tags integration`) | Pass |
| Lint (`golangci-lint run ./...`) | Pass — 0 issues |
| Security (`gosec -exclude-generated ./...`) | Pass — exit 0 |
| Vuln (`govulncheck ./...`) | Pass — no vulns |
| Unit tests | Pass (+1 config, +2 ratelimit) |
| Integration (`-tags integration -p 1 ./...`) | Pass — 18 ok / 0 FAIL; ratelimit 3/3 incl. cross-replica proof |

## Files Reviewed
- `backend/pkg/ratelimit/store.go` (Added)
- `backend/pkg/ratelimit/store_test.go` (Added)
- `backend/pkg/ratelimit/store_integration_test.go` (Added)
- `backend/cmd/api/main.go` (Modified)
- `backend/pkg/config/config.go` / `config_test.go` (Modified)
- `.env.example`, `docs/SECURITY.md` (Modified)

## Checked and found clean
- **`fiber.Storage` contract**: Get-miss → `(nil,nil)`; empty key/val ignored on Set; TTL forwarded straight from `manager.set`. Verified against Fiber source. ✅
- **asynq isolation**: `ratelimit:` prefix + SCAN-scoped Reset; integration test asserts a seeded `asynq:*` key survives Reset (no `FLUSHDB`). ✅
- **Client lifecycle**: `Close()` is a no-op; only main's `defer rdb.Close()` owns the client (no double-close). ✅
- **No secrets / no PII logged**; per-op context timeout prevents hangs; fail-open prevents 500s. ✅
- **No magic numbers**: Max from config, window/prefix/timeout/scan-batch are named consts. ✅
