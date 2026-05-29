-- Representative Master JD seed. must_have_criteria uses the object form the
-- scorer expects: {"min_education_level":N,"min_experience_months":M}.
-- Idempotent by title_en.

INSERT INTO positions (title_th, title_en, level, department, must_have_criteria, keywords, format_types, is_active)
SELECT v.title_th, v.title_en, v.level, v.department, v.must_have::jsonb, v.keywords, v.format_types, TRUE
FROM (VALUES
  ('แคชเชียร์',            'Cashier',            'Staff',   'Front',  '{"min_education_level":1,"min_experience_months":6}',  ARRAY['cashier','POS','customer service'], ARRAY[]::varchar[]),
  ('พนักงานขับโฟล์คลิฟท์', 'Forklift Driver',    'Staff',   'GR',     '{"min_education_level":1,"min_experience_months":0}',  ARRAY['forklift','warehouse'],             ARRAY[]::varchar[]),
  ('ผู้จัดการแผนกเนื้อ',   'Butchery Manager',   'Manager', 'Fresh',  '{"min_education_level":3,"min_experience_months":24}', ARRAY['butchery','meat','fresh'],          ARRAY['A','B']::varchar[]),
  ('ผู้จัดการฝ่ายบุคคล',   'HR Manager',         'Manager', 'HR',     '{"min_education_level":3,"min_experience_months":36}', ARRAY['recruitment','payroll','HR'],       ARRAY[]::varchar[]),
  ('ช่างซ่อมบำรุงอาคาร',   'GA Staff',           'Staff',   'GA',     '{"min_education_level":1,"min_experience_months":0}',  ARRAY['ช่างแอร์','ซ่อมบำรุง','maintenance'], ARRAY[]::varchar[])
) AS v(title_th, title_en, level, department, must_have, keywords, format_types)
WHERE NOT EXISTS (SELECT 1 FROM positions p WHERE p.title_en = v.title_en);
