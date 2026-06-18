-- Generated PDF letters (Module-3 3.3): interview invitation + offer letter, one
-- current letter per (application, type). The PDF lives in blob; this row is the
-- audit record + the handle for re-download. Additive.
CREATE TABLE IF NOT EXISTS letters (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    type           TEXT NOT NULL CHECK (type IN ('interview', 'offer')),
    blob_url       TEXT NOT NULL,
    created_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, type)
);
CREATE INDEX IF NOT EXISTS idx_letters_application ON letters (application_id);
