CREATE TABLE IF NOT EXISTS auth_bootstrap_state (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    identifier TEXT NOT NULL,
    secret_digest BLOB NOT NULL,
    signing_key BLOB NOT NULL,
    initialized_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    session_id TEXT PRIMARY KEY,
    subject TEXT NOT NULL,
    issued_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at
    ON admin_sessions (expires_at);
