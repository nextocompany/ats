-- Dynamic RBAC: move authorization from compile-time Go role-string allowlists
-- to a data-driven role→permission model a super_admin can edit at runtime.
--
-- Design:
--   * PERMISSIONS are a fixed catalog (one key per gateable action in code — a
--     genuinely new capability is a new call site, so it still needs code). This
--     table exists mainly so the admin UI can label/group the matrix.
--   * ROLES, the ROLE→PERMISSION matrix, and each role's data SCOPE
--     (all/subregion/store) are dynamic and CRUD-able.
--   * The seed below reproduces EXACTLY the current hardcoded matrix (the Go
--     allowlists in internal/{settings,hrauth,executive,reports,reengage,members,
--     applications} + the role→scope switch in internal/rbac/scope.go), so the
--     cutover is a behavior no-op. A parity test asserts this.
--   * super_admin is additionally a hard code bypass in the authorizer, so it can
--     never be locked out regardless of edits here.
--
-- Roles live in their own table (TEXT key) rather than constraining users.role —
-- users.role stays a free-text VARCHAR(50) (Entra-claim roles may not have a row),
-- and the service layer validates assignability, failing closed for unknown roles.

CREATE TABLE IF NOT EXISTS rbac_permissions (
    key       TEXT PRIMARY KEY,            -- matches a Go constant in internal/rbac
    label_en  TEXT NOT NULL,
    label_th  TEXT NOT NULL,
    category  TEXT NOT NULL DEFAULT 'general', -- UI grouping only
    sort      INTEGER NOT NULL DEFAULT 0       -- display order within a category
);

