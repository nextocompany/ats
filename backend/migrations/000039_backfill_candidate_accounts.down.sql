-- Best-effort reversal of the silent backfill. A backfill is not cleanly
-- reversible once the data is in use, so this only undoes UNTOUCHED shells and
-- never destroys anything a person or an HR user has since interacted with.
--
-- A "backfill shell" is an account that still looks exactly as the backfill left
-- it: never email-verified, no LINE/Google provider, no login session, and no HR
-- CRM (notes/tags). If any of those exist the account has been used — leave it.

-- 1) Unlink candidates that point at an untouched backfill shell.
UPDATE candidates c
SET account_id = NULL,
    updated_at = NOW()
FROM candidate_accounts a
WHERE c.account_id = a.id
  AND a.email_verified = FALSE
  AND a.line_user_id IS NULL
  AND a.google_sub IS NULL
  AND NOT EXISTS (SELECT 1 FROM candidate_sessions s WHERE s.account_id = a.id)
  AND NOT EXISTS (SELECT 1 FROM member_notes n WHERE n.account_id = a.id)
  AND NOT EXISTS (SELECT 1 FROM member_tags  t WHERE t.account_id = a.id);

-- 2) Delete the now-orphaned shells (no remaining linked candidate, still pristine).
DELETE FROM candidate_accounts a
WHERE a.email_verified = FALSE
  AND a.line_user_id IS NULL
  AND a.google_sub IS NULL
  AND NOT EXISTS (SELECT 1 FROM candidates s        WHERE s.account_id = a.id)
  AND NOT EXISTS (SELECT 1 FROM candidate_sessions s WHERE s.account_id = a.id)
  AND NOT EXISTS (SELECT 1 FROM member_notes n       WHERE n.account_id = a.id)
  AND NOT EXISTS (SELECT 1 FROM member_tags  t       WHERE t.account_id = a.id);
