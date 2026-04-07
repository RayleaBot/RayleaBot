-- name: InsertLogSummary :exec
INSERT INTO management_logs (ts, level, source, message, plugin_id, request_id)
VALUES (?, ?, ?, ?, ?, ?);

-- name: PruneLogsBefore :exec
DELETE FROM management_logs WHERE ts < ?;
