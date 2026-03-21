CREATE TABLE plugin_grants (
	plugin_id TEXT NOT NULL,
	capability TEXT NOT NULL,
	granted_at TEXT NOT NULL,
	PRIMARY KEY (plugin_id, capability)
);
