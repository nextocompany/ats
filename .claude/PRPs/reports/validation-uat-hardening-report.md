# Implementation Report: Validation & UAT hardening (PRP-3)

## Summary
Delivered the **code + tooling + docs** to validate the existing AI pipeline on real
data: a CI-tested parse-accuracy scorecard library, an opt-in e2e accuracy harness,
RBAC `CandidatesClause` coverage, an English education/language scoring test, and the
UAT runbook + CV test-set spec + RBAC matrix. The actual measurement run (real CVs on
staging) is an **operator step** — by design it cannot run in CI (needs real Azure +
a private PII CV set).

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium | Medium |
| Confidence | 7/10 | held |
| Files Changed | ~10 | 6 new + 3 updated (+ cvset/.gitignore) |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Ground-truth schema + scorecard logic | ✅ | `e2e/scorecard.go` (no build tag → unit-tested in CI) + 6 unit tests |
| 2 | Accuracy harness (opt-in e2e) | ✅ | `e2e/accuracy_test.go` (tag `e2e`, `CVSET_DIR`-gated) + `e2e/cvset/.gitignore` |
| 3 | Branch assignment cases | ✅ (no change) | Already covered (nearest/no-vacancy/unknown-province/location) — verified, not duplicated |
| 4 | RBAC matrix coverage | ✅ | Added `TestCandidatesClause` (was a gap); `docs/uat/rbac-matrix.md` |
| 5 | Scoring invariants | ✅ | Added English education+language test (Thai/caps/gate already covered) |
| 6 | UAT runbook + test-set spec | ✅ | `docs/uat/{validation-runbook,cv-test-set,rbac-matrix}.md` |
| 7 | Execute on staging | ⏭ Deferred | Operator step — needs labeled CV set + staging Azure |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static (go vet, incl. `-tags e2e`) | ✅ Pass | |
| gofmt | ✅ Pass | my files clean (pre-existing integration files untouched) |
| Unit Tests | ✅ Pass | `go test ./...` exit 0, 28 pkgs; scorecard/rbac/scoring green |
| Build | ✅ Pass | `go build ./...` + e2e compiles under tag |
| Operator run | ⏭ | parse-accuracy + load + RBAC E2E on staging (runbook) |

## Files Changed
| File | Action |
|---|---|
| `backend/e2e/scorecard.go` (+ `scorecard_test.go`) | CREATE |
| `backend/e2e/accuracy_test.go` | CREATE (tag e2e) |
| `backend/e2e/cvset/.gitignore` | CREATE |
| `docs/uat/cv-test-set.md`, `validation-runbook.md`, `rbac-matrix.md` | CREATE |
| `backend/internal/rbac/scope_test.go` | UPDATE (+TestCandidatesClause) |
| `backend/internal/scoring/scoring_test.go` | UPDATE (+English edu/lang) |

## Deviations from Plan
- **Branch tests not extended** — the existing `assigner_test.go` already covers
  nearest-store, no-vacancy talent pool, unknown-province talent pool, and location
  score. Adding more would be redundant; verified coverage instead.
- **Scoring** — Thai education/language, breakdown caps, and the must-have gate were
  already tested; only the English path was missing, so one focused test was added.
- `scorecard.go` lives in the `e2e` package WITHOUT the build tag so it (and its
  tests) run in normal CI, while the harness that uses it is tagged `e2e`.

## Issues Encountered
- gofmt struct-tag/loader alignment after edits → `gofmt -w`. No logic issues.

## Tests Written
| Test File | Tests | Coverage |
|---|---|---|
| `e2e/scorecard_test.go` | 6 | normalize/diff, phone/email norm, experience tolerance, skills recall, empty-skip, aggregate/macro/OCR |
| `internal/rbac/scope_test.go` | +1 | CandidatesClause all/subregion/store/fail-closed |
| `internal/scoring/scoring_test.go` | +1 | English bachelor gate + English-only language = 5 |

## Operator Next Steps (not code)
1. Assemble 20–30 labeled CVs in `backend/e2e/cvset/` (see `docs/uat/cv-test-set.md`).
2. Run the accuracy harness + load test on staging (real Azure); paste results into `docs/uat/validation-runbook.md`.
3. Walk the RBAC matrix per role.
4. Any threshold miss → file a targeted tuning ticket (separate PRP — keep this baseline clean).

## Next Steps
- [ ] Code review via `/code-review`
- [ ] PR + merge (CI-safe; no deploy needed — tooling + tests only)
- [ ] Operator: run the measurement on staging
