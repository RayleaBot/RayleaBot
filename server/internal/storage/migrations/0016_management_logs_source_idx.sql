CREATE INDEX IF NOT EXISTS idx_management_logs_source
    ON management_logs (source, ts DESC, id DESC);
