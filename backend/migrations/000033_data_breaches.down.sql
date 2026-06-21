-- Reverse 000033. Remove the RBAC key first (FK-less, so plain DELETE), then the
-- table (indexes drop with it).
DELETE FROM rbac_role_permissions WHERE permission = 'breach.manage';
DELETE FROM rbac_permissions WHERE key = 'breach.manage';

DROP TABLE IF EXISTS data_breaches;
