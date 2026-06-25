package thirdparty

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Service) List(ctx context.Context) ([]Account, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, last_used_at, proxy_url, proxy_enabled, updated_at FROM third_party_accounts ORDER BY platform ASC, account_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list third-party accounts: %w", err)
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		var account Account
		var enabled int
		var proxyEnabled int
		var credentialCheckedAt sql.NullString
		var lastUsedAt sql.NullString
		var updatedAt string
		if err := rows.Scan(
			&account.Platform,
			&account.AccountID,
			&account.Label,
			&enabled,
			&account.SecretKey,
			&account.Profile.UID,
			&account.Profile.Nickname,
			&account.Profile.AvatarURL,
			&account.Credential.State,
			&credentialCheckedAt,
			&account.Credential.LastError,
			&lastUsedAt,
			&account.ProxyURL,
			&proxyEnabled,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan third-party account: %w", err)
		}
		account.Enabled = enabled != 0
		account.ProxyEnabled = proxyEnabled != 0
		account.Configured = s.secretConfigured(ctx, account.SecretKey)
		account.Credential.State = normalizeCredentialState(account.Credential.State)
		account.Credential.CheckedAt = parseOptionalTime(credentialCheckedAt)
		account.LastUsedAt = parseOptionalTime(lastUsedAt)
		account.UpdatedAt = parseTime(updatedAt)
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate third-party accounts: %w", err)
	}
	return accounts, nil
}

func (s *Service) ListEnabled(ctx context.Context, platform string) ([]Account, error) {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.QueryContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, last_used_at, proxy_url, proxy_enabled, updated_at FROM third_party_accounts WHERE platform = ? AND enabled = 1 AND credential_state != 'invalid' ORDER BY account_id ASC`, platform)
	if err != nil {
		return nil, fmt.Errorf("list enabled third-party accounts: %w", err)
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		var account Account
		var enabled int
		var proxyEnabled int
		var credentialCheckedAt sql.NullString
		var lastUsedAt sql.NullString
		var updatedAt string
		if err := rows.Scan(
			&account.Platform,
			&account.AccountID,
			&account.Label,
			&enabled,
			&account.SecretKey,
			&account.Profile.UID,
			&account.Profile.Nickname,
			&account.Profile.AvatarURL,
			&account.Credential.State,
			&credentialCheckedAt,
			&account.Credential.LastError,
			&lastUsedAt,
			&account.ProxyURL,
			&proxyEnabled,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan third-party account: %w", err)
		}
		account.Enabled = enabled != 0
		account.ProxyEnabled = proxyEnabled != 0
		account.Configured = s.secretConfigured(ctx, account.SecretKey)
		account.Credential.State = normalizeCredentialState(account.Credential.State)
		account.Credential.CheckedAt = parseOptionalTime(credentialCheckedAt)
		account.LastUsedAt = parseOptionalTime(lastUsedAt)
		account.UpdatedAt = parseTime(updatedAt)
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate third-party accounts: %w", err)
	}
	return accounts, nil
}
