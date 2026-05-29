# Plan: Sprint 2 — AI Scoring + Branch Assignment + Dedup

## Summary
Extend the existing `process_application` task with pipeline **Steps 3–6**: dedup (F09), AI scoring against the Master JD (F03), the must-have gate, and branch assignment (F04). Scoring is **hybrid** — deterministic Go for the gate, location, education, and experience; LLM only for semantic skills match + Thai strengths + red flags, behind the same mock-default seam as Sprint 1. Ships representative seed data (province→subregion map, synthetic stores, sample positions, demo vacancies) so assignment is testable now.

## User Story
As **HR**, I want **each parsed candidate automatically deduped, scored against the position's Master JD, gated on must-haves, and assigned to the nearest store with an open vacancy (or the talent pool)**, so that **my ranked inbox shows only viable candidates already routed to the right branch**.

## Problem → Solution
**Current state (post-Sprint 1):** The pipeline stops at `parsed` — every application, including duplicates and unqualified candidates, lands undifferentiated with no score or branch.
**Desired state:** After parse, the pipeline (Step 3) reconciles duplicates, (Step 4) scores 0–100 with a breakdown + strengths + red flags, (Step 5) auto-rejects must-have failures, and (Step 6) assigns a store from open vacancies in the candidate's subregion or flags talent-pool. The application ends `scored` (passed) / `rejected` (gate fail), with `ai_score`, `assigned_store_id`, and dedup state populated.

## Metadata
- **Complexity**: Large (extends pipeline + 3 new domains + seed data; ~30 files)
- **Source PRD**: PRP v1.0 — Sprint 2 (W5–6, "AI Scoring + Branch Assignment + Dedup", 30 MD)
- **Covers**: F03 (Scoring), F04 (Branch Assignment), F09 (Dedup/Merge); Pipeline §8 Steps 3–6
- **Decisions locked**: Scoring = **hybrid (rules + LLM)**; Seed data = **generate representative now**
- **Estimated Files**: ~30

---

## UX Design
**N/A — backend/pipeline.** Output is richer application records (score, breakdown, assignment, dedup) consumed by the HR Dashboard in Sprint 4.

### Interaction Changes
| Touchpoint | Before (S1) | After (S2) | Notes |
|---|---|---|---|
| Application end state | `parsed` | `scored` / `rejected` | gate decides |
| Duplicate submission | two candidate rows | second marked `is_duplicate_of`, application repointed to canonical | F09 |
| Assignment | none | `assigned_store_id` or `talent_pool=true` | F04 |
| Score | none | `ai_score`, `ai_score_breakdown`, `ai_summary`, `ai_red_flags`, `ai_suggested_positions`, `must_have_passed` | columns already exist (000001) |

---

## Mandatory Reading (existing code to extend)
| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/pipeline/process.go` | 32–135 | The task handler to extend; add Steps 3–6 after parse in `run()` |
| P0 | `backend/internal/ai/factory.go` | 8–15 | Add a `Scorer` to the provider factory (mock/azure) the same way |
| P0 | `backend/internal/applications/repository.go` | 12–110 | Add `SetScore`, `SetAssignment`, `SetCanonicalCandidate`, gate status writes |
| P0 | `backend/internal/applications/model.go` | 11–15 | Add `StatusScored`, `StatusRejected`; add score/assignment fields |
| P0 | `backend/internal/candidates/repository.go` | 13–95 | Add `FindDuplicates`, `MarkDuplicateOf` |
| P1 | `backend/internal/ai/profile.go` | all | Profile is the scorer input |
| P1 | `backend/internal/ai/azure_parser.go` | all | Mirror REST + JSON-response pattern for the azure scorer |
| P1 | `backend/pkg/database/postgres.go` | all | pool injection convention for new repos |
| P2 | `backend/migrations/000001_init_schema.up.sql` | all | positions/stores/vacancies/applications columns used |
| P2 | `.claude/PRPs/plans/completed/sprint-1-intake-cv-parser.plan.md` | conventions | mirror the S1 conventions verbatim |

## External Documentation
| Topic | Source | Key Takeaway |
|---|---|---|
| Levenshtein in Go | `github.com/agext/levenshtein` or hand-rolled | name fuzzy match (distance ≤ 2 per F09). Prefer a tiny vetted lib or a ~30-line DP impl — keep it dependency-light. |
| Haversine distance | standard math | nearest-store ranking from candidate province centroid → store lat/lng (F04 step 6). No lib needed. |
| Azure OpenAI JSON | (already used in S1 `azure_parser.go`) | reuse the chat-completions REST + `response_format=json_object` pattern for the LLM scoring sub-call. |

### Research Notes
```
KEY_INSIGHT: Dedup runs in the pipeline (Step 3), AFTER parse — not at intake.
APPLIES_TO: process.go ordering.
GOTCHA: Intake already created a candidate (S1). When Step 3 finds a >0.9 match, repoint application.candidate_id to the CANONICAL candidate and mark the just-created one is_duplicate_of canonical (do NOT delete — preserve audit). 0.7–0.9 → leave both, set application needs_manual_review + a pending flag for HR. Exact id_card/phone/email match ⇒ confidence 1.0.

