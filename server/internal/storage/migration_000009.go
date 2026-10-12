package storage

const accessListAccountsSchema = `
ALTER TABLE access_list_entries RENAME TO access_list_entries_000008;
CREATE TABLE access_list_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_kind TEXT NOT NULL CHECK (list_kind IN ('blacklist', 'whitelist')),
    source_protocol TEXT NOT NULL CHECK (source_protocol IN ('onebot11', 'qqofficial')),
    bot_id TEXT NOT NULL,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('user', 'group')),
    target_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CHECK ((source_protocol = 'onebot11' AND bot_id = '') OR bot_id <> ''),
    UNIQUE (list_kind, source_protocol, bot_id, entry_type, target_id)
);
INSERT INTO access_list_entries (id, list_kind, source_protocol, bot_id, entry_type, target_id, reason, created_at)
SELECT id, list_kind, source_protocol, bot_id, entry_type, target_id, reason, created_at
FROM (
    SELECT *, row_number() OVER (
        PARTITION BY list_kind, source_protocol, bot_id, entry_type, target_id
        ORDER BY created_at, id
    ) AS account_row
    FROM access_list_entries_000008
)
WHERE account_row = 1;
INSERT INTO sqlite_sequence(name, seq)
SELECT 'access_list_entries', seq FROM sqlite_sequence WHERE name = 'access_list_entries_000008'
AND NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'access_list_entries');
UPDATE sqlite_sequence SET seq = max(seq, coalesce(
    (SELECT seq FROM sqlite_sequence WHERE name = 'access_list_entries_000008'), 0))
WHERE name = 'access_list_entries';
DROP TABLE access_list_entries_000008;
`
