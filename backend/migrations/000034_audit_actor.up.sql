-- PDPA Phase 5.1 audit hardening: record WHO and WHERE for PDPA-relevant events.
--
-- activity_logs already has user_id / ip_address / user_agent columns (000001),
-- but they were never populated, and user_id carries a FK to users(id). HR actors
-- authenticate via Entra SSO and carry an OID that has no row in `users` (only
-- local password accounts do); candidate self-service DSAR actors are rows in
-- candidate_accounts. Neither is in `users`, so writing their id as the actor
-- would 23503-fail against the FK.
--
-- Drop the FK so user_id becomes a plain audit pointer - the same FK-less actor
-- rule already used by vacancies.created_by (000029), dsar_requests (000032), and
-- data_breaches (000033). The column stays UUID; the application writes the OID /
-- account id / local user id as appropriate.
ALTER TABLE activity_logs DROP CONSTRAINT IF EXISTS activity_logs_user_id_fkey;

-- Compliance queries filter the audit trail by action over a time window (e.g.
-- every dsar_* / consent_* / breach_* / view_resume event in the last 12 months).
CREATE INDEX IF NOT EXISTS idx_activity_logs_action ON activity_logs (action, created_at DESC);
