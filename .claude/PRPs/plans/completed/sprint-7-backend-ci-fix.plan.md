# Plan: Sprint 7 — Backend CI Fix (Lint + govulncheck green on main)

## Summary
The backend CI jobs `build-and-test` (Lint step) and `security` (govulncheck step) are red on `main`, forcing every frontend PR to be admin-merged. This plan makes both jobs green by (a) moving golangci-lint to v2 via `golangci-lint-action@v8` with a pinned, Go-1.26-built version, and (b) clearing all 9 govulncheck findings by bumping the Go toolchain to **1.26.4** (fixes 8 stdlib CVEs) and `golang.org/x/net` to **v0.55.0** (fixes the one module CVE).

## User Story
As a developer on the ATS repo, I want the backend `build-and-test` and `security` CI jobs to pass on `main`, so that PRs merge on green checks without `--admin` overrides.

## Problem → Solution
Backend CI red on `main` (golangci-lint toolchain mismatch + 9 govulncheck vulns) → both jobs green via golangci-lint v2 action upgrade + Go 1.26.4 toolchain + `golang.org/x/net` v0.55.0.

## Metadata
- **Complexity**: Small
- **Source PRD**: N/A (standalone Sprint 7 slice)
- **PRD Phase**: Sprint 7 — Backend CI fix
- **Estimated Files**: 3 (`.github/workflows/ci.yml`, `backend/go.mod`, `backend/go.sum`) + 1 optional (`backend/.golangci.yml`)

---

## ⚠️ Reality Update vs. Memory `ats-backend-ci-broken-s7.md`

The memory (written 2026-05-30) is **partially stale** — re-reproduced live on 2026-06-03 with local `go1.26.1` + `golangci-lint v2.11.3`:

| Memory claim (2026-05-30) | Current reality (2026-06-03) | Source of change |
|---|---|---|
| Lint fails: golangci-lint `v1.64.8` (go1.24) can't load `go 1.26.1` config | Still the root cause. golangci-lint is now at **v2.x**; v2 needs `action@v7+`. v2.11.3 reports **0 issues** on this code. | golangci-lint v2 GA |
| govulncheck flags **1** vuln via `idna.ToASCII` (x/net transitive) | govulncheck now flags **9** vulns: **8 Go stdlib** (crypto/x509, crypto/tls, net, net/http, net/textproto) + **1 module** `golang.org/x/net@v0.54.0`. The `azure_parser.go:88` trace now maps to stdlib TLS/HTTP2 CVEs, not idna. | New Go patch releases + newly disclosed CVEs |

**Implication**: the durable fix is **upgrade the Go toolchain patch (1.26.1 → 1.26.4) + bump `golang.org/x/net` (v0.54.0 → v0.55.0)** — NOT an idna allowlist. No `govulncheck -show` allowlist is needed; all 9 are fixed by available releases.

### Live-reproduced evidence
```
$ golangci-lint version
golangci-lint has version 2.11.3 built with go1.26.1 ...
$ golangci-lint run ./...
0 issues.                       # ← v2 is clean on this codebase

$ go version
go version go1.26.1 darwin/arm64
$ govulncheck ./...             # 9 CALLED vulns:
#1 GO-2026-5039 net/textproto  stdlib  fixed go1.26.4
#2 GO-2026-5037 crypto/x509    stdlib  fixed go1.26.4
#3 GO-2026-5026 golang.org/x/net@v0.54.0  MODULE  fixed v0.55.0
#4 GO-2026-4971 net            stdlib  fixed go1.26.3
#5 GO-2026-4947 crypto/x509    stdlib  fixed go1.26.2
#6 GO-2026-4946 crypto/x509    stdlib  fixed go1.26.2
#7 GO-2026-4918 net/http       stdlib  fixed go1.26.3   (azure_parser.go:88)
#8 GO-2026-4870 crypto/tls     stdlib  fixed go1.26.2   (azure_parser.go:88)
#9 GO-2026-4866 crypto/x509    stdlib  fixed go1.26.2
```
go1.26.4 fixes #1,#2,#4,#5,#6,#7,#8,#9 (patch fixes are cumulative). v0.55.0 fixes #3. → 0 remaining.

---

