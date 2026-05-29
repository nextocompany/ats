DROP INDEX IF EXISTS idx_applications_talent_pool;
DROP INDEX IF EXISTS idx_candidates_full_name;

ALTER TABLE applications
  DROP COLUMN IF EXISTS dedup_confidence,
  DROP COLUMN IF EXISTS dedup_state,
  DROP COLUMN IF EXISTS talent_pool;
