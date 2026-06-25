-- Reverse 000045. full_name is unchanged (it was kept in sync with name_th), so
-- dropping these columns loses only the EN name + the cosmetic display name.
ALTER TABLE candidate_accounts DROP COLUMN IF EXISTS name_en;
ALTER TABLE candidate_accounts DROP COLUMN IF EXISTS name_th;
ALTER TABLE candidate_accounts DROP COLUMN IF EXISTS display_name;
