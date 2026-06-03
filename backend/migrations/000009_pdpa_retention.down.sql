DROP INDEX IF EXISTS idx_candidates_retention_pending;
ALTER TABLE candidates DROP COLUMN IF EXISTS pdpa_anonymized_at;
