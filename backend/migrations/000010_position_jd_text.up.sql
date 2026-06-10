-- Master JD text for richer CV↔JD scoring. `responsibilities` already exists
-- (000001); add `qualifications` so the scorer can compare a candidate against
-- the full job description (responsibilities + qualifications), not just keywords.
ALTER TABLE positions ADD COLUMN IF NOT EXISTS qualifications TEXT;
