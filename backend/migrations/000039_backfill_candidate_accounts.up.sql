-- Phase-1 of the Candidates+Members unify: the HR "Candidates" surface becomes
-- account-keyed (candidate_accounts), so every person — including 0-application
-- portal signups — shows in one list. Accountless per-intake candidates (walk-in /
-- bulk / SEEK / PeopleSoft rows whose account_id is NULL) would vanish from an
-- account-based list. This one-time, SILENT backfill links them: for each
-- accountless candidate carrying a non-empty email, ensure a candidate_accounts
-- row exists and point candidates.account_id at it.
--
-- SILENT: no emails, no auth flow, no intake change. Backfilled accounts are NOT
-- email_verified (the person never logged in) and carry no providers — they are a
-- directory shell that Phase 2's at-intake provisioning will later own. Accountless
-- candidates with NO email (rare legacy) are left untouched and stay reachable via
-- the Applications inbox.
--
-- candidate_accounts.email is stored lowercased+trimmed (candidateauth normalises
-- on signup), and is UNIQUE. We insert and match on lower(trim(email)) so a
-- differently-cased candidate email links to (not duplicates) an existing account.

-- 1) Create a shell account for each distinct accountless candidate email that
--    does not already have one. DISTINCT ON collapses many candidates sharing an
--    email to a single account; ON CONFLICT(email) DO NOTHING is belt-and-braces
--    against a concurrent/existing row.
INSERT INTO candidate_accounts (email, full_name, phone, province, email_verified, status)
SELECT DISTINCT ON (lower(trim(c.email)))
       lower(trim(c.email)),
       COALESCE(c.full_name, ''),
       c.phone,
       c.province,
       FALSE,
       'active'
FROM candidates c
WHERE c.account_id IS NULL
  AND c.email IS NOT NULL
  AND trim(c.email) <> ''
  AND NOT EXISTS (
      SELECT 1 FROM candidate_accounts a WHERE a.email = lower(trim(c.email))
  )
ORDER BY lower(trim(c.email)), c.created_at ASC
ON CONFLICT (email) DO NOTHING;

-- 2) Link every accountless candidate with an email to its (now-existing) account.
UPDATE candidates c
SET account_id = a.id,
    updated_at = NOW()
FROM candidate_accounts a
WHERE c.account_id IS NULL
  AND c.email IS NOT NULL
  AND trim(c.email) <> ''
  AND a.email = lower(trim(c.email));
