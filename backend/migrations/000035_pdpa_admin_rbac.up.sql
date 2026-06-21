-- PDPA Phase 5.4: the pdpa.admin permission gates the DPO/PDPA console (DSAR
-- held-queue actioning, consent lookup, retention/breach/ROPA overview). Like
-- breach.manage it is a sensitive DPO/legal function, seeded for super_admin only
-- (a hard code bypass anyway); operators grant it to a DPO/legal role at runtime
-- via the dynamic-RBAC matrix UI. Mirrors internal/rbac/permissions.go.
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('pdpa.admin', 'PDPA / DPO console', 'คอนโซล PDPA / DPO', 'system', 50)
ON CONFLICT (key) DO NOTHING;

INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('super_admin', 'pdpa.admin')
ON CONFLICT DO NOTHING;
