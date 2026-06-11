-- Demo candidates + applications for a client walkthrough. Idempotent (guards by
-- id_card / (candidate,position)). Mirrors what the AI pipeline would have produced
-- so the HR ranked inbox, candidate detail, and analytics all look alive without a
-- live AI provider. Safe to re-run; remove with: DELETE FROM applications/candidates
-- WHERE id_card LIKE '99%'. (Demo id_cards are the reserved 99-prefix block.)

-- 1) Candidates ------------------------------------------------------------------
INSERT INTO candidates (full_name, phone, email, id_card, province, subregion, status, pdpa_consent, pdpa_consent_at, pdpa_version, source_channel, created_at)
SELECT v.full_name, v.phone, v.email, v.id_card, v.province, v.subregion, v.status,
       TRUE, NOW(), '1.0', v.source_channel, NOW() - (v.age_days || ' days')::interval
FROM (VALUES
  ('ณัฐพล ศรีสุข',        '0810000001', 'natthapon@example.com',  '9900000000001', 'กรุงเทพมหานคร', 'BKK East',        'available', 'career_portal', 2),
  ('สุดารัตน์ ใจงาม',     '0810000002', 'sudarat@example.com',    '9900000000002', 'เชียงใหม่',     'Upper North',     'available', 'career_portal', 3),
  ('ก้องภพ วงศ์ทอง',      '0810000003', 'kongphop@example.com',   '9900000000003', 'เชียงใหม่',     'Upper North',     'available', 'line',          4),
  ('ปิยะดา แสงทอง',       '0810000004', 'piyada@example.com',     '9900000000004', 'กรุงเทพมหานคร', 'BKK East',        'available', 'referral',      5),
  ('อนุชา พรหมมา',        '0810000005', 'anucha@example.com',     '9900000000005', 'เชียงราย',      'Upper North',     'hired',     'career_portal', 9),
  ('วีรพล มั่นคง',        '0810000006', 'weeraphon@example.com',  '9900000000006', 'อุดรธานี',      'Upper Northeast', 'hired',     'walk_in',       11),
  ('จิราพร ทองดี',        '0810000007', 'jiraporn@example.com',   '9900000000007', 'เชียงราย',      'Upper North',     'available', 'career_portal', 3),
  ('ธีรเดช คำมูล',        '0810000008', 'theeradet@example.com',  '9900000000008', 'นนทบุรี',       'BKK West 1',      'available', 'line',          6),
  ('มนทกานต์ บุญมี',      '0810000009', 'monthakan@example.com',  '9900000000009', 'อุดรธานี',      'Upper Northeast', 'available', 'referral',      4),
  ('ศิริพร แก้วมณี',      '0810000010', 'siriporn@example.com',   '9900000000010', 'สงขลา',         'Lower South',     'available', 'career_portal', 7),
  ('ชัยวัฒน์ เพชรน้อย',   '0810000011', 'chaiwat@example.com',    '9900000000011', 'กรุงเทพมหานคร', 'BKK East',        'available', 'career_portal', 2),
  ('กฤษณะ ชาญชัย',        '0810000012', 'kritsana@example.com',   '9900000000012', 'เชียงใหม่',     'Upper North',     'available', 'walk_in',       8),
  ('อรนุช สายบัว',        '0810000013', 'oranuch@example.com',    '9900000000013', 'กรุงเทพมหานคร', 'BKK East',        'available', 'line',          5),
  ('ภานุวัฒน์ ดวงดี',     '0810000014', 'phanuwat@example.com',   '9900000000014', 'สุราษฎร์ธานี',  'Upper South',     'available', 'career_portal', 6)
) AS v(full_name, phone, email, id_card, province, subregion, status, source_channel, age_days)
WHERE NOT EXISTS (SELECT 1 FROM candidates c WHERE c.id_card = v.id_card);

-- 2) Applications (one per candidate; linked by id_card + position title_en) ------
INSERT INTO applications (
  candidate_id, position_id, store_id, assigned_store_id,
  ai_score, ai_score_breakdown, ai_summary, ai_red_flags,
  must_have_passed, status, talent_pool, ocr_confidence, needs_manual_review,
  parsed_at, hired_at, created_at
)
SELECT c.id, p.id, d.store_no, d.store_no,
       d.ai_score, d.breakdown::jsonb, d.summary, d.red_flags,
       d.passed, d.status, d.talent_pool, d.ocr, d.manual_review,
       c.created_at, CASE WHEN d.status = 'hired' THEN c.created_at + INTERVAL '2 days' ELSE NULL END, c.created_at
