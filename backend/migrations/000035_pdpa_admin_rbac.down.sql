-- Reverse 000035. Remove the RBAC key (FK-less, so plain DELETE).
DELETE FROM rbac_role_permissions WHERE permission = 'pdpa.admin';
DELETE FROM rbac_permissions WHERE key = 'pdpa.admin';
