CREATE TABLE IF NOT EXISTS scheduler_jobs (
    job_id     TEXT PRIMARY KEY,
    plugin_id  TEXT NOT NULL,
    cron_expr  TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    enabled    INTEGER NOT NULL DEFAULT 1,
    next_run   TEXT NOT NULL,
    last_run   TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_next_run ON scheduler_jobs (next_run) WHERE enabled = 1;
CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_plugin_id ON scheduler_jobs (plugin_id);
