-- Scorecard perspective: split interview feedback into TA (recruiter) vs Line
-- Manager assessments. Additive; legacy rows default to 'ta'.
ALTER TABLE interview_feedback
    ADD COLUMN IF NOT EXISTS perspective TEXT NOT NULL DEFAULT 'ta';

CREATE INDEX IF NOT EXISTS idx_interview_feedback_perspective
    ON interview_feedback (application_id, perspective);
