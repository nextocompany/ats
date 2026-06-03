# Plan: Redis-Backed Rate Limiter for Public API

## Summary
Replace the public API's in-memory Fiber rate limiter (which counts per process, so N api replicas allow N×30 req/min/IP) with a shared Redis-backed store, so the 30 req/min/IP limit holds across all replicas. Implemented as a small custom `fiber.Storage` adapter over the api's **existing** go-redis client — no new dependency, no second connection pool. Closes the `docs/SECURITY.md` follow-up noting the limiter is "in-memory per-instance."

## User Story
As a **platform operator protecting the public career portal**, I want **the apply/status rate limit enforced cluster-wide rather than per process**, so that **a single IP can't multiply its effective request budget by hitting different api replicas, and abuse controls actually hold under horizontal scaling**.

## Problem → Solution
**Current state:** `backend/cmd/api/main.go:178` mounts `limiter.New(...)` with Fiber's default in-memory storage on `/api/v1/public/*`. Each api process keeps its own counter, so with R replicas the real limit is R×30/min/IP. The limit silently weakens exactly when you scale to handle load (or an attack).
**Desired state:** The limiter reads/writes counters in the shared Redis already used by asynq, so the window count is global. The adapter reuses the existing `rdb *goredis.Client`, namespaces keys under `ratelimit:`, fails **open** on a Redis outage (availability over strict limiting for a public endpoint), and never touches non-rate-limit keys (no `FLUSHDB`).

## Metadata
- **Complexity**: Medium
- **Source PRD**: N/A (free-form — Sprint 7 slice from session `2026-06-03-s7-ps-hmac`; SECURITY.md follow-up)
- **PRD Phase**: N/A (standalone)
- **Estimated Files**: 8 (3 new, 5 updated)

---

## UX Design

### Before / After
N/A — internal/infra change. No user-facing UX. Observable behaviour is unchanged for a single replica; under multiple replicas the limit now holds globally (a throttled client still gets `429 rate limit exceeded`, just sooner/correctly).

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| `/api/v1/public/*` limiter | in-memory counter per process | shared Redis counter (`ratelimit:<ip>`) | same Max/window, same 429 body |
| api ↔ Redis | Redis used for asynq only | Redis also stores rate-limit counters | distinct key prefix; no collision |
| Redis outage | limiter keeps working (local memory) | limiter **fails open** (allows requests) | documented trade-off; abuse limit is soft, availability prioritised |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 (critical) | `backend/cmd/api/main.go` | 48-52, 76-87, 176-187 | Where the limiter is mounted, the `publicRateMax` const, and where `rdb` is built (available before the limiter) |
| P0 (critical) | `backend/pkg/redis/redis.go` | 1-27 | The go-redis client wrapper pattern + error-wrap style to mirror in the new package |
| P0 (critical) | `<fiber module>/middleware/limiter/manager.go` | 12-80 | Proves Storage values are opaque msgpack bytes; `Get` miss must return `(nil,nil)`; with Storage set there is NO memory fallback |
| P1 (important) | `<fiber module>/limiter/config.go` (Storage field) | ~54 | `limiter.Config.Storage fiber.Storage` is the field to set |
| P1 (important) | `backend/pkg/config/config.go` | 74-81, 135-139, 199-233 | Field block, defaults, `getenv`/`getenvInt` helpers (getenvInt added in the PDPA slice) |
| P1 (important) | `backend/internal/middleware/auth_test.go` | 1-40 | `fiber.New()` + `app.Test(httptest.NewRequest(...))` test idiom to mirror for the limiter behaviour test |
| P2 (reference) | `backend/internal/pdpa/retention_integration_test.go` | 1-50 | `//go:build integration` + `dsn()`/env-driven connection + cleanup pattern (mirror for a Redis-driven integration test) |
| P2 (reference) | `backend/internal/health/*.go` (redis checker call site in main.go:144) | — | Confirms `rdb.Ping` is the liveness contract; the store borrows the same client |

> Resolve `<fiber module>` with: `go list -m -f '{{.Dir}}' github.com/gofiber/fiber/v2` → `/middleware/limiter/`.

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| `fiber.Storage` contract | Fiber v2.52.13 `app.go:43` | `Get(key)→(nil,nil)` on miss; `Set(key,val,exp)` with `exp==0` = no expiry, empty key/val ignored; plus `Delete`, `Reset`, `Close` |
| Fiber limiter + custom Storage | Fiber v2 limiter `manager.go` | Limiter does get→increment-in-Go→set (read-modify-write at app layer). Not atomic across replicas → counter is **approximately** correct (soft limit). Still strictly better than per-instance. |
| go-redis v9 key miss | go-redis v9.20.0 | `Get` returns `redis.Nil` sentinel on miss — translate to `(nil,nil)`, do NOT propagate as an error |

