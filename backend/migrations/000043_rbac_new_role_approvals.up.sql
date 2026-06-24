-- RBAC redesign P3 (additive): grant the new roles their approval-chain
-- permissions so the hiring-approval flow works once users sit on the new roles.
--
-- Approval authorization is by PERMISSION (approval.decide.lN), not by the chain
-- step's role label — so granting these makes the new roles valid approvers without
-- touching the hardcoded chain in internal/applications/approval.go. The chain's
-- role label still drives SLA-reminder targeting; remapping those labels (and the
-- hr_directory notify/line-manager role lists) to the new roles, plus the user
-- reassignment + old-role retirement, is the reviewed CUTOVER step (see
-- .claude/PRPs/plans/rbac-role-redesign-analysis.plan.md §8) — intentionally NOT
-- bundled here because it is code-coupled and touches production identities.
--
-- Mapping mirrors the old chain: hr_store≈hr_staff (submit+L1), area_hr≈hr_manager
-- (L2), hiring_manager_*≈sgm (L3, the manager decision), ta≈regional_director (L4).
-- hiring_manager_* receive ONLY a decide permission (read-only on operations,
-- approver on the hire decision) — consistent with the 2-axis model.

INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('hr_store', 'approval.submit'),
    ('hr_store', 'approval.decide.l1'),

    ('area_hr', 'approval.decide.l2'),

    ('hiring_manager_store', 'approval.decide.l3'),
    ('hiring_manager_ho',    'approval.decide.l3'),

    ('ta', 'approval.decide.l4')
ON CONFLICT DO NOTHING;
