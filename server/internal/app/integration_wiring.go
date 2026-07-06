package app

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
)

type integrationRenderer interface {
	BrowserLaunchConfig() (string, []string)
}

type integrationDeps struct {
	Config        config.Config
	Platform      PlatformState
	Renderer      integrationRenderer
	HTTPTransport http.RoundTripper
	Clock         func() time.Time
}

type integrationState struct {
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
	AccountValidator  *AccountValidator
}

func buildIntegrations(deps integrationDeps) (integrationState, error) {
	thirdPartyService, err := thirdparty.NewService(deps.Platform.Storage, deps.Platform.Secrets)
	if err != nil {
		return integrationState{}, err
	}

	return integrationState{
		ThirdParty:        thirdPartyService,
		ThirdPartyQRLogin: buildQRLoginService(deps, thirdPartyService),
		AccountValidator:  newDefaultAccountValidator(deps.HTTPTransport, deps.Clock),
	}, nil
}

func buildQRLoginService(deps integrationDeps, accountStore *thirdparty.Service) *thirdparty.QRLoginService {
	browserPath, browserArgs := browserLaunchConfig(deps)
	douyinBrowser := douyin.NewChromedpBrowser(browserPath, browserArgs, deps.HTTPTransport)
	return thirdparty.NewQRLoginService(map[string]thirdparty.QRLoginProvider{
		bilibilisession.Platform: bilibilisession.NewProvider(deps.HTTPTransport, deps.Clock),
		weibo.Platform:           weibo.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
		douyin.Platform:          douyin.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport), douyinBrowser),
		netease_music.Platform:   netease_music.NewProvider(thirdparty.NewHTTPClient(deps.HTTPTransport)),
	}, deps.Clock, thirdparty.WithQRLoginAccountStore(accountStore))
}

func browserLaunchConfig(deps integrationDeps) (string, []string) {
	browserPath := deps.Config.Render.BrowserPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		browserPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	return browserPath, browserArgs
}

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
		if v.bilibili == nil {
			return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
		}
		return v.bilibili.CheckCookie(ctx, cookie)
	}
	if v.thirdParty == nil {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, thirdparty.ErrInvalidAccount
	}
	return v.thirdParty.CheckCookie(ctx, normalized, cookie)
}
