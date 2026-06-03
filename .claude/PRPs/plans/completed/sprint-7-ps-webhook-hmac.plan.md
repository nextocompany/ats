# Plan: Sprint 7 — PeopleSoft Webhook HMAC Authentication

## Summary
The inbound PeopleSoft machine endpoints (`POST /api/v1/ps/vacancy-opened`, `/vacancy-closed`, `/sync-hired`) are state-changing but **unauthenticated** — a documented gap (`docs/SECURITY.md` + the explicit bypass in `internal/middleware/auth.go:49`). This adds **HMAC-SHA256 shared-secret** verification as Fiber middleware on those three POST routes: PeopleSoft signs the raw request body, and the API rejects missing/invalid signatures with 401 via a constant-time compare. It's gated through the existing mock-default-behind-config seam — local/CI (`PS_PROVIDER=mock`, no secret) stay open and green; production (`PS_PROVIDER=real`) requires `PS_WEBHOOK_SECRET` and fails fast at startup without it. `GET /api/v1/ps/health` stays open as a probe.

## User Story
As the **PeopleSoft Integration Broker (a machine client)**, I want my webhook calls to be authenticated by a shared HMAC secret, so that only PeopleSoft — not any party who can reach the API — can open/close vacancies or trigger hired-sync.

## Problem → Solution
`/api/v1/ps/*` POSTs accept any caller (auth middleware explicitly skips the group) → the three POSTs require a valid `X-PS-Signature` HMAC-SHA256 over the raw body when a webhook secret is configured; real PS mode mandates the secret at startup.

## Metadata
- **Complexity**: Medium
- **Source PRD**: N/A (standalone Sprint 7 slice; backlog item from `docs/SECURITY.md` "Follow-up")
- **PRD Phase**: Sprint 7 — PeopleSoft webhook HMAC auth
- **Estimated Files**: ~9 (1 new middleware + 1 new test; config, config_test, routes, webhook_test, main.go, .env.example, docs/SECURITY.md updated)

---

## Decision: scope of HMAC (resolved by evidence, not assumption)

The brief flagged `/sync-hired` as possibly a manual/HR-triggered action needing different auth. Resolved during exploration:
- **No frontend/dashboard caller** for `/api/v1/ps/sync-hired` (grep of `frontend/` + `career-portal/` = none). The dashboard's hired-sync path goes through `applications` status-PATCH → `psService.SyncHired` internally (`internal/applications/handler.go:165`), **not** this HTTP endpoint.
- **`docs/SECURITY.md:17-19`** explicitly classifies all `/api/v1/ps/*` as "PeopleSoft machine webhooks / machine endpoints" and prescribes "an HMAC/shared-secret check."
- **Decision**: apply HMAC to all **three POSTs** (`vacancy-opened`, `vacancy-closed`, `sync-hired`); leave `GET /health` open.
- **Alternative considered & rejected**: put `/sync-hired` behind HR (Entra) auth instead — rejected because it has no HR-console caller, lives in the machine group, and mixing HR-user auth into the machine group contradicts the SECURITY.md model. If a dashboard "push to PS" button is ever added, it should call a *new* HR-authed route, not this one.

---

