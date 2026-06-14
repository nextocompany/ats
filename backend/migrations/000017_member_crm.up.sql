-- HR member-management CRM (Phase C): per-member HR notes and tags.
-- Both reference candidate_accounts with ON DELETE CASCADE so they follow the
-- account if it is ever hard-deleted (accounts are normally anonymized in place,
-- not deleted, but CASCADE keeps these tables consistent either way).
-- Notes are HR-only and never exposed on the public portal /me.
CREATE TABLE IF NOT EXISTS member_notes (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
  author_email VARCHAR(255) NOT NULL DEFAULT '',
  body         TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_member_notes_account ON member_notes (account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS member_tags (
  account_id UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
  tag        VARCHAR(50) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (account_id, tag)
);
-- Reverse lookup for the directory's tag filter (members carrying tag X).
CREATE INDEX IF NOT EXISTS idx_member_tags_tag ON member_tags (tag);
