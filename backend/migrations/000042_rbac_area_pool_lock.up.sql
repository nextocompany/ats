-- RBAC redesign — foundation (additive, behavior-preserving for existing roles).
--
-- Adds the structural pieces the new 2-axis role model needs WITHOUT retiring the
-- old roles yet (cutover + user migration + approval-chain remap happen later, see
-- .claude/PRPs/plans/rbac-role-redesign-analysis.plan.md). Everything here is
-- additive: new tables, new nullable columns, an extended scope_kind CHECK, and new
-- roles seeded ALONGSIDE the existing 7. Existing role scoping is untouched.
--
-- New visibility scopes:
--   * area        — a dynamic, admin-managed grouping of ~10-20 stores (replaces the
--                   compiled-in 13-value subregion enum for visibility; subregion is
--                   kept only as a branch-assignment attribute).
--   * requisition — a hiring manager sees only candidates in positions they opened.

-- ── 1. Area (dynamic store grouping) ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS areas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(120) NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which stores belong to an area (M:N, editable; a store may sit in many areas).
CREATE TABLE IF NOT EXISTS area_stores (
    area_id   UUID NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    store_no  INTEGER NOT NULL REFERENCES stores(store_no),
    PRIMARY KEY (area_id, store_no)
);
CREATE INDEX IF NOT EXISTS idx_area_stores_store ON area_stores (store_no);

-- Which area(s) a user covers (M:N, changes often; one area_hr may cover several).
CREATE TABLE IF NOT EXISTS user_areas (
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    area_id  UUID NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, area_id)
);
CREATE INDEX IF NOT EXISTS idx_user_areas_area ON user_areas (area_id);

-- ── 2. Requisition ownership (real hiring-manager link) ─────────────────────
-- created_by is an audit UUID that may be an Entra OID with no users row; the
-- visibility link must be a resolvable users.id (populate via email at req-open).
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS hiring_manager_user_id UUID REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_vacancies_hiring_manager ON vacancies (hiring_manager_user_id);

-- ── 3. Link application ↔ vacancy (currently absent) ────────────────────────
-- Lets the requisition scope join application → vacancy → owning hiring manager.
-- Populated at branch-assign time (the assigner already resolves the open vacancy).
ALTER TABLE applications ADD COLUMN IF NOT EXISTS vacancy_id UUID REFERENCES vacancies(id);
CREATE INDEX IF NOT EXISTS idx_applications_vacancy ON applications (vacancy_id);

-- ── 4. 3-day pickup / pool-release tracking (sweep wired in a later phase) ───
ALTER TABLE applications ADD COLUMN IF NOT EXISTS picked_up_at        TIMESTAMPTZ;
ALTER TABLE applications ADD COLUMN IF NOT EXISTS picked_up_by        UUID REFERENCES users(id);
ALTER TABLE applications ADD COLUMN IF NOT EXISTS released_to_pool_at TIMESTAMPTZ;
-- Sweep candidate index: store-specific apps not yet picked up.
CREATE INDEX IF NOT EXISTS idx_applications_pickup_sla
    ON applications (created_at)
    WHERE picked_up_at IS NULL AND talent_pool = FALSE AND assigned_store_id IS NOT NULL;

-- ── 5. Candidate lock (enforcement wired in a later phase) ──────────────────
-- Keyed by the canonical candidates.id (not account_id) so bulk/PS/legacy
-- candidates without a portal account can still be locked. One person = one lock.
CREATE TABLE IF NOT EXISTS candidate_locks (
    candidate_id  UUID PRIMARY KEY REFERENCES candidates(id) ON DELETE CASCADE,
    locked_by     UUID NOT NULL REFERENCES users(id),
    locked_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_candidate_locks_expires ON candidate_locks (expires_at);

-- ── 6. Extend the scope_kind catalog ───────────────────────────────────────
ALTER TABLE rbac_roles DROP CONSTRAINT IF EXISTS rbac_roles_scope_kind_check;
ALTER TABLE rbac_roles ADD CONSTRAINT rbac_roles_scope_kind_check
    CHECK (scope_kind IN ('all', 'subregion', 'store', 'area', 'requisition'));

-- ── 7. New permission: manage areas ────────────────────────────────────────
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('area.admin', 'Manage areas', 'จัดการ area', 'system', 40)
ON CONFLICT (key) DO NOTHING;

-- ── 8. Seed the new roles ALONGSIDE the existing 7 (cutover retires the old) ─
INSERT INTO rbac_roles (key, label_en, label_th, scope_kind, is_builtin) VALUES
    ('hr_store',             'HR (store)',            'HR สาขา',              'store',       TRUE),
    ('area_hr',              'Area HR',               'HR เขต (area)',        'area',        TRUE),
    ('hiring_manager_store', 'Hiring manager (store)','ผู้จัดการสาขา (สรรหา)','requisition', TRUE),
    ('hiring_manager_ho',    'Hiring manager (HO)',   'ผู้จัดการสรรหา (HO)',  'requisition', TRUE),
    ('ta',                   'Talent acquisition',    'ฝ่ายสรรหา (TA)',       'all',         TRUE)
ON CONFLICT (key) DO NOTHING;

-- New roles' permission grants.
-- hr_store + area_hr: full candidate operate within their scope (no approval — the
-- hiring manager approves). ta: same operate, company-wide, plus report export.
-- hiring_manager_* : read-only (scope only, NO operate perms); the approver
-- permission is added during the approval-chain remap (later phase).
INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('hr_store', 'reports.view'),
    ('hr_store', 'bulk.upload'),
    ('hr_store', 'assignment.write'),
    ('hr_store', 'offer.write'),
    ('hr_store', 'onboarding.write'),
    ('hr_store', 'letter.write'),
    ('hr_store', 'scorecard.ta'),

    ('area_hr', 'reports.view'),
    ('area_hr', 'bulk.upload'),
    ('area_hr', 'assignment.write'),
    ('area_hr', 'offer.write'),
    ('area_hr', 'onboarding.write'),
    ('area_hr', 'letter.write'),
    ('area_hr', 'scorecard.ta'),

    ('ta', 'reports.view'),
    ('ta', 'reports.export'),
    ('ta', 'bulk.upload'),
    ('ta', 'assignment.write'),
    ('ta', 'offer.write'),
    ('ta', 'onboarding.write'),
    ('ta', 'letter.write'),
    ('ta', 'scorecard.ta')
ON CONFLICT DO NOTHING;
