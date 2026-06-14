-- HR member-management (admin) — lifecycle + search support on candidate_accounts.
-- Adds an account status (active by default so existing members are unaffected),
-- suspend/anonymize audit timestamps, and indexes the directory list filters
-- (status + recency). suspended_by has NO FK to users: mock/dev operator ids need
-- not exist in the users table (same decision as application_fit_analyses.generated_by).
ALTER TABLE candidate_accounts
  ADD COLUMN IF NOT EXISTS status        VARCHAR(16) NOT NULL DEFAULT 'active', -- active | suspended | anonymized
  ADD COLUMN IF NOT EXISTS suspended_at  TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS suspended_by  UUID,
  ADD COLUMN IF NOT EXISTS anonymized_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_candidate_accounts_status  ON candidate_accounts (status);
CREATE INDEX IF NOT EXISTS idx_candidate_accounts_created ON candidate_accounts (created_at DESC);

-- candidates.account_id was added in 000013 without an index; the member directory
-- joins applications→candidates by account_id (applications-per-member count), so
-- index it to avoid a candidates scan per member row.
CREATE INDEX IF NOT EXISTS idx_candidates_account_id ON candidates (account_id) WHERE account_id IS NOT NULL;
