-- Per-position screening-score weights. Optional JSONB on the positions catalog;
-- NULL means "use the default weights" (the scorer falls back). Shape:
--   {"experience":34,"skills":22,"education":11,"language":11,"location":22}  (sum 100)
-- Mirrors the existing must_have_criteria JSONB pattern. The importref CSV upsert
-- does not list this column, so re-imports preserve configured weights.
ALTER TABLE positions
  ADD COLUMN score_weights JSONB;
