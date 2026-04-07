-- name: CountNamespace :one
SELECT COUNT(*) FROM system_configs WHERE namespace = ?;

-- name: UpsertConfig :exec
INSERT INTO system_configs (namespace, key, value_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace, key) DO UPDATE SET
    value_json = excluded.value_json,
    updated_at = excluded.updated_at;

-- name: SeedConfig :exec
INSERT INTO system_configs (namespace, key, value_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace, key) DO NOTHING;
