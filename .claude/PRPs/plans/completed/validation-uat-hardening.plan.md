# Plan: Validation & UAT hardening (parsing / scoring / branch / RBAC / load)

## Summary
Prove the already-built AI pipeline meets acceptance quality on **real, varied data**:
measure CV-parsing accuracy on 20–30 labeled real CVs, validate AI-scoring-vs-JD
agreement, confirm branch-assignment correctness, verify the RBAC access matrix
end-to-end, and run the load test on staging. This is a **measurement + UAT** effort
— minimal production code; the deliverables are a labeled CV test-set spec, an
opt-in accuracy harness, expanded branch/RBAC tests, and a scored validation runbook.

## User Story
As the **delivery owner**, I want evidence that parsing, scoring, branch assignment,
access control, and throughput meet agreed thresholds on real CP Axtra data, so that
the POC can be signed off with numbers, not assertions.

## Problem → Solution
Parsing/scoring/branch/RBAC are implemented and unit-tested with mocks, but never
measured on real varied CVs at acceptance thresholds, and load was never run. → A
labeled real-CV set + an accuracy harness + a per-role RBAC matrix + a load run
produce a scorecard against explicit pass thresholds.

## Metadata
- **Complexity**: Medium (test/measurement-heavy; little new production code)
- **Source PRD**: `.claude/PRPs/plans/delivery-scope-roadmap.md` (PRP-3)
- **PRD Phase**: PRP-3 (P1) — **depends on PRP-2** (bulk path feeds the CV set + load)
- **Estimated Files**: ~10 (harness + tests + docs; fixtures are gitignored)

---

## Prerequisites (operator/stakeholder inputs — not code)
1. **20–30 real CVs** spanning the variety matrix (below), placed in a **gitignored**
   `e2e/cvset/` dir, each with a `*.expected.json` ground-truth file (PII — never commit).
2. A **staging stack with real Azure AI** (`AI_PROVIDER=azure`, DocIntel + OpenAI
   configured) — mocks cannot validate real accuracy.
3. Stakeholder sign-off on the **pass thresholds** in the runbook (initial defaults provided).

> GOTCHA: Real CVs contain PII. The cvset dir MUST be gitignored; run only on
> staging; purge with the cleanup query after. Never load the set into prod hr_db.

---

## Current state (verified)
- **Parsing**: Azure DocIntel OCR + GPT-4o parser, Thai/English prompt; OCR
  confidence persisted; `<0.7` flags manual review (`pipeline/process.go:152`).
  Parsed fields: `internal/ai/profile.go` (Personal/Experience/Education/Skills/Languages).
- **Scoring**: hybrid rule+LLM vs Master JD; 5-dim breakdown (exp30/skills20/edu10/
  lang10/loc20); must-have gate (`internal/scoring/`). Unit-tested (`scoring_test.go`).
- **Branch**: rule-based geospatial nearest-store + talent-pool fallback
  (`internal/branch/assigner.go`). Unit-tested (`assigner_test.go`).
- **RBAC**: 7 roles → all/subregion/store scope (`internal/rbac/scope.go`).
  Unit-tested (`scope_test.go`).
- **Load**: k6 + drain method delivered in PRP-2 (`loadtest/`), not yet run.
- **e2e**: build-tagged harness with upload + async-poll helpers (`e2e/harness_test.go`).

---

## CV variety matrix (the test-set must cover)
| Dimension | Variants to include |
|---|---|
| Language | Thai-only, English-only, mixed TH/EN |
| File type | PDF (digital), PDF (scanned image), DOCX, JPG/PNG photo |
| Layout | single-column, two-column, table-heavy, photo+sidebar |
| Length | 1-page, 2–3 page |
| Quality | clean, low-res scan, rotated/skewed |
| Content | fresh grad (little exp), experienced, career-changer, missing email/phone |

Aim for ≥20, ideally 30, distributed across the above (not all in one bucket).

---