```
KEY_INSIGHT: Setting limiter.Config.Storage makes Fiber use ONLY that store (no in-memory fallback).
APPLIES_TO: The whole approach — one adapter is sufficient; no dual-write.
GOTCHA: Get-miss MUST be (nil,nil), not an error, or the limiter mis-reads every request as a fresh window.

KEY_INSIGHT: The shared Redis also holds asynq's queue (keys prefixed `asynq:`).
APPLIES_TO: Reset() and key naming.
GOTCHA: Reset() must NEVER FLUSHDB — it would wipe the job queue. Scope it to `ratelimit:*` via SCAN+DEL.

KEY_INSIGHT: fiber.Storage methods take no context.Context.
APPLIES_TO: Every Redis call in the adapter.
GOTCHA: Use a short per-op context timeout; on error fail OPEN (return nil) so a Redis blip never 500s/locks out the public apply flow.
```

---

## Patterns to Mirror

### REDIS_CLIENT_WRAPPER
```go
// SOURCE: backend/pkg/redis/redis.go:1-27
package redis
// ...
func Connect(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	// ...
}
```
→ New package `ratelimit` wraps (does not own) a `*goredis.Client`; errors wrapped `ratelimit: <op>: %w`.

### CONFIG_PATTERN
```go
// SOURCE: backend/pkg/config/config.go:135-139 (PDPA slice) + getenvInt helper
RetentionDays: getenvInt("RETENTION_DAYS", 365),
```
→ Add `RateLimitPublicMax int` via `getenvInt("RATE_LIMIT_PUBLIC_MAX", 30)`.

### LIMITER_MOUNT (current, to modify)
```go
// SOURCE: backend/cmd/api/main.go:178-185
app.Use("/api/v1/public", limiter.New(limiter.Config{
	Max:          publicRateMax,
	Expiration:   time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
	LimitReached: func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
	},
}))
```

### FIBER_TEST_IDIOM
```go
// SOURCE: backend/internal/middleware/auth_test.go:14-31
app := fiber.New()
app.Use(h)
app.Get("/", func(c *fiber.Ctx) error { return nil })
resp, _ := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
```

### INTEGRATION_TEST_HEADER
```go
// SOURCE: backend/internal/pdpa/retention_integration_test.go:1-18
//go:build integration
// env-driven connection with a sane local default; t.Cleanup to release
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/pkg/ratelimit/store.go` | CREATE | `RedisStore` implementing `fiber.Storage` over the shared go-redis client |
| `backend/pkg/ratelimit/store_test.go` | CREATE | Unit test for key-prefix/miss/empty-key logic that needs no live Redis |
| `backend/pkg/ratelimit/store_integration_test.go` | CREATE | `//go:build integration` — Set/Get/TTL-expiry/Delete/Reset-scope against live Redis + cross-replica limiter behaviour |
| `backend/cmd/api/main.go` | UPDATE | Mount limiter with `Storage: ratelimit.New(rdb)`; use `cfg.RateLimitPublicMax` |
| `backend/pkg/config/config.go` | UPDATE | Add `RateLimitPublicMax` field + default |
| `backend/pkg/config/config_test.go` | UPDATE | +1 test: default + parsed override |
| `.env.example` (repo ROOT) | UPDATE | Document `RATE_LIMIT_PUBLIC_MAX` |
| `docs/SECURITY.md` | UPDATE | Flip "in-memory per-instance" note → Redis-backed, cluster-wide |

## NOT Building

- **Atomic distributed counting (Lua INCR/sliding-window in Redis).** Fiber's limiter does read-modify-write at the app layer; perfect cross-replica atomicity would mean replacing the limiter middleware entirely. The Redis-shared fixed window is approximately correct and strictly better than per-instance — sufficient for soft abuse control. Out of scope.
- **Rate limiting on authenticated HR/dashboard or PS webhook routes.** Only `/api/v1/public/*` (the unauthenticated abuse surface). HMAC already guards PS; HR is behind auth.
- **Per-route or per-endpoint distinct limits, burst/leaky-bucket tiers, or CAPTCHA.** Single IP-keyed window, as today.
- **A new Redis connection/pool or swapping to a third-party Fiber storage package** (`gofiber/storage/redis`). We reuse the existing `rdb` to avoid a redundant pool and dependency.
- **Fail-closed on Redis outage.** Deliberately fail open (see Risks).

