ALTER TABLE plugin_grants
ADD COLUMN expires_at TEXT;

CREATE INDEX idx_plugin_grants_expires_at
ON plugin_grants(expires_at);
