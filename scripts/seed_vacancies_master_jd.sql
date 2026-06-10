-- Open recruitment vacancies for 10 Master JD positions, spread across stores
-- and subregions so the career portal lists them and branch assignment (F04) has
-- geographic variety. Links by stable ps_position_code → existing stores by
-- store_no. Idempotent on ps_vacancy_id (re-run safe; re-opens if closed).
INSERT INTO vacancies (ps_vacancy_id, store_id, position_id, headcount, status, opened_at)
SELECT v.ps_id, v.store_no, p.id, v.headcount, 'open', NOW()
FROM (VALUES
  -- Cashier — high-volume, several stores
  ('V-MJD-10-CASHIER',          10, 'CASHIER',                3),
  ('V-MJD-11-CASHIER',          11, 'CASHIER',                2),
  ('V-MJD-01-CASHIER',           1, 'CASHIER',                2),
  ('V-MJD-10-CASHIER_HELPER',   10, 'CASHIER_HELPER',         2),
  ('V-MJD-40-CASHIER_HELPER',   40, 'CASHIER_HELPER',         1),
  -- Fresh food floor staff
  ('V-MJD-01-BUTCHERY_STAFF',    1, 'BUTCHERY_STAFF',         1),
  ('V-MJD-30-BUTCHERY_STAFF',   30, 'BUTCHERY_STAFF',         1),
  ('V-MJD-51-FISH_SEAFOOD',     51, 'FISH_SEAFOOD_STAFF',     1),
  ('V-MJD-50-FISH_SEAFOOD',     50, 'FISH_SEAFOOD_STAFF',     1),
  ('V-MJD-10-BAKERY_STAFF',     10, 'BAKERY_STAFF',           1),
  ('V-MJD-11-BAKERY_STAFF',     11, 'BAKERY_STAFF',           1),
  ('V-MJD-12-FVS_STAFF',        12, 'FRUIT_VEG_SPICE_STAFF',  2),
  -- Goods-receiving / logistics
  ('V-MJD-20-GR_OPERATIONS',    20, 'GR_OPERATIONS',          2),
  ('V-MJD-32-GR_OPERATIONS',    32, 'GR_OPERATIONS',          1),
  ('V-MJD-30-FORKLIFT',         30, 'FORKLIFT_DRIVER',        1),
  ('V-MJD-32-FORKLIFT',         32, 'FORKLIFT_DRIVER',        1),
  -- Supervisor + Manager
  ('V-MJD-10-BAKERY_SUP',       10, 'BAKERY_SUPERVISOR',      1),
  ('V-MJD-40-FRESH_MANAGER',    40, 'FRESH_MANAGER',          1)
) AS v(ps_id, store_no, code, headcount)
JOIN positions p ON p.ps_position_code = v.code AND p.is_active = TRUE
ON CONFLICT (ps_vacancy_id) DO UPDATE SET
  store_id    = EXCLUDED.store_id,
  position_id = EXCLUDED.position_id,
  headcount   = EXCLUDED.headcount,
  status      = 'open',
  opened_at   = COALESCE(vacancies.opened_at, NOW());
