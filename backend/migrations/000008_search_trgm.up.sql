-- Sprint 5c: trigram indexes powering the mock (Postgres) candidate search.
-- pg_trgm enables fast fuzzy ILIKE over name/province.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_candidates_fullname_trgm ON candidates USING gin (full_name gin_trgm_ops);
CREATE INDEX idx_candidates_province_trgm ON candidates USING gin (province gin_trgm_ops);
