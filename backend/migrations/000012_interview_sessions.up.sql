-- Slice 2.5 — AI Pre-Interview.
-- One AI screening-interview session per application. HR invites a candidate;
-- the candidate completes an adaptive text chat via an opaque access token; the
-- AI writes a structured evaluation HR reviews before deciding. The interview
-- lifecycle lives here (not on applications.status) so the existing funnel and
-- allowed status transitions are untouched.
CREATE TABLE IF NOT EXISTS interview_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
    access_token    TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'invited', -- invited | in_progress | completed | expired
    conversation    JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{role,content,ts}]
    turn_count      INT  NOT NULL DEFAULT 0,
    version         INT  NOT NULL DEFAULT 0, -- optimistic-lock counter (no DB lock held across the LLM call)
    interview_score NUMERIC(5,2),
    recommendation  TEXT,                            -- strong_recommend | recommend | neutral | caution
    strengths       JSONB,
    concerns        JSONB,
    summary         TEXT,
    invited_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interview_sessions_app ON interview_sessions (application_id);
