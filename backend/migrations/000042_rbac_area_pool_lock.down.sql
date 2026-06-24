-- Reverse 000042. Drops the new roles/permission, the area/lock structures, the
-- new columns, and restores the original scope_kind CHECK (all/subregion/store).

-- New role grants + roles (role_permissions cascade on role delete).
DELETE FROM rbac_roles WHERE key IN
    ('hr_store', 'area_hr', 'hiring_manager_store', 'hiring_manager_ho', 'ta');

-- area.admin permission (and any stray grants).
DELETE FROM rbac_role_permissions WHERE permission = 'area.admin';
DELETE FROM rbac_permissions WHERE key = 'area.admin';

-- Restore the original scope_kind CHECK (must happen after the 'area'/'requisition'
-- roles above are gone, or the constraint would reject existing rows).
ALTER TABLE rbac_roles DROP CONSTRAINT IF EXISTS rbac_roles_scope_kind_check;
ALTER TABLE rbac_roles ADD CONSTRAINT rbac_roles_scope_kind_check
    CHECK (scope_kind IN ('all', 'subregion', 'store'));

-- Candidate lock.
DROP TABLE IF EXISTS candidate_locks;

-- Application tracking columns + sweep index.
DROP INDEX IF EXISTS idx_applications_pickup_sla;
ALTER TABLE applications DROP COLUMN IF EXISTS released_to_pool_at;
ALTER TABLE applications DROP COLUMN IF EXISTS picked_up_by;
ALTER TABLE applications DROP COLUMN IF EXISTS picked_up_at;

-- Application ↔ vacancy link.
DROP INDEX IF EXISTS idx_applications_vacancy;
ALTER TABLE applications DROP COLUMN IF EXISTS vacancy_id;

-- Requisition hiring-manager link.
DROP INDEX IF EXISTS idx_vacancies_hiring_manager;
ALTER TABLE vacancies DROP COLUMN IF EXISTS hiring_manager_user_id;

-- Area structures.
DROP TABLE IF EXISTS user_areas;
DROP TABLE IF EXISTS area_stores;
DROP TABLE IF EXISTS areas;
