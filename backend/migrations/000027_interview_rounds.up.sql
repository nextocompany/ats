-- Multi-round human interviews: number each appointment per application. Existing
-- rows (the schema already allowed multiple, the read query just hid them behind
-- LIMIT 1) are backfilled by created_at order so the unique index holds.
ALTER TABLE interview_appointments ADD COLUMN IF NOT EXISTS round_no INT NOT NULL DEFAULT 1;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY application_id ORDER BY created_at, id) AS rn
    FROM interview_appointments
)
UPDATE interview_appointments ia
SET round_no = ranked.rn
FROM ranked
WHERE ia.id = ranked.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_interview_appointments_app_round
    ON interview_appointments (application_id, round_no);
