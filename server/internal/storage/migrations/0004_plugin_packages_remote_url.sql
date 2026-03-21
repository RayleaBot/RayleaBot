-- Widen source_type CHECK to include remote_url.
-- SQLite does not support ALTER CHECK, so recreate the table.
CREATE TABLE plugin_packages_new (
	plugin_id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL CHECK (source_type IN ('local_directory', 'local_zip', 'remote_url')),
	source_ref TEXT NOT NULL,
	version TEXT NOT NULL,
	manifest_hash TEXT NOT NULL,
	package_hash TEXT NOT NULL,
	installed_at TEXT NOT NULL
);

INSERT INTO plugin_packages_new SELECT * FROM plugin_packages;
DROP TABLE plugin_packages;
ALTER TABLE plugin_packages_new RENAME TO plugin_packages;
