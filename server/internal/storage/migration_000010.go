package storage

const botAccountsSchema = `
CREATE TABLE bot_accounts (
    source_protocol TEXT NOT NULL CHECK (source_protocol IN ('onebot11', 'qqofficial')),
    bot_id TEXT NOT NULL CHECK (length(bot_id) > 0),
    nickname TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    adapter_id TEXT UNIQUE,
    adapter_display_name TEXT NOT NULL,
    adapter_app_id TEXT NOT NULL DEFAULT '',
    last_adapter_id TEXT NOT NULL,
    last_adapter_display_name TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (source_protocol, bot_id)
);`