---

## Step-by-Step Tasks

### Task 1: `RedisStore` adapter (`pkg/ratelimit/store.go`)
- **ACTION**: Create the package + `RedisStore` implementing `fiber.Storage`.
- **IMPLEMENT**:
  ```go
  // Package ratelimit adapts the shared go-redis client to fiber.Storage so the
  // public-API limiter counts requests cluster-wide instead of per process.
  package ratelimit

  import (
      "context"
      "errors"
      "time"

      goredis "github.com/redis/go-redis/v9"
      "github.com/rs/zerolog/log"
  )

  // keyPrefix namespaces limiter keys so they never collide with asynq's keys in
  // the shared Redis database.
  const keyPrefix = "ratelimit:"

  // opTimeout bounds each Redis call; on timeout/error the store fails OPEN (the
  // limiter treats it as a miss) so a Redis blip never blocks the public flow.
  const opTimeout = 3 * time.Second

  // RedisStore implements fiber.Storage over a borrowed *goredis.Client. It does
  // NOT own the client — Close is a no-op; the api owns the client lifecycle.
  type RedisStore struct{ client *goredis.Client }

  // New builds a RedisStore over an existing client.
  func New(client *goredis.Client) *RedisStore { return &RedisStore{client: client} }

  func (s *RedisStore) Get(key string) ([]byte, error) {
      if key == "" {
          return nil, nil
      }
      ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
      defer cancel()
      val, err := s.client.Get(ctx, keyPrefix+key).Bytes()
      if errors.Is(err, goredis.Nil) {
          return nil, nil // miss
      }
      if err != nil {
          log.Warn().Err(err).Msg("ratelimit: get failed; failing open")
          return nil, nil // fail open
      }
      return val, nil
  }

  func (s *RedisStore) Set(key string, val []byte, exp time.Duration) error {
      if key == "" || len(val) == 0 {
          return nil // contract: ignore empty key/val
      }
      ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
      defer cancel()
      if err := s.client.Set(ctx, keyPrefix+key, val, exp).Err(); err != nil {
          log.Warn().Err(err).Msg("ratelimit: set failed; failing open")
      }
      return nil // never propagate — limiter must not 500 on a Redis blip
  }

  func (s *RedisStore) Delete(key string) error {
      if key == "" {
          return nil
      }
      ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
      defer cancel()
      if err := s.client.Del(ctx, keyPrefix+key).Err(); err != nil {
          log.Warn().Err(err).Msg("ratelimit: delete failed")
      }
      return nil
  }

  // Reset deletes ONLY ratelimit:* keys. It MUST NOT FLUSHDB — the same Redis
  // holds the asynq job queue. Uses SCAN to avoid blocking Redis on a big keyspace.
  func (s *RedisStore) Reset() error {
      ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
      defer cancel()
      iter := s.client.Scan(ctx, 0, keyPrefix+"*", 100).Iterator()
      var batch []string
      for iter.Next(ctx) {
          batch = append(batch, iter.Val())
          if len(batch) >= 100 {
              if err := s.client.Del(ctx, batch...).Err(); err != nil {
                  return fmt.Errorf("ratelimit: reset del: %w", err)
              }
              batch = batch[:0]
          }
      }
      if err := iter.Err(); err != nil {
          return fmt.Errorf("ratelimit: reset scan: %w", err)
      }
      if len(batch) > 0 {
          if err := s.client.Del(ctx, batch...).Err(); err != nil {
              return fmt.Errorf("ratelimit: reset del: %w", err)
          }
      }
      return nil
  }

  // Close is a no-op: the client is owned by the api process, not this adapter.
  func (s *RedisStore) Close() error { return nil }
  ```
- **MIRROR**: REDIS_CLIENT_WRAPPER. Satisfies the `fiber.Storage` interface confirmed in `manager.go`.
- **IMPORTS**: `context`, `errors`, `fmt`, `time`, `goredis "github.com/redis/go-redis/v9"`, `github.com/rs/zerolog/log`. (Add `"fmt"` — used in Reset.)
- **GOTCHA**:
  - Get-miss returns `(nil,nil)` — never an error (the limiter mis-reads otherwise).
  - Reset is SCAN-scoped to `ratelimit:*`; **never** `FLUSHDB`/`FlushAll`.
  - Close is a no-op (don't close the shared client — main's `defer rdb.Close()` owns it).
  - Set/Get fail OPEN (log + return nil) so a Redis hiccup can't 500 the public endpoints.