## Mandatory Reading
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/e2e/harness_test.go` | all | Upload + async-poll helpers + DB access to mirror for the accuracy test |
| P0 | `backend/e2e/full_flow_test.go` | all | End-to-end flow shape (intake→poll→assert) to mirror |
| P0 | `backend/internal/ai/profile.go` | all | Parsed field schema = ground-truth JSON shape to diff against |
| P0 | `backend/internal/applications/bulk_handler.go` | all | Bulk endpoint to feed the CV set (or single intake per file) |
| P1 | `backend/internal/scoring/scoring_test.go` | all | Scoring assertions to extend (breakdown sums, must-have, ranges) |
| P1 | `backend/internal/scoring/rules.go` | 20-136 | Thai/English education + language scoring — accuracy expectations |
| P1 | `backend/internal/branch/assigner_test.go` | all | Branch test table to extend with known province→store cases |
| P1 | `backend/internal/branch/assigner.go` | 35-124 | Assignment + LocationScore logic being validated |
| P1 | `backend/internal/rbac/scope_test.go` | all | RBAC scope cases — confirm matrix coverage, fill gaps |
| P1 | `backend/internal/rbac/scope.go` | all | The role→scope rules under test |
| P2 | `backend/internal/applications/repository.go` | ExistsInScope + List | Per-record + list scoping enforced for the RBAC E2E checks |
| P2 | `loadtest/README.md` | all | Load run + drain-measurement procedure to execute |
| P2 | `backend/pkg/config/config.go` | AI provider flags | `UsesAzureAI()` etc — the harness must assert real provider on staging |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Field-match metrics | (internal) | Use normalized exact-match for name/phone/email; fuzzy/contains for skills/education; report per-field % + macro average |
| Azure DocIntel confidence | Azure docs | Confidence is per-page/line; the pipeline already averages it — use it as a parse-quality signal, not a pass/fail by itself |

> No external libraries needed — accuracy diffing is plain Go string/slice comparison.

---

## Patterns to Mirror

### E2E_UPLOAD_POLL (mirror for the accuracy harness)
```go
// SOURCE: backend/e2e/harness_test.go + full_flow_test.go
// 1. POST multipart resume to the API; 2. poll the applications table by id until
//    status ∈ {scored,rejected,parsed,failed}; 3. read parsed_profile / ai_score.
//go:build e2e
pool := mustPool(t)
appID := uploadResume(t, apiBase(), positionID, cvBytes, filename)   // helper style
waitForTerminal(t, pool, appID, 90*time.Second)
```

### BRANCH_TABLE_TEST (extend)
```go
// SOURCE: backend/internal/branch/assigner_test.go
tests := []struct{ name, province string; wantStore *int; wantTalentPool bool }{ ... }
```

### RBAC_SCOPE_TEST (extend / confirm)
```go
// SOURCE: backend/internal/rbac/scope_test.go
// Each role → expected Kind() + ApplicationsClause/CandidatesClause shape.
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/e2e/accuracy_test.go` | CREATE | Opt-in (build tag `e2e` + `CVSET_DIR` env) harness: upload each labeled CV, poll, diff parsed vs expected, print scorecard |
| `backend/e2e/cvset/.gitignore` | CREATE | Ignore the real CV set + expected JSON (PII) |
| `backend/e2e/scorecard.go` | CREATE | Field-match scoring + scorecard formatting (pure, unit-testable) |
| `backend/e2e/scorecard_test.go` | CREATE | Unit-test the diff/metrics logic with synthetic data (no real CVs) |
| `backend/internal/branch/assigner_test.go` | UPDATE | Add known province→store + talent-pool cases (real subregion logic) |
| `backend/internal/rbac/scope_test.go` | UPDATE | Fill any role-matrix gaps (all 7 roles × applications/candidates clauses) |
| `backend/internal/scoring/scoring_test.go` | UPDATE | Assert breakdown sums to total, dim caps, must-have gate edges |
| `docs/uat/cv-test-set.md` | CREATE | Variety matrix + ground-truth JSON schema + how to assemble the set |
| `docs/uat/validation-runbook.md` | CREATE | Full UAT procedure, pass thresholds, capture templates (parse/score/branch/RBAC/load) |
| `docs/uat/rbac-matrix.md` | CREATE | Per-role expected visibility/actions checklist for E2E sign-off |

## NOT Building
- No changes to parsing/scoring/branch/RBAC **logic** (this PRP measures; fixes, if
  any, become their own follow-up tickets from the findings).
- No committing real CVs or PII (fixtures gitignored).
- No new UI.
- No CI gating on the accuracy run (it needs real Azure + a private CV set) — it's an
  operator artifact, like the load test.
- No re-implementation of the load test (reuse PRP-2's `loadtest/`).

---

## Step-by-Step Tasks

### Task 1: Ground-truth schema + scorecard logic
- **ACTION**: Define the `*.expected.json` shape (subset of `ai.Profile` worth grading) and a pure `scorecard.go` that diffs a parsed profile against expected and returns per-field match + aggregates.
- **IMPLEMENT**: Exact-match (normalized: trim, lowercase, strip spaces/dashes for phone) for name/phone/email; presence + fuzzy/contains for skills/languages/education level; counts → per-field % + macro average + OCR-confidence stats.
- **MIRROR**: `internal/ai/profile.go` field names.
- **GOTCHA**: Normalize Thai/English + whitespace before comparing; phone digits only; email lowercase.
- **VALIDATE**: `scorecard_test.go` with synthetic parsed/expected pairs (no real CVs) → `go test ./e2e/... ` (non-e2e build still compiles scorecard via a normal file; keep scorecard.go WITHOUT the e2e build tag so it unit-tests in CI).

### Task 2: Accuracy harness (opt-in e2e)
- **ACTION**: `accuracy_test.go` (build tag `e2e`): when `CVSET_DIR` is set, for each `*.pdf|*.docx|*.png|*.jpg` + matching `*.expected.json`, upload via the API, poll to terminal, load parsed profile, run the scorecard, print a per-CV + aggregate scorecard; assert macro thresholds.
- **IMPLEMENT**: Reuse harness upload/poll helpers. Read parsed profile from the blob URL or the parsed-profile JSON the pipeline stored. Skip cleanly when `CVSET_DIR` unset (so the normal e2e run is unaffected).
- **MIRROR**: E2E_UPLOAD_POLL.
- **GOTCHA**: Requires real Azure on the target stack — assert/skip with a clear message if the provider is mock. Each CV takes seconds (OCR+LLM); use a generous poll timeout + run sequentially to respect Azure TPM.
- **VALIDATE**: `go test -tags e2e -run Accuracy ./e2e/...` against staging with a small 3-CV set first.

### Task 3: Branch assignment cases
- **ACTION**: Extend `assigner_test.go` with known province→expected-store/subregion + talent-pool-fallback cases using the seeded store data.
- **MIRROR**: BRANCH_TABLE_TEST.
- **GOTCHA**: Nearest-store depends on store coordinates; pick provinces with unambiguous nearest stores; include a province with no open vacancy → talent pool.
- **VALIDATE**: `go test ./internal/branch/...`.

### Task 4: RBAC matrix coverage
- **ACTION**: Ensure `scope_test.go` covers all 7 roles for both `ApplicationsClause` and `CandidatesClause` (all/subregion/store + store-less fail-closed); write `docs/uat/rbac-matrix.md` for the end-to-end per-role checklist.
- **MIRROR**: RBAC_SCOPE_TEST.
- **GOTCHA**: The fail-closed case (store role with nil store → `1=0`) is the security-critical one — assert it explicitly.
- **VALIDATE**: `go test ./internal/rbac/...`.

### Task 5: Scoring invariants
- **ACTION**: Extend `scoring_test.go`: breakdown dims within caps, sum ≤ total, must-have gate rejects below thresholds, Thai + English education/language map correctly.
- **MIRROR**: existing `scoring_test.go`.
- **VALIDATE**: `go test ./internal/scoring/...`.

### Task 6: UAT runbook + test-set spec
- **ACTION**: Write `docs/uat/cv-test-set.md` (variety matrix + expected.json schema + assembly steps) and `docs/uat/validation-runbook.md` (procedure + thresholds + capture templates for each dimension + load run via `loadtest/`).
- **IMPLEMENT**: Initial **pass thresholds** (stakeholder-confirmable): name ≥95%, email ≥90%, phone ≥90%, education-level ≥85%, years-of-experience within ±1yr ≥80%, skills recall ≥70%; OCR avg confidence ≥0.80 with <20% flagged manual-review; scoring: 0 crashes, breakdown valid on 100%, human-agreement on hi/mid/lo band ≥80% of spot-checks; branch: 100% of known cases correct; RBAC: 100% matrix pass; load: API p95<2s & error<1% at 30 VUs, record pipeline CVs/min + 429 rate.
- **VALIDATE**: docs render; thresholds reviewed.

### Task 7: Execute (operator, on staging)
- **ACTION**: Run the accuracy harness on the 20–30 CV set, the load test, and the RBAC E2E matrix; capture results into the runbook templates; file any failures as follow-up tickets.
- **GOTCHA**: Staging only; purge the CV set + loadtest rows after (cleanup queries).
- **VALIDATE**: Completed scorecard + load numbers + RBAC matrix all recorded; pass/fail per threshold.

---

## Testing Strategy
### Unit Tests (CI, no real data)
| Test | Input | Expected |
|---|---|---|
| scorecard normalize+diff | synthetic parsed vs expected | correct per-field % + macro avg |
| scorecard phone/email norm | "08-1234-5678" vs "0812345678" | match |
| branch known province | province with one near store | expected store id |
| branch no vacancy | province w/o open vacancy | talent pool |
| rbac all 7 roles | each role | correct Kind + clause |
| rbac store role nil store | store role, nil store | `1=0` (fail closed) |
| scoring breakdown | sample profile+JD | dims ≤ caps, sum ≤ total |

### Operator Tests (staging, real Azure + real CVs)
- [ ] Accuracy scorecard ≥ thresholds across the variety matrix
- [ ] Scoring spot-check agreement ≥ 80%
- [ ] Load: k6 thresholds pass; pipeline CVs/min + 429 recorded
- [ ] RBAC matrix: each role sees only its scope (UI + API)

### Edge Cases Checklist
- [ ] Scanned/rotated low-res CV (low OCR confidence → manual-review flag)
- [ ] Missing email or phone on the CV (parser returns empty, not garbage)
- [ ] English-only and Thai-only CVs both parse
- [ ] Career-changer / fresh-grad scoring sanity (no false high/low)
- [ ] Store-scoped HR user cannot see another store's candidates (API 404/empty)

---

## Validation Commands
### Static + Unit (CI)
```bash
cd backend && go vet ./... && go test ./internal/scoring/... ./internal/branch/... ./internal/rbac/... ./e2e/...
```
EXPECT: pass (scorecard unit tests included; e2e accuracy test skipped without CVSET_DIR)

### Accuracy harness (staging, real Azure)
```bash
CVSET_DIR=./e2e/cvset E2E_API_URL=https://<staging-api> DB_URL=<staging-db> \
  go test -tags e2e -run Accuracy -timeout 30m ./e2e/...
