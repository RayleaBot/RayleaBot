package thirdparty

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func (s *Service) Upsert(ctx context.Context, request UpsertRequest) (Account, error) {
	platform, err := normalizePlatform(request.Platform)
	if err != nil {
		return Account{}, err
	}
	accountID, err := normalizeAccountID(request.AccountID)
	if err != nil {
		return Account{}, err
	}
	label := strings.TrimSpace(request.Label)
	secretKey := secretKeyFor(platform, accountID)
	now := s.now().UTC()
	profile := request.Profile.normalized()
	credential := request.Credential.normalized()
	proxyURL, proxyEnabled, err := s.resolveProxyConfig(ctx, platform, accountID, request.ProxyURL, request.ProxyEnabled)
	if err != nil {
		return Account{}, err
	}

	if strings.TrimSpace(request.Cookie) != "" {
		if request.Validate != nil {
			checkedProfile, checkedCredential, err := request.Validate(ctx, request.Cookie)
			// Only overwrite the profile if the validator returned non-empty data.
			// This preserves the QR-login profile when the validator fails to refetch.
			if !checkedProfile.Empty() {
				profile = checkedProfile.normalized()
			}
			credential = checkedCredential.normalized()
			if err != nil && credential.State == CredentialUnknown {
				checkedAt := now
				credential = CredentialStatus{
					State:     CredentialInvalid,
					CheckedAt: &checkedAt,
					LastError: err.Error(),
				}
			}
		} else if credential.State == "" || credential.State == CredentialUnknown {
			checkedAt := now
			credential = CredentialStatus{State: CredentialUnknown, CheckedAt: &checkedAt}
		}
		sealed, err := secrets.SealString(ctx, s.secrets, request.Cookie)
		if err != nil {
			return Account{}, fmt.Errorf("seal third-party account secret: %w", err)
		}
		if err := s.secrets.Set(ctx, secretKey, sealed); err != nil {
			return Account{}, fmt.Errorf("store third-party account secret: %w", err)
		}
	}

	if _, err := s.write.ExecContext(ctx,
		`INSERT INTO third_party_accounts (platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, proxy_url, proxy_enabled, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(platform, account_id) DO UPDATE SET
		   label = excluded.label,
		   enabled = excluded.enabled,
		   secret_key = excluded.secret_key,
		   profile_uid = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_uid ELSE excluded.profile_uid END,
		   profile_nickname = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_nickname ELSE excluded.profile_nickname END,
		   profile_avatar_url = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_avatar_url ELSE excluded.profile_avatar_url END,
		   credential_state = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_state ELSE excluded.credential_state END,
		   credential_checked_at = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_checked_at ELSE excluded.credential_checked_at END,
		   credential_last_error = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_last_error ELSE excluded.credential_last_error END,
		   proxy_url = excluded.proxy_url,
		   proxy_enabled = excluded.proxy_enabled,
		   updated_at = excluded.updated_at`,
		platform,
		accountID,
		label,
		boolInt(request.Enabled),
		secretKey,
		profile.UID,
		profile.Nickname,
		profile.AvatarURL,
		credential.State,
		nullableTime(credential.CheckedAt),
		credential.LastError,
		proxyURL,
		boolInt(proxyEnabled),
		now.Format(time.RFC3339),
	); err != nil {
		return Account{}, fmt.Errorf("upsert third-party account: %w", err)
	}
	accounts, err := s.List(ctx)
	if err != nil {
		return Account{}, err
	}
	for _, account := range accounts {
		if account.Platform == platform && account.AccountID == accountID {
			return account, nil
		}
	}
	return Account{}, fmt.Errorf("read saved third-party account: %w", sql.ErrNoRows)
}

func (s *Service) resolveProxyConfig(ctx context.Context, platform, accountID string, requestURL *string, requestEnabled *bool) (string, bool, error) {
	proxyURL := ""
	proxyEnabled := false
	if requestURL == nil || requestEnabled == nil {
		var storedEnabled int
		err := s.read.QueryRowContext(ctx, `SELECT proxy_url, proxy_enabled FROM third_party_accounts WHERE platform = ? AND account_id = ?`, platform, accountID).Scan(&proxyURL, &storedEnabled)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("read third-party account proxy config: %w", err)
		}
		proxyEnabled = storedEnabled != 0
	}
	if requestURL != nil {
		normalized, err := normalizeProxyURL(*requestURL)
		if err != nil {
			return "", false, err
		}
		proxyURL = normalized
	}
	if requestEnabled != nil {
		proxyEnabled = *requestEnabled
	}
	if err := validateProxyConfig(proxyURL, proxyEnabled); err != nil {
		return "", false, err
	}
	return proxyURL, proxyEnabled, nil
}

func (s *Service) Delete(ctx context.Context, platform, accountID string) error {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return err
	}
	accountID, err = normalizeAccountID(accountID)
	if err != nil {
		return err
	}
	secretKey := secretKeyFor(platform, accountID)
	if _, err := s.write.ExecContext(ctx, `DELETE FROM third_party_accounts WHERE platform = ? AND account_id = ?`, platform, accountID); err != nil {
		return fmt.Errorf("delete third-party account: %w", err)
	}
	if err := s.secrets.Delete(ctx, secretKey); err != nil {
		return fmt.Errorf("delete third-party account secret: %w", err)
	}
	return nil
}

func (s *Service) MarkUsed(ctx context.Context, account Account) error {
	if account.Platform == "" || account.AccountID == "" {
		return nil
	}
	platform, err := normalizePlatform(account.Platform)
	if err != nil {
		return err
	}
	accountID, err := normalizeAccountID(account.AccountID)
	if err != nil {
		return err
	}
	_, err = s.write.ExecContext(ctx,
		`UPDATE third_party_accounts SET last_used_at = ? WHERE platform = ? AND account_id = ?`,
		s.now().UTC().Format(time.RFC3339), platform, accountID,
	)
	if err != nil {
		return fmt.Errorf("mark third-party account used: %w", err)
	}
	return nil
}

func (s *Service) UpdateCredentialStatus(ctx context.Context, platform, accountID string, profile AccountProfile, credential CredentialStatus) error {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return err
	}
	accountID, err = normalizeAccountID(accountID)
	if err != nil {
		return err
	}
	profile = profile.normalized()
	credential = credential.normalized()
	_, err = s.write.ExecContext(ctx,
		`UPDATE third_party_accounts
		 SET profile_uid = ?, profile_nickname = ?, profile_avatar_url = ?,
		     credential_state = ?, credential_checked_at = ?, credential_last_error = ?
		 WHERE platform = ? AND account_id = ?`,
		profile.UID,
		profile.Nickname,
		profile.AvatarURL,
		credential.State,
		nullableTime(credential.CheckedAt),
		credential.LastError,
		platform,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("update third-party credential status: %w", err)
	}
	return nil
}