FROM (VALUES
  -- id_card, title_en, store_no, ai_score, breakdown, summary, red_flags, passed, status, talent_pool, ocr, manual_review
  ('9900000000001', 'Cashier',          10, 92.5, '{"experience":28,"skills":19,"education":9,"language":9,"location":18}', 'ประสบการณ์แคชเชียร์ 3 ปี ใช้ระบบ POS คล่อง บริการลูกค้าดีเยี่ยม', NULL, TRUE, 'scored',      FALSE, 0.96, FALSE),
  ('9900000000002', 'Cashier',          1,  89.0, '{"experience":26,"skills":18,"education":9,"language":8,"location":20}', 'มีประสบการณ์ค้าปลีก 2 ปี ใกล้สาขา ทำงานเป็นกะได้', NULL, TRUE, 'scored',      FALSE, 0.94, FALSE),
  ('9900000000003', 'Butchery Manager', 1,  87.5, '{"experience":29,"skills":18,"education":9,"language":7,"location":18}', 'หัวหน้าแผนกเนื้อ 5 ปี บริหารทีม 8 คน ควบคุมต้นทุนได้ดี', NULL, TRUE, 'shortlisted', FALSE, 0.93, FALSE),
  ('9900000000004', 'HR Manager',       10, 85.0, '{"experience":27,"skills":17,"education":10,"language":9,"location":17}','ประสบการณ์สรรหา+payroll 6 ปี สื่อสารดี วุฒิ ป.ตรี HR', NULL, TRUE, 'interview',   FALSE, 0.95, FALSE),
  ('9900000000005', 'Cashier',          2,  91.0, '{"experience":27,"skills":19,"education":9,"language":9,"location":20}', 'แคชเชียร์มืออาชีพ ผ่านสัมภาษณ์ จ้างแล้ว เริ่มงานสาขาเชียงราย', NULL, TRUE, 'hired',       FALSE, 0.97, FALSE),
  ('9900000000006', 'Forklift Driver',  30, 84.0, '{"experience":29,"skills":16,"education":7,"language":6,"location":20}', 'มีใบขับขี่โฟล์คลิฟท์ ประสบการณ์คลังสินค้า 4 ปี จ้างแล้ว', NULL, TRUE, 'hired',       FALSE, 0.90, FALSE),
  ('9900000000007', 'GA Staff',         2,  78.0, '{"experience":24,"skills":15,"education":7,"language":7,"location":18}', 'ช่างซ่อมบำรุงทั่วไป งานแอร์+ไฟฟ้า ประสบการณ์ 3 ปี', NULL, TRUE, 'scored',      FALSE, 0.88, FALSE),
  ('9900000000008', 'Cashier',          11, 72.5, '{"experience":20,"skills":16,"education":8,"language":8,"location":15}', 'ประสบการณ์ค้าปลีก 1 ปี เรียนรู้เร็ว แต่ระยะทางไกลสาขา', NULL, TRUE, 'scored',      FALSE, 0.55, TRUE),
  ('9900000000009', 'Forklift Driver',  30, 81.0, '{"experience":27,"skills":16,"education":7,"language":6,"location":18}', 'ขับโฟล์คลิฟท์ 3 ปี ปลอดภัยเป็นเลิศ อยู่ใกล้สาขาอุดร', NULL, TRUE, 'shortlisted', FALSE, 0.91, FALSE),
  ('9900000000010', 'GA Staff',         51, 66.0, '{"experience":18,"skills":13,"education":7,"language":6,"location":17}', 'ประสบการณ์ซ่อมบำรุงพื้นฐาน ต้องฝึกเพิ่มด้านระบบไฟฟ้า', NULL, TRUE, 'scored',      FALSE, 0.86, FALSE),
  ('9900000000011', 'Cashier',          10, 41.0, '{"experience":8,"skills":9,"education":6,"language":7,"location":11}',   'นักศึกษาจบใหม่ ยังไม่มีประสบการณ์ POS', 'ไม่ผ่านเกณฑ์ประสบการณ์ขั้นต่ำ 6 เดือน', FALSE, 'rejected', TRUE,  0.89, FALSE),
  ('9900000000012', 'Butchery Manager', 1,  38.5, '{"experience":10,"skills":8,"education":5,"language":6,"location":9}',   'ประสบการณ์ไม่ตรงสายงานบริหารแผนกเนื้อ', 'ขาดประสบการณ์บริหาร 24 เดือน + วุฒิไม่ถึงเกณฑ์', FALSE, 'rejected', FALSE, 0.84, FALSE),
  ('9900000000013', 'HR Manager',       10, 45.0, '{"experience":12,"skills":10,"education":8,"language":8,"location":7}',  'พื้นฐาน HR ดี แต่ประสบการณ์บริหารยังน้อย', 'ขาดประสบการณ์บริหาร 36 เดือนตามเกณฑ์', FALSE, 'rejected', TRUE,  0.92, FALSE),
  ('9900000000014', 'Cashier',          50, 69.0, '{"experience":19,"skills":15,"education":8,"language":8,"location":14}', 'ประสบการณ์บริการลูกค้า ร้านสะดวกซื้อ 1.5 ปี', NULL, TRUE, 'scored',      FALSE, 0.87, FALSE)
) AS d(id_card, title_en, store_no, ai_score, breakdown, summary, red_flags, passed, status, talent_pool, ocr, manual_review)
JOIN candidates c ON c.id_card = d.id_card
-- One position per title_en. Prod has duplicate title_en rows (70 positions / 65
-- distinct titles); a plain JOIN would emit two applications per duplicated title.
JOIN (SELECT DISTINCT ON (title_en) id, title_en FROM positions ORDER BY title_en, id) p
  ON p.title_en = d.title_en
WHERE NOT EXISTS (
  SELECT 1 FROM applications a WHERE a.candidate_id = c.id AND a.position_id = p.id
);
