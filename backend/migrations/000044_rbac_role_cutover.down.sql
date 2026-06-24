-- Reverse the RBAC role cutover: re-create the old roles with their original
-- permission matrix (000028 + 000029), remove the requisition grants 000044 ported
-- onto the new roles, and move users + approval steps back.
--
-- LOSSY ROLLBACK: the up-migration collapsed two old roles into area_hr
-- (hr_manager + operation_director). This down cannot tell them apart, so every
-- area_hr user reverts to hr_manager. operation_director assignments are not
-- recoverable from role alone — re-assign by hand if needed.

-- 1. Re-create the old built-in roles (000028 seed).
INSERT INTO rbac_roles (key, label_en, label_th, scope_kind, is_builtin) VALUES
    ('regional_director',  'Regional director',  'ผู้อำนวยการเขต',        'all',       TRUE),
    ('operation_director', 'Operation director', 'ผู้อำนวยการปฏิบัติการ', 'subregion', TRUE),
    ('sgm',                'Store GM',           'ผู้จัดการสาขา',         'store',     TRUE),
    ('hr_manager',         'HR manager',         'ผู้จัดการ HR',          'store',     TRUE),
    ('hr_staff',           'HR staff',           'เจ้าหน้าที่ HR',        'store',     TRUE)
ON CONFLICT (key) DO NOTHING;

-- 2. Restore the old role→permission matrix (000028 + 000029).
INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    -- regional_director
    ('regional_director', 'executive.view'),
    ('regional_director', 'reports.view'),
    ('regional_director', 'reports.export'),
    ('regional_director', 'reengage.trigger'),
    ('regional_director', 'approval.decide.l4'),
    ('regional_director', 'requisition.manage'),
    ('regional_director', 'requisition.approve'),
    -- operation_director
    ('operation_director', 'reports.view'),
    ('operation_director', 'reengage.trigger'),
    ('operation_director', 'requisition.manage'),
    ('operation_director', 'requisition.approve'),
    -- sgm
    ('sgm', 'reports.view'),
    ('sgm', 'bulk.upload'),
    ('sgm', 'assignment.write'),
    ('sgm', 'onboarding.write'),
    ('sgm', 'letter.write'),
    ('sgm', 'scorecard.lm'),
    ('sgm', 'approval.decide.l3'),
    ('sgm', 'requisition.manage'),
    -- hr_manager
    ('hr_manager', 'members.admin'),
    ('hr_manager', 'reports.view'),
    ('hr_manager', 'bulk.upload'),
    ('hr_manager', 'assignment.write'),
    ('hr_manager', 'offer.write'),
    ('hr_manager', 'onboarding.write'),
    ('hr_manager', 'letter.write'),
    ('hr_manager', 'scorecard.ta'),
    ('hr_manager', 'approval.decide.l2'),
    ('hr_manager', 'requisition.manage'),
    -- hr_staff
    ('hr_staff', 'reports.view'),
    ('hr_staff', 'bulk.upload'),
    ('hr_staff', 'onboarding.write'),
    ('hr_staff', 'letter.write'),
    ('hr_staff', 'scorecard.ta'),
    ('hr_staff', 'approval.submit'),
    ('hr_staff', 'approval.decide.l1')
ON CONFLICT DO NOTHING;

-- 3. Remove the requisition grants the up-migration ported onto the new roles.
DELETE FROM rbac_role_permissions
WHERE (role_key = 'ta'                   AND permission IN ('requisition.manage', 'requisition.approve'))
   OR (role_key = 'area_hr'              AND permission IN ('requisition.manage', 'requisition.approve'))
   OR (role_key = 'hiring_manager_store' AND permission = 'requisition.manage')
   OR (role_key = 'hiring_manager_ho'    AND permission = 'requisition.manage');

-- 4. Move users + approval steps back (lossy on area_hr → see header).
UPDATE users SET role = 'hr_staff'          WHERE role = 'hr_store';
UPDATE users SET role = 'hr_manager'        WHERE role = 'area_hr';
UPDATE users SET role = 'sgm'               WHERE role = 'hiring_manager_store';
UPDATE users SET role = 'regional_director' WHERE role = 'ta';

UPDATE approval_steps SET role = 'hr_staff'          WHERE role = 'hr_store';
UPDATE approval_steps SET role = 'hr_manager'        WHERE role = 'area_hr';
UPDATE approval_steps SET role = 'sgm'               WHERE role = 'hiring_manager_store';
UPDATE approval_steps SET role = 'regional_director' WHERE role = 'ta';