## UX Design
Internal/CI change — no user-facing UX transformation.

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 (critical) | `.github/workflows/ci.yml` | 28–104 | The 3 jobs; Lint step (43–47), all `setup-go` blocks (30–33, 65–68, 91–94), govulncheck step (95–99) |
| P0 (critical) | `backend/go.mod` | 1–3, 39 | `go 1.26.1` directive (no `toolchain` line); `golang.org/x/net v0.54.0 // indirect` |
| P2 (reference) | `backend/internal/ai/azure_parser.go` | 88 | The `a.http.Do(req)` call govulncheck traces for the net/http + crypto/tls stdlib CVEs (fixed by toolchain bump, no code change) |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| golangci-lint-action v2 support | https://github.com/golangci/golangci-lint-action | `@v6` targets golangci-lint **v1**; golangci-lint **v2** requires `@v7`+ (current is `@v8`). Use `version:` pinned to a v2 tag. |
| golangci-lint v2 config | https://golangci-lint.run/ | v2 runs a sane default linter set with **no config file**; a `.golangci.yml` is optional. If added it MUST start with `version: "2"`. |
| Go vuln DB | https://pkg.go.dev/vuln/ | A vuln only lists a "Fixed in" version once that patch is released, so go1.26.4 is confirmed available. |
| setup-go version resolution | https://github.com/actions/setup-go | `go-version: "1.26.4"` installs exactly that patch (deterministic); `"1.26"` floats to newest 1.26.x. |

---

## Patterns to Mirror

### CI_JOB_SETUP_GO (the block repeated in all 3 jobs — keep them identical)
```yaml
# SOURCE: .github/workflows/ci.yml:30-33 (also 65-68, 91-94)
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
          cache-dependency-path: backend/go.sum
```

### CI_LINT_STEP (the step to upgrade)
```yaml
# SOURCE: .github/workflows/ci.yml:43-47
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          working-directory: backend
```

### GO_MOD_HEADER
```
# SOURCE: backend/go.mod:1-3
module github.com/nexto/hr-ats

go 1.26.1
```

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `.github/workflows/ci.yml` | UPDATE | Pin `go-version: "1.26.4"` in all 3 `setup-go` blocks; upgrade Lint to `golangci-lint-action@v8` with pinned v2 version |
| `backend/go.mod` | UPDATE | Add `toolchain go1.26.4`; bump `golang.org/x/net` to `v0.55.0` |
| `backend/go.sum` | UPDATE | Regenerated by `go get`/`go mod tidy` (do not hand-edit) |
| `backend/.golangci.yml` | CREATE (optional) | Pin `version: "2"` + explicit default linters so a future floating action `version` can't silently change the linter set |

## NOT Building

- ❌ No idna allowlist / `govulncheck` suppression config (all vulns have real fixes).
- ❌ No source-code changes to `azure_parser.go` or any Go file (the `:88` trace is fixed by the toolchain bump).
- ❌ No changes to the `e2e` job (already green) beyond the shared `setup-go` pin.
- ❌ No changes to `gosec` or `pnpm audit` steps (not reported broken; verify-only).
- ❌ Not touching frontend CI, Playwright-in-CI, PS-webhook HMAC, PDPA sweep, or the Redis limiter (separate S7 slices).

---

## Step-by-Step Tasks

### Task 1: Bump Go toolchain + x/net in the backend module
- **ACTION**: In `backend/`, raise the toolchain and the vulnerable module, then tidy.
- **IMPLEMENT**:
  ```bash
  cd backend
  go get go@1.26.4                      # adds/updates `go` + `toolchain` directives
  go get golang.org/x/net@v0.55.0       # fixes GO-2026-5026
  go mod tidy
  ```
  Expected `go.mod` result: `go 1.26.4` (or retains `go 1.26.1` plus a new `toolchain go1.26.4` line — either is acceptable as long as the toolchain ≥ 1.26.4) and `golang.org/x/net v0.55.0 // indirect`.
- **MIRROR**: GO_MOD_HEADER.
- **IMPORTS**: none.
- **GOTCHA**: `golang.org/x/net` is **indirect** (`go mod why` says the main module doesn't import it directly — it's pulled transitively). `go get golang.org/x/net@v0.55.0` still pins it in `go.sum`; do NOT add it to the direct `require` block by hand. If `go mod tidy` drops it back to v0.54.0 because a dependency caps it, instead add an explicit `require golang.org/x/net v0.55.0` and re-run tidy (a forced minimum-version bump). Verify with the VALIDATE step which version actually lands.
- **VALIDATE**:
  ```bash
  grep "golang.org/x/net" go.mod          # expect v0.55.0
  go build ./... && go vet ./...          # expect clean (baseline already clean)
  go test ./... -cover                    # expect pass (unit; no integration tag)
  ```

