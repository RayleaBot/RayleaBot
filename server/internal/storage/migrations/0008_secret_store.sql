CREATE TABLE IF NOT EXISTS secret_store (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
