-- Candidate identity split: separate the cosmetic provider display name from the
-- two name fields that are actually matched against the parsed resume.
--
--   display_name — pulled from the LINE/Google login profile, OPTIONAL, cosmetic,
--                  NEVER used in the resume name-match (it's usually a nickname/
--                  handle that wouldn't match a real CV — the source of false
--                  name_mismatch flags before this change).
--   name_th      — Thai full name "first last", REQUIRED at apply, matched.
--   name_en      — English full name "first last", REQUIRED at apply, matched.
--
-- The resume name-match accepts the parsed name if it loosely matches name_th OR
-- name_en (a CV is in one language). full_name is kept as the canonical
-- (= name_th) for candidates.full_name / dedup / search / HR display.
--
-- Additive + nullable: app layer enforces required-ness at apply/profile so this
-- never breaks existing rows. Prod was wiped (0 accounts) so no backfill needed;
-- the COALESCE below seeds name_th from any pre-existing full_name defensively.

ALTER TABLE candidate_accounts ADD COLUMN IF NOT EXISTS display_name VARCHAR(255);
ALTER TABLE candidate_accounts ADD COLUMN IF NOT EXISTS name_th      VARCHAR(255);
ALTER TABLE candidate_accounts ADD COLUMN IF NOT EXISTS name_en      VARCHAR(255);

-- Defensive backfill: any legacy account with a full_name keeps it as its Thai
-- name so the match still has something to compare. No-op on a wiped prod.
UPDATE candidate_accounts
   SET name_th = full_name
 WHERE COALESCE(name_th, '') = '' AND COALESCE(full_name, '') <> '';
