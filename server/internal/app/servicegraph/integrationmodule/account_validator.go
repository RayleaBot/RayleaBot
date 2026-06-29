package integrationmodule

import (
	"context"
	"net/http"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

type AccountValidator struct {
	bilibili   *bilibilisession.AccountClient
	thirdParty *thirdparty.AccountValidator
}

func newDefaultAccountValidator(transport http.RoundTripper, now func() time.Time) *AccountValidator {
	return NewAccountValidator(transport, now, bilibilisession.NewAccountClient(transport, now, nil))
}

func NewAccountValidator(transport http.RoundTripper, now func() time.Time, bilibiliClient *bilibilisession.AccountClient) *AccountValidator {
	validator := thirdparty.NewAccountValidator(transport, now)
	validator.RegisterPlatform(thirdparty.PlatformWeibo, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return weibo.FetchAccountProfile(ctx, client, cookies)
	})
	validator.RegisterPlatform(thirdparty.PlatformDouyin, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return douyin.FetchAccountProfile(ctx, client, cookies)
	})
	validator.RegisterPlatform(thirdparty.PlatformNeteaseMusic, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return netease_music.FetchAccountProfile(ctx, client, cookies)
	})
	return &AccountValidator{
		bilibili:   bilibiliClient,
		thirdParty: validator,
	}
}

func (v *AccountValidator) CheckCookie(ctx context.Context, platform string, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, err
	}
	if normalized == thirdparty.PlatformBilibili {
		if v == nil || v.bilibili == nil {
			return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
		}
		return v.bilibili.CheckCookie(ctx, cookie)
	}
	if v == nil || v.thirdParty == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
	}
	return v.thirdParty.CheckCookie(ctx, normalized, cookie)
}
