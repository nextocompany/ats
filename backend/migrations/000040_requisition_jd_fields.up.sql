-- Requisition detailed JD fields. Adds per-opening job-description and metadata
-- columns to the shared vacancies table (manual requisitions), and a position-level
-- benefits column to positions so benefits can be prefilled and surfaced on the
-- public career portal (Phase 2). All vacancies columns are nullable or defaulted
-- so the PeopleSoft sync Upsert and the seed_vacancies* scripts are unaffected.

ALTER TABLE vacancies
  ADD COLUMN responsibilities TEXT,
  ADD COLUMN qualifications    TEXT,
  ADD COLUMN benefits          TEXT,
  ADD COLUMN other_details     TEXT,
  ADD COLUMN employment_type   VARCHAR(20),
  ADD COLUMN salary_min        INTEGER,
  ADD COLUMN salary_max        INTEGER,
  ADD COLUMN priority          VARCHAR(20) NOT NULL DEFAULT 'normal',
  ADD COLUMN open_reason       VARCHAR(20);

-- Position-level benefits, mirroring positions.responsibilities / positions.qualifications.
-- Populated via the same seed/script path as the other Master JD text (no UI editor).
ALTER TABLE positions
  ADD COLUMN benefits TEXT;
