CREATE TABLE access_list_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_kind TEXT NOT NULL CHECK (list_kind IN ('blacklist', 'whitelist')),
    source_protocol TEXT NOT NULL CHECK (source_protocol IN ('onebot11', 'qqofficial')),
    source_adapter TEXT NOT NULL,
    bot_id TEXT NOT NULL,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CHECK ((source_protocol = 'onebot11' AND source_adapter = '' AND bot_id = '') OR (source_adapter <> '' AND bot_id <> '')),
    UNIQUE (list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id)
);

INSERT INTO access_list_entries (list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id, reason, created_at)
SELECT 'blacklist', 'onebot11', '', '', entry_type, target_id, reason, created_at FROM blacklist_entries;
INSERT INTO access_list_entries (list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id, reason, created_at)
SELECT 'whitelist', 'onebot11', '', '', entry_type, target_id, reason, created_at FROM whitelist_entries;

DROP TABLE blacklist_entries;
DROP TABLE whitelist_entries;
