ALTER TABLE third_party_accounts RENAME TO third_party_accounts_legacy;

CREATE TABLE third_party_accounts (
    platform TEXT NOT NULL CHECK (platform IN ('bilibili', 'weibo', 'douyin', 'netease_music')),
    account_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    secret_key TEXT NOT NULL,
    profile_uid TEXT NOT NULL DEFAULT '',
    profile_nickname TEXT NOT NULL DEFAULT '',
    profile_avatar_url TEXT NOT NULL DEFAULT '',
    credential_state TEXT NOT NULL DEFAULT 'unknown' CHECK (credential_state IN ('unknown', 'valid', 'invalid')),
    credential_checked_at TEXT,
    credential_last_error TEXT NOT NULL DEFAULT '',
    last_used_at TEXT,
    proxy_url TEXT NOT NULL DEFAULT '',
    proxy_enabled INTEGER NOT NULL DEFAULT 0 CHECK (proxy_enabled IN (0, 1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (platform, account_id)
);

INSERT INTO third_party_accounts (
    platform, account_id, label, enabled, secret_key,
    profile_uid, profile_nickname, profile_avatar_url,
    credential_state, credential_checked_at, credential_last_error,
    last_used_at, proxy_url, proxy_enabled, updated_at
)
SELECT
    platform, account_id, label, enabled, secret_key,
    profile_uid, profile_nickname, profile_avatar_url,
    credential_state, credential_checked_at, credential_last_error,
    last_used_at, proxy_url, proxy_enabled, updated_at
FROM third_party_accounts_legacy;

DROP TABLE third_party_accounts_legacy;

CREATE INDEX IF NOT EXISTS idx_third_party_accounts_platform
    ON third_party_accounts (platform);
