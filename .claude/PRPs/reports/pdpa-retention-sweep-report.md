# Implementation Report: PDPA Retention Sweep

## Summary
Implemented a scheduler-driven background job that anonymizes candidate PII once the documented ≤1-year retention window elapses and the candidate is no longer in an active pipeline. Anonymization is in-place (redact candidate + application PII columns, delete resume blobs, null consent-ledger IPs, write a `retention_anonymize` audit entry) — not hard delete — to preserve referential integrity and de-identified analytics across the six tables that FK to `candidates(id)`. The job is gated OFF by default behind `RETENTION_SWEEP_ENABLED`, mirroring the codebase's mock-default seam pattern.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Medium-Large | Medium-Large (as predicted) |
| Confidence | 9/10 | 10/10 — single pass, no surprises |
| Files Changed | 13 (5 new, 8 updated) | 14 (6 new incl. migration down, 8 updated) |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | Migration 000009 (column + partial index) | ✅ Complete | Round-trips up/down |
| 2 | Config fields + getenvInt/getenvBool | ✅ Complete | `strconv` added |
| 3 | Queue task `retention:sweep` | ✅ Complete | mirrors `report:export` + `asynq.Unique` |
| 4 | RetentionService + Sweep | ✅ Complete | tx-per-candidate, persist-before-blob ordering |
| 5 | HandleRetentionSweep asynq handler | ✅ Complete | |
| 6 | blob.Delete + DeleteStored | ✅ Complete | idempotent on BlobNotFound |
| 7 | Scheduler wiring (gated register) | ✅ Complete | |
| 8 | Worker wiring (construct + HandleFunc) | ✅ Complete | added `internal/pdpa` import |
| 9 | .env.example + SECURITY.md | ✅ Complete | "future task" → implemented |
| 10 | Config unit tests | ✅ Complete | 2 tests |
| 11 | Integration test (eligibility + idempotency) | ✅ Complete | 2 tests, both pass against live stack |
| 12 | activity action const | ✅ Complete | `ActionRetentionAnonymize` |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static Analysis | ✅ Pass | `go vet` clean; golangci-lint `0 issues`; gosec exit 0; govulncheck no vulns |
| Unit Tests | ✅ Pass | full `go test ./...` ok; +2 config tests |
| Build | ✅ Pass | `go build ./...` ok |
| Integration | ✅ Pass | pdpa 2/2; full `-tags integration ./...` ok (no regressions) |
| Edge Cases | ✅ Pass | window boundary, active-pipeline skip, idempotency, consent-IP null all asserted |

## Files Changed

| File | Action | Lines |
|---|---|---|
| `backend/migrations/000009_pdpa_retention.up.sql` | CREATED | +9 |
| `backend/migrations/000009_pdpa_retention.down.sql` | CREATED | +2 |
| `backend/internal/pdpa/retention.go` | CREATED | +192 |
| `backend/internal/pdpa/worker.go` | CREATED | +27 |
| `backend/internal/pdpa/retention_integration_test.go` | CREATED | +210 |
| `backend/pkg/queue/tasks.go` | UPDATED | +40 |
| `backend/pkg/config/config.go` | UPDATED | +32 |
| `backend/pkg/config/config_test.go` | UPDATED | +46 |
| `backend/pkg/blob/blob.go` | UPDATED | +21 |
| `backend/cmd/scheduler/main.go` | UPDATED | +17 |
| `backend/cmd/worker/main.go` | UPDATED | +7 / -1 |
| `backend/internal/activity/activity.go` | UPDATED | +2 |
| `.env.example` | UPDATED | +9 |
| `docs/SECURITY.md` | UPDATED | +6 / -2 |

## Deviations from Plan
None. Implemented exactly as planned. The plan listed 13 files; the down-migration is a 6th new file (was implied by Task 1), bringing the actual count to 14.

## Issues Encountered
- One Edit on `activity.go` was rejected with "File has not been read yet" because earlier inspection used shell `cat`, not the Read tool — the exact session lesson the plan's Task 9 GOTCHA flagged. Resolved by reading via the Read tool first, then editing.

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `backend/pkg/config/config_test.go` | 2 | retention defaults + enabled/parsed values |
| `backend/internal/pdpa/retention_integration_test.go` | 2 | eligibility boundaries (expired+terminal / expired+active / recent), redaction of candidate+app+consent, blob+audit side effects, idempotent re-run |

## Design Decisions Confirmed in Implementation
- **Anonymize in place, not delete** — 6 FKs to `candidates(id)`; de-identification preserves analytics + audit.
- **Enable-gate at scheduler** — disabled envs never enqueue; handler stays pure for manual ops/testing.
- **Persist-before-side-effect** — DB redaction commits before best-effort blob deletes, so a storage error cannot roll back erasure.
- **Consent ledger kept** as legal proof, only its `ip_address` nulled.
- **Defensive clamp** — `RetentionDays<=0` → 365, preventing a misconfig mass-erase.

## Post-Implementation Review Fix (2026-06-03)
`/code-review` surfaced one **HIGH** issue (fixed before PR): the sweep erased only `applications.resume_blob_url`, missing the pipeline-derived PII blob columns added in migration 000003 — `raw_file_blob_url`, `ocr_text_blob_url`, `parsed_profile_blob_url`. The OCR-text blob is the full resume text (densest PII). Fixed: `piiBlobs` now UNIONs all four columns for storage deletion and the anonymize UPDATE nulls all four pointers; integration test strengthened to seed all four and assert 4 storage deletions + all columns NULL. Re-validated (build/vet/lint/gosec/integration all pass). See `.claude/PRPs/reviews/pdpa-retention-sweep-review.md`.

## Next Steps
- [x] Code review via `/code-review` — APPROVE (1 HIGH found + fixed)
- [ ] Create PR via `/prp-pr` (branch `feat/s7-pdpa-retention`, NO attribution, squash-merge)
- [ ] Open eligibility question for review: `hired` candidates are currently treated as erasable (PII pushed to PeopleSoft at hire). Confirm with the client whether hired PII must be retained in the ATS longer than 1 year.
