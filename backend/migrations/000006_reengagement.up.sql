-- Sprint 5a: candidate re-engagement contact log (suppression). One row per
-- (candidate, position) prevents re-contacting the same person for the same role.

CREATE TABLE reengagement_contacts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    candidate_id UUID NOT NULL REFERENCES candidates (id),
    position_id  UUID NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
    channel      VARCHAR(20) CHECK (channel IN ('line', 'email')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (candidate_id, position_id)
);

CREATE INDEX idx_reengagement_contacts_position ON reengagement_contacts (position_id);
