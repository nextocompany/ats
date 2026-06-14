-- Runtime, admin-managed system settings. A tiny key/value table for flags an
-- operator can flip from the HR console without a redeploy. First use: the
-- "allow all Entra tenants to sign in" toggle (auth middleware reads it per
-- request, cached). updated_by records the admin email for a basic audit trail.
CREATE TABLE IF NOT EXISTS system_settings (
    key        VARCHAR(100) PRIMARY KEY,
    value_bool BOOLEAN,
    updated_by VARCHAR(255) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the tenant toggle ON: any Entra directory may sign in by default. A
-- super_admin can later restrict to the static AZURE_AD_ALLOWED_TENANTS allowlist
-- from the admin console. (Issuer-binding and per-token verification still apply,
-- and no-role users get the most restrictive scope.)
INSERT INTO system_settings (key, value_bool)
VALUES ('allow_all_entra_tenants', TRUE)
ON CONFLICT (key) DO NOTHING;
