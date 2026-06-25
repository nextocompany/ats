-- Records every application status transition so the candidate-facing status
-- page can show a curated milestone timeline. A database trigger captures the
-- change at the data layer because applications.status is mutated from many
-- places — repository SetStatus/SetRejection/SetHired AND inline UPDATEs inside
-- the approval and offer-accept transactions — so there is no single app-layer
-- chokepoint to instrument. The trigger is complete by construction and fires
-- within the same transaction as the status write.
--
-- to_status values are the plain status strings (no PII). The row is removed
-- when its application is deleted (ON DELETE CASCADE), so PDPA erasure that
-- deletes an application also drops its history.
CREATE TABLE IF NOT EXISTS application_status_history (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    from_status    VARCHAR(50),
    to_status      VARCHAR(50) NOT NULL,
    changed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ash_application_changed
    ON application_status_history (application_id, changed_at);

CREATE OR REPLACE FUNCTION log_application_status_change()
RETURNS trigger AS $$
BEGIN
    -- The initial 'pending' is an INSERT default, not an UPDATE, so it is never
    -- recorded here; the candidate timeline synthesises "applied" from
    -- applications.created_at instead.
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO application_status_history (application_id, from_status, to_status, changed_at)
        VALUES (NEW.id, OLD.status, NEW.status, NOW());
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_application_status_history
    AFTER UPDATE OF status ON applications
    FOR EACH ROW
    EXECUTE FUNCTION log_application_status_change();