KEY_INSIGHT: Hybrid scoring split.
APPLIES_TO: Step 4.
GOTCHA: Deterministic in Go → must_have gate (education level ≥ required, experience_months ≥ min), location (0–20 via subregion/distance), education bonus (0–10), experience (0–30 from months vs required). LLM → skills semantic match (0–20) + 3 Thai strengths + red_flags + suggested_positions. Total = sum, clamp 0–100. Mock scorer returns deterministic LLM-part values.

KEY_INSIGHT: Branch assignment depends on vacancies which come from PeopleSoft (Sprint 3).
APPLIES_TO: Step 6 + seed.
GOTCHA: Seed demo vacancies so assignment is exercisable now. If no open vacancy in subregion ⇒ assigned_store_id NULL + talent_pool=true. Multiple matches ⇒ rank by store distance from candidate province.
```

---

## Patterns to Mirror

### PIPELINE_STEP (extend run() in order)
```go
// SOURCE: backend/internal/pipeline/process.go:81-135 — each step wraps errors with %w and logs with step context.
// after Step 2 (parse) + UpdateProfileFields:
canonicalID, dupState := pr.dedup.Reconcile(ctx, candID, profile)        // Step 3
score := pr.scorer.Score(ctx, profile, jd)                                // Step 4 (hybrid)
if !score.MustHavePassed { return pr.reject(ctx, appID, score) }          // Step 5
assignment := pr.assigner.Assign(ctx, canonicalID, profile, positionID)   // Step 6
return pr.apps.SetScoreAndAssignment(ctx, appID, score, assignment)
```

### PROVIDER_FACTORY (add Scorer)
```go
// SOURCE: backend/internal/ai/factory.go:8 — return the third impl alongside OCR/Parser.
func New(cfg *config.Config) (OCR, Parser, Scorer) { ... mock vs azure ... }
```

### SMALL_INTERFACE + DI (golang/patterns)
```go
// internal/scoring/scorer.go — accept interfaces, return structs; inject via constructor.
type Scorer interface { Score(ctx context.Context, p ai.Profile, jd JD) (Result, error) }
```

### REPOSITORY (mirror S1 repos)
```go
// SOURCE: backend/internal/applications/repository.go:30-110 — pgxpool, wrapped errors, COALESCE reads.
func (r *pgRepository) SetScore(ctx, id uuid.UUID, s Score) error { /* UPDATE applications SET ai_* ... */ }
```

### ERROR_HANDLING / LOGGING
```go
// Mirror S1: fmt.Errorf("pipeline: score: %w", err); logger.Info().Int("score", s.Total).Bool("gate", s.MustHavePassed).Msg(...)
```

---

## Files to Change
| File | Action | Justification |
|---|---|---|
| `backend/migrations/000004_assignment_dedup.up/.down.sql` | CREATE | `applications.talent_pool BOOLEAN`, `dedup_state VARCHAR`, `dedup_confidence NUMERIC`; index `candidates(phone)`,`(email)` already exist — add `candidates(full_name)` trigram? (keep simple: btree) |
| `backend/internal/positions/{model,repository}.go` | CREATE | load Master JD (must_have/nice_to_have/keywords) for scoring |
| `backend/internal/stores/{model,repository}.go` | CREATE | store lookup by subregion + coordinates for assignment |
| `backend/internal/stores/subregion.go` | CREATE | province→subregion map (77→13) + province centroid coords |
| `backend/internal/vacancies/{model,repository}.go` | CREATE | open-vacancy query for assignment |
| `backend/internal/dedup/{dedup,dedup_test}.go` | CREATE | Levenshtein name + exact phone/email/id_card; Reconcile() |
| `backend/internal/scoring/{scorer,rules,mock,azure,jd,result,scoring_test}.go` | CREATE | hybrid scorer: deterministic rules + LLM sub-call; mock + azure |
| `backend/internal/branch/{assigner,assigner_test}.go` | CREATE | subregion→vacancy→nearest-store ranking; talent-pool fallback |
| `backend/internal/ai/factory.go` | UPDATE | also return `Scorer` (mock/azure) |
| `backend/internal/candidates/repository.go` | UPDATE | `FindDuplicates`, `MarkDuplicateOf` |
| `backend/internal/applications/{model,repository}.go` | UPDATE | new statuses + `SetScore`/`SetAssignment`/`SetCanonicalCandidate` |
| `backend/internal/pipeline/process.go` | UPDATE | add Steps 3–6; inject dedup/scorer/assigner/repos |
| `backend/cmd/worker/main.go` | UPDATE | construct + inject the new collaborators |
| `backend/migrations/000005_seed_reference.up/.down.sql` OR `scripts/seed_*.sql` | CREATE | province→subregion is code; stores/positions/vacancies demo seed |
| `scripts/seed_stores.sql`, `scripts/seed_positions.sql`, `scripts/seed_vacancies.sql` | CREATE | representative seed; importer-friendly format |
| `Makefile` | UPDATE | `make seed` target |
| `backend/internal/pipeline/process_integration_test.go` | UPDATE | assert score/assignment/dedup end states |

## NOT Building (Sprint 3+)
- **PeopleSoft vacancy sync** (real vacancies) — S2 uses seeded demo vacancies; F04 logic is real, data source is stubbed.
- **Notifications** (LINE) on reject/pass — gate sets status only; enqueue-notify is Sprint 5.
- **Career Portal / HR Dashboard UI** — Sprint 3/4.
- **Real Storelist.csv / Master JD import** — representative data now; importer accepts the real files later.
- **Custom per-requisition criteria** (F11) — Sprint later.
- **Semantic search** (F12), **PDPA UI** (F13), **re-engagement** (F06).

---

## Step-by-Step Tasks

### Task 1: Migration 000004 (assignment + dedup columns)
- **ACTION**: Add `applications.talent_pool BOOLEAN DEFAULT FALSE`, `dedup_state VARCHAR(20)`, `dedup_confidence NUMERIC(4,3)`. (Score/assignment columns already exist from 000001.)
- **MIRROR**: MIGRATION_NAMING; additive only.
- **VALIDATE**: `make migrate-up`/`down 1` round-trip.

### Task 2: Positions repository
- **ACTION**: `internal/positions/{model,repository}.go` — `FindByID` returns title, level, must_have_criteria, nice_to_have_criteria, keywords, required experience/education (parsed from JSONB).
- **MIRROR**: candidates repo (`repository.go:53`).
- **GOTCHA**: must_have_criteria is JSONB array — define a typed struct and `json.Unmarshal`.
- **VALIDATE**: integration test reads a seeded position.

### Task 3: Stores repo + province→subregion map
- **ACTION**: `internal/stores/{model,repository}.go` + `subregion.go` (77 provinces → 13 subregions + per-province centroid lat/lng).
- **IMPLEMENT**: `ResolveSubregion(province) string`; `FindOpenInSubregion(subregion, positionID)` joins vacancies.
- **GOTCHA**: 13 subregion names must match PRP F04 exactly (BKK East, … Lower South).
- **VALIDATE**: unit test `ResolveSubregion("เชียงใหม่") == "Upper North"` (per the map you define).

### Task 4: Vacancies repository
- **ACTION**: `internal/vacancies/{model,repository}.go` — `FindOpen(ctx, subregion, positionID) []Vacancy` (status='open', join stores).
- **VALIDATE**: integration test with seeded vacancies.

### Task 5: Dedup (F09, Step 3)
- **ACTION**: `internal/dedup/dedup.go` + test.
- **IMPLEMENT**: `Reconcile(ctx, newCandID, profile) (canonicalID uuid.UUID, state string, confidence float64)`. Exact id_card/phone/email match ⇒ 1.0 auto-merge; fuzzy name (Levenshtein ≤2) + one contact match ⇒ 0.7–0.9; >0.9 repoint + `MarkDuplicateOf`; 0.7–0.9 ⇒ leave, state `pending_review`.
- **MIRROR**: candidates repo `FindDuplicates`.
- **GOTCHA**: never delete the duplicate row (audit). Repoint `application.candidate_id` to canonical.
- **VALIDATE**: unit test the matching math (table-driven); integration test the repoint.

### Task 6: Hybrid scorer (F03, Step 4)
- **ACTION**: `internal/scoring/{jd,result,rules,scorer,mock,azure}.go` + test.
- **IMPLEMENT**: `JD` (from positions), `Result{MustHavePassed, Total, Breakdown{Experience,Skills,Education,Language,Location}, Strengths[3], RedFlags, SuggestedPositions}`. **rules.go** (deterministic): gate (education ≥ required, experience_months ≥ min), location 0–20, education 0–10, experience 0–30, language 0–10. **azure scorer**: LLM sub-call for skills 0–20 + Thai strengths + red flags (reuse `azure_parser.go` REST pattern). **mock scorer**: deterministic LLM-part. `Scorer.Score` composes rules + LLM-part, clamps 0–100.
- **MIRROR**: PROVIDER_FACTORY, SMALL_INTERFACE.
- **GOTCHA**: clamp total; gate failure short-circuits (no need to call LLM if gate fails — saves a token call).
- **VALIDATE**: unit test rules (gate pass/fail, score math) with mock LLM-part; deterministic.

### Task 7: Branch assignment (F04, Step 6)
- **ACTION**: `internal/branch/assigner.go` + test.
- **IMPLEMENT**: `Assign(ctx, candidateProvince, positionID, formatTypes) Assignment{StoreID *int, TalentPool bool}`. Resolve subregion → `vacancies.FindOpen` → filter by position format compatibility → rank by Haversine(candidate province centroid, store) → pick nearest. None ⇒ talent pool.
- **GOTCHA**: candidate may lack coordinates — use province centroid from `subregion.go`. Missing store coords ⇒ deprioritize, don't crash.
- **VALIDATE**: unit test: 3 provinces → correct subregion + nearest seeded store; no-vacancy → talent pool.

### Task 8: Applications + candidates repo extensions
- **ACTION**: Add `StatusScored`/`StatusRejected`; `SetScore`, `SetAssignment`, `SetCanonicalCandidate`, `SetDedupState`. Candidates: `FindDuplicates`, `MarkDuplicateOf`.
- **MIRROR**: `applications/repository.go:87` SetParseResults shape.
- **VALIDATE**: integration tests.

### Task 9: Wire Steps 3–6 into the pipeline
- **ACTION**: Extend `process.go run()` after parse: Step 3 dedup → Step 4 score → Step 5 gate (reject + return) → Step 6 assign → persist; end status `scored`. Inject `dedup`, `scorer`, `assigner`, `positions`, `vacancies`, `stores` repos into `Processor`.
- **MIRROR**: PIPELINE_STEP ordering + error/log conventions.
- **GOTCHA**: keep idempotent (re-run yields same canonical/score/assignment). Gate-reject still records score breakdown + must_have_passed=false.
- **VALIDATE**: integration test — happy → `scored` with score+assignment; gate-fail → `rejected`; duplicate → repointed.

### Task 10: Worker wiring + seed data + Make
- **ACTION**: `cmd/worker/main.go` constructs the new collaborators (scorer via `ai.New` 3rd return). Add `scripts/seed_*.sql` (stores across 13 subregions/format types, ~sample positions with must_have JSONB, demo vacancies) + `make seed`. Update README.
- **VALIDATE**: `make up && make migrate-up && make seed` then full e2e: upload → `scored` + `assigned_store_id` set.

---

## Testing Strategy
### Unit / Integration
| Test | Input | Expected | Edge? |
|---|---|---|---|
| dedup exact id_card | same id_card | confidence 1.0, auto-merge | No |
| dedup fuzzy name | name dist 2 + same phone | 0.7–0.9 pending_review | Yes |
| dedup no match | different person | new canonical | No |
| rules gate fail | education below required | MustHavePassed=false | Yes |
| rules score math | known profile vs JD | expected breakdown sum | No |
| scorer clamp | inflated parts | total ≤ 100 | Yes |
| assign nearest | province + 2 vacancies | nearest store chosen | No |
| assign no vacancy | subregion w/o open vacancy | talent_pool=true, store nil | Yes |
| subregion resolve | sample provinces | correct subregion | No |
| pipeline happy | parsed app | status `scored`, ai_score set, assigned | No |
| pipeline gate-fail | unqualified | status `rejected`, must_have_passed=false | Yes |
| pipeline duplicate | 2nd of same person | application repointed to canonical | Yes |

### Edge Cases Checklist
- [ ] Candidate with no province → talent pool (can't resolve subregion)
- [ ] Position with empty must_have_criteria → gate passes by default
- [ ] LLM scorer error → retry; after retries app `failed` (not silently 0-scored)
- [ ] Store missing coordinates → still rankable (deprioritized)
- [ ] Idempotent re-run (asynq retry) → no duplicate dedup repoint

---

## Validation Commands
### Static / Unit
```bash
cd backend && go vet ./... && golangci-lint run && go test -race ./...
```
EXPECT: clean; new packages (dedup, scoring, branch, stores, vacancies, positions) ≥80% on tested logic.

### Integration + Seed
```bash
make up && make migrate-up && make seed
cd backend && go test -tags integration ./... 
```

### End-to-end (the Sprint 2 gate)
```bash
make up && make migrate-up && make seed
POS=$(psql ... "SELECT id FROM positions WHERE ... LIMIT 1")   # a position with a seeded open vacancy
curl -F resume=@cv.pdf;type=application/pdf -F position_id=$POS -F full_name=... -F province=เชียงใหม่ \
  localhost:8080/api/v1/applications
