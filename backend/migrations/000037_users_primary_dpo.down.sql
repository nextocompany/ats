-- Revert 000037: drop the primary DPO designation column.
ALTER TABLE users DROP COLUMN IF EXISTS is_primary_dpo;
