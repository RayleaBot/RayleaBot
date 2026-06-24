package thirdparty

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func (s *Service) ReadCookie(ctx context.Context, account Account) (string, error) {
	key := strings.TrimSpace(account.SecretKey)
	if key == "" {
		return "", secrets.ErrNotFound
	}
	value, err := s.secrets.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return secrets.OpenString(ctx, s.secrets, value)
}

func (s *Service) UpdateCookie(ctx context.Context, account Account, cookie string) error {
	platform, err := normalizePlatform(account.Platform)
	if err != nil {
		return err
	}
	accountID, err := normalizeAccountID(account.AccountID)
	if err != nil {
		return err
	}
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return secrets.ErrNotFound
	}
	secretKey := secretKeyFor(platform, accountID)
	sealed, err := secrets.SealString(ctx, s.secrets, cookie)
	if err != nil {
		return fmt.Errorf("seal third-party account secret: %w", err)
	}
	if err := s.secrets.Set(ctx, secretKey, sealed); err != nil {
		return fmt.Errorf("store third-party account secret: %w", err)
	}
	_, err = s.write.ExecContext(ctx,
		`UPDATE third_party_accounts SET secret_key = ?, updated_at = ? WHERE platform = ? AND account_id = ?`,
		secretKey,
		s.now().UTC().Format(time.RFC3339),
		platform,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("update third-party account secret: %w", err)
	}
	return nil
}
