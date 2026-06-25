DROP TRIGGER IF EXISTS trg_application_status_history ON applications;
DROP FUNCTION IF EXISTS log_application_status_change();
DROP TABLE IF EXISTS application_status_history;
