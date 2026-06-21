-- PDPA erasure support (Phase 1): index the one foreign key that the unified
-- EraseSubject engine joins on but that had no index. Every other table the
-- erasure cascade touches is already indexed on its lookup column:
--   onboarding_documents(application_id), letters(application_id),
--   interview_feedback(application_id), interview_sessions(application_id UNIQUE),
--   offers(application_id UNIQUE), application_fit_analyses(application_id PK),
--   member_notes(account_id), member_tags(account_id PK), candidate_sessions(account_id).
-- notifications.candidate_id is a foreign key with no supporting index, so the
-- per-subject erasure UPDATE (and any future per-candidate notification query)
-- would otherwise scan the whole table.
CREATE INDEX IF NOT EXISTS idx_notifications_candidate ON notifications (candidate_id);

-- reengagement_logs only has a PARTIAL unique index on (candidate_id, trigger_type)
-- WHERE trigger_type IS NOT NULL, which the erasure DELETE WHERE candidate_id = $1
-- cannot rely on (it must also reach NULL-trigger rows). A plain candidate_id index
-- keeps the per-subject delete from scanning this growing log table.
CREATE INDEX IF NOT EXISTS idx_reengagement_logs_candidate ON reengagement_logs (candidate_id);
