CREATE TABLE IF NOT EXISTS management_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          TEXT NOT NULL,
    level       TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    source      TEXT NOT NULL,
    message     TEXT NOT NULL,
    plugin_id   TEXT NOT NULL DEFAULT '',
    request_id  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_management_logs_ts
    ON management_logs (ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_plugin
    ON management_logs (plugin_id, ts DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_management_logs_request
    ON management_logs (request_id, ts DESC, id DESC);
