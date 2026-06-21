-- PDPA consent v2 (Phase 2): versioned consent registry + a unified ledger.
--
-- 1) consent_documents is the authoritative source of the CURRENT privacy/consent
--    notice version. New consents stamp this version instead of a hardcoded "1.0";
--    old-version consents stay valid history. One bilingual document set (th + en)
--    per version; exactly one version is_current at a time.
CREATE TABLE IF NOT EXISTS consent_documents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version      VARCHAR(10) NOT NULL,            -- e.g. '1.0'
    locale       VARCHAR(5)  NOT NULL,            -- 'th' | 'en'
    title        TEXT NOT NULL DEFAULT '',
    body         TEXT NOT NULL DEFAULT '',        -- notice text (rendered on /privacy in Phase 4)
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_current   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (version, locale)
);

-- At most one current document per locale (partial unique index).
CREATE UNIQUE INDEX IF NOT EXISTS uq_consent_documents_current
    ON consent_documents (locale) WHERE is_current;

-- Seed the existing notice as v1.0 (matches the value the apps have been sending),
-- so historical consents stamped "1.0" align with a real registry row.
INSERT INTO consent_documents (version, locale, title, body, is_current)
VALUES
    ('1.0','th','นโยบายความเป็นส่วนตัวและความยินยอม (PDPA)',
        'ข้าพเจ้ายินยอมให้ CP Axtra เก็บรวบรวม ใช้ และเปิดเผยข้อมูลส่วนบุคคลของข้าพเจ้าเพื่อวัตถุประสงค์ในการสรรหาและคัดเลือกบุคลากร ตามพระราชบัญญัติคุ้มครองข้อมูลส่วนบุคคล พ.ศ. 2562',
        TRUE),
    ('1.0','en','Privacy Notice and Consent (PDPA)',
        'I consent to CP Axtra collecting, using, and disclosing my personal data for recruitment and selection purposes, in accordance with the Personal Data Protection Act B.E. 2562.',
        TRUE)
ON CONFLICT (version, locale) DO NOTHING;

-- 2) Unify the consent stores: pdpa_consents becomes the single ledger. It already
--    keys on candidate_id (apply flow); add account_id so portal account events
--    (signup consent, withdrawal) record into the SAME trail before any candidate
--    row exists. Both columns are nullable; a row carries whichever is known.
ALTER TABLE pdpa_consents ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES candidate_accounts(id);
CREATE INDEX IF NOT EXISTS idx_pdpa_consents_account ON pdpa_consents (account_id);
