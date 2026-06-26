-- Executive ROI cost assumptions. The ATS stores no cost/finance data (no HRIS
-- integration), so the ROI & cost-per-hire dashboard is driven by a single set of
-- admin-configured assumptions. This is a single-row table: the boolean primary
-- key with a CHECK (id) guarantees exactly one row can ever exist, so reads never
-- need an ORDER BY / LIMIT and writes are a plain upsert on id = TRUE.
--
-- All cost figures are NULLable: an unset assumption means the ROI cards render a
-- "set assumptions" empty-state rather than a fabricated number (the page never
-- divides by a fabricated cost). updated_by records the editor's email (mirrors
-- system_settings — no FK, since SSO editors may have an Entra OID with no
-- resolvable users row).
CREATE TABLE IF NOT EXISTS executive_cost_config (
    id                           BOOLEAN     PRIMARY KEY DEFAULT TRUE CHECK (id),
    currency                     VARCHAR(8)  NOT NULL DEFAULT 'THB',
    system_cost_monthly          NUMERIC(14,2),
    traditional_cost_per_hire    NUMERIC(14,2),
    vacancy_cost_per_day         NUMERIC(14,2),
    traditional_time_to_hire_days NUMERIC(7,2),
    updated_by                   VARCHAR(255) NOT NULL DEFAULT '',
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the single row with NULL cost figures (unset → ROI shows the empty-state).
INSERT INTO executive_cost_config (id) VALUES (TRUE)
ON CONFLICT (id) DO NOTHING;
