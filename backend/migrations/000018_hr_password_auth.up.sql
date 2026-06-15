-- HR password authentication: a second sign-in path alongside Entra SSO.
-- Local HR accounts store a bcrypt password hash on the existing users table
-- (NULL = SSO-only user, no password set). hr_sessions are opaque server-side
-- sessions (sha256-hashed token, httpOnly cookie) mirroring candidate_sessions —
-- revocable and expiring, unlike a stateless JWT.

-- TEXT (not VARCHAR(n)) leaves headroom for a future hash algorithm (Argon2id
-- strings are longer than bcrypt's 60 bytes) without an ALTER.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash       TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_updated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS hr_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,               -- sha256(token), never plaintext
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hr_sessions_user ON hr_sessions (user_id);
-- Explicit index on the session-lookup hot path. The UNIQUE constraint already
-- creates one implicitly; this makes the dependency visible so a future change to
-- the uniqueness constraint cannot silently drop the lookup index.
CREATE INDEX IF NOT EXISTS idx_hr_sessions_token ON hr_sessions (token_hash);
