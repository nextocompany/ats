# Plan: Per-Position Screening-Score Weights

## Summary
Let HR (settings.admin) configure, per position, how much each of the 5 screening dimensions matters. The AI screening composite becomes a **weighted** combination instead of a flat sum, resolved from the candidate's `position_id` at scoring time. Weights live on `positions.score_weights` (JSONB); positions without a config use the current defaults, which reproduce today's scores exactly.

## User Story
As an HR admin, I want to set per-position importance weights for the screening dimensions, so the AI score reflects what each role actually values.

## Problem to Solution
Screening score = flat sum of 5 capped sub-scores (caps 30/20/10/10/20 are hardcoded, same for every position). After: score = weighted combination using per-position weights (default 30/20/10/10/20 = identical to today), editable in a new admin page.

## Metadata
- **Complexity**: Medium-Large
- **Next migration**: `000041` (prod at v40)
- **Decisions (locked):** per-POSITION (not requisition); 5 existing dimensions; gate (must-have) unchanged; new page gated `settings.admin`.

---

## Current behavior (from exploration)
- `internal/scoring/scorer.go:45` `compositeScorer.Score(ctx, profile, jd, locationScore)` returns 0-100.
- Combine lines: gate-FAIL `scorer.go:64` = `clamp(Exp+Edu+Lang+Loc)`; gate-PASS `scorer.go:77` = `clamp(Exp+Skills+Edu+Lang+Loc)`.
- Sub-scores + their max caps: Experience 0-30 (`rules.go:79`), Skills(LLM) 0-20 (`scorer.go:68-72`), Education 0-10 (`rules.go:107`), Language 0-10 (`rules.go:121`), Location 0-20 (caller, `scorer.go:46`).
- `JD` struct `internal/scoring/jd.go:15-22`; built in `internal/pipeline/process.go:325-332` from `positions.Position`.
- Score stored `applications.ai_score` + `ai_score_breakdown` JSONB (`repository.go:425`, `process.go:474`).
- Positions are READ-ONLY today (importref CSV only). `positions.FindByID` JSONB scan pattern: `model.go:63-86` (mustHaveRaw + json.Unmarshal). importref `DO UPDATE SET` does NOT list new cols → safe.
- `nice_to_have_criteria JSONB` is a dead-column precedent for `score_weights JSONB`.
- `rbac.PermSettingsAdmin = "settings.admin"` already in catalog (`permissions.go:11`); super_admin has all; others grantable via RBAC matrix UI. No permission migration.

---

## The weighted model — PERCENTAGE model (user-chosen)
Today's ceiling is 90 (caps 30+20+10+10+20), not 100. We move to a true 0-100% scale:
```
cap_i     = {exp30, skills20, edu10, lang10, loc20}   (intrinsic sub-score ranges)
ratio_i   = subscore_i / cap_i ∈ [0,1]
total     = round( Σ weight_i * ratio_i ),  clamped 0-100
weights are PERCENTAGES summing to 100
DEFAULT   = {experience:34, skills:22, education:11, language:11, location:22}  (sum 100)
```
- Default approximates today's 3:2:1:1:2 ratio (caps/90 normalized to 100, largest-remainder rounding puts the spare +1 on experience). Max candidate now scores 100 (was 90); **within-position ranking is preserved** (near-uniform ~x1.11 rescale) and **all positions share one 0-100 scale** (inbox stays comparable).
- **Visible scores rise ~11%** (e.g. 72 -> ~80). Confirmed acceptable (prod nearly empty of legacy scored candidates). Existing stored scores are NOT recomputed (only new applications).
- Gate-FAIL keeps Skills=0 -> its weighted term is 0 (same shape as today dropping Skills).
- **Breakdown display:** keep `ai_score_breakdown` storing the RAW per-dimension sub-scores (fixed caps, unchanged so the inbox detail still renders) AND add the effective `weights` into the breakdown JSON for explainability. `Total` = weighted. Inbox rebuild is out of scope.

---

## Files to Change

