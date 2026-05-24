-- name: SaveJob :exec
INSERT INTO scheduler_jobs (job_id, plugin_id, log_label, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
    log_label = excluded.log_label,
    cron_expr = excluded.cron_expr,
    payload = excluded.payload,
    enabled = excluded.enabled,
    next_run = excluded.next_run,
    last_run = excluded.last_run,
    updated_at = excluded.updated_at;

-- name: LoadJobs :many
SELECT job_id, plugin_id, log_label, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at
FROM scheduler_jobs ORDER BY created_at ASC;

-- name: DeleteJob :exec
DELETE FROM scheduler_jobs WHERE job_id = ?;

-- name: DeleteJobsByPlugin :exec
DELETE FROM scheduler_jobs WHERE plugin_id = ?;
