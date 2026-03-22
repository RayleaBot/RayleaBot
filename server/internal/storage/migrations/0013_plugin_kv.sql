CREATE TABLE plugin_kv (
	plugin_id TEXT NOT NULL,
	key TEXT NOT NULL,
	value_json TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (plugin_id, key)
);

CREATE INDEX idx_plugin_kv_plugin_id
ON plugin_kv(plugin_id);
