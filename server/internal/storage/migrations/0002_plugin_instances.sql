CREATE TABLE IF NOT EXISTS plugin_instances (
    plugin_id TEXT PRIMARY KEY,
    desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
    updated_at TEXT NOT NULL
);
