-- RBAC redesign CUTOVER (data migration) — reassign existing users from the legacy
-- 7-role model to the new 2-axis roles. REVIEW BEFORE DEPLOY: this changes the
-- access of real HR identities (advisor-flagged). It is intentionally conservative:
--   * Old built-in roles are RETAINED in rbac_roles (not deleted), so this is a
--     reversible safety net and nothing is orphaned. A separate, post-UAT cleanup
--     migration may retire them once confirmed unused.
--   * Reassignment runs FIRST (below); because users.role is free-text with no FK,
--     leaving any user on a now-unknown role would fail closed (lockout) — so we
--     never remove a role a user might still hold.
--
-- Mapping (confirmed with the client, see plan §8):
--   hr_staff            -> hr_store              (store operate)
--   hr_manager          -> area_hr              (area operate; needs area assignment)
--   operation_director  -> area_hr              (was subregion; needs area assignment)
--   sgm                 -> hiring_manager_store (read-only ops + approver)
--   regional_director   -> ta                   (all operate)
--   auditor             -> auditor              (unchanged; read-only all)
--   super_admin         -> super_admin          (unchanged)
--
-- ⚠️ area_hr users see NO candidates until an admin defines areas (areas/area_stores)
-- and assigns them (user_areas). That is fail-closed (safe) but is operational setup,
-- not part of this migration.

UPDATE users
SET role = CASE role
    WHEN 'hr_staff'           THEN 'hr_store'
    WHEN 'hr_manager'         THEN 'area_hr'
    WHEN 'operation_director' THEN 'area_hr'
    WHEN 'sgm'                THEN 'hiring_manager_store'
    WHEN 'regional_director'  THEN 'ta'
    ELSE role
END
WHERE role IN ('hr_staff', 'hr_manager', 'operation_director', 'sgm', 'regional_director');

-- Re-label still-pending approval steps so SLA reminders target the new-role holders.
-- (Decided/historical steps keep their original label for the audit trail.)
UPDATE approval_steps
SET role = CASE role
    WHEN 'hr_staff'          THEN 'hr_store'
    WHEN 'hr_manager'        THEN 'area_hr'
    WHEN 'sgm'               THEN 'hiring_manager_store'
    WHEN 'regional_director' THEN 'ta'
    ELSE role
END
WHERE status = 'pending'
  AND role IN ('hr_staff', 'hr_manager', 'sgm', 'regional_director');
