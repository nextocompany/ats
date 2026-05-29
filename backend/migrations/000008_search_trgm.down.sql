DROP INDEX IF EXISTS idx_candidates_province_trgm;
DROP INDEX IF EXISTS idx_candidates_fullname_trgm;
-- pg_trgm left installed; it is harmless and may be used elsewhere.
