package common

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func PersistQRCodeLoginAccount(ctx context.Context, accounts AccountStore, platform, cookie string, profile thirdparty.AccountProfile, now time.Time) (thirdparty.Account, error) {
	if accounts == nil {
		return thirdparty.Account{}, nil
	}
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return thirdparty.Account{}, ErrLoginCredentialMissing
	}
	accountID := qrLoginAccountID(profile)
	label := strings.TrimSpace(profile.Nickname)
	if label == "" {
		label = accountID
	}
	checkedAt := now.UTC()
	return accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  platform,
		AccountID: accountID,
		Label:     label,
		Enabled:   true,
		Cookie:    cookie,
		Profile:   profile,
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialValid,
			CheckedAt: &checkedAt,
		},
	})
}

func qrLoginAccountID(profile thirdparty.AccountProfile) string {
	if accountID, err := thirdparty.NormalizeAccountID(profile.UID); err == nil {
		return accountID
	}
	return "primary"
}