## UX Design
Internal / machine-to-machine change — **no user-facing UX transformation.** (HR console and candidate portal are unaffected; only the PS↔API contract gains a signature header.)

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| `POST /api/v1/ps/vacancy-opened\|closed\|sync-hired` | any caller → 200 | must send `X-PS-Signature: <hex hmac-sha256(body)>` when secret configured; else 401 | enforced when `PS_WEBHOOK_SECRET` set (always in real mode) |
| `GET /api/v1/ps/health` | open | open (unchanged) | probe stays unauthenticated |
| Local dev / CI (`PS_PROVIDER=mock`, no secret) | open | open (unchanged) | tests + e2e stay green without signing |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/peoplesoft/routes.go` | 1-12 | The group + 3 POSTs + health to guard; signature changes here |
| P0 | `backend/internal/middleware/auth.go` | 13-60 | **The seam pattern to mirror**: provider-gated middleware, fail-fast, fiber 401, `isUnauthedPath` |
| P0 | `backend/pkg/config/config.go` | 43-49, 87-178 | Add `PSWebhookSecret` + real-mode validation; `UsesRealPeopleSoft()` helper at :178 |
| P0 | `backend/internal/peoplesoft/webhook_test.go` | 58-73, 132-148 | `testApp`/`post` harness + table-test style to extend (RegisterRoutes signature changes) |
| P1 | `backend/internal/peoplesoft/webhook.go` | 46-146 | Handlers downstream of the middleware; confirm `BodyParser` still works after `c.Body()` |
| P1 | `backend/pkg/httpx/*.go` | all | `ErrorHandler` + `OK`/`Fail` envelope; 401 should flow through `fiber.NewError` |
| P1 | `backend/cmd/api/main.go` | 135-171 | Middleware order (helmet→logger→Auth) and the `peoplesoft.RegisterRoutes(...)` call to update |
| P2 | `backend/pkg/config/config_test.go` | 56-93 | `TestLoad_*RequiresKeys` pattern to mirror for the new secret validation |
| P2 | `docs/SECURITY.md` | 14-19 | The "Follow-up" line to flip to resolved |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| Fiber `c.Body()` vs `BodyParser` | gofiber v2 docs | `c.Body()` returns the buffered raw body and does **not** consume it — `BodyParser` in the handler still works afterward. Safe to HMAC the raw body in middleware. |
| HMAC compare | Go `crypto/hmac` | Use `hmac.Equal(expected, got)` (constant-time). Never `==` / `bytes.Equal` on MACs. |
| Signature encoding | n/a (internal contract) | Define as **hex** of `HMAC_SHA256(secret, rawBody)` in `X-PS-Signature`. Document so PeopleSoft signs identically. |

---

## Patterns to Mirror

### PROVIDER_GATED_MIDDLEWARE (the seam — mirror this exactly)
```go
// SOURCE: backend/internal/middleware/auth.go:18-43
func Auth(ctx context.Context, cfg *config.Config) (fiber.Handler, error) {
	if !cfg.UsesRealAuth() {
		return MockJWT(cfg.IsDevelopment()), nil
	}
	verifier, err := auth.NewEntraVerifier(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return func(c *fiber.Ctx) error {
		if isUnauthedPath(c.Path()) {
			return c.Next()
		}
		id, err := verifier.Verify(c.UserContext(), bearerToken(c))
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
		}
		// ... set locals ...
		return c.Next()
	}, nil
}
```

### FIBER_401 (how to reject)
```go
// SOURCE: backend/internal/middleware/auth.go:36
return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
// httpx.ErrorHandler turns this into the {success:false,error:"..."} envelope with the 4xx message preserved.
```

### CONFIG_REAL_VALIDATION (fail-fast when real mode lacks a secret)
```go
// SOURCE: backend/pkg/config/config.go:153-157
if c.UsesRealPeopleSoft() {
	if c.PSIBBaseURL == "" || c.PSIBTokenURL == "" || c.PSIBClientID == "" || c.PSIBClientSecret == "" {
		return nil, fmt.Errorf("config: PS_IB_BASE_URL, PS_IB_TOKEN_URL, PS_IB_CLIENT_ID, PS_IB_CLIENT_SECRET are required when PS_PROVIDER=real")
	}
}
```

### CONFIG_FIELD + LOAD (string env, no default for secrets)
```go
// SOURCE: backend/pkg/config/config.go:44-49 (struct) and :114-119 (Load)
PSProvider             string
// ...
PSProvider:             getenv("PS_PROVIDER", "mock"),
PSIBClientSecret:       os.Getenv("PS_IB_CLIENT_SECRET"),   // secrets via os.Getenv (no fallback)
```

### CONFIG_TEST_REQUIRES (mirror for the new validation)
```go
// SOURCE: backend/pkg/config/config_test.go:76-81
t.Setenv("AZURE_BLOB_CONNECTION_STRING", "conn")
t.Setenv("PS_PROVIDER", "real")
if _, err := Load(); err == nil {
	t.Fatal("expected error when PS_PROVIDER=real without IB creds")
}
```

### TEST_HARNESS (extend; note RegisterRoutes gains a secret arg)
```go
// SOURCE: backend/internal/peoplesoft/webhook_test.go:58-73
func testApp(h *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	RegisterRoutes(app, h)        // <-- becomes RegisterRoutes(app, h, "") in mock tests
	return app
}
func post(t *testing.T, app *fiber.App, path, body string) int { /* httptest POST, returns status */ }
```

### LOGGING (zerolog, structured; warn for rejects without leaking the secret)
```go
// SOURCE: backend/internal/peoplesoft/webhook.go:63-64
log.Warn().Str("position_code", req.PositionCode).Str("ps_vacancy_id", req.PSVacancyID).
	Msg("peoplesoft: unmapped position code — storing vacancy unmapped")
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/internal/peoplesoft/webhook_auth.go` | CREATE | `VerifyHMAC(secret) fiber.Handler` middleware |
| `backend/internal/peoplesoft/webhook_auth_test.go` | CREATE | Table tests: valid / invalid / missing / wrong-secret / health-open |
| `backend/internal/peoplesoft/routes.go` | UPDATE | Accept `secret string`; mount middleware on the 3 POSTs (health registered before, stays open) |
| `backend/pkg/config/config.go` | UPDATE | Add `PSWebhookSecret` field + `Load` + real-mode fail-fast validation |
| `backend/pkg/config/config_test.go` | UPDATE | Assert `PS_PROVIDER=real` without `PS_WEBHOOK_SECRET` errors; with it, loads |
| `backend/internal/peoplesoft/webhook_test.go` | UPDATE | Update `testApp` to new `RegisterRoutes` signature (pass `""`) so existing tests still compile/pass |
| `backend/cmd/api/main.go` | UPDATE | Pass `cfg.PSWebhookSecret` into `peoplesoft.RegisterRoutes` |
| `backend/.env.example` | UPDATE | Add `PS_WEBHOOK_SECRET=` near the PS block |
| `docs/SECURITY.md` | UPDATE | Flip the PS "Follow-up" line to resolved; document the `X-PS-Signature` contract |

## NOT Building

- ❌ No change to `/api/v1/ps/health` (stays an open probe).
- ❌ No HR/Entra auth on `/sync-hired` (see Decision — it's a machine endpoint).
- ❌ No replay/nonce store or mTLS — timestamp-tolerance replay protection is documented as **optional future hardening**, not built now (keeps scope tight; the shared-secret HMAC is the documented requirement).
- ❌ No new signing logic in the PS **outbound** client (`rest.go`/`client.go`) — this slice is inbound verification only.
- ❌ No rotation tooling beyond documenting it (SECURITY.md already has a rotation runbook).
- ❌ Not touching the Redis limiter / PDPA / Playwright S7 items.

---

## Step-by-Step Tasks

### Task 1: Add `PSWebhookSecret` to config with real-mode fail-fast
- **ACTION**: Add the field, load it, and require it when `PS_PROVIDER=real`.
- **IMPLEMENT**:
  - Struct (near the PS block, after `PSCSVFallbackContainer`): `PSWebhookSecret string`
  - In `Load()`: `PSWebhookSecret: os.Getenv("PS_WEBHOOK_SECRET"),`
  - In the `if c.UsesRealPeopleSoft()` block, extend validation:
    ```go
    if c.PSWebhookSecret == "" {
        return nil, fmt.Errorf("config: PS_WEBHOOK_SECRET is required when PS_PROVIDER=real")
    }
    ```
    (Add as a separate `if` inside the existing `UsesRealPeopleSoft()` block, or a new line — keep the existing IB-creds check intact.)
- **MIRROR**: CONFIG_FIELD + LOAD, CONFIG_REAL_VALIDATION.
- **IMPORTS**: none new (`fmt`, `os` already imported).
- **GOTCHA**: secrets use `os.Getenv` (no `getenv` default) — never bake a default secret. Keep the existing `PSIB*` validation; just add the secret requirement alongside it.
- **VALIDATE**: `cd backend && go build ./... && go vet ./...`

### Task 2: config_test for the new requirement
- **ACTION**: Add two assertions mirroring `TestLoad_*`.
- **IMPLEMENT**:
  - Extend/clone the real-PS test: set `PS_PROVIDER=real` + the four `PS_IB_*` but **omit** `PS_WEBHOOK_SECRET` → expect error mentioning `PS_WEBHOOK_SECRET`.
  - A positive case: same + `PS_WEBHOOK_SECRET=shh` → `Load()` succeeds and `c.PSWebhookSecret == "shh"`.
  - Remember the always-required `AZURE_BLOB_CONNECTION_STRING`, `DB_URL`, `REDIS_URL` (set them as the other tests do).
- **MIRROR**: CONFIG_TEST_REQUIRES.
- **IMPORTS**: none new.
- **GOTCHA**: use `t.Setenv` (auto-cleanup) exactly like the existing tests; don't leak env between tests.
- **VALIDATE**: `cd backend && go test ./pkg/config/ -run TestLoad -v`

### Task 3: Create the HMAC verification middleware
- **ACTION**: New file `backend/internal/peoplesoft/webhook_auth.go` exporting `VerifyHMAC(secret string) fiber.Handler`.
- **IMPLEMENT**:
  ```go
  package peoplesoft

  import (
  	"crypto/hmac"
  	"crypto/sha256"
  	"encoding/hex"

  	"github.com/gofiber/fiber/v2"
  	"github.com/rs/zerolog/log"
  )

  // signatureHeader is where PeopleSoft sends the hex HMAC-SHA256 of the raw body.
  const signatureHeader = "X-PS-Signature"

  // VerifyHMAC returns middleware that authenticates inbound PeopleSoft webhooks by
  // verifying X-PS-Signature == hex(HMAC-SHA256(secret, rawBody)) in constant time.
  // It is mounted only on the state-changing POST routes (not /health).
  func VerifyHMAC(secret string) fiber.Handler {
  	key := []byte(secret)
  	return func(c *fiber.Ctx) error {
  		got := c.Get(signatureHeader)
  		if got == "" {
  			return fiber.NewError(fiber.StatusUnauthorized, "missing webhook signature")
  		}
  		mac := hmac.New(sha256.New, key)
  		mac.Write(c.Body()) // buffered raw body; does NOT consume — BodyParser still works downstream
  		want := hex.EncodeToString(mac.Sum(nil))
  		if !hmac.Equal([]byte(want), []byte(got)) {
  			log.Warn().Str("path", c.Path()).Msg("peoplesoft: webhook signature mismatch")
  			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook signature")
  		}
  		return c.Next()
  	}
  }
  ```
- **MIRROR**: PROVIDER_GATED_MIDDLEWARE (shape), FIBER_401, LOGGING.
- **IMPORTS**: as shown.
- **GOTCHA**: (1) `hmac.Equal` not `==` (constant-time). (2) Never log the secret or the expected signature. (3) `c.Body()` is the **buffered** body — calling it here does not prevent `c.BodyParser` in the handler (Fiber re-reads from the same buffer); confirmed in Task 6 by existing handler tests passing. (4) Compare hex strings via `hmac.Equal` on their bytes; differing lengths return false safely.
- **VALIDATE**: covered by Task 4 tests + `go build ./...`.

### Task 4: Table-driven tests for the middleware
- **ACTION**: New `backend/internal/peoplesoft/webhook_auth_test.go`.
- **IMPLEMENT**: Build an app with the secret enforced, then assert:
  - valid signature → handler runs (200)
  - wrong secret used to sign → 401
  - tampered body (sign A, send B) → 401
  - missing `X-PS-Signature` → 401
  - `GET /health` with no signature → 200 (open)
  Add a signing helper:
  ```go
  func sign(secret, body string) string {
  	m := hmac.New(sha256.New, []byte(secret))
  	m.Write([]byte(body))
  	return hex.EncodeToString(m.Sum(nil))
  }
  ```
  Drive requests with `httptest.NewRequest` setting the header (reuse the `post` style but add a header variant). Use a fake repo (`&fakeVac{}`, `fakePos{}`) and `RegisterRoutes(app, h, "testsecret")`.
- **MIRROR**: TEST_HARNESS.
- **IMPORTS**: `crypto/hmac`, `crypto/sha256`, `encoding/hex`, `net/http/httptest`, `strings`, `testing`, fiber, httpx.
- **GOTCHA**: a valid-signature POST will reach the handler, which calls the fake repo — make sure the fake returns nil (it does). Sign the **exact** body string you send (byte-identical), or the MAC won't match.
- **VALIDATE**: `cd backend && go test ./internal/peoplesoft/ -run HMAC -v`

### Task 5: Mount the middleware in routes.go (health stays open)
- **ACTION**: Change `RegisterRoutes` to accept the secret and guard only the POSTs.
- **IMPLEMENT**:
  ```go
  // RegisterRoutes mounts the PeopleSoft integration endpoints. When secret != ""
  // the state-changing POST webhooks require a valid X-PS-Signature (HMAC-SHA256).
  func RegisterRoutes(app *fiber.App, h *Handler, secret string) {
  	ps := app.Group("/api/v1/ps")
  	ps.Get("/health", h.Health) // open probe — registered before the guard
  	if secret != "" {
  		ps.Use(VerifyHMAC(secret))
  	}
  	ps.Post("/vacancy-opened", h.VacancyOpened)
  	ps.Post("/vacancy-closed", h.VacancyClosed)
  	ps.Post("/sync-hired", h.SyncHired)
  }
  ```
- **MIRROR**: existing routes.go structure.
- **IMPORTS**: unchanged (fiber).
- **GOTCHA**: `ps.Use(...)` applies only to routes registered **after** it — register `/health` first so it stays open. When `secret == ""` (mock/CI), no guard is mounted (preserves current behavior + existing tests). The app-level `middleware.Auth` already skips `/api/v1/ps` (auth.go:49), so this HMAC guard is the only auth on the group — correct.
- **VALIDATE**: `cd backend && go build ./...`

### Task 6: Fix the existing webhook_test harness for the new signature
- **ACTION**: Update `testApp` to call `RegisterRoutes(app, h, "")` so the existing handler tests compile and stay open (no signing needed).
- **IMPLEMENT**: change line 60 `RegisterRoutes(app, h)` → `RegisterRoutes(app, h, "")`.
- **MIRROR**: TEST_HARNESS.
- **IMPORTS**: none.
- **GOTCHA**: This proves the "no secret = open" path AND that `BodyParser` is unaffected (these tests POST JSON and assert 200). Keep them passing — they're the regression guard for the Fiber `c.Body()` gotcha.
- **VALIDATE**: `cd backend && go test ./internal/peoplesoft/ -v`

### Task 7: Wire the secret in main.go
- **ACTION**: Pass `cfg.PSWebhookSecret` to `RegisterRoutes`.
- **IMPLEMENT**: `peoplesoft.RegisterRoutes(app, peoplesoft.NewHandler(vacancyRepo, positionRepo, psService, cfg.PSProvider, reengageTrigger), cfg.PSWebhookSecret)` (main.go:171).
- **MIRROR**: existing call site.
- **IMPORTS**: none.
- **GOTCHA**: in real mode config validation already guarantees the secret is non-empty (Task 1), so the guard is always mounted in prod; in mock it's `""` → open. No nil/empty surprises.
- **VALIDATE**: `cd backend && go build ./...`

### Task 8: `.env.example` + docs/SECURITY.md
- **ACTION**: Document the new env var and the signature contract; flip the follow-up.
- **IMPLEMENT**:
  - `.env.example` (near the PS block, ~after `PS_IB_*`): add `PS_WEBHOOK_SECRET=` with a one-line comment `# required when PS_PROVIDER=real; PeopleSoft signs each webhook body: X-PS-Signature: hex(HMAC-SHA256(secret, rawBody))`.
  - `docs/SECURITY.md`: change the "Follow-up" bullet (lines 18-19) to a resolved bullet, e.g. *"PeopleSoft webhooks (`/api/v1/ps/*` POSTs) are authenticated with an `X-PS-Signature` HMAC-SHA256 of the raw body (`PS_WEBHOOK_SECRET`), constant-time verified; `/health` stays open. (Sprint 7)"*. Note replay protection as a remaining optional hardening.
