-- name: InsertLogSummary :exec
INSERT OR IGNORE INTO management_logs (log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLogSummary :one
SELECT log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json
FROM management_logs
WHERE log_id = ?
LIMIT 1;

-- name: PruneLogsBefore :exec
-- Non-numeric timestamps may be SQLite's dynamic "now" or "subsec" values.
-- Keep them as candidates without evaluating them inside a persistent index.
DELETE FROM management_logs
WHERE (CASE WHEN ts GLOB '[0-9]*' THEN julianday(ts) ELSE -1 END) < julianday(CAST(sqlc.arg(cutoff) AS TEXT))
    AND julianday(ts) < julianday(CAST(sqlc.arg(cutoff) AS TEXT));
