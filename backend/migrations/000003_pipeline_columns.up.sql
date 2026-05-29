-- Sprint 1: columns the intake/OCR/parse pipeline writes to (additive only).

ALTER TABLE applications
  ADD COLUMN raw_file_blob_url       TEXT,
  ADD COLUMN raw_file_type           VARCHAR(10),
  ADD COLUMN ocr_text_blob_url       TEXT,
  ADD COLUMN parsed_profile_blob_url TEXT,
  ADD COLUMN ocr_confidence          NUMERIC(4,3),
  ADD COLUMN needs_manual_review     BOOLEAN DEFAULT FALSE,
  ADD COLUMN queue_task_id           VARCHAR(120),
  ADD COLUMN parsed_at               TIMESTAMPTZ;

CREATE INDEX idx_applications_queue_task_id ON applications (queue_task_id);
