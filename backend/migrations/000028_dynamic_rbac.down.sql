-- Reverse 000028_dynamic_rbac: drop the RBAC tables in dependency order.
-- Authorization falls back to the compile-time Go allowlists (still present).
DROP TABLE IF EXISTS rbac_role_permissions;
DROP TABLE IF EXISTS rbac_roles;
DROP TABLE IF EXISTS rbac_permissions;
