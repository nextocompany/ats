-- Requisition management: let HR/leadership open position openings manually
-- through the dashboard, in addition to PeopleSoft-synced vacancies. Built on the
-- existing `vacancies` table so an approved manual requisition is immediately
-- live in branch assignment, the executive overview, the careers portal, and
-- reports (all of which read `vacancies WHERE status = 'open'`).
--
-- Lifecycle (status, app-enforced; the column stays free-form VARCHAR(50)):
--   manual create → 'pending_approval' → (approve) → 'open' → (close) → 'closed'
--   plus 'cancelled'. PeopleSoft rows keep 'open'/'filled'/'cancelled' as before.
-- A 'pending_approval' row is invisible to every consumer (they match 'open'),
-- so the approval step is an implicit, free gate.

-- created_by / approved_by are audit pointers, NOT foreign keys: the actor may be
-- an Entra SSO user (DevUser.ID = the token OID), who has no row in `users` (that
-- table only holds local password-login accounts). A FK would 23503-fail for them.
ALTER TABLE vacancies
  ADD COLUMN source      VARCHAR(20) NOT NULL DEFAULT 'peoplesoft', -- 'peoplesoft' | 'manual'
  ADD COLUMN created_by  UUID,
  ADD COLUMN approved_by UUID,
  ADD COLUMN approved_at TIMESTAMPTZ,
  ADD COLUMN created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN updated_at  TIMESTAMPTZ NOT NULL DEFAULT now();

-- Supports the requisitions list (filter by source/status) without disturbing the
-- existing idx_vacancies_lookup used by the branch assigner.
CREATE INDEX idx_vacancies_source_status ON vacancies (source, status);

-- ── RBAC: two new permission keys (mirror internal/rbac/permissions.go) ───────
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('requisition.manage',  'Manage requisitions',  'จัดการการเปิดรับ',   'hiring', 60),
    ('requisition.approve', 'Approve requisitions', 'อนุมัติการเปิดรับ',  'hiring', 61)
ON CONFLICT (key) DO NOTHING;

-- Store-level roles open requisitions; leadership approves them. super_admin is a
-- hard code bypass but is granted explicitly for matrix completeness.
INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('regional_director',  'requisition.manage'),
    ('regional_director',  'requisition.approve'),
    ('operation_director', 'requisition.manage'),
    ('operation_director', 'requisition.approve'),
    ('sgm',                'requisition.manage'),
    ('hr_manager',         'requisition.manage')
ON CONFLICT DO NOTHING;

INSERT INTO rbac_role_permissions (role_key, permission)
    SELECT 'super_admin', key FROM rbac_permissions WHERE key LIKE 'requisition.%'
ON CONFLICT DO NOTHING;
