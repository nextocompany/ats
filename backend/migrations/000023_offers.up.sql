-- Offer package for a hired-track application (Module-3 3.6). One offer per
-- application; its lifecycle is draft -> sent -> accepted/declined/expired. Accept
-- flips the application to 'hired' (+ PeopleSoft push); decline flips it to
-- 'rejected'. Additive; existing applications are unaffected.
CREATE TABLE IF NOT EXISTS offers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'draft', -- draft | sent | accepted | declined | expired
    salary         NUMERIC(12,2),
    start_date     DATE,
    terms          TEXT,
    created_by     UUID REFERENCES users(id),
    sent_at        TIMESTAMPTZ,
    responded_at   TIMESTAMPTZ,
    expires_at     TIMESTAMPTZ,
    decline_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers (status);
