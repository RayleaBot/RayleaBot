-- name: GetKV :one
SELECT value_json FROM plugin_kv WHERE plugin_id = ? AND key = ?;

-- name: GetKVSize :one
SELECT COALESCE(size_bytes, 0) FROM plugin_kv WHERE plugin_id = ? AND key = ?;

-- name: GetKVTotalSize :one
SELECT CAST(COALESCE(SUM(size_bytes), 0) AS INTEGER) FROM plugin_kv WHERE plugin_id = ?;

-- name: UpsertKV :exec
INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(plugin_id, key) DO UPDATE SET
    value_json = excluded.value_json,
    size_bytes = excluded.size_bytes,
    updated_at = excluded.updated_at;

-- name: DeleteKV :execresult
DELETE FROM plugin_kv WHERE plugin_id = ? AND key = ?;

-- ListKVKeys uses ESCAPE clause not supported by sqlc's SQLite parser.
-- Kept as hand-written SQL in pluginkv/repository.go.
