-- Tracks the at-most-once "interview is tomorrow" reminder per appointment, so a
-- periodic sweep never reminds the same booking twice. NULL = not yet reminded.
-- Mirrors the approval_steps.escalated suppression flag.
ALTER TABLE interview_appointments ADD COLUMN IF NOT EXISTS reminder_sent_at TIMESTAMPTZ;

-- Partial index for the reminder sweep: it only ever scans rows still awaiting a
-- reminder, ordered/filtered by scheduled_at.
CREATE INDEX IF NOT EXISTS idx_interview_appt_reminder_due
    ON interview_appointments (scheduled_at)
    WHERE reminder_sent_at IS NULL;
