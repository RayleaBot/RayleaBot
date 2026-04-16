ALTER TABLE management_logs ADD COLUMN boot_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_management_logs_boot_ts
    ON management_logs (boot_id, ts DESC, id DESC);
