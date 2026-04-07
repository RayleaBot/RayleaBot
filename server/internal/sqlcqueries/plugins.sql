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
INSERT INTO plugin_packages (plugin_id, source_type, source_ref, version, manifest_hash, package_hash, installed_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(plugin_id) DO UPDATE SET
    source_type = excluded.source_type,
    source_ref = excluded.source_ref,
    version = excluded.version,
    manifest_hash = excluded.manifest_hash,
    package_hash = excluded.package_hash,
    installed_at = excluded.installed_at;

-- name: DeletePackageMetadata :exec
DELETE FROM plugin_packages WHERE plugin_id = ?;

-- name: LoadAllPackageMetadata :many
SELECT plugin_id, source_type, source_ref, version, manifest_hash, package_hash, installed_at
FROM plugin_packages;

-- name: LoadGrants :many
SELECT plugin_id, capability, scope_json, granted_at, expires_at
FROM plugin_grants WHERE plugin_id = ? ORDER BY capability;

-- name: LoadAllGrants :many
SELECT plugin_id, capability, expires_at
FROM plugin_grants ORDER BY plugin_id, capability;

-- name: SaveGrant :exec
INSERT INTO plugin_grants (plugin_id, capability, scope_json, granted_at, expires_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(plugin_id, capability) DO UPDATE SET
    scope_json = excluded.scope_json,
    granted_at = excluded.granted_at,
    expires_at = excluded.expires_at;

-- name: DeleteGrant :exec
DELETE FROM plugin_grants WHERE plugin_id = ? AND capability = ?;

-- name: DeleteAllGrants :exec
DELETE FROM plugin_grants WHERE plugin_id = ?;
