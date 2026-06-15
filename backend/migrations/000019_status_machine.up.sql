-- Candidate status state machine: a mandatory rejection reason (internal — never
-- sent to the candidate) and human-interview appointments (date/time + onsite/online
-- + Teams join link). Status transitions themselves are enforced in Go
-- (internal/applications/transitions.go), keeping the column a plain VARCHAR as the
-- rest of the pipeline does.

ALTER TABLE applications ADD COLUMN IF NOT EXISTS rejection_reason TEXT;

CREATE TABLE IF NOT EXISTS interview_appointments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id    UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    scheduled_at      TIMESTAMPTZ NOT NULL,
    duration_min      INT NOT NULL DEFAULT 60,
    mode              TEXT NOT NULL,            -- 'onsite' | 'online'
    location_text     TEXT,                     -- onsite address / room, or online note
    online_join_url   TEXT,                     -- Teams join link (online only)
    calendar_event_id TEXT,                     -- Graph event id (for a future cancel/reschedule)
    created_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_interview_appointments_app ON interview_appointments (application_id);
