-- name: GetSecret :one
SELECT value FROM secret_store WHERE key = ?;

-- name: UpsertSecret :exec
INSERT INTO secret_store (key, value, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at;

-- name: DeleteSecret :exec
DELETE FROM secret_store WHERE key = ?;

-- name: ListSecretKeys :many
SELECT key FROM secret_store ORDER BY key;
