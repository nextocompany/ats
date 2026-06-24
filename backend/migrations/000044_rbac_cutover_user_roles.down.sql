-- Reverse the cutover reassignment. LOSSY by nature: hr_manager and
-- operation_director both mapped to area_hr, so area_hr cannot be disambiguated on
-- the way back — it is left as-is (operator must fix those by hand if needed). The
-- unambiguous mappings are restored. Old roles were never deleted, so they are
-- valid targets here.

UPDATE users
SET role = CASE role
    WHEN 'hr_store'             THEN 'hr_staff'
    WHEN 'hiring_manager_store' THEN 'sgm'
    WHEN 'ta'                   THEN 'regional_director'
    ELSE role            -- area_hr left untouched (ambiguous: hr_manager vs operation_director)
END
WHERE role IN ('hr_store', 'hiring_manager_store', 'ta');

UPDATE approval_steps
SET role = CASE role
    WHEN 'hr_store'             THEN 'hr_staff'
    WHEN 'area_hr'              THEN 'hr_manager'
    WHEN 'hiring_manager_store' THEN 'sgm'
    WHEN 'ta'                   THEN 'regional_director'
    ELSE role
END
WHERE status = 'pending'
  AND role IN ('hr_store', 'area_hr', 'hiring_manager_store', 'ta');
