-- Multi-level hiring approval chain (Module-3 3.5): Staff -> HR Manager -> SGM ->
-- Regional Director. One request per application hire decision; four ordered step
-- rows per request. Additive; existing applications are unaffected.
CREATE TABLE IF NOT EXISTS approval_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | rejected
    current_level   INT  NOT NULL DEFAULT 2,          -- next pending step (L1 done at creation)
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at      TIMESTAMPTZ,
    decision_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_approval_requests_application ON approval_requests (application_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests (status);

CREATE TABLE IF NOT EXISTS approval_steps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id  UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    level       INT  NOT NULL,                      -- 1..4
    role        TEXT NOT NULL,                      -- hr_staff | hr_manager | sgm | regional_director
    status      TEXT NOT NULL DEFAULT 'pending',    -- pending | approved | rejected
    approver_id UUID REFERENCES users(id),
    comment     TEXT,
    due_at      TIMESTAMPTZ,                         -- set only while this step is the active pending one
    escalated   BOOLEAN NOT NULL DEFAULT FALSE,
    decided_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (request_id, level)
);
CREATE INDEX IF NOT EXISTS idx_approval_steps_request ON approval_steps (request_id);
-- SLA sweep query: active pending steps past due, not yet escalated.
CREATE INDEX IF NOT EXISTS idx_approval_steps_sla ON approval_steps (status, due_at) WHERE status = 'pending';