CREATE TABLE IF NOT EXISTS rbac_roles (
    key         TEXT PRIMARY KEY,           -- slug: [a-z0-9_]+ (e.g. hr_manager)
    label_en    TEXT NOT NULL,
    label_th    TEXT NOT NULL,
    scope_kind  TEXT NOT NULL DEFAULT 'store'
                CHECK (scope_kind IN ('all', 'subregion', 'store')),
    is_builtin  BOOLEAN NOT NULL DEFAULT FALSE, -- built-ins cannot be deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rbac_role_permissions (
    role_key   TEXT NOT NULL REFERENCES rbac_roles(key) ON DELETE CASCADE,
    permission TEXT NOT NULL,              -- FK-less: catalog is code-owned
    PRIMARY KEY (role_key, permission)
);
CREATE INDEX IF NOT EXISTS idx_rbac_role_permissions_role ON rbac_role_permissions (role_key);

-- ── Seed: permission catalog ────────────────────────────────────────────────
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('settings.admin',      'System settings',          'การตั้งค่าระบบ',            'system', 10),
    ('users.admin',         'User accounts',            'บัญชีผู้ใช้',                'system', 20),
    ('rbac.admin',          'Roles & permissions',      'บทบาทและสิทธิ์',            'system', 30),
    ('executive.view',      'Executive overview',       'ภาพรวมผู้บริหาร',           'reporting', 10),
    ('reports.view',        'View reports',             'ดูรายงาน',                  'reporting', 20),
    ('reports.export',      'Export reports',           'ส่งออกรายงาน',              'reporting', 30),
    ('reengage.trigger',    'Trigger re-engagement',    'สั่งติดต่อกลับ',            'operations', 10),
    ('members.admin',       'Member management',        'จัดการสมาชิก',              'candidates', 10),
    ('members.erase',       'Erase member (PDPA)',      'ลบข้อมูลสมาชิก (PDPA)',     'candidates', 20),
    ('bulk.upload',         'Bulk CV upload',           'อัปโหลด CV จำนวนมาก',       'candidates', 30),
    ('assignment.write',    'Reassign branch',          'จัดสาขาผู้สมัคร',           'candidates', 40),
    ('offer.write',         'Manage offers',            'จัดการข้อเสนองาน',          'hiring', 10),
    ('onboarding.write',    'Review onboarding docs',   'ตรวจเอกสารออนบอร์ด',        'hiring', 20),
    ('letter.write',        'Generate letters',         'ออกหนังสือ',                'hiring', 30),
    ('scorecard.ta',        'TA scorecard',             'สกอร์การ์ด TA',             'hiring', 40),
    ('scorecard.lm',        'Line-manager scorecard',   'สกอร์การ์ดผู้จัดการสาขา',   'hiring', 50),
    ('approval.submit',     'Submit for approval',      'ส่งขออนุมัติ',              'approvals', 10),
    ('approval.decide.l1',  'Approve — Staff (L1)',     'อนุมัติ — เจ้าหน้าที่ (L1)', 'approvals', 20),
    ('approval.decide.l2',  'Approve — HR Manager (L2)','อนุมัติ — ผู้จัดการ HR (L2)','approvals', 30),
    ('approval.decide.l3',  'Approve — SGM (L3)',       'อนุมัติ — ผู้จัดการสาขา (L3)','approvals', 40),
    ('approval.decide.l4',  'Approve — Regional (L4)',  'อนุมัติ — ผู้อำนวยการเขต (L4)','approvals', 50)
ON CONFLICT (key) DO NOTHING;

-- ── Seed: the 7 built-in roles (scope mirrors internal/rbac/scope.go) ────────
INSERT INTO rbac_roles (key, label_en, label_th, scope_kind, is_builtin) VALUES
    ('super_admin',        'Super admin',        'ผู้ดูแลระบบสูงสุด',    'all',       TRUE),
    ('regional_director',  'Regional director',  'ผู้อำนวยการเขต',       'all',       TRUE),
    ('auditor',            'Auditor',            'ผู้ตรวจสอบ',           'all',       TRUE),
    ('operation_director', 'Operation director', 'ผู้อำนวยการปฏิบัติการ','subregion', TRUE),
    ('sgm',                'Store GM',           'ผู้จัดการสาขา',        'store',     TRUE),
    ('hr_manager',         'HR manager',         'ผู้จัดการ HR',         'store',     TRUE),
    ('hr_staff',           'HR staff',           'เจ้าหน้าที่ HR',       'store',     TRUE)
ON CONFLICT (key) DO NOTHING;

-- ── Seed: role→permission matrix (EXACTLY the current Go allowlists) ─────────
-- super_admin gets every permission (also a hard code bypass in the authorizer).
INSERT INTO rbac_role_permissions (role_key, permission)
    SELECT 'super_admin', key FROM rbac_permissions
ON CONFLICT DO NOTHING;

INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    -- regional_director: executive + reports(+export) + reengage + L4 approve
    ('regional_director', 'executive.view'),
    ('regional_director', 'reports.view'),
    ('regional_director', 'reports.export'),
    ('regional_director', 'reengage.trigger'),
    ('regional_director', 'approval.decide.l4'),
    -- auditor: read-only company-wide
    ('auditor', 'executive.view'),
    ('auditor', 'reports.view'),
    -- operation_director: subregion reports + reengage
    ('operation_director', 'reports.view'),
    ('operation_director', 'reengage.trigger'),
    -- sgm (store GM / line manager)
    ('sgm', 'reports.view'),
    ('sgm', 'bulk.upload'),
    ('sgm', 'assignment.write'),
    ('sgm', 'onboarding.write'),
    ('sgm', 'letter.write'),
    ('sgm', 'scorecard.lm'),
    ('sgm', 'approval.decide.l3'),
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
    -- hr_staff
    ('hr_staff', 'reports.view'),
    ('hr_staff', 'bulk.upload'),
    ('hr_staff', 'onboarding.write'),
    ('hr_staff', 'letter.write'),
    ('hr_staff', 'scorecard.ta'),
    ('hr_staff', 'approval.submit'),
    ('hr_staff', 'approval.decide.l1')
ON CONFLICT DO NOTHING;
