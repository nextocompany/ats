-- Career-portal candidate membership (signup/login). A persistent candidate
-- identity that survives across applications: signup via LINE / Google / email-OTP,
-- a saved profile + one saved resume, and an httpOnly session. Applications still
-- create their own per-submission candidates row (the scoring pipeline is unchanged);
-- members link back via candidates.account_id.

-- candidate_accounts owns the login identity + saved profile/resume. Each identity
-- column is UNIQUE-when-present (NULL allowed many times) so an account may have
-- only one provider (email-only, line-only, google-only) and later link more.
CREATE TABLE IF NOT EXISTS candidate_accounts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name        VARCHAR(255) NOT NULL DEFAULT '',
    email            VARCHAR(255) UNIQUE,            -- nullable; unique when present
    email_verified   BOOLEAN NOT NULL DEFAULT FALSE,
    phone            VARCHAR(20),
    line_user_id     TEXT UNIQUE,                    -- verified LINE `sub`; nullable/unique
    line_display_id  VARCHAR(100),                   -- the @line id the user types (optional)
    google_sub       TEXT UNIQUE,                    -- verified Google `sub`; nullable/unique
    province         VARCHAR(100),
    resume_blob_url  TEXT,
    resume_file_type VARCHAR(10),                    -- pdf | docx | image
    pdpa_consent     BOOLEAN NOT NULL DEFAULT FALSE,
    pdpa_version     VARCHAR(10),
    pdpa_consent_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- candidate_sessions: opaque httpOnly session tokens (sha256-hashed at rest),
-- revocable, TTL via expires_at.
CREATE TABLE IF NOT EXISTS candidate_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id  UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,               -- sha256(token), never plaintext
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_candidate_sessions_account ON candidate_sessions (account_id);

-- email_otps: short-lived 6-digit passwordless codes (sha256-hashed), single-use.
CREATE TABLE IF NOT EXISTS email_otps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       VARCHAR(255) NOT NULL,
    code_hash   TEXT NOT NULL,                      -- sha256(code), never plaintext
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    attempts    INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_email_otps_email ON email_otps (email);

-- Link a (member) application's candidate row back to the owning account.
ALTER TABLE candidates ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES candidate_accounts(id);
