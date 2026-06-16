# Validation & UAT Runbook (PRP-3)

Proves the AI pipeline meets acceptance quality on real data. Run on a **staging**
stack with **real Azure AI** (`AI_PROVIDER=azure`). Capture results into the tables
below; any miss → a follow-up tuning ticket (keep this run measurement-only).

## Pass thresholds (initial — confirm with stakeholder)
| Dimension | Metric | Target |
|---|---|---|
| Parsing | name accuracy | ≥ 95% |
| Parsing | email accuracy | ≥ 90% |
| Parsing | phone accuracy | ≥ 90% |
| Parsing | education level | ≥ 85% |
| Parsing | experience (±12mo) | ≥ 80% |
| Parsing | skills recall | ≥ 70% |
| Parsing | macro average | ≥ 80% |
| Parsing | OCR confidence mean | ≥ 0.80, < 20% flagged manual-review |
| Scoring | no crashes / valid breakdown | 100% |
| Scoring | human band agreement (hi/mid/lo) | ≥ 80% of spot-checks |
| Branch | known cases correct | 100% |
| RBAC | matrix pass | 100% |
| Load | API p95 / error @ 30 VU | < 2s / < 1% |
| Load | pipeline drain | record CVs/min + 429 rate |

---

## 1. Parse accuracy (automated harness)
Prereq: labeled set in `backend/e2e/cvset/` (see `cv-test-set.md`); staging real Azure.
```bash
cd backend
CVSET_DIR=./e2e/cvset \
  E2E_API_URL=https://<staging-api> \
  DB_URL=<staging-db-url> \
  HR_COOKIE="hr_auth=<session>" \
  MACRO_MIN=0.80 \
  go test -tags e2e -run Accuracy -timeout 30m ./e2e/...
```
The harness prints a scorecard (per-field % + macro + OCR stats) and fails if macro <
`MACRO_MIN`. Paste the scorecard here:

```
(scorecard output)
```

## 2. Scoring vs JD (spot-check)
Pick ~8 candidates across positions. For each, record the AI score + your expected
band, and whether they agree.

| Candidate | Position | AI score | Expected band (hi/mid/lo) | Agree? | Notes |
|---|---|---|---|---|---|

Also confirm: breakdown dims within caps and sum ≤ total (covered by
`internal/scoring/scoring_test.go`), and the must-have gate auto-rejects clearly
unqualified CVs.

## 3. Branch assignment
Covered by `internal/branch/assigner_test.go` (nearest-store, no-vacancy talent pool,
unknown-province talent pool, location score). Spot-check 3–5 real candidates:

| Candidate province | Expected store/subregion | Assigned | Correct? |
|---|---|---|---|

## 4. RBAC matrix
Follow `docs/uat/rbac-matrix.md` per role (+ negative checks). Record pass/fail.

## 5. Load (staging)
```bash
k6 run -e TARGET=https://<staging-api> -e POSITION_ID=<uuid> \
  -e COOKIE="hr_auth=<session>" -e SAMPLE=./loadtest/sample.pdf loadtest/intake-load.js
```
Then run the **pipeline-drain** measurement from `loadtest/README.md`.

| Metric | Result |
|---|---|
| API p95 | |
| API error rate | |
| Pipeline drain (CVs/min) | |
| Azure 429 rate | |
| WORKER_CONCURRENCY used | |

## Cleanup (staging)
```sql
DELETE FROM applications WHERE candidate_id IN (
  SELECT id FROM candidates WHERE source_channel IN ('loadtest','bulk_upload')
);
DELETE FROM candidates WHERE source_channel IN ('loadtest','bulk_upload');
```

## Sign-off
- [ ] All thresholds met, or each miss has a follow-up ticket
- [ ] No PII committed (cvset gitignored, purged from staging)
- [ ] Results pasted above + reviewed with stakeholder
