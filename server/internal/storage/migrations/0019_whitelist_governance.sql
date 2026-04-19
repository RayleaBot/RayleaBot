CREATE TABLE IF NOT EXISTS whitelist_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_type  TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id   TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE(entry_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_whitelist_entries_lookup
    ON whitelist_entries (entry_type, target_id);

CREATE TABLE IF NOT EXISTS whitelist_state (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    enabled      INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    updated_at   TEXT NOT NULL
);

INSERT INTO whitelist_state (singleton_id, enabled, updated_at)
VALUES (1, 0, '1970-01-01T00:00:00Z')
ON CONFLICT (singleton_id) DO NOTHING;
