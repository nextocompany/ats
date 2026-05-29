-- Representative store seed across the 13 subregions (synthetic; swap for the
-- real Storelist.csv via the same table later). Format codes A–F kept simple so
-- position.format_types filtering is clean. Provinces match subregion.go keys.

INSERT INTO stores (store_no, store_name, format_type, subregion, operation_director, regional_ceo, province, latitude, longitude) VALUES
  (1,  'CM Central',        'A', 'Upper North',          'OD North',  'CEO North',  'เชียงใหม่',     18.7960, 98.9790),
  (2,  'Chiang Rai',        'B', 'Upper North',          'OD North',  'CEO North',  'เชียงราย',      19.9100, 99.8400),
  (3,  'Phitsanulok',       'A', 'Lower North',          'OD North',  'CEO North',  'พิษณุโลก',      16.8211, 100.2659),
  (10, 'Bangkok Rama IX',   'A', 'BKK East',             'OD BKK',    'CEO BKK',    'กรุงเทพมหานคร', 13.7563, 100.5018),
  (11, 'Nonthaburi',        'B', 'BKK West 1',           'OD BKK',    'CEO BKK',    'นนทบุรี',       13.8591, 100.5217),
  (12, 'Samut Sakhon',      'D', 'BKK West 2',           'OD BKK',    'CEO BKK',    'สมุทรสาคร',     13.5475, 100.2746),
  (20, 'Ayutthaya',         'A', 'Central North',        'OD Central','CEO Central','พระนครศรีอยุธยา',14.3692,100.5877),
  (21, 'Kanchanaburi',      'C', 'Central West',         'OD Central','CEO Central','กาญจนบุรี',     14.0227, 99.5328),
  (30, 'Udon Thani',        'A', 'Upper Northeast',      'OD NE',     'CEO NE',     'อุดรธานี',      17.4138, 102.7870),
  (31, 'Ubon Ratchathani',  'B', 'Lower East Northeast', 'OD NE',     'CEO NE',     'อุบลราชธานี',   15.2440, 104.8473),
  (32, 'Nakhon Ratchasima', 'A', 'Lower West Northeast', 'OD NE',     'CEO NE',     'นครราชสีมา',    14.9799, 102.0978),
  (40, 'Chonburi',          'A', 'East',                 'OD East',   'CEO East',   'ชลบุรี',        13.3611, 100.9847),
  (50, 'Surat Thani',       'B', 'Upper South',          'OD South',  'CEO South',  'สุราษฎร์ธานี',  9.1382,  99.3215),
  (51, 'Songkhla Hat Yai',  'A', 'Lower South',          'OD South',  'CEO South',  'สงขลา',         7.1897,  100.5951)
ON CONFLICT (store_no) DO NOTHING;