- **VALIDATE**: `go build ./pkg/ratelimit/...`; confirm it satisfies the interface with a compile-time assertion `var _ fiber.Storage = (*RedisStore)(nil)` in the test (Task 2).

### Task 2: Pure unit test (`pkg/ratelimit/store_test.go`)
- **ACTION**: Test the no-Redis-needed branches + interface compliance.
- **IMPLEMENT**:
  - `var _ fiber.Storage = (*RedisStore)(nil)` (compile-time interface check).
  - `TestRedisStore_EmptyKeyIsNoop`: `New(nil).Get("")` → `(nil,nil)` no panic; `Set("", ...)`/`Set("k", nil, ...)` → nil (these return before touching the nil client).
  - `TestRedisStore_CloseIsNoop`: `New(nil).Close()` → nil.
- **MIRROR**: standard Go table/unit test.
- **GOTCHA**: Passing a `nil` client is safe ONLY for the empty-key/empty-val/Close paths (they return before dereferencing). Do NOT call Get with a non-empty key on a nil client in this unit test — that needs the integration test.
- **VALIDATE**: `go test ./pkg/ratelimit/...`.

### Task 3: Config knob
- **ACTION**: Add `RateLimitPublicMax` to `Config`, default, and use it.
- **IMPLEMENT**:
  - Struct (near the report/retention block):
    ```go
    // Public API rate limit (Sprint 7): max requests per IP per minute on
    // /api/v1/public/*. Enforced cluster-wide via the Redis-backed store.
    RateLimitPublicMax int
    ```
  - In `Load`:
    ```go
    RateLimitPublicMax: getenvInt("RATE_LIMIT_PUBLIC_MAX", 30),
    ```
- **MIRROR**: CONFIG_PATTERN (`getenvInt`, added in the PDPA slice).
- **GOTCHA**: No fail-fast; a sane default (30) means the var is optional. If someone sets `0`, Fiber's limiter treats Max=0 as "no limit" — acceptable (explicit opt-out), but note it.
- **VALIDATE**: `go build ./pkg/config/...`; `go test ./pkg/config/...`.

### Task 4: Wire the limiter to Redis
- **ACTION**: Update `backend/cmd/api/main.go`.
- **IMPLEMENT**:
  - Add import `"github.com/nexto/hr-ats/pkg/ratelimit"`.
  - Replace the limiter block (line 178):
    ```go
    app.Use("/api/v1/public", limiter.New(limiter.Config{
        Max:          cfg.RateLimitPublicMax,
        Expiration:   publicRateWindow,
        Storage:      ratelimit.New(rdb),
        KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
        LimitReached: func(c *fiber.Ctx) error {
            return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
        },
    }))
    ```
  - Replace the `publicRateMax = 30` const with `publicRateWindow = time.Minute` (Max now comes from config). Remove the now-unused `publicRateMax`.
- **MIRROR**: LIMITER_MOUNT.
- **GOTCHA**: `rdb` exists from line 76-87 (built before this line) — no reordering needed. Keep `Expiration` as a named const, not a magic literal.
- **VALIDATE**: `go build ./cmd/api/...`.

### Task 5: Env + docs
- **ACTION**: Update repo-root `.env.example` and `docs/SECURITY.md`.
- **IMPLEMENT**:
  - `.env.example` (near the other API knobs):
    ```bash
    # Public API rate limit — max requests per IP per minute on /api/v1/public/*.
    # Enforced cluster-wide via Redis (shared across api replicas).
    RATE_LIMIT_PUBLIC_MAX=30
    ```
  - `docs/SECURITY.md`: locate the line describing the public rate limiter as in-memory/per-instance and update to: Redis-backed, shared across replicas, fails open on Redis outage, keyed by client IP under `ratelimit:*`.
- **GOTCHA**: Use the Read tool on both files before Edit (session lesson — shell grep does not satisfy the Edit precondition).
- **VALIDATE**: `grep -n RATE_LIMIT_PUBLIC_MAX .env.example`; SECURITY.md no longer says "in-memory".

