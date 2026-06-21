DROP INDEX IF EXISTS idx_pdpa_consents_account;
ALTER TABLE pdpa_consents DROP COLUMN IF EXISTS account_id;
DROP INDEX IF EXISTS uq_consent_documents_current;
DROP TABLE IF EXISTS consent_documents;
