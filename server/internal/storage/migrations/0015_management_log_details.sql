ALTER TABLE management_logs ADD COLUMN log_id TEXT NOT NULL DEFAULT '';

ALTER TABLE management_logs ADD COLUMN details_json TEXT NOT NULL DEFAULT '{}';

UPDATE management_logs
SET log_id = 'log_legacy_' || printf('%04d', id)
WHERE log_id = '';

UPDATE management_logs
SET details_json = '{}'
WHERE details_json = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_management_logs_log_id
    ON management_logs (log_id);