### Task 6: Config test
- **ACTION**: Add to `backend/pkg/config/config_test.go`.
- **IMPLEMENT**:
  ```go
  func TestLoad_RateLimitDefault(t *testing.T) {
      t.Setenv("DB_URL", "postgres://localhost/db")
      t.Setenv("REDIS_URL", "redis://localhost:6379")
      t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
      c, err := Load()
      if err != nil { t.Fatalf("unexpected error: %v", err) }
      if c.RateLimitPublicMax != 30 {
          t.Errorf("expected default RateLimitPublicMax 30, got %d", c.RateLimitPublicMax)
      }
      t.Setenv("RATE_LIMIT_PUBLIC_MAX", "5")
      c2, _ := Load()
      if c2.RateLimitPublicMax != 5 {
          t.Errorf("expected 5, got %d", c2.RateLimitPublicMax)
      }
  }
  ```
- **MIRROR**: `TestLoad_RetentionDefaults`.
- **VALIDATE**: `go test ./pkg/config/...`.

### Task 7: Integration test — store + cross-replica behaviour (`pkg/ratelimit/store_integration_test.go`)
- **ACTION**: Create `//go:build integration` test against the live Redis from the stack.
- **IMPLEMENT**:
  - `redisURL()` helper: `os.Getenv("REDIS_URL")` else `"redis://localhost:6379"` (mirror `dsn()`).
  - Connect via `goredis.ParseURL` + `NewClient`; `t.Cleanup` closes it. Pre-clean: `Reset()` (deletes only `ratelimit:*`).
  - `TestRedisStore_SetGetExpiryDelete`:
    - `Set("ip1", []byte("x"), 1*time.Second)` → `Get("ip1")` == `x`.
    - sleep > 1s → `Get("ip1")` == `(nil,nil)` (TTL expiry).
    - `Set` then `Delete("ip1")` → `Get` == nil.
    - Get on absent key → `(nil,nil)`.
  - `TestRedisStore_ResetScopedToPrefix`:
    - Seed an unrelated key `asynq:fake` directly via the raw client.
    - `Set("ipX", ...)` via the store; call `Reset()`.
    - Assert `Get("ipX")` == nil AND the raw client still has `asynq:fake` (Reset must not touch it).
  - `TestLimiter_SharedAcrossInstances` (the core proof):
    - Build TWO `fiber.New()` apps, each mounting `limiter.New(limiter.Config{Max: 3, Expiration: time.Minute, Storage: ratelimit.New(client), KeyGenerator: fixed "1.2.3.4", LimitReached: 429})` over a `GET /p`.
    - Issue 2 requests to app A, 1 to app B (total 3) → all `200`.
    - 4th request (to either app) → `429`. Proves the window is shared via Redis, not per-process.
    - Use a unique KeyGenerator IP per test run prefix to avoid cross-test bleed; `Reset()` in cleanup.
