# Implementation Report: Sprint 2 — AI Scoring + Branch Assignment + Dedup

## Summary
Extended the `process_application` task with pipeline **Steps 3–6**: dedup (F09), hybrid scoring against the Master JD (F03), the must-have gate, and branch assignment (F04). Scoring is deterministic in Go for the gate/location/education/experience and uses the LLM (mock by default) only for semantic skills + Thai strengths + red flags. Ships representative seed data (province→subregion map for all 77 provinces, 14 stores across the 13 subregions, 5 positions, 6 demo vacancies). Verified end-to-end on the live stack: an application now ends `scored` (with score breakdown + assigned store) or `rejected`, and resubmissions auto-merge to the canonical candidate.

## Assessment vs Reality
| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Large (as predicted) |
| Confidence | 8/10 | Single-pass; 1 wrong test assertion + 1 lint nit fixed |
| Files Changed | ~30 | 22 created + 9 modified |

## Tasks Completed
| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000004 (dedup/assignment cols) | ✅ | talent_pool, dedup_state, dedup_confidence + indexes; reversible |
| 2 | Positions repository | ✅ | parses must_have_criteria JSONB object |
| 3 | Stores repo + province→subregion map | ✅ | 77 provinces → 13 subregions + centroids |
| 4 | Vacancies repository | ✅ | FindOpen (join stores) + CountOpenForPosition |
| 5 | Dedup (F09) | ✅ | exact id_card/phone/email + Levenshtein name; auto/ pending thresholds |
| 6 | Hybrid scorer (F03) | ✅ | rules + LLM-part (mock/azure); gate short-circuits LLM |
| 7 | Branch assignment (F04) | ✅ | subregion→vacancy→nearest (haversine); talent-pool fallback; LocationScore |
| 8 | Repo extensions | ✅ | candidates FindDuplicates/MarkDuplicateOf; applications SetScore/SetAssignment/SetCanonicalCandidate/SetDedupState + new statuses |
| 9 | Pipeline Steps 3–6 | ✅ | dedup→score→gate→assign; idempotent; structured logs |
| 10 | Worker wiring + seed + Make | ✅ | NewScorer/dedup/assigner injected; `make seed`; README |

## Validation Results
| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ | `go vet` (+`-tags integration`) clean; `golangci-lint` 0 issues |
| Unit Tests (`-race`) | ✅ | dedup, scoring, branch, applications, ai, config |
| Build | ✅ | `go build` + `docker compose build` |
| Integration (`-tags integration`) | ✅ | scored+assigned / gate-fail rejected / duplicate repointed / low-confidence / parse-failure |
| Edge Cases | ✅ | gate short-circuit (no LLM), talent-pool fallback, migration 000004 round-trip |

### End-to-end evidence (live stack, mock provider, seeded data)
```
Upload (province=เชียงใหม่, Cashier) → status "scored", ai_score 86
  breakdown {experience:30, skills:19, education:7, language:10, location:20}
  must_have_passed true, assigned_store_id 1 (nearest in Upper North), 3 Thai strengths
Resubmit same person → dedup_state auto_merged (0.95), application repointed to canonical,
  new candidate is_duplicate_of = canonical
```

## Files Changed
22 created, 9 modified. New packages: `internal/positions`, `internal/stores`, `internal/vacancies`, `internal/dedup`, `internal/scoring`, `internal/branch`; migration `000004`; `scripts/seed_*.sql`.

## Deviations from Plan
1. **Scorer factory lives in `scoring.NewScorer`, not `ai.New`.** WHY: `scoring` imports `ai` (for Profile); adding a Scorer return to `ai.New` would create an `ai → scoring → ai` import cycle. The mock-default seam is preserved.
2. **Location score is computed by the assigner (`LocationScore`) and passed into `Scorer.Score`,** rather than computed inside scoring. WHY: keeps `scoring` decoupled from vacancy/store data; scoring stays pure + unit-testable.
3. **`must_have_criteria` seeded as an object** `{"min_education_level","min_experience_months"}` rather than the PRP's `[{criterion,weight}]` array. WHY: a typed gate is deterministic and testable; the importer can map the real array form later.
4. **Azure scorer uses REST/net-http** (same decision as Sprint 1's Azure providers) — no uncertain SDK; mock is the default path.
5. **Seed format codes simplified to A–F** so `position.format_types` filtering is clean against synthetic stores.

## Issues Encountered
- One unit-test assertion expected total 100 where the correct hybrid total is 83 (30+16+7+10+20) — fixed the test, not the implementation.
- One staticcheck nit (S1016: struct→struct conversion) — applied `LLMPart(parsed)`.
- Shared `pgdata` volume retained a leftover `ทดสอบ` position from integration-test fixtures, making the seed count look like 6; not a bug (integration tests truncate at setup; re-seed before e2e).

## Tests Written
| Test File | Tests | Area |
|---|---|---|
| `internal/dedup/dedup_test.go` | 2 | decide() matrix + Levenshtein (incl. Thai) |
| `internal/scoring/scoring_test.go` | 3 | gate-fail skips LLM, happy-path math, clamping |
| `internal/branch/assigner_test.go` | 4 | nearest store, talent-pool fallbacks, LocationScore tiers |
| `internal/pipeline/process_integration_test.go` (tag) | 5 | scored+assigned, gate-fail, duplicate repoint, low-confidence, parse-failure |

## Next Steps
- [ ] Code review via `/code-review`
- [ ] Open PR (branch `feat/sprint-2-scoring-branch-dedup`)
- [ ] Sprint 3: PeopleSoft integration (real vacancies + hired sync) + Career Portal + data migration.