# poll → application: status "scored", ai_score 0–100, ai_score_breakdown, assigned_store_id set
# submit the SAME person again → second application repointed to the canonical candidate
# submit an unqualified profile → status "rejected", must_have_passed=false
```
EXPECT: scored + assigned for a qualified candidate in a seeded subregion; rejected on gate fail; dedup repoint on resubmit.

---

## Acceptance Criteria
- [ ] Pipeline runs Steps 3–6; application ends `scored` (pass) or `rejected` (gate fail).
- [ ] `ai_score`, `ai_score_breakdown`, `ai_summary` (3 Thai strengths), `ai_red_flags`, `ai_suggested_positions`, `must_have_passed` populated.
- [ ] Duplicates reconciled (repoint + `is_duplicate_of`); 0.7–0.9 flagged for HR.
- [ ] `assigned_store_id` set from nearest open vacancy, else `talent_pool=true`.
- [ ] Hybrid scoring: deterministic parts unit-tested; mock default needs no Azure keys.
- [ ] Representative seed loads via `make seed`; province→subregion covers 77 provinces.
- [ ] All validation levels pass incl. `-race`; coverage ≥80% on new tested packages.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Fuzzy dedup false-merge | Med | High | Conservative thresholds; >0.9 auto only on exact contact match; 0.7–0.9 → HR review, never silent merge |
| Synthetic seed misrepresents real geography | Med | Med | Province centroids accurate; stores synthetic but in real subregions; importer accepts real CSV later |
| LLM scoring variance | Med | Med | Hybrid keeps gate+location+education+experience deterministic; LLM only qualitative; mock default for CI |
| Idempotency on dedup retry | Med | High | Check `dedup_state` before repointing; repoint is set-canonical (idempotent), not create |
| Vacancies empty (no PS yet) | High | Low | Seed demo vacancies; talent-pool fallback is the documented no-vacancy path |

## Notes
- The `process_application` task is **extended, not replaced** — Steps 1–2 (S1) stay; 3–6 append; Step 7 (notify) is Sprint 5.
- `internal/stores/subregion.go` (province→subregion + centroids) is real data and survives the swap to the real Storelist.csv.
- Scoring's LLM sub-call reuses the S1 Azure REST pattern — no new SDK.