- **MIRROR**: INTEGRATION_TEST_HEADER (pdpa), FIBER_TEST_IDIOM (auth_test).
- **GOTCHA**:
  - Runs under CI's serialized integration step (`-p 1`), so key bleed across packages is not a concern, but still `Reset()` on setup+cleanup for hygiene.
  - The limiter counts the SAME key only if both apps use the identical `KeyGenerator` return — hardcode the IP in the test (don't rely on `c.IP()` from httptest, which is empty/loopback).
- **VALIDATE**: `make up && make migrate-up` (Redis comes up with the stack) then `cd backend && go test -tags integration -p 1 ./pkg/ratelimit/... -count=1 -v`.

### Task 8: Full validation sweep
- **ACTION**: Run the CI-mirroring suite.
- **VALIDATE**: see Validation Commands.

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge Case? |
|---|---|---|---|
| interface assertion | `var _ fiber.Storage = (*RedisStore)(nil)` | compiles | contract |
| `EmptyKeyIsNoop` | `Get("")`, `Set("",...)`, `Set("k",nil,...)` | `(nil,nil)` / nil, no client deref | empty inputs |
| `CloseIsNoop` | `Close()` | nil | lifecycle |
| `RateLimitDefault` | env unset / `=5` | 30 / 5 | config |

### Integration Tests
| Test | Input | Expected | Edge Case? |
|---|---|---|---|
| `SetGetExpiryDelete` | set/get/ttl/delete | round-trips; expires; deletes | TTL boundary |
| `ResetScopedToPrefix` | seed `asynq:fake` + `ratelimit:*` | Reset clears only `ratelimit:*` | no FLUSHDB |
| `LimiterSharedAcrossInstances` | 3 ok then 4th across 2 apps | 200×3, then 429 | cross-replica proof |

### Edge Cases Checklist
- [x] Get miss → `(nil,nil)` (not error)
- [x] Empty key / empty value ignored
- [x] TTL expiry frees the window
- [x] Reset never wipes asynq keys
- [x] Redis outage → fail open (Set/Get log + nil) — covered by code review; hard to assert in CI without killing Redis, documented
- [x] `Max=0` → limiter disables (explicit opt-out)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./... && go build ./...
```
EXPECT: zero errors.

### Unit Tests
```bash
cd backend && go test ./pkg/ratelimit/... ./pkg/config/...
```
EXPECT: pass.

### Lint + Security (CI mirror)
```bash
cd backend && golangci-lint run ./... && gosec -exclude-generated ./... && GOTOOLCHAIN=go1.26.4 govulncheck ./...
```
EXPECT: golangci-lint `0 issues`; gosec exit 0; govulncheck clean.

### Integration (serialized, as CI runs it)
```bash
make up && make migrate-up
cd backend && go test -tags integration -p 1 ./pkg/ratelimit/... -count=1
# and the full suite for no regressions:
cd backend && go test -tags integration -p 1 ./... -count=1
```
EXPECT: all pass.

### Manual Validation
- [ ] With stack up, `for i in $(seq 1 35); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/public/positions; done` → first ~30 `200`, then `429`.
- [ ] `docker compose exec redis redis-cli KEYS 'ratelimit:*'` shows the IP key; `KEYS 'asynq:*'` still present (untouched).
- [ ] Stop Redis briefly (`docker compose stop redis`) → public endpoint still serves (fails open), logs `ratelimit: get failed; failing open`; restart Redis.

---

## Acceptance Criteria
- [ ] `/api/v1/public/*` limiter uses the Redis-backed store; counter shared across replicas (proven by `LimiterSharedAcrossInstances`).
- [ ] `RATE_LIMIT_PUBLIC_MAX` config drives the limit (default 30); window const = 1 min.
- [ ] Reset is prefix-scoped (asynq keys survive); Close is a no-op; Redis errors fail open.
- [ ] All validation commands pass; no regressions in the full integration suite.
- [ ] `docs/SECURITY.md` + `.env.example` updated.

## Completion Checklist
- [ ] Implements `fiber.Storage` faithfully (Get-miss `(nil,nil)`, empty-arg handling)
- [ ] Reuses existing `rdb` (no new dep / second pool)
- [ ] Errors wrapped `ratelimit: <op>: %w` where propagated; fail-open paths logged
- [ ] No PII/secret logging; no magic numbers (Max from config, window/prefix/timeout named consts)
- [ ] Tests follow `//go:build integration` + fiber `app.Test` idioms
- [ ] Self-contained — no further codebase searching needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `Reset()` accidentally wipes asynq queue | Low | Critical | SCAN-scoped to `ratelimit:*`; never FLUSHDB; integration test asserts asynq key survives |
| Redis outage blocks public apply flow | Medium | High | Fail OPEN (log + nil); availability prioritised for a public endpoint; documented |
| Read-modify-write race undercounts across replicas | Medium | Low | Accepted soft-limit; still far better than per-instance; atomic counting explicitly out of scope |
| Closing the borrowed client twice | Low | Medium | `Close()` is a no-op; only main's `defer rdb.Close()` closes it |
| Key collision with asynq | Very Low | High | Distinct `ratelimit:` prefix; verified in Reset test |

## Notes
- **Why a custom adapter over `gofiber/storage/redis`:** reuses the single existing connection/pool and URL, adds no dependency, and is ~60 lines — consistent with the repo's thin-wrapper infra (`pkg/redis`, `pkg/blob`). The third-party package would open a second pool to the same Redis. Documented as the considered alternative.
- **Fail-open is deliberate:** the limiter is abuse mitigation, not a hard quota; locking out legitimate candidates because Redis blipped is worse than briefly allowing un-throttled traffic. Mentioned in SECURITY.md.
- **Soft limit:** Fiber's fixed window + non-atomic RMW means the global count can slightly overshoot under heavy concurrency. This is acceptable for the threat model and called out so no one mistakes it for a precise quota.
- Session continuity: branch `feat/s7-redis-ratelimit`, NO commit attribution, squash-merge; CI green (incl. the `-p 1` integration fix from the PDPA slice) so it merges without `--admin`. After implement → `/code-review` → `/prp-pr`.
```
