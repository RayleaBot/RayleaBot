-- name: GetKV :one
SELECT value_json, size_bytes, expires_at_ms FROM plugin_kv
WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key)
AND (expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms));

-- name: GetKVWriteState :one
SELECT COALESCE(size_bytes, 0) AS size_bytes,
    CAST(typeof(size_bytes) = 'integer' AND size_bytes = sqlc.arg(next_size)
        AND typeof(expires_at_ms) = typeof(sqlc.narg(next_expiry))
        AND expires_at_ms IS sqlc.narg(next_expiry) AS INTEGER) AS metadata_matches
FROM plugin_kv
WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key)
AND (expires_at_ms IS NULL OR expires_at_ms > sqlc.arg(now_ms));

-- name: GetKVTotalSize :one
-- Nonnegative integer sizes sum identically in any order. Historical signed or
-- noninteger sizes retain the original table scan's aggregate/overflow behavior.
SELECT CAST(CASE WHEN EXISTS (
    SELECT 1 FROM plugin_kv AS anomaly INDEXED BY idx_plugin_kv_size_anomaly
    WHERE (typeof(anomaly.size_bytes) <> 'integer' OR anomaly.size_bytes < 0)
    AND (anomaly.expires_at_ms IS NULL OR anomaly.expires_at_ms > sqlc.arg(now_ms))
) THEN (
    SELECT COALESCE(SUM(legacy.size_bytes), 0) FROM plugin_kv AS legacy NOT INDEXED
    WHERE legacy.expires_at_ms IS NULL OR legacy.expires_at_ms > sqlc.arg(now_ms)
) ELSE (
    SELECT COALESCE(SUM(current.size_bytes), 0) FROM plugin_kv AS current
    WHERE current.expires_at_ms IS NULL OR current.expires_at_ms > sqlc.arg(now_ms)
) END AS INTEGER);

-- name: UpsertKV :exec
INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at, expires_at_ms)
VALUES (sqlc.arg(plugin_id), sqlc.arg(key), sqlc.arg(value_json), sqlc.arg(size_bytes), sqlc.arg(updated_at), sqlc.narg(expires_at_ms))
ON CONFLICT(plugin_id, key) DO UPDATE SET
    value_json = excluded.value_json,
    size_bytes = excluded.size_bytes,
    updated_at = excluded.updated_at,
    expires_at_ms = excluded.expires_at_ms;

-- name: UpdateKVValue :exec
UPDATE plugin_kv SET value_json = sqlc.arg(value_json), updated_at = sqlc.arg(updated_at)
WHERE plugin_id = sqlc.arg(plugin_id) AND key = sqlc.arg(key);

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
