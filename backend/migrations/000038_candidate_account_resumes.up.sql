-- Candidate resume library: a candidate account keeps a history of uploaded CVs
-- (the portal caps it at 5) and marks one as the default used for quick-apply.
-- candidate_accounts.resume_blob_url / resume_file_type stay as a denormalized
-- pointer to the DEFAULT resume so the existing quick-apply read path is unchanged.

CREATE TABLE IF NOT EXISTS candidate_account_resumes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id        UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
    blob_key          TEXT NOT NULL,
    original_filename VARCHAR(255) NOT NULL DEFAULT '',
    file_type         VARCHAR(10)  NOT NULL,
    is_default        BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_car_account_created
    ON candidate_account_resumes (account_id, created_at DESC);

-- At most one default resume per account.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_car_default
    ON candidate_account_resumes (account_id) WHERE is_default;

-- Backfill: every account that already has a single saved resume keeps it as the
-- default entry of its new library, so quick-apply and the pointer stay valid.
INSERT INTO candidate_account_resumes (account_id, blob_key, file_type, is_default, created_at)
SELECT id, resume_blob_url, COALESCE(NULLIF(resume_file_type, ''), 'pdf'), TRUE, now()
FROM candidate_accounts
WHERE resume_blob_url IS NOT NULL AND resume_blob_url <> '';
