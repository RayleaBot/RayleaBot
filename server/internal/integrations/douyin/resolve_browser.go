package douyin

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func resolveDouyinUserWithBrowser(ctx context.Context, browser UserResolveBrowser, query string, cookieAttempts []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	if browser == nil {
		return nil, false, nil
	}
	return browser.ResolveUser(ctx, query, cookieAttempts)
}
