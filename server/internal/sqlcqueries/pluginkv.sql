-- name: GetKV :one
SELECT value_json, size_bytes, expires_at_ms FROM plugin_kv
WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key)
AND (expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms));

-- name: GetKVSize :one
SELECT COALESCE(size_bytes, 0) FROM plugin_kv
WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key)
AND (expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms));

-- name: GetKVTotalSize :one
SELECT CAST(COALESCE(SUM(size_bytes), 0) AS INTEGER) FROM plugin_kv
WHERE expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms);

-- name: UpsertKV :exec
INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at, expires_at_ms)
VALUES (sqlc.arg(plugin_id), sqlc.arg(key), sqlc.arg(value_json), sqlc.arg(size_bytes), sqlc.arg(updated_at), sqlc.narg(expires_at_ms))
ON CONFLICT(plugin_id, key) DO UPDATE SET
    value_json = excluded.value_json,
    size_bytes = excluded.size_bytes,
    updated_at = excluded.updated_at,
    expires_at_ms = excluded.expires_at_ms;

-- name: DeleteKV :execrows
DELETE FROM plugin_kv WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key)
AND (expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms));

-- name: DeleteExpiredKV :execrows
DELETE FROM plugin_kv WHERE rowid IN (
  SELECT expired.rowid FROM plugin_kv AS expired WHERE expired.expires_at_ms IS NOT NULL AND expired.expires_at_ms <= sqlc.arg(now_ms)
  ORDER BY expired.expires_at_ms, expired.rowid LIMIT 1000
);

-- ListKVKeys uses ESCAPE clause not supported by sqlc's SQLite parser.
-- Kept in plugins/storage/kv.go and registered as a SQL exception.