```
EXPECT: scorecard printed; macro thresholds asserted

### Load (staging)
```bash
k6 run -e TARGET=https://<staging-api> -e POSITION_ID=<uuid> -e COOKIE="hr_auth=..." -e SAMPLE=./loadtest/sample.pdf loadtest/intake-load.js
```
EXPECT: thresholds pass; record CVs/min via the drain method

### Manual
- [ ] Per-role login walkthrough vs `docs/uat/rbac-matrix.md`

---

## Acceptance Criteria
- [ ] Scorecard tooling exists + unit-tested; accuracy run produces per-field + macro numbers
- [ ] Parsing meets the agreed thresholds across the variety matrix (or gaps ticketed)
- [ ] Scoring invariants hold; spot-check agreement ≥ target
- [ ] Branch known-case tests 100% green
- [ ] RBAC matrix 100% (unit + E2E)
- [ ] Load run recorded (API p95/error + pipeline CVs/min + 429 rate)
- [ ] `go vet` + CI unit tests green

## Completion Checklist
- [ ] No production logic changed (measurement only; fixes ticketed separately)
- [ ] No PII committed (cvset gitignored)
- [ ] Thresholds reviewed with stakeholder
- [ ] Findings captured in the runbook + follow-up tickets filed

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Real parsing accuracy below threshold | Med | High | This PRP exists to find it; gaps → prompt/rule tuning tickets (separate PRP) |
| PII leak via committed CVs | Med | High | gitignore cvset; staging-only; cleanup query |
| Azure TPM throttles the accuracy run | Med | Low | run sequentially + generous timeout; it's measurement, not latency-sensitive |
| Ground-truth labeling effort underestimated | Med | Med | start with 20; expand to 30; labels are stakeholder-provided |
| Branch nearest-store ambiguity in tests | Low | Low | choose unambiguous provinces for assertions |

## Notes
- Findings here drive the next iteration: if parsing/scoring miss thresholds, raise
  targeted tuning PRPs (prompt changes, rule weights) — keep this PRP measurement-only
  so the baseline is clean.
- Branch assignment is rule-based by design; if UAT shows it needs AI/manual override,
  that is a **new** scope decision for the stakeholder, not a fix within PRP-3.
- The accuracy harness doubles as a regression guard: re-run after any
  prompt/model/scoring change to catch drift.
```
