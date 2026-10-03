package storage

const kvMetadataIndexesSchema = `
DROP INDEX idx_plugin_kv_plugin_id;
CREATE INDEX idx_plugin_kv_metadata
    ON plugin_kv(plugin_id, key, expires_at_ms, size_bytes);
CREATE INDEX idx_plugin_kv_size_anomaly
    ON plugin_kv(expires_at_ms)
    WHERE typeof(size_bytes) <> 'integer' OR size_bytes < 0;
`
