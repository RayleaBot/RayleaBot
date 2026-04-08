-- name: InsertLogSummary :exec
INSERT INTO management_logs (log_id, ts, level, source, message, plugin_id, request_id, details_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLogSummary :one
SELECT log_id, ts, level, source, message, plugin_id, request_id, details_json
FROM management_logs
WHERE log_id = ?
LIMIT 1;

-- name: PruneLogsBefore :exec
DELETE FROM management_logs WHERE ts < ?;