### Backend
| File | Action | What |
|---|---|---|
| `backend/migrations/000041_position_score_weights.{up,down}.sql` | CREATE | `ALTER TABLE positions ADD COLUMN score_weights JSONB;` (nullable) / drop |
| `backend/internal/scoring/weights.go` | CREATE | canonical `Weights` type + `DefaultWeights()` + `Valid()` + caps consts + `WeightedTotal(Breakdown, Weights)` |
| `backend/internal/scoring/scorer.go` | UPDATE | replace the two sum lines (`:64`,`:77`) with `WeightedTotal(bd, jd.Weights)`; default weights if zero-value |
| `backend/internal/scoring/jd.go` | UPDATE | add `Weights Weights` field to `JD` |
| `backend/internal/scoring/*_test.go` | UPDATE | assert default weights == old sum on sample breakdowns; a custom-weight case |
| `backend/internal/positions/model.go` | UPDATE | `Position.ScoreWeights *scoring.Weights` (or positions-local mirror if import cycle); select+scan `score_weights`; add `UpdateScoreWeights` to repo interface + impl |
| `backend/internal/positions/handler.go` | UPDATE | `DetailItem` += `score_weights` (effective = stored or default); new `PUT /api/v1/positions/:id/score-weights` gated `settings.admin`, validate sum==100 |
| `backend/internal/pipeline/process.go` | UPDATE | set `jd.Weights` from `pos.ScoreWeights` (or `DefaultWeights()` if nil) at `:325-332` |
| `backend/cmd/api/main.go` | UPDATE | (only if the new route needs explicit wiring beyond RegisterRoutes — positions routes already registered) |

### Frontend (dashboard)
| File | Action | What |
|---|---|---|
| `frontend/lib/types.ts` | UPDATE | `ScoreWeights` type; `PositionDetail.score_weights` |
| `frontend/lib/queries.ts` | UPDATE | `useUpdatePositionWeights()` (PUT) + reuse `usePosition` for read |
| `frontend/lib/roles.ts` | UPDATE | `canAdminSettings(me)` helper over `PERMS.settingsAdmin` |
| `frontend/components/shell/nav-config.tsx` | UPDATE | nav entry behind `canAdminSettings` |
| `frontend/app/(app)/scoring/page.tsx` | CREATE | pick position -> edit 5 weights -> live total must equal 100 -> save; mirror `requisitions/page.tsx` gating + `RolesPermissions.tsx` working-copy/save |
| `frontend/messages/{en,th}.json` | UPDATE | new `scoring` namespace |

---

## Key tasks
1. **Migration 000041** - `positions.score_weights JSONB` nullable. Verify up/down on PG16.
2. **scoring/weights.go** - `Weights{Experience,Skills,Education,Language,Location int}`, caps consts (30/20/10/10/20), `DefaultWeights()`, `Valid()` (each >=0, sum==100), `WeightedTotal(bd, w)` (nil/zero-value -> default). Unit tests: default == flat sum (table of breakdowns), custom weight shifts total as expected, clamp 0-100.
3. **scorer.go** - use `jd.Weights`; if `jd.Weights.Sum()!=100` fall back to `DefaultWeights()` (defensive). Replace both combine lines.
4. **jd.go / process.go** - thread weights from position.
5. **positions model/repo/handler** - scan score_weights (nil->nil); `UpdateScoreWeights`; `DetailItem.score_weights` always effective; `PUT .../:id/score-weights` validate+persist; `ErrNotFound`->404, invalid->400.
6. **Frontend** - new `/scoring` page, hook, gate, nav, i18n. Inputs: 5 `Input type=number` 0-100 + live total badge/meter; Save disabled unless total==100. Load effective weights via `usePosition`.
7. **Integration test** (real PG, `-tags integration`): UpdateScoreWeights then FindByID round-trips; pipeline scoring with custom weights yields expected total.

## NOT building
- Recomputing existing stored scores (only new applications use new weights).
- Per-requisition weights (technically infeasible: apply is position-keyed; user declined funnel rebuild).
- Configurable gate thresholds (must-have stays; importref-managed).
- Changing the LLM prompt or sub-score formulas (only the COMBINE step + a config column).
- New RBAC permission (reuse `settings.admin`).

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Formula change shifts everyone's scores | Low | High | Default weights reproduce today EXACTLY (math + unit test on sample breakdowns) |
| Invalid persisted weights break scoring | Low | High | Validate sum==100 at write; scorer falls back to default if invalid/nil |
| Import cycle positions<->scoring | Med | Low | If `scoring` imports `positions`, use a positions-local mirror type + map in pipeline |
| Breakdown vs Total mismatch confuses HR | Med | Low | Keep breakdown = raw per-dimension achievement (fixed caps); Total = weighted; document |

## Validation
- `go build ./... && go vet && gofmt -l` clean; `go test ./...` 0 FAIL (incl new weights unit tests).
- migration up/down on PG16; integration round-trip (update weights -> FindByID; scored total with custom weights).
- frontend `tsc --noEmit` + `eslint` clean.
- Manual: /scoring set weights sum=100 save; new upload to that position scores by the new blend.

## Notes
- HARD RULES: no em dash; deploy after dev; NO Co-Authored-By.
- Deploy = migration 000041 -> roll api + dashboard (worker shares backend image with the scorer change -> **worker MUST also roll** this time, unlike phases 1/2). Confirm at deploy.
