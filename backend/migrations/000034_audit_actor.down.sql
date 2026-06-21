-- Reverse 000034. Drop the action index. The user_id FK is intentionally NOT
-- re-added: by the time this runs the column may already hold Entra OIDs and
-- candidate-account ids that have no row in users(id), so re-adding the FK would
-- fail. The column simply reverts to an unconstrained UUID audit pointer.
DROP INDEX IF EXISTS idx_activity_logs_action;
