-- Reverse 000043: drop the new roles' approval-chain permission grants.
DELETE FROM rbac_role_permissions WHERE role_key IN
    ('hr_store', 'area_hr', 'hiring_manager_store', 'hiring_manager_ho', 'ta')
  AND permission IN
    ('approval.submit', 'approval.decide.l1', 'approval.decide.l2', 'approval.decide.l3', 'approval.decide.l4');
