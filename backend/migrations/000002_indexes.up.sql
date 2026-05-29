-- Indexes for foreign keys and common HR Dashboard filters.
-- Note: id_card is already UNIQUE (auto-indexed), so it is not repeated here.

CREATE INDEX idx_applications_status ON applications (status);
CREATE INDEX idx_applications_ai_score ON applications (ai_score DESC);
CREATE INDEX idx_applications_candidate_id ON applications (candidate_id);
CREATE INDEX idx_applications_assigned_store_id ON applications (assigned_store_id);

CREATE INDEX idx_candidates_subregion ON candidates (subregion);
CREATE INDEX idx_candidates_status ON candidates (status);
CREATE INDEX idx_candidates_phone ON candidates (phone);
CREATE INDEX idx_candidates_email ON candidates (email);

CREATE INDEX idx_vacancies_lookup ON vacancies (store_id, position_id, status);

CREATE INDEX idx_activity_logs_entity ON activity_logs (entity_type, entity_id);
