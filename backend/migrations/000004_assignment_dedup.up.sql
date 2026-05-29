-- Sprint 2: dedup + branch-assignment state on applications (additive).
-- Score columns (ai_score, ai_score_breakdown, ...) and assigned_store_id
-- already exist from 000001.

ALTER TABLE applications
  ADD COLUMN talent_pool      BOOLEAN DEFAULT FALSE,
  ADD COLUMN dedup_state      VARCHAR(20),
  ADD COLUMN dedup_confidence NUMERIC(4,3);

CREATE INDEX idx_candidates_full_name ON candidates (full_name);
CREATE INDEX idx_applications_talent_pool ON applications (talent_pool);
