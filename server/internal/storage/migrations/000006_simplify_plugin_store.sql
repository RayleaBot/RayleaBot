ALTER TABLE plugin_packages RENAME TO plugin_packages_before_store_simplification;

CREATE TABLE plugin_packages (
    plugin_id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL CHECK (source_type IN ('local_directory', 'local_zip', 'remote_url', 'catalog', 'development')),
    source_ref TEXT NOT NULL,
    version TEXT NOT NULL,
    package_hash TEXT NOT NULL,
    installed_at TEXT NOT NULL
);

INSERT INTO plugin_packages (
    plugin_id, source_type, source_ref, version, package_hash, installed_at
)
SELECT
    plugin_id, source_type, source_ref, version, package_hash, installed_at
FROM plugin_packages_before_store_simplification;

DROP TABLE plugin_packages_before_store_simplification;

CREATE TABLE plugin_store_sources (
    source_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    official INTEGER NOT NULL DEFAULT 0 CHECK (official IN (0, 1))
);

INSERT INTO plugin_store_sources (source_id, name, url, official)
VALUES ('official', 'RayleaBot 官方插件', 'https://raw.githubusercontent.com/RayleaBot/plugin-catalog/main/catalog.json', 1);

CREATE TABLE plugin_store_catalog_cache (
    source_id TEXT PRIMARY KEY REFERENCES plugin_store_sources(source_id) ON DELETE CASCADE,
    catalog_json TEXT NOT NULL,
    refreshed_at TEXT NOT NULL
);
