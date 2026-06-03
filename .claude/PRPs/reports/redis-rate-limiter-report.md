# Implementation Report: Redis-Backed Rate Limiter for Public API

## Summary
Replaced the public API limiter's in-memory Fiber storage with a shared Redis-backed store so the 30 req/min/IP window on `/api/v1/public/*` holds cluster-wide instead of per process. Implemented as a ~110-line custom `fiber.Storage` adapter (`pkg/ratelimit`) over the api's existing go-redis client — no new dependency, no second connection pool. Reset is scoped to `ratelimit:*` (never `FLUSHDB`), Close is a no-op (borrowed client), and Redis errors fail open.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium | Medium (as predicted) |
| Confidence | 9/10 | 10/10 — single pass, no surprises |
| Files Changed | 8 (3 new, 5 updated) | 8 (3 new, 5 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | `RedisStore` adapter (`pkg/ratelimit/store.go`) | ✅ Complete | fail-open, prefix-scoped Reset, no-op Close |
| 2 | Pure unit test + interface assertion | ✅ Complete | `var _ fiber.Storage = (*RedisStore)(nil)` |
| 3 | `RateLimitPublicMax` config | ✅ Complete | default 30 via `getenvInt` |
| 4 | Wire limiter to Redis store | ✅ Complete | `Storage: ratelimit.New(rdb)`; `publicRateMax` const → `publicRateWindow` |
| 5 | `.env.example` + `docs/SECURITY.md` | ✅ Complete | "in-memory" follow-up → Redis-backed |
| 6 | Config unit test | ✅ Complete | default + parsed override |
| 7 | Integration test (store + cross-replica) | ✅ Complete | 3 integration tests, incl. shared-window proof |
| 8 | Full validation sweep | ✅ Complete | all green |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; golangci-lint `0 issues`; gosec exit 0; govulncheck no vulns |
| Unit Tests | ✅ Pass | full `go test ./...` ok; +1 config test, +2 ratelimit unit tests |
| Build | ✅ Pass | `go build ./...` ok |
| Integration | ✅ Pass | ratelimit 3/3; full `-tags integration -p 1 ./...` = 18 ok / 0 FAIL |
| Edge Cases | ✅ Pass | miss→(nil,nil), empty-arg noop, TTL expiry, Reset-scope, cross-replica 429 |

## Files Changed

| File | Action | Lines |
|---|---|---|
| `backend/pkg/ratelimit/store.go` | CREATED | +112 |
| `backend/pkg/ratelimit/store_test.go` | CREATED | +37 |
| `backend/pkg/ratelimit/store_integration_test.go` | CREATED | +160 |
| `backend/cmd/api/main.go` | UPDATED | +10 / -6 |
| `backend/pkg/config/config.go` | UPDATED | +6 |
| `backend/pkg/config/config_test.go` | UPDATED | +23 |
| `.env.example` | UPDATED | +5 |
| `docs/SECURITY.md` | UPDATED | +5 / -2 |

## Deviations from Plan
None. Implemented exactly as planned.

## Issues Encountered
None. (The shell `cwd` reset between some `make` invocations and `go test`, surfacing a "directory prefix does not contain main module" error once — re-ran from `backend/`. No code impact.)

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `pkg/ratelimit/store_test.go` | 2 + interface assert | empty-key/empty-val noop, no-op Close, `fiber.Storage` compliance |
| `pkg/ratelimit/store_integration_test.go` | 3 | Set/Get/TTL-expiry/Delete; Reset scoped to `ratelimit:*` (asynq key survives); two fiber apps sharing one Redis store enforce a single combined window (4th req → 429) |
| `pkg/config/config_test.go` | 1 | `RateLimitPublicMax` default 30 + parsed override |

## Design Decisions Confirmed in Implementation
- **Custom adapter over `gofiber/storage/redis`** — reuses the existing client/pool/URL, zero new deps, ~110 lines; matches the repo's thin-wrapper infra (`pkg/redis`, `pkg/blob`).
- **Fail open on Redis outage** — Get/Set log + return nil so a Redis blip can't 500 or lock out the public apply flow. Availability prioritised for an abuse-mitigation (not hard-quota) control.
- **Reset scoped to `ratelimit:*` via SCAN** — never `FLUSHDB`; integration test asserts a seeded `asynq:*` key survives Reset.
- **Close is a no-op** — the api owns `rdb` (`defer rdb.Close()`); the adapter only borrows it.
- **Soft limit acknowledged** — Fiber's fixed-window read-modify-write isn't atomic across replicas, so the global count can slightly overshoot under heavy concurrency; acceptable for the threat model and far better than the previous per-instance behaviour.

## Post-Implementation Review (2026-06-04)
`/code-review` → APPROVE with comments. The new code is clean (faithful `fiber.Storage`, asynq-isolated, no-op Close, fail-open). One **HIGH** flagged: the limiter keys on `c.IP()`, which behind a trusted proxy/LB resolves to the LB IP (all clients share one bucket). It's **pre-existing** (same KeyGenerator as before) and topology-dependent; a naive `ProxyHeader` fix would create a spoofable bypass. Per user decision, **shipping as-is and tracking trusted-proxy config as a follow-up** (documented in `docs/SECURITY.md` Rate limiting + flagged on the PR). See `.claude/PRPs/reviews/redis-rate-limiter-review.md`.

## Next Steps
- [x] Code review via `/code-review` — APPROVE with comments
- [ ] Follow-up: configure Fiber trusted-proxy in prod (needs LB CIDR)
- [ ] Create PR via `/prp-pr` (branch `feat/s7-redis-ratelimit`, NO attribution, squash-merge)
