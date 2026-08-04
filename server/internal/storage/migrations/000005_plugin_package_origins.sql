ALTER TABLE plugin_packages RENAME TO plugin_packages_legacy;

CREATE TABLE plugin_packages (
    plugin_id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL CHECK (source_type IN ('local_directory', 'local_zip', 'remote_url', 'catalog', 'development')),
    source_ref TEXT NOT NULL,
    version TEXT NOT NULL,
    manifest_hash TEXT NOT NULL,
    package_hash TEXT NOT NULL,
    archive_hash TEXT NOT NULL DEFAULT '',
    publisher_id TEXT NOT NULL DEFAULT '',
    publisher_name TEXT NOT NULL DEFAULT '',
    publisher_verified INTEGER NOT NULL DEFAULT 0 CHECK (publisher_verified IN (0, 1)),
    catalog_digest TEXT NOT NULL DEFAULT '',
    installed_at TEXT NOT NULL
);

INSERT INTO plugin_packages (
    plugin_id, source_type, source_ref, version, manifest_hash, package_hash,
    archive_hash, publisher_id, publisher_name, publisher_verified, catalog_digest, installed_at
)
SELECT
    plugin_id, source_type, source_ref, version, manifest_hash, package_hash,
    '', '', '', 0, '', installed_at
FROM plugin_packages_legacy;

DROP TABLE plugin_packages_legacy;
