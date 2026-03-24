CREATE TABLE system_configs (
	namespace TEXT NOT NULL,
	key TEXT NOT NULL,
	value_json TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (namespace, key)
);

CREATE INDEX idx_system_configs_namespace
ON system_configs(namespace);
