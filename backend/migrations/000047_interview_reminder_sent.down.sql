DROP INDEX IF EXISTS idx_interview_appt_reminder_due;
ALTER TABLE interview_appointments DROP COLUMN IF EXISTS reminder_sent_at;
