-- Time-based re-engagement (6/12-month dormant nudge): suppression so a candidate
-- receives at most one nudge per trigger type. Reuses the legacy reengagement_logs
-- table (candidate_id, trigger_type, sent_at) which was previously unused. The
-- partial unique index lets RecordTimeContact upsert at-most-once per
-- (candidate, trigger_type), e.g. 'time_6mo' / 'time_12mo'.
CREATE UNIQUE INDEX IF NOT EXISTS uq_reengagement_logs_candidate_trigger
    ON reengagement_logs (candidate_id, trigger_type)
    WHERE trigger_type IS NOT NULL;
