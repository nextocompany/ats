-- Post-hire onboarding documents (Module-3 3.8). One current document per
-- (application, doc_type); the file lives in blob, this row is the record + the
-- review state. The candidate uploads via the career-portal, HR reviews
-- (approve/reject with a reason). Onboarding completion is derived (every required
-- doc_type approved) — the application funnel is unaffected. Additive.
CREATE TABLE IF NOT EXISTS onboarding_documents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    doc_type       TEXT NOT NULL CHECK (doc_type IN ('id_card','house_registration','education_certificate','bank_book','tax_document','photo','health_check','military_certificate','name_change')),
    status         TEXT NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    blob_url       TEXT NOT NULL,
    file_name      TEXT,
    file_type      TEXT,
    review_reason  TEXT,
    uploaded_by    UUID REFERENCES candidate_accounts(id),
    reviewed_by    UUID REFERENCES users(id),
    uploaded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, doc_type)
);
CREATE INDEX IF NOT EXISTS idx_onboarding_documents_application ON onboarding_documents (application_id);