### Task 2: Clear govulncheck locally
- **ACTION**: Re-run govulncheck with the bumped toolchain to confirm 0 findings.
- **IMPLEMENT**:
  ```bash
  cd backend
  export PATH="$PATH:$(go env GOPATH)/bin"
  go install golang.org/x/vuln/cmd/govulncheck@latest
  GOTOOLCHAIN=go1.26.4 govulncheck ./...
  ```
- **MIRROR**: mirrors the CI `security` step (ci.yml:95–99).
- **IMPORTS**: none.
- **GOTCHA**: govulncheck reports against the **toolchain that builds it**. Local default is `go1.26.1`, so you MUST run under 1.26.4 (`GOTOOLCHAIN=go1.26.4`, or after `go get go@1.26.4` which makes `auto` fetch it) or the 8 stdlib vulns will still show. In CI this is controlled by `setup-go` (Task 3).
- **VALIDATE**: output ends with `No vulnerabilities found.` (exit 0).

### Task 3: Pin Go 1.26.4 across all three CI jobs
- **ACTION**: In `.github/workflows/ci.yml`, change `go-version: "1.26"` → `go-version: "1.26.4"` in **all three** `setup-go` blocks (build-and-test ~line 31, e2e ~line 66, security ~line 92).
- **IMPLEMENT**: replace each occurrence:
  ```yaml
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26.4"
          cache-dependency-path: backend/go.sum
  ```
- **MIRROR**: CI_JOB_SETUP_GO.
- **IMPORTS**: none.
- **GOTCHA**: there are **3** identical blocks — update all 3 so the `security` job's govulncheck builds with 1.26.4 stdlib (the actual fix for 8 of 9 vulns). Missing the `security` block leaves it red. Pin the exact patch (not `"1.26"`) so the result is deterministic and doesn't regress when a newer-but-not-yet-patched 1.26.x appears.
- **VALIDATE**: `grep -n 'go-version' .github/workflows/ci.yml` → 3 lines, all `"1.26.4"`.

### Task 4: Upgrade the Lint step to golangci-lint v2
- **ACTION**: In `.github/workflows/ci.yml` Lint step (~43–47), bump the action major to `@v8` and pin a v2 golangci-lint version built with Go 1.26.
- **IMPLEMENT**:
  ```yaml
      - name: Lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: v2.11.3
          working-directory: backend
  ```
- **MIRROR**: CI_LINT_STEP (the block being replaced).
- **IMPORTS**: none.
- **GOTCHA**: `golangci-lint-action@v6` only knows how to run golangci-lint **v1** — leaving `@v6` while requesting a v2 `version:` will fail. Must move to `@v8` (or `@v7`). Pin `version: v2.11.3` (a build made with go1.26.1, verified locally) rather than `latest` so a future v2.x built with an older Go can't reintroduce the toolchain mismatch. Do NOT keep `version: latest`.
- **VALIDATE**: locally `cd backend && golangci-lint run ./...` → `0 issues.` (matches what CI will run).

### Task 5 (OPTIONAL hardening): Pin a v2 golangci config
- **ACTION**: Add `backend/.golangci.yml` so the linter set is explicit and immune to action/version default drift.
- **IMPLEMENT**:
  ```yaml
  version: "2"
  linters:
    default: standard   # errcheck, govet, ineffassign, staticcheck, unused
  ```
- **MIRROR**: n/a (new file; follows golangci-lint v2 schema).
- **IMPORTS**: none.
- **GOTCHA**: a v2 config **must** declare `version: "2"` on line 1 or golangci-lint errors. Keep it minimal — `default: standard` reproduces today's no-config behavior (verified 0 issues). Do not enable extra linters in this slice (out of scope; could surface new findings and balloon the PR).
- **VALIDATE**: `cd backend && golangci-lint run ./...` → still `0 issues.`

---

## Testing Strategy

### Unit Tests
No new product code, so no new unit tests. Existing suite must still pass after the dependency bump.

| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| `go test ./... -cover` | bumped go.mod/go.sum | all pass, no regression | dependency-bump regression |
| `golangci-lint run ./...` | v2.11.3 | `0 issues.` | new-linter false positives |
| `govulncheck ./...` under go1.26.4 | bumped toolchain + x/net | `No vulnerabilities found.` | a vuln with no released fix |

