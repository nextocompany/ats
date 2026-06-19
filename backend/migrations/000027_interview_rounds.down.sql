DROP INDEX IF EXISTS uq_interview_appointments_app_round;
ALTER TABLE interview_appointments DROP COLUMN IF EXISTS round_no;
