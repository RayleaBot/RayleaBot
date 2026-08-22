package accountvalidation

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

type Validator struct {
	bilibili   *bilibilisession.AccountClient
	thirdParty *thirdparty.AccountValidator
}

func NewDefault(transport http.RoundTripper, now func() time.Time) *Validator {
	return New(transport, now, bilibilisession.NewAccountClient(transport, now, nil))
}

func New(transport http.RoundTripper, now func() time.Time, bilibiliClient *bilibilisession.AccountClient) *Validator {
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
	return &Validator{
		bilibili:   bilibiliClient,
		thirdParty: validator,
	}
}

func (v *Validator) CheckCookie(ctx context.Context, platform string, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, err
	}
	if normalized == thirdparty.PlatformBilibili {
		if v.bilibili == nil {
			return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
		}
		profile, status, checkErr := v.bilibili.CheckCookie(ctx, cookie)
		if checkErr == nil {
			return profile, status, nil
		}
		bilibiliErr := bilibilisession.AsError(checkErr)
		explicitAuth := bilibiliErr != nil && bilibiliErr.Kind == bilibilisession.ErrorAuth &&
			(bilibiliErr.HTTPStatus == 0 || bilibiliErr.HTTPStatus == http.StatusUnauthorized || bilibiliErr.Code == -101 || bilibiliErr.Code == -102 || bilibiliErr.Code == -658)
		if explicitAuth {
			status.State = thirdparty.CredentialInvalid
			status.LastError = "Bilibili 账号 CK 已失效，请重新扫码"
		} else {
			status.State = thirdparty.CredentialUnknown
			status.LastError = "Bilibili CK 状态暂时无法确认，请稍后重试"
		}
		return profile, status, checkErr
	}
	if v.thirdParty == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
	}
	return v.thirdParty.CheckCookie(ctx, normalized, cookie)
}
