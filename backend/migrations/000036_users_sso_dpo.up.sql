-- 000036: persist SSO users + DPO designation foundation (PDPA DPO feature, phase 1).
--
-- The HR console authenticates via Entra SSO OR a local password account. SSO
-- identities were previously never written to the DB (role/identity came from the
-- token claims at request time). To let admins manage authorization in-app and
-- designate Data Protection Officers across BOTH sign-in methods, SSO users are now
-- upserted into `users` on login (JIT provisioning).
--
-- Columns added (all additive, safe defaults):
--   source  - where the account originates: 'local' (password) or 'sso' (Entra JIT).
--   phone   - DPO contact phone (PDPA s.41); not available from Entra tokens, set in-app.
--   is_dpo  - marks an account as a published Data Protection Officer, independent of
--             the permission role (a user can be hr_manager AND a DPO).

ALTER TABLE users ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'local';
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone  VARCHAR(20);
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_dpo BOOLEAN NOT NULL DEFAULT FALSE;

-- Partial index: the public /privacy page reads only the (small) DPO set.
CREATE INDEX IF NOT EXISTS idx_users_is_dpo ON users (is_dpo) WHERE is_dpo = TRUE;
