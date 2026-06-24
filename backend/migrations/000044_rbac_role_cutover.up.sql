-- RBAC role cutover: migrate from the old single-axis roles to the new 2-axis
-- model, then retire the old roles. The new roles + their operate/approval grants
-- were already seeded ALONGSIDE the old ones (000042/000043); this migration moves
-- the live users onto them, ports the requisition grants the new-role seeds missed
-- (000029 granted them to old roles only), and removes the retired roles.
--
-- Mapping (confirmed with client):
--   hr_staff           -> hr_store
--   hr_manager         -> area_hr
--   operation_director -> area_hr
--   sgm                -> hiring_manager_store
--   regional_director  -> ta
--   auditor, super_admin: unchanged
--
-- SEQUENCE MATTERS: reassign users FIRST, then retire the roles. (users.role is
-- free text with no FK, so a user left on a deleted role key would fail closed to
-- store scope — a lockout. Reassigning first guarantees nobody is stranded.)
--
-- OPS PREREQUISITE: area_hr / hiring_manager_* users see nothing until an admin
-- defines areas (area_stores + user_areas) and requisitions have an owner
-- (vacancies.hiring_manager_user_id). That setup is out of band — fail-closed by
-- design, but plan it alongside this migration.

-- 1. Reassign live users old -> new role.
UPDATE users SET role = 'hr_store'             WHERE role = 'hr_staff';
UPDATE users SET role = 'area_hr'              WHERE role IN ('hr_manager', 'operation_director');
UPDATE users SET role = 'hiring_manager_store' WHERE role = 'sgm';
UPDATE users SET role = 'ta'                    WHERE role = 'regional_director';

-- 2. Reassign in-flight approval step role labels (drive SLA escalation + the
--    "next pending level" notification targeting). Matches approvalChain in
--    internal/applications/approval.go.
UPDATE approval_steps SET role = 'hr_store'             WHERE role = 'hr_staff';
UPDATE approval_steps SET role = 'area_hr'              WHERE role = 'hr_manager';
UPDATE approval_steps SET role = 'hiring_manager_store' WHERE role = 'sgm';
UPDATE approval_steps SET role = 'ta'                    WHERE role = 'regional_director';

-- 3. Port the requisition grants (000029) to the mapped new roles — the new-role
--    seeds (000042/000043) never granted these, so without this the requisition
--    feature silently breaks for migrated users. Faithful 1:1 port:
--      regional_director(manage,approve)  -> ta
--      operation_director(manage,approve) -> area_hr
--      hr_manager(manage)                 -> area_hr (dup of above)
--      sgm(manage)                        -> hiring_manager_store
INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('ta',                   'requisition.manage'),
    ('ta',                   'requisition.approve'),
    ('area_hr',              'requisition.manage'),
    ('area_hr',              'requisition.approve'),
    ('hiring_manager_store', 'requisition.manage')
ON CONFLICT DO NOTHING;

-- 4. Retire the old roles. Deleting an rbac_roles row CASCADEs its
--    rbac_role_permissions (FK ON DELETE CASCADE).
DELETE FROM rbac_roles WHERE key IN
    ('hr_staff', 'hr_manager', 'sgm', 'operation_director', 'regional_director');
