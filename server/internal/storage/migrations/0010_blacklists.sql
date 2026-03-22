CREATE TABLE IF NOT EXISTS blacklist_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_type  TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id   TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE(entry_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_blacklist_entries_lookup
    ON blacklist_entries (entry_type, target_id);
