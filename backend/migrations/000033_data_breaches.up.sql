-- PDPA Phase 5.3: personal-data breach register + 72h notification tracking.
--
-- Thai PDPA s.37(4) obliges the data controller to notify the PDPC of a personal
-- data breach within 72 hours of becoming aware of it (and, where the breach is
-- likely to result in a high risk to the rights and freedoms of data subjects,
-- to notify the affected subjects without delay). There is no public PDPC API,
-- so submission stays manual: this register tracks the obligation, drives the
-- 72h countdown (computed in Go from discovered_at), and generates the
-- notification content. Mirrors the requisitions CRUD package.
--
-- created_by / resolved_by are audit pointers, NOT foreign keys: the actor may be
-- an Entra SSO user (DevUser.ID = the token OID), who has no row in `users`. A FK
-- would 23503-fail for them (same rule as vacancies.created_by in 000029).
CREATE TABLE IF NOT EXISTS data_breaches (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title                    TEXT NOT NULL,
    description              TEXT NOT NULL,
    severity                 TEXT NOT NULL DEFAULT 'medium'
                               CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status                   TEXT NOT NULL DEFAULT 'open'
                               CHECK (status IN ('open', 'contained', 'resolved')),
    affected_subjects        INTEGER NOT NULL DEFAULT 0 CHECK (affected_subjects >= 0),
    data_categories          TEXT NOT NULL DEFAULT '',  -- free-text list of PII categories involved
    -- 72h clock starts at discovered_at (s.37(4) "becoming aware"); occurred_at is
    -- when the breach actually happened (may be unknown / earlier).
    discovered_at            TIMESTAMPTZ NOT NULL,
    occurred_at              TIMESTAMPTZ,
    -- High-risk breaches additionally require notifying the affected subjects.
    high_risk                BOOLEAN NOT NULL DEFAULT FALSE,
    -- Obligation timestamps: NULL until the controller actually notifies.
    pdpc_notified_at         TIMESTAMPTZ,
    subjects_notified_at     TIMESTAMPTZ,
    remediation              TEXT NOT NULL DEFAULT '',
    created_by               UUID,  -- audit pointer, no FK (see header)
    resolved_by              UUID,  -- audit pointer, no FK
    resolved_at              TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Powers the register list (newest first, filter by status/severity) and the
-- "PDPC notification overdue" surfacing (open breaches not yet notified).
CREATE INDEX IF NOT EXISTS idx_data_breaches_status ON data_breaches (status, severity);
CREATE INDEX IF NOT EXISTS idx_data_breaches_discovered ON data_breaches (discovered_at DESC);

-- ── RBAC: one new permission key (mirror internal/rbac/permissions.go) ────────
-- breach.manage gates the entire register. It is a sensitive DPO/legal function,
-- so it is seeded for super_admin only (a hard code bypass anyway); operators can
-- grant it to other roles at runtime via the dynamic-RBAC matrix UI.
INSERT INTO rbac_permissions (key, label_en, label_th, category, sort) VALUES
    ('breach.manage', 'Manage breach register', 'จัดการทะเบียนเหตุละเมิดข้อมูล', 'system', 40)
ON CONFLICT (key) DO NOTHING;

INSERT INTO rbac_role_permissions (role_key, permission) VALUES
    ('super_admin', 'breach.manage')
ON CONFLICT DO NOTHING;
