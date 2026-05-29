-- Demo open vacancies (Sprint 3 replaces this source with PeopleSoft sync).
-- Links seeded stores to seeded positions by lookup. Idempotent by ps_vacancy_id.

INSERT INTO vacancies (ps_vacancy_id, store_id, position_id, headcount, status, opened_at)
SELECT v.ps_id, v.store_no, p.id, v.headcount, 'open', NOW()
FROM (VALUES
  ('V-SEED-CM-CASHIER',   1,  'Cashier',          2),
  ('V-SEED-CR-CASHIER',   2,  'Cashier',          1),
  ('V-SEED-BKK-CASHIER',  10, 'Cashier',          3),
  ('V-SEED-CM-BUTCHERY',  1,  'Butchery Manager', 1),
  ('V-SEED-UDON-FORK',    30, 'Forklift Driver',  2),
  ('V-SEED-CR-GA',        2,  'GA Staff',         1)
) AS v(ps_id, store_no, title_en, headcount)
JOIN positions p ON p.title_en = v.title_en
WHERE NOT EXISTS (SELECT 1 FROM vacancies x WHERE x.ps_vacancy_id = v.ps_id);
