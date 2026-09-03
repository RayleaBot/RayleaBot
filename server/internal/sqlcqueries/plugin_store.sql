-- name: ListPluginStoreSources :many
SELECT source_id, name, url, official
FROM plugin_store_sources
ORDER BY official DESC, name ASC, source_id ASC;

-- name: GetPluginStoreSource :one
SELECT source_id, name, url, official
FROM plugin_store_sources
WHERE source_id = ?;

-- name: CreatePluginStoreSource :exec
INSERT INTO plugin_store_sources (source_id, name, url, official)
VALUES (?, ?, ?, 0);

-- name: UpdatePluginStoreSource :execrows
UPDATE plugin_store_sources
SET name = ?, url = ?
WHERE source_id = ? AND official = 0;

-- name: DeletePluginStoreSource :execrows
DELETE FROM plugin_store_sources
WHERE source_id = ? AND official = 0;

-- name: LoadPluginStoreCatalogCaches :many
SELECT source_id, catalog_json, refreshed_at
FROM plugin_store_catalog_cache;

-- name: SavePluginStoreCatalogCache :exec
INSERT INTO plugin_store_catalog_cache (source_id, catalog_json, refreshed_at)
VALUES (?, ?, ?)
ON CONFLICT(source_id) DO UPDATE SET
    catalog_json = excluded.catalog_json,
    refreshed_at = excluded.refreshed_at;
