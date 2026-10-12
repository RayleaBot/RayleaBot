-- name: ListBotAccounts :many
SELECT * FROM bot_accounts;

-- name: UnbindBotAccount :exec
UPDATE bot_accounts
SET last_adapter_id = adapter_id, last_adapter_display_name = adapter_display_name,
    adapter_id = NULL, adapter_app_id = ''
WHERE source_protocol = ? AND bot_id = ? AND adapter_id IS NOT NULL;

-- name: UpsertBotAccount :exec
INSERT INTO bot_accounts (source_protocol, bot_id, nickname, avatar_url, adapter_id,
    adapter_display_name, adapter_app_id, last_adapter_id, last_adapter_display_name, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (source_protocol, bot_id) DO UPDATE SET
    nickname = excluded.nickname, avatar_url = excluded.avatar_url,
    adapter_id = excluded.adapter_id, adapter_display_name = excluded.adapter_display_name,
    adapter_app_id = excluded.adapter_app_id, last_adapter_id = excluded.last_adapter_id,
    last_adapter_display_name = excluded.last_adapter_display_name, last_seen_at = excluded.last_seen_at;

-- name: DeleteBotAccount :exec
DELETE FROM bot_accounts WHERE source_protocol = ? AND bot_id = ?;
