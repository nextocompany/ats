DROP INDEX IF EXISTS idx_applications_queue_task_id;

ALTER TABLE applications
  DROP COLUMN IF EXISTS parsed_at,
  DROP COLUMN IF EXISTS queue_task_id,
  DROP COLUMN IF EXISTS needs_manual_review,
  DROP COLUMN IF EXISTS ocr_confidence,
  DROP COLUMN IF EXISTS parsed_profile_blob_url,
  DROP COLUMN IF EXISTS ocr_text_blob_url,
  DROP COLUMN IF EXISTS raw_file_type,
  DROP COLUMN IF EXISTS raw_file_blob_url;
