-- name: AccessListContains :one
SELECT EXISTS(SELECT 1 FROM access_list_entries
WHERE list_kind = ? AND entry_type = ? AND target_id = ? AND source_protocol = ?
AND ((source_adapter = ? AND bot_id = ?) OR (source_protocol = 'onebot11' AND source_adapter = '' AND bot_id = '')));

-- name: AccessListGet :one
SELECT * FROM access_list_entries
WHERE list_kind = ? AND source_protocol = ? AND source_adapter = ? AND bot_id = ? AND entry_type = ? AND target_id = ?;

-- name: AccessListAdd :exec
INSERT INTO access_list_entries (list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id, reason, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (list_kind, source_protocol, source_adapter, bot_id, entry_type, target_id) DO UPDATE SET reason = excluded.reason;

-- name: AccessListRemove :execrows
DELETE FROM access_list_entries
WHERE list_kind = ? AND source_protocol = ? AND source_adapter = ? AND bot_id = ? AND entry_type = ? AND target_id = ?;

-- name: AccessListList :many
SELECT * FROM access_list_entries WHERE list_kind = ? AND entry_type = ? ORDER BY created_at DESC, id DESC;

-- name: WhitelistEnabled :one
SELECT enabled FROM whitelist_state WHERE singleton_id = 1;

-- name: SetWhitelistEnabled :exec
INSERT INTO whitelist_state (singleton_id, enabled, updated_at) VALUES (1, ?, ?)
ON CONFLICT (singleton_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at;
