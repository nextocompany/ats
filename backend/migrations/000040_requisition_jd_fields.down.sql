-- Reverse 000040.

ALTER TABLE positions
  DROP COLUMN IF EXISTS benefits;

ALTER TABLE vacancies
  DROP COLUMN IF EXISTS open_reason,
  DROP COLUMN IF EXISTS priority,
  DROP COLUMN IF EXISTS salary_max,
  DROP COLUMN IF EXISTS salary_min,
  DROP COLUMN IF EXISTS employment_type,
  DROP COLUMN IF EXISTS other_details,
  DROP COLUMN IF EXISTS benefits,
  DROP COLUMN IF EXISTS qualifications,
  DROP COLUMN IF EXISTS responsibilities;
