-- Reverse 000029. Remove the new RBAC keys first (FK-less, so plain DELETE), then
-- the index and the added columns. PeopleSoft-synced vacancies are unaffected.
DELETE FROM rbac_role_permissions WHERE permission LIKE 'requisition.%';
DELETE FROM rbac_permissions WHERE key LIKE 'requisition.%';

DROP INDEX IF EXISTS idx_vacancies_source_status;

ALTER TABLE vacancies
  DROP COLUMN IF EXISTS source,
  DROP COLUMN IF EXISTS created_by,
  DROP COLUMN IF EXISTS approved_by,
  DROP COLUMN IF EXISTS approved_at,
  DROP COLUMN IF EXISTS created_at,
  DROP COLUMN IF EXISTS updated_at;
