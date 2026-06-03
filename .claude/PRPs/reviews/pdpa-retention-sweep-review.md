# Code Review: PDPA Retention Sweep (local, branch `feat/s7-pdpa-retention`)

**Reviewed**: 2026-06-03
**Mode**: Local (uncommitted changes vs `main`)
**Decision**: APPROVE (1 HIGH found and fixed during review)

## Summary
A scheduler-driven, enable-gated job that anonymizes expired candidate PII in place. Implementation closely mirrors the `report:export` slice and existing pdpa/reengage patterns. Review surfaced one genuine compliance hole — the sweep erased only the intake resume blob, missing the three pipeline-derived PII blob columns (raw file, OCR text, parsed profile). Fixed and re-verified before sign-off.

## Findings

### CRITICAL
None.

### HIGH
1. **Incomplete PII erasure — pipeline resume blobs left intact** (`backend/internal/pdpa/retention.go`) — **FIXED.**
   The original sweep gathered/nulled only `applications.resume_blob_url` (+ filename, ai_summary, ai_red_flags). But migration `000003_pipeline_columns` adds `raw_file_blob_url`, `ocr_text_blob_url`, and `parsed_profile_blob_url`, which the pipeline populates (`applications/repository.go:81,108-109`). The OCR-text blob is the full text of the candidate's resume — the single most PII-dense object — and it (plus the raw upload and parsed profile) was neither redacted nor deleted from storage. For a PDPA retention feature this defeats the purpose.
   **Fix applied:** `piiBlobs` now `UNION`s all four blob columns for deletion; the anonymize `UPDATE` nulls all four pointers. Integration test strengthened to seed all four and assert 4 storage deletions + all columns NULL. Re-ran: pass.

### MEDIUM
None.

### LOW
1. **Orphaned-blob residue on crash between commit and blob-delete** (`retention.go` Sweep) — By design, DB redaction commits before best-effort blob deletes (persist-before-side-effect, matching `reports`). If the process dies after commit but before deletion, the blob is orphaned in storage with no DB pointer, and the now-marked candidate is never re-swept — leaving residual PII in Blob. Acceptable trade-off for v1 (documented), but a future periodic orphan-blob reaper would close it fully. No change.
2. **`notifications.payload` / `activity_logs` JSONB not scrubbed** — Both tables FK to `candidates` and could in principle hold PII in JSONB. Verified: nothing currently `INSERT`s into `notifications` (the notify path posts to LINE/email + logs, no persistence), and candidate `activity_logs` entries record status/IDs/consent, not names. No residual PII today; flagged for re-check if those tables start carrying contact data. No change.
3. **`hired` candidates treated as erasable** (`retention.go` eligibility) — `hired` is terminal, so a hired candidate's PII is anonymized after the window. Intentional (PII pushed to PeopleSoft at hire) and documented, but a policy question for the client: confirm hired PII need not be retained in the ATS beyond 1 year. No code change pending confirmation.

## Validation Results

| Check | Result |
|---|---|
| Build (`go build ./...`) | Pass |
| Vet (`go vet ./...`, incl. `-tags integration`) | Pass |
| Lint (`golangci-lint run ./...`) | Pass — 0 issues |
| Security (`gosec -exclude-generated ./...`) | Pass — exit 0 |
| Unit tests (`go test ./...`) | Pass |
| Integration (`-tags integration ./internal/pdpa/...`) | Pass — 2/2 after fix |
| Migration round-trip (000009 down→up) | Pass |

## Files Reviewed
- `backend/internal/pdpa/retention.go` (Added) — **fix applied during review**
- `backend/internal/pdpa/worker.go` (Added)
- `backend/internal/pdpa/retention_integration_test.go` (Added) — **strengthened during review**
- `backend/migrations/000009_pdpa_retention.up.sql` / `.down.sql` (Added)
- `backend/pkg/queue/tasks.go` (Modified)
- `backend/pkg/config/config.go` / `config_test.go` (Modified)
- `backend/pkg/blob/blob.go` (Modified)
- `backend/cmd/scheduler/main.go` / `cmd/worker/main.go` (Modified)
- `backend/internal/activity/activity.go` (Modified)
- `.env.example`, `docs/SECURITY.md` (Modified)

## Notes on what was checked and found clean
- **SQL injection**: all queries parameterized (`$1`/`$2`); no string concatenation. ✅
- **Secrets**: none introduced; sweep logs candidate UUIDs only, never PII values. ✅
- **Error handling**: every error wrapped `pdpa: <op>: %w`; per-candidate failures logged and skipped, not swallowed silently. ✅
- **Concurrency/idempotency**: `asynq.Unique` TTL + `WHERE pdpa_anonymized_at IS NULL` re-check in UPDATE; idempotent re-run verified (2nd sweep returns 0, no repeat blob deletes). ✅
- **Misconfig safety**: `RetentionDays<=0`→365 clamp; disabled by default; scheduler only registers cron when enabled. ✅
- **Function/file size**: largest function (`Sweep`) ~40 lines; `retention.go` ~200 lines. Within limits. ✅
- **Migration**: sequential 000009; partial index predicate is IMMUTABLE-safe; round-trips. ✅
