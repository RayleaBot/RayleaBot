-- name: StartMessageTracking :exec
INSERT OR IGNORE INTO message_stats_tracking(singleton_id, started_at_ms) VALUES (1, ?);

-- name: GetMessageTracking :one
SELECT started_at_ms FROM message_stats_tracking WHERE singleton_id = 1;

-- name: RecoverMessageStatsOffline :exec
UPDATE message_stats_offline SET ended_at_ms = (
    SELECT COALESCE(stopped_at_ms, last_alive_at_ms) FROM message_stats_runs WHERE id = run_id
) WHERE ended_at_ms IS NULL;

-- name: RecoverMessageStatsRuns :exec
UPDATE message_stats_runs SET stopped_at_ms = last_alive_at_ms WHERE stopped_at_ms IS NULL;

-- name: StartMessageStatsRun :one
INSERT INTO message_stats_runs(started_at_ms, last_alive_at_ms) VALUES (?, ?) RETURNING id;

-- name: UpdateMessageStatsRun :exec
UPDATE message_stats_runs SET last_alive_at_ms = ?, stopped_at_ms = ? WHERE id = ?;

-- name: UpsertMessageStatsAdapter :exec
INSERT INTO message_stats_adapters(adapter_id, protocol, last_received_at_ms) VALUES (?, ?, ?)
ON CONFLICT(adapter_id) DO UPDATE SET protocol = excluded.protocol,
    last_received_at_ms = CASE
        WHEN message_stats_adapters.last_received_at_ms IS NULL THEN excluded.last_received_at_ms
        WHEN excluded.last_received_at_ms IS NULL THEN message_stats_adapters.last_received_at_ms
        ELSE MAX(message_stats_adapters.last_received_at_ms, excluded.last_received_at_ms) END;

-- name: AddMessageStatsHour :exec
INSERT INTO message_stats_hours(hour_start, adapter_id, received, sent) VALUES (?, ?, ?, ?)
ON CONFLICT(hour_start, adapter_id) DO UPDATE SET
    received = message_stats_hours.received + excluded.received,
    sent = message_stats_hours.sent + excluded.sent;

-- name: UpsertMessageStatsOffline :exec
INSERT INTO message_stats_offline(run_id, adapter_id, started_at_ms, ended_at_ms) VALUES (?, ?, ?, ?)
ON CONFLICT(run_id, adapter_id, started_at_ms) DO UPDATE SET ended_at_ms = excluded.ended_at_ms;

-- name: ListMessageStatsAdapters :many
SELECT * FROM message_stats_adapters;

-- name: ListMessageStatsOffline :many
SELECT * FROM message_stats_offline
WHERE started_at_ms < sqlc.arg(as_of_ms)
	AND (ended_at_ms IS NOT NULL OR run_id <> sqlc.arg(current_run_id))
    AND (ended_at_ms IS NULL OR ended_at_ms > sqlc.arg(start_ms))
    AND (ended_at_ms IS NULL OR ended_at_ms - started_at_ms >= 30000)
ORDER BY started_at_ms DESC, adapter_id DESC, run_id DESC LIMIT 1001;

-- name: ListMessageStatsStops :many
SELECT r.stopped_at_ms AS started_at_ms, n.started_at_ms AS ended_at_ms
FROM message_stats_runs n CROSS JOIN message_stats_runs r
    ON r.id = (SELECT MAX(previous_run.id) FROM message_stats_runs previous_run WHERE previous_run.id < n.id)
WHERE n.started_at_ms > sqlc.arg(start_ms)
    AND r.stopped_at_ms IS NOT NULL AND r.stopped_at_ms < n.started_at_ms
    AND r.stopped_at_ms < sqlc.arg(as_of_ms)
ORDER BY r.stopped_at_ms DESC LIMIT 1001;
