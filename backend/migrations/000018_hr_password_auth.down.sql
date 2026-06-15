DROP TABLE IF EXISTS hr_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS password_updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
