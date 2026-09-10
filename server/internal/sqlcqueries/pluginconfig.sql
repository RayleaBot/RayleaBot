-- name: ListConfigsByNamespace :many
SELECT key, value_json
FROM system_configs
WHERE namespace = ?
ORDER BY key ASC;

-- name: UpsertConfig :exec
INSERT INTO system_configs (namespace, key, value_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace, key) DO UPDATE SET
    value_json = excluded.value_json,
    updated_at = excluded.updated_at;
