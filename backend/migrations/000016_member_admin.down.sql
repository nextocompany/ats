DROP INDEX IF EXISTS idx_candidates_account_id;
DROP INDEX IF EXISTS idx_candidate_accounts_created;
DROP INDEX IF EXISTS idx_candidate_accounts_status;
ALTER TABLE candidate_accounts
  DROP COLUMN IF EXISTS anonymized_at,
  DROP COLUMN IF EXISTS suspended_by,
  DROP COLUMN IF EXISTS suspended_at,
  DROP COLUMN IF EXISTS status;
