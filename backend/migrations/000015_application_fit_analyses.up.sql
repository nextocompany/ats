-- AI cross-position fit analysis.
-- One current analysis per application (HR-triggered, re-generatable → upsert on
-- the application_id PK). Combines the CV-screening result and the AI pre-interview
-- evaluation, then recommends which Master JD position(s) the candidate fits — or
-- states plainly that none fit. Kept off the applications table because the result
-- is a nested structure (recommended[] with reasons[]); a dedicated JSONB-backed
-- table mirrors interview_sessions and records who/which-model produced it.
CREATE TABLE IF NOT EXISTS application_fit_analyses (
    application_id  UUID PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    overall_fit     VARCHAR(16) NOT NULL DEFAULT 'weak', -- strong | moderate | weak | none
    summary         TEXT NOT NULL DEFAULT '',            -- 2-3 Thai sentences
    strengths       JSONB NOT NULL DEFAULT '[]'::jsonb,  -- []string (Thai)
    concerns        JSONB NOT NULL DEFAULT '[]'::jsonb,  -- []string (Thai)
    recommended     JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [{position_id,title,fit_score,reasons[]}]
    no_match_reason TEXT NOT NULL DEFAULT '',            -- set when overall_fit = 'none'
    model           VARCHAR(64) NOT NULL DEFAULT '',     -- provider/deployment for audit
    generated_by    UUID,                                -- HR user id (no FK: mock/dev users need not exist in users)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