- **MIRROR**: existing `.env.example` comment style + SECURITY.md bullet style.
- **IMPORTS**: n/a.
- **GOTCHA**: keep `PS_WEBHOOK_SECRET=` **empty** in `.env.example` (never commit a real/sample secret that could be mistaken for valid).
- **VALIDATE**: visual; `grep PS_WEBHOOK_SECRET backend/.env.example`.

---

## Testing Strategy

### Unit Tests

| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `TestVerifyHMAC_ValidSignature` | body signed with the configured secret | 200 (handler runs) | no |
| `TestVerifyHMAC_WrongSecret` | body signed with a different secret | 401 `invalid webhook signature` | yes |
| `TestVerifyHMAC_TamperedBody` | sign body A, send body B | 401 | yes |
| `TestVerifyHMAC_MissingHeader` | no `X-PS-Signature` | 401 `missing webhook signature` | yes |
| `TestVerifyHMAC_HealthOpen` | `GET /health`, no signature | 200 | yes |
| `TestLoad_RealPeopleSoftRequiresWebhookSecret` | `PS_PROVIDER=real`, no `PS_WEBHOOK_SECRET` | `Load()` error | yes |
| `TestLoad_RealPeopleSoftWithWebhookSecret` | real + secret set | loads, field populated | no |
| existing `TestVacancyOpened_*` / `TestVacancyClosed` | mock mode (`secret=""`) | unchanged 200/400 | regression |

