-- name: LoadDesiredStates :many
SELECT plugin_id, desired_state FROM plugin_instances;

-- name: SaveDesiredState :exec
INSERT INTO plugin_instances (plugin_id, desired_state, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(plugin_id) DO UPDATE SET
    desired_state = excluded.desired_state,
    updated_at = excluded.updated_at;

-- name: DeleteDesiredState :exec
DELETE FROM plugin_instances WHERE plugin_id = ?;

-- name: SavePackageMetadata :exec
INSERT INTO plugin_packages (
    plugin_id, source_type, source_ref, version, manifest_hash, package_hash,
    archive_hash, publisher_id, publisher_name, publisher_verified, catalog_digest, installed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(plugin_id) DO UPDATE SET
    source_type = excluded.source_type,
    source_ref = excluded.source_ref,
    version = excluded.version,
    manifest_hash = excluded.manifest_hash,
    package_hash = excluded.package_hash,
    archive_hash = excluded.archive_hash,
    publisher_id = excluded.publisher_id,
    publisher_name = excluded.publisher_name,
    publisher_verified = excluded.publisher_verified,
    catalog_digest = excluded.catalog_digest,
    installed_at = excluded.installed_at;

-- name: DeletePackageMetadata :exec
DELETE FROM plugin_packages WHERE plugin_id = ?;

-- name: LoadAllPackageMetadata :many
SELECT plugin_id, source_type, source_ref, version, manifest_hash, package_hash,
       archive_hash, publisher_id, publisher_name, publisher_verified, catalog_digest, installed_at
FROM plugin_packages;
