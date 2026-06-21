-- DSAR held-request queue (PDPA Phase 3): when a candidate self-requests erasure
-- but a linked subject is under legal hold (hired - employment data retained on a
-- different lawful basis), the request cannot be auto-fulfilled. It is queued here
-- for HR/DPO to action through the Phase 5 admin console instead of silently
-- erasing or silently refusing.
CREATE TABLE IF NOT EXISTS dsar_requests (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
    request_type TEXT NOT NULL DEFAULT 'erasure',  -- erasure (+ future: access/rectify)
    status       TEXT NOT NULL DEFAULT 'pending',   -- pending | completed | rejected
    reason       TEXT,                              -- why held (e.g. 'hired')
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at  TIMESTAMPTZ,
    -- resolving HR actor id; NO FK to users(id): Entra SSO actors carry an OID that
    -- is not in the users table (see requisition-management lesson).
    resolved_by  UUID
);

CREATE INDEX IF NOT EXISTS idx_dsar_requests_status ON dsar_requests (status);
CREATE INDEX IF NOT EXISTS idx_dsar_requests_account ON dsar_requests (account_id);
