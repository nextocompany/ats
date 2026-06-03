-- Sprint 7: PDPA retention sweep. pdpa_anonymized_at marks candidates whose PII
-- has been erased so the daily sweep is idempotent (re-runs skip set rows). The
-- partial index keeps the eligibility scan cheap as the table grows: only rows
-- still pending anonymization are indexed by their retention clock (created_at).
ALTER TABLE candidates ADD COLUMN pdpa_anonymized_at TIMESTAMPTZ;

CREATE INDEX idx_candidates_retention_pending
    ON candidates (created_at)
    WHERE pdpa_anonymized_at IS NULL;