### Edge Cases Checklist
- [ ] `go mod tidy` keeps `golang.org/x/net` at v0.55.0 (didn't get downgraded by a transitive cap)
- [ ] `go.sum` has no stale/duplicate x/net entries after tidy
- [ ] All 3 `setup-go` blocks pinned (esp. the `security` job)
- [ ] gosec step still passes under go1.26.4 (verify; not assumed broken)
- [ ] `migrate up/down` round-trip still runs (build-and-test job tail, unaffected by these changes)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go vet ./...
```
EXPECT: zero output, exit 0.

### Lint (mirrors CI Lint step)
```bash
cd backend && golangci-lint run ./...
```
EXPECT: `0 issues.`

### Unit Tests
```bash
cd backend && go test ./... -cover
```
EXPECT: all packages `ok` / `no test files`, no `FAIL`.

### Security (mirrors CI security job)
```bash
cd backend && export PATH="$PATH:$(go env GOPATH)/bin"
GOTOOLCHAIN=go1.26.4 govulncheck ./...
go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec -exclude-generated ./...
```
EXPECT: `No vulnerabilities found.` + gosec exit 0.

### Build Verification
```bash
cd backend && go build ./...
```
EXPECT: exit 0.

### CI Validation (the real acceptance gate)
```bash
# On a feat/ branch, after pushing:
gh pr checks <PR#> --watch
```
EXPECT: `build-and-test`, `e2e`, and `security` all ✓ green — no `--admin` override needed.

### Manual Validation
- [ ] Open the PR; confirm all three backend jobs are green on the PR's checks tab.
- [ ] Confirm the `security` job log shows govulncheck `No vulnerabilities found.` (not skipped).
- [ ] Confirm the Lint job log shows golangci-lint `v2.11.3` running (correct version actually used).

---

## Acceptance Criteria
- [ ] `build-and-test` job green (Lint step passes with golangci-lint v2)
- [ ] `security` job green (govulncheck reports 0 vulnerabilities)
- [ ] `e2e` job still green (unaffected)
- [ ] No source-code (`.go`) changes; only `ci.yml`, `go.mod`, `go.sum` (+ optional `.golangci.yml`)
- [ ] A subsequent frontend PR merges on green checks without `--admin`

## Completion Checklist
- [ ] All 3 `setup-go` blocks pinned to `1.26.4`
- [ ] Lint uses `golangci-lint-action@v8` + pinned `v2.11.3`
- [ ] `golang.org/x/net` is `v0.55.0` in go.mod and go.sum
- [ ] `go.mod` toolchain ≥ 1.26.4
- [ ] Local `go vet` / `go test` / `golangci-lint run` / `govulncheck` all clean
- [ ] No hardcoded values; no scope creep beyond CI fix
- [ ] Update memory `ats-backend-ci-broken-s7.md` once merged (mark resolved) or delete it

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| New govulncheck CVE lands between local check and CI run | Medium | Med (security job flaky-red) | govulncheck always uses latest DB; if a new stdlib CVE appears, bump to the next Go patch. Accept this is an ongoing maintenance reality, not a one-time fix. |
| `go mod tidy` downgrades x/net back to v0.54.0 via a transitive cap | Low | High (vuln returns) | Add explicit `require golang.org/x/net v0.55.0`; re-verify with `grep` in Task 1 VALIDATE. |
| golangci-lint v2 default linters differ from v1 → new findings | Low (verified 0 locally) | Med (lint red) | Already reproduced `0 issues` locally with v2.11.3; optional `.golangci.yml` (Task 5) pins the set. |
| `gosec` regresses under go1.26.4 | Low | Med | Validation step runs gosec explicitly before pushing. |
| go1.26.4 not on GitHub runners yet | Very Low | Med | Vuln DB lists it as "Fixed in" (= released); setup-go fetches exact patches. Fallback: `"1.26"` floating. |

## Notes
- **Why this supersedes the memory**: the idna/`ToASCII` finding from 2026-05-30 is no longer how govulncheck traces the issue; new Go patch releases (1.26.2/1.26.3/1.26.4) and freshly disclosed CVEs reshaped the set. The correct durable fix is "stay current on the Go patch + the one vulnerable module," which this plan does. After merge, update or delete `ats-backend-ci-broken-s7.md`.
- **PR mechanics** (from session notes): single private repo `nextocompany/ats`, branch `feat/…`, commits use **NO attribution**, squash-merge. This PR touches **0 frontend files** and only backend CI/deps, so once green it merges normally (no `--admin`).
- **Local dev note**: developers on `go1.26.1` will have `GOTOOLCHAIN=auto` fetch `go1.26.4` automatically once `go.mod` requires it — no manual SDK install needed.
- **Suggested commit**: `ci: fix backend lint + govulncheck on main (golangci-lint v2, go 1.26.4, x/net v0.55.0)`