### Edge Cases Checklist
- [ ] Missing signature header → 401 (not 500)
- [ ] Empty body POST with valid signature over empty body → handler runs (then 400 from existing payload validation — that's fine, auth passed)
- [ ] Wrong-length / non-hex signature → 401 (hmac.Equal on differing bytes is false, safe)
- [ ] `/health` never requires a signature
- [ ] Mock mode (`secret==""`) → all routes open, existing tests green
- [ ] `BodyParser` still parses after `c.Body()` in middleware (proven by existing handler tests)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./...
```
EXPECT: exit 0.

### Unit Tests (affected packages)
```bash
cd backend && go test ./internal/peoplesoft/ ./pkg/config/ -v
```
EXPECT: all pass, including new HMAC + config tests and unchanged existing tests.

### Full Suite + Lint + Security (mirror CI)
```bash
cd backend && go test ./... && golangci-lint run ./... && gosec -exclude-generated ./...
```
EXPECT: tests ok, `0 issues.`, gosec exit 0. (govulncheck unaffected.)

### Build
```bash
cd backend && go build ./...
```
EXPECT: exit 0.

### Manual Validation (optional, against the live stack)
```bash
# make up && make migrate-up first; run api with PS_WEBHOOK_SECRET=shh PS_PROVIDER=mock (or set secret to force enforce)
SECRET=shh; BODY='{"ps_vacancy_id":"V-1","headcount":1}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$SECRET" -hex | sed 's/^.* //')
# valid → 200
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/api/v1/ps/vacancy-opened \
  -H "Content-Type: application/json" -H "X-PS-Signature: $SIG" -d "$BODY"
# missing sig → 401
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/api/v1/ps/vacancy-opened \
  -H "Content-Type: application/json" -d "$BODY"
```
EXPECT: 200 then 401. (Note: enforcement requires the api process to be started with a non-empty `PS_WEBHOOK_SECRET`.)

---

## Acceptance Criteria
- [ ] All three `/api/v1/ps` POSTs reject missing/invalid `X-PS-Signature` with 401 when a secret is configured
- [ ] Valid HMAC over the raw body → 200 and the handler runs (BodyParser intact)
- [ ] `GET /api/v1/ps/health` stays open
- [ ] `PS_PROVIDER=real` without `PS_WEBHOOK_SECRET` fails fast at startup
- [ ] Mock/CI (no secret) behavior unchanged — existing tests + e2e green
- [ ] `go test ./... && golangci-lint run && gosec` all green; build clean

## Completion Checklist
- [ ] Constant-time compare (`hmac.Equal`), secret never logged
- [ ] Middleware mirrors the `middleware.Auth` seam shape + `fiber.NewError` 401s
- [ ] Config validation mirrors the existing `UsesRealPeopleSoft()` block
- [ ] Tests follow the table-driven `webhook_test.go` harness
- [ ] `.env.example` + `docs/SECURITY.md` updated; no committed secret
- [ ] No scope creep (no replay store, no outbound signing, no HR auth on sync-hired)
- [ ] Self-contained — no further codebase searching needed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `c.Body()` in middleware breaks downstream `BodyParser` | Low | High | Fiber buffers the body; the unchanged handler tests (Task 6) are the regression guard — they POST JSON and must still 200 |
| Signature encoding mismatch with PeopleSoft (hex vs base64, body whitespace) | Medium | High (real integration 401s) | Pin the contract in `docs/SECURITY.md` (hex, raw body, no re-serialization); provide the `openssl` example so PS signs identically |
| `RegisterRoutes` signature change misses a caller | Low | Med (build break) | Only two callers (main.go, webhook_test.go); both updated in Tasks 6-7; `go build ./...` catches any miss |
| Someone sets `PS_WEBHOOK_SECRET` in mock and breaks local e2e | Low | Low | e2e/CI run `PS_PROVIDER=mock` with no secret → guard not mounted; document that setting the secret enforces it |
| Replay attack (captured valid request resent) | Low (shared secret + TLS) | Med | Out of scope this slice; documented as optional timestamp-tolerance hardening follow-up |

## Notes
- **Why HMAC over mTLS / IP allowlist**: HMAC shared-secret is what `docs/SECURITY.md` prescribes, is symmetric-simple for the PS Integration Broker to implement, and needs no infra changes. mTLS/IP-allowlist are deployment concerns that can layer on later.
- **Contract for the PeopleSoft team**: `X-PS-Signature: <lowercase hex of HMAC-SHA256(PS_WEBHOOK_SECRET, exact-raw-request-body)>`. The body must be signed byte-for-byte as sent (no re-serialization).
- **PR mechanics**: branch `feat/s7-ps-webhook-hmac`, NO attribution, squash-merge. Backend-only; CI is now green so it should merge without `--admin`.
- **Suggested commit**: `feat(sprint-7): HMAC auth on PeopleSoft webhooks (X-PS-Signature)`.
- This closes the `docs/SECURITY.md` PS follow-up; after merge that bullet is resolved.
