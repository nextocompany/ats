DROP INDEX IF EXISTS idx_applications_public_token;
ALTER TABLE applications DROP COLUMN IF EXISTS public_token;

DROP INDEX IF EXISTS idx_positions_ps_code;
ALTER TABLE positions DROP COLUMN IF EXISTS ps_position_code;
