ALTER TABLE third_party_accounts ADD COLUMN profile_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE third_party_accounts ADD COLUMN profile_nickname TEXT NOT NULL DEFAULT '';
ALTER TABLE third_party_accounts ADD COLUMN profile_avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE third_party_accounts ADD COLUMN credential_state TEXT NOT NULL DEFAULT 'unknown' CHECK (credential_state IN ('unknown', 'valid', 'invalid'));
ALTER TABLE third_party_accounts ADD COLUMN credential_checked_at TEXT;
ALTER TABLE third_party_accounts ADD COLUMN credential_last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE third_party_accounts ADD COLUMN last_used_at TEXT;
ALTER TABLE third_party_accounts ADD COLUMN proxy_url TEXT NOT NULL DEFAULT '';
ALTER TABLE third_party_accounts ADD COLUMN proxy_enabled INTEGER NOT NULL DEFAULT 0 CHECK (proxy_enabled IN (0, 1));
