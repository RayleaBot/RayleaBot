-- name: SaveTask :exec
INSERT INTO tasks (task_id, task_type, status, progress, summary, started_at, finished_at, result_json, error_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(task_id) DO UPDATE SET
    status = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.status
        ELSE excluded.status
    END,
    progress = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.progress
        ELSE excluded.progress
    END,
    summary = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.summary
        ELSE excluded.summary
    END,
    started_at = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.started_at
        ELSE excluded.started_at
    END,
    finished_at = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.finished_at
        ELSE excluded.finished_at
    END,
    result_json = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.result_json
        ELSE excluded.result_json
    END,
    error_json = CASE
        WHEN tasks.status IN ('succeeded', 'failed', 'cancelled', 'interrupted')
            AND excluded.status IN ('pending', 'running')
        THEN tasks.error_json
        ELSE excluded.error_json
    END;

-- name: LoadTasks :many
SELECT task_id, task_type, status, progress, summary, started_at, finished_at, result_json, error_json
FROM tasks ORDER BY created_at ASC;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE task_id = ?;
