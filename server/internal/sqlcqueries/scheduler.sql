-- name: SaveJob :exec
INSERT INTO scheduler_jobs (
    job_id, plugin_id, log_label, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at,
    last_duration_ms, last_error_code, last_error_message, last_error_at,
    success_count, failure_count, timeout_count, retry_count, other_count
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
    log_label = excluded.log_label,
    cron_expr = excluded.cron_expr,
    payload = excluded.payload,
    enabled = excluded.enabled,
    next_run = excluded.next_run,
    last_run = excluded.last_run,
    updated_at = excluded.updated_at;

-- name: LoadJobs :many
SELECT
    job_id, plugin_id, log_label, cron_expr, payload, enabled, next_run, last_run, created_at, updated_at,
    last_duration_ms, last_error_code, last_error_message, last_error_at,
    success_count, failure_count, timeout_count, retry_count, other_count
FROM scheduler_jobs ORDER BY created_at ASC;

-- name: RecordJobRunSuccess :exec
UPDATE scheduler_jobs
SET
    last_run = ?,
    last_duration_ms = ?,
    success_count = success_count + 1,
    updated_at = ?
WHERE job_id = ?;

-- name: RecordJobRunFailure :exec
UPDATE scheduler_jobs
SET
    last_run = ?,
    last_duration_ms = ?,
    last_error_code = ?,
    last_error_message = ?,
    last_error_at = ?,
    failure_count = failure_count + ?,
    timeout_count = timeout_count + ?,
    retry_count = retry_count + ?,
    other_count = other_count + ?,
    updated_at = ?
WHERE job_id = ?;

-- name: UpdateJobSchedule :exec
UPDATE scheduler_jobs
SET
    next_run = ?,
    updated_at = ?
WHERE job_id = ?;

-- name: DeleteJob :exec
DELETE FROM scheduler_jobs WHERE job_id = ?;

-- name: DeleteJobsByPlugin :exec
DELETE FROM scheduler_jobs WHERE plugin_id = ?;
