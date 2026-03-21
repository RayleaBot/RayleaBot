CREATE TABLE plugin_packages (
	plugin_id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL CHECK (source_type IN ('local_directory', 'local_zip')),
	source_ref TEXT NOT NULL,
	version TEXT NOT NULL,
	manifest_hash TEXT NOT NULL,
	package_hash TEXT NOT NULL,
	installed_at TEXT NOT NULL
);
