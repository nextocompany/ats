-- slice 2.3: persist the verified LINE user id (the `sub` from the LIFF id-token)
-- so real LINE push (Messaging API) has a valid `to` handle. Nullable: legacy and
-- demo candidates have none; not unique (a candidate may reapply).
ALTER TABLE candidates ADD COLUMN IF NOT EXISTS line_user_id TEXT;
