-- Interview feedback: structured outcome a hiring panel records during the human
-- interview stage (status='interview'). One row per interviewer/round (many per
-- application). Recorded independently of the "mark interviewed" transition; write
-- access is gated in Go (sgm/hr_manager/super_admin). The per-competency ratings
-- live in a JSONB column (mirrors applications.ai_score_breakdown), keeping the
-- column set stable if the competency list changes.

CREATE TABLE IF NOT EXISTS interview_feedback (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    appointment_id  UUID REFERENCES interview_appointments(id) ON DELETE SET NULL,
    interviewer_id  UUID REFERENCES users(id),       -- who recorded it (snapshot via join on read)
    overall_rating  INT NOT NULL,                    -- 1..5
    recommendation  TEXT NOT NULL,                   -- 'pass' | 'hold' | 'fail'
    competencies    JSONB NOT NULL DEFAULT '{}'::jsonb, -- {communication,technical,experience,culture_fit} each 0..5 (0 = not rated)
    strengths       TEXT,
    concerns        TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_interview_feedback_app ON interview_feedback (application_id);
