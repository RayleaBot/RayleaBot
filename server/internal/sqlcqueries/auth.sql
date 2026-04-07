-- name: LoadBootstrap :one
SELECT identifier, secret_digest, signing_key, initialized_at
FROM auth_bootstrap_state WHERE singleton_id = 1;

-- name: CountBootstrap :one
SELECT COUNT(*) FROM auth_bootstrap_state WHERE singleton_id = 1;

-- name: InsertBootstrap :exec
INSERT INTO auth_bootstrap_state (singleton_id, identifier, secret_digest, signing_key, initialized_at)
VALUES (1, ?, ?, ?, ?);

-- name: LoadSessions :many
SELECT session_id, subject, issued_at, expires_at FROM admin_sessions;

-- name: UpsertSession :exec
INSERT INTO admin_sessions (session_id, subject, issued_at, expires_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    subject = excluded.subject,
    issued_at = excluded.issued_at,
    expires_at = excluded.expires_at;

-- name: DeleteSession :exec
DELETE FROM admin_sessions WHERE session_id = ?;
