# Load testing — CV intake + pipeline throughput

Two things to measure, because they have different bottlenecks:

1. **Intake API under concurrency** (HTTP) — upload + enqueue. Fast; bounded by the
   API + blob upload. Measured by `intake-load.js` (k6).
2. **Pipeline drain throughput** (async worker) — OCR + LLM parse + score per CV.
   This is the **real** capacity constraint and the headline number for "load จริง".
   Measured by the drain procedure below.

> ⚠️ Run against a **staging / throwaway** stack. Never point load tests at the
> production `hr_db` — they create real candidate/application rows. Cleanup query at
> the bottom.

## Prerequisites
- [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/) installed (`brew install k6`).
- A sample CV file locally (pdf/docx/png/jpg) — **not committed**. Pass via `-e SAMPLE=`.
- A valid position uuid (`GET /api/v1/positions`).
- An authenticated HR session cookie (`hr_auth=…`) or an Entra bearer token. To get a
  cookie: log in to the dashboard and copy the `hr_auth` cookie, or hit
  `POST /api/v1/auth/login` and read the `Set-Cookie`.

## 1. Intake API load (k6)
```bash
k6 run \
  -e TARGET=https://hrats-staging-api.example \
  -e POSITION_ID=<uuid> \
  -e COOKIE="hr_auth=<session>" \
  -e SAMPLE=./loadtest/sample.pdf \
  loadtest/intake-load.js
```
Ramps 0→30 VUs over 2 min. **Thresholds** (the run fails if breached):
- `http_req_failed rate < 1%`
- `http_req_duration p(95) < 2s`

These are **initial targets** — adjust in `options.thresholds` once a baseline is known.

## 2. Pipeline drain throughput
The async worker is the constraint. To measure CVs/min end-to-end:

1. Note the current scored/rejected counts:
   ```bash
   curl -s "$TARGET/api/v1/applications?status=scored&limit=1"   # read total from meta
   curl -s "$TARGET/api/v1/applications?status=rejected&limit=1"
   ```
2. Submit a known batch (e.g. 50 CVs) via the dashboard **Bulk upload** page or the
   bulk-intake endpoint.
3. Poll the same counts every ~30s until they stop rising; `Δterminal / minutes` =
   **CVs/min** drain rate.

### Tuning knobs
- `WORKER_CONCURRENCY` (worker env, default 10) — in-flight pipeline tasks.
- **Azure OpenAI TPM** + **DocIntel rate** — the true ceiling. Raising
  `WORKER_CONCURRENCY` past the TPM budget produces HTTP 429s (the pipeline retries,
  so no data loss — just a slower, capped drain). The cross-position fit feature
  already required a TPM bump (gpt4omini-gs 10→50); expect to tune TPM and
  concurrency **together**.

### Expected bottleneck
OCR (DocIntel) + parse + score each make Azure calls per CV. At default concurrency
the drain rate tracks the Azure tier, not the worker CPU. Report the measured
CVs/min and the observed 429 rate so the Azure tier can be sized to the SLA.

## Cleanup (staging)
Load-test rows use `source_channel='loadtest'` (k6) / `'bulk_upload'` (UI). Remove on
staging with a scoped delete (adjust as needed):
```sql
DELETE FROM applications WHERE candidate_id IN (
  SELECT id FROM candidates WHERE source_channel = 'loadtest'
);
DELETE FROM candidates WHERE source_channel = 'loadtest';
```
