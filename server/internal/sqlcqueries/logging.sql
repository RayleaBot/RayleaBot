-- name: InsertLogSummary :exec
INSERT OR IGNORE INTO management_logs (log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLogSummary :one
SELECT log_id, boot_id, ts, level, source, message, plugin_id, request_id, details_json
FROM management_logs
WHERE log_id = ?
LIMIT 1;

-- name: PruneLogsBefore :exec
DELETE FROM management_logs
WHERE julianday(ts) < julianday(CAST(sqlc.arg(cutoff) AS TEXT));
